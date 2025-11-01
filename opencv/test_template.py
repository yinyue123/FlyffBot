import cv2
import numpy as np
import time
import os

class TemplateMatchDebugger:
    def __init__(self):
        self.original_image = None
        self.template = None
        self.mouse_pos = None
        self.display_image = None

        # 匹配方法字典
        self.match_methods = {
            0: ('TM_CCOEFF_NORMED', cv2.TM_CCOEFF_NORMED, True),    # 值越大越好
            1: ('TM_CCORR_NORMED', cv2.TM_CCORR_NORMED, True),      # 值越大越好
            2: ('TM_SQDIFF_NORMED', cv2.TM_SQDIFF_NORMED, False),   # 值越小越好
            3: ('TM_CCOEFF', cv2.TM_CCOEFF, True),                  # 值越大越好
            4: ('TM_CCORR', cv2.TM_CCORR, True),                    # 值越大越好
            5: ('TM_SQDIFF', cv2.TM_SQDIFF, False)                  # 值越小越好
        }

        # 结果处理方法
        self.result_methods = {
            0: 'NMS (非极大值抑制)',
            1: '取最佳匹配',
            2: '取前N个',
            3: '显示所有匹配'
        }

    def mouse_callback(self, event, x, y, flags, param):
        """鼠标回调：记录鼠标位置"""
        if event == cv2.EVENT_MOUSEMOVE:
            self.mouse_pos = (x, y)

    def run(self):
        """运行调试工具"""
        print("=" * 60)
        print("模板匹配调试工具")
        print("=" * 60)

        # 获取模板图片路径
        template_path = input("\n请输入模板图片路径: ").strip().strip("'\"")
        self.template = cv2.imread(template_path)

        if self.template is None:
            print("❌ 无法读取模板图片！")
            return

        print(f"✓ 模板大小: {self.template.shape[1]}x{self.template.shape[0]}")

        # 获取待检测图片路径
        image_path = input("请输入待检测图片路径: ").strip().strip("'\"")
        self.original_image = cv2.imread(image_path)

        if self.original_image is None:
            print("❌ 无法读取待检测图片！")
            return

        print(f"✓ 图片大小: {self.original_image.shape[1]}x{self.original_image.shape[0]}")

        # 初始化ROI为全图
        h, w = self.original_image.shape[:2]
        self.roi_rect = (0, 0, w, h)

        # 开始调试
        self.debug_with_trackbars()

    def nms_boxes(self, boxes, scores, threshold=0.3):
        """非极大值抑制"""
        if len(boxes) == 0:
            return []

        # 转换为numpy数组
        boxes_array = np.array(boxes, dtype=np.float32)
        scores_array = np.array(scores, dtype=np.float32)

        x1 = boxes_array[:, 0]
        y1 = boxes_array[:, 1]
        x2 = boxes_array[:, 2]
        y2 = boxes_array[:, 3]

        areas = (x2 - x1 + 1) * (y2 - y1 + 1)
        order = scores_array.argsort()[::-1]

        keep = []
        while order.size > 0:
            i = order[0]
            keep.append(i)

            xx1 = np.maximum(x1[i], x1[order[1:]])
            yy1 = np.maximum(y1[i], y1[order[1:]])
            xx2 = np.minimum(x2[i], x2[order[1:]])
            yy2 = np.minimum(y2[i], y2[order[1:]])

            w = np.maximum(0.0, xx2 - xx1 + 1)
            h = np.maximum(0.0, yy2 - yy1 + 1)

            overlap = (w * h) / areas[order[1:]]

            order = order[np.where(overlap <= threshold)[0] + 1]

        return keep

    def match_template_multiscale(self, image, template, method,
                                   scale_min, scale_max, scale_steps,
                                   threshold, result_method, top_n, nms_threshold,
                                   resize_factor, timeout=1.0):
        """
        多尺度模板匹配

        参数:
            image: 待检测图像
            template: 模板图像
            method: 匹配方法
            scale_min: 最小缩放比例
            scale_max: 最大缩放比例
            scale_steps: 缩放步数
            threshold: 匹配阈值
            result_method: 结果处理方法 (0=NMS, 1=最佳, 2=前N个, 3=全部)
            top_n: 取前N个
            nms_threshold: NMS阈值
            resize_factor: 图像降低分辨率比例
            timeout: 超时时间（秒），默认1.0秒
        """
        start_time = time.time()
        method_name, method_cv, is_higher_better = self.match_methods[method]

        # 降低分辨率
        if resize_factor < 1.0:
            new_w = int(image.shape[1] * resize_factor)
            new_h = int(image.shape[0] * resize_factor)
            image = cv2.resize(image, (new_w, new_h), interpolation=cv2.INTER_AREA)
            template = cv2.resize(template,
                                 (int(template.shape[1] * resize_factor),
                                  int(template.shape[0] * resize_factor)),
                                 interpolation=cv2.INTER_AREA)

        # 转换为灰度图（提高性能）
        image_gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY) if len(image.shape) == 3 else image
        template_gray = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY) if len(template.shape) == 3 else template

        h, w = template_gray.shape
        all_matches = []

        # 生成缩放比例
        if scale_steps <= 1:
            scales = [1.0]
        else:
            scales = np.linspace(scale_min, scale_max, scale_steps)

        # 多尺度匹配
        is_timeout = False
        for scale in scales:
            # 检查是否超时
            if time.time() - start_time > timeout:
                is_timeout = True
                print(f"\n⚠️ 处理超时({timeout}s)，已跳过剩余尺度")
                break

            # 缩放模板
            scaled_w = int(w * scale)
            scaled_h = int(h * scale)

            # 检查缩放后的模板是否有效
            if scaled_w < 3 or scaled_h < 3:
                continue
            if scaled_w > image_gray.shape[1] or scaled_h > image_gray.shape[0]:
                continue

            resized_template = cv2.resize(template_gray, (scaled_w, scaled_h),
                                         interpolation=cv2.INTER_AREA if scale < 1.0 else cv2.INTER_CUBIC)

            # 模板匹配
            try:
                result = cv2.matchTemplate(image_gray, resized_template, method_cv)
            except cv2.error as e:
                print(f"匹配失败，scale={scale:.2f}: {e}")
                continue

            # 再次检查超时（匹配操作后）
            if time.time() - start_time > timeout:
                is_timeout = True
                print(f"\n⚠️ 处理超时({timeout}s)，已跳过剩余尺度")
                break

            # 根据匹配方法确定阈值比较方式
            if is_higher_better:
                locations = np.where(result >= threshold)
            else:
                locations = np.where(result <= threshold)

            # 收集所有匹配
            for pt in zip(*locations[::-1]):
                x, y = pt
                score = result[y, x]

                # 还原到原始图像坐标（如果降低了分辨率）
                if resize_factor < 1.0:
                    x = int(x / resize_factor)
                    y = int(y / resize_factor)
                    scaled_w = int(scaled_w / resize_factor)
                    scaled_h = int(scaled_h / resize_factor)

                all_matches.append({
                    'x': x,
                    'y': y,
                    'w': scaled_w,
                    'h': scaled_h,
                    'score': float(score),
                    'scale': scale,
                    'box': [x, y, x + scaled_w, y + scaled_h]
                })

        # 根据result_method处理结果
        if result_method == 0:  # NMS
            if len(all_matches) == 0:
                return [], is_timeout

            boxes = [m['box'] for m in all_matches]
            scores = [m['score'] if is_higher_better else -m['score'] for m in all_matches]

            keep_indices = self.nms_boxes(boxes, scores, nms_threshold)
            filtered_matches = [all_matches[i] for i in keep_indices]

        elif result_method == 1:  # 取最佳匹配
            if len(all_matches) == 0:
                return [], is_timeout

            if is_higher_better:
                best_match = max(all_matches, key=lambda x: x['score'])
            else:
                best_match = min(all_matches, key=lambda x: x['score'])

            filtered_matches = [best_match]

        elif result_method == 2:  # 取前N个
            if is_higher_better:
                sorted_matches = sorted(all_matches, key=lambda x: x['score'], reverse=True)
            else:
                sorted_matches = sorted(all_matches, key=lambda x: x['score'])

            filtered_matches = sorted_matches[:top_n]

        else:  # 显示所有
            filtered_matches = all_matches

        return filtered_matches, is_timeout

    def debug_with_trackbars(self):
        """带滑块的调试界面"""
        print("\n" + "=" * 60)
        print("操作说明:")
        print("- 调整滑块找到最佳参数")
        print("\n匹配方法 (Method 0-5):")
        print("  0: TM_CCOEFF_NORMED (推荐)")
        print("  1: TM_CCORR_NORMED")
        print("  2: TM_SQDIFF_NORMED (值越小越好)")
        print("  3: TM_CCOEFF")
        print("  4: TM_CCORR")
        print("  5: TM_SQDIFF (值越小越好)")
        print("\n结果处理 (Result_Method 0-3):")
        print("  0: NMS 非极大值抑制 (推荐)")
        print("  1: 取最佳匹配")
        print("  2: 取前N个")
        print("  3: 显示全部")
        print("\n快捷键:")
        print("- 按 'q' 退出并显示最终参数")
        print("- 按 's' 保存当前图像")
        print("=" * 60)

        # 创建窗口
        trackbar_window = 'Trackbars'
        display_window = 'Display'

        cv2.namedWindow(trackbar_window)
        cv2.namedWindow(display_window)
        cv2.setMouseCallback(display_window, self.mouse_callback)

        # 创建滑块
        # 匹配方法 (0-5)
        cv2.createTrackbar('Method', trackbar_window, 0, 5, lambda x: None)

        # 阈值 (0-100, 实际值除以100)
        cv2.createTrackbar('Threshold%', trackbar_window, 80, 100, lambda x: None)

        # 多尺度参数
        cv2.createTrackbar('Scale_Min%', trackbar_window, 80, 200, lambda x: None)   # 0.8-2.0
        cv2.createTrackbar('Scale_Max%', trackbar_window, 120, 200, lambda x: None)  # 0.8-2.0
        cv2.createTrackbar('Scale_Steps', trackbar_window, 5, 30, lambda x: None)

        # 结果处理方法 (0-3)
        cv2.createTrackbar('Result_Method', trackbar_window, 0, 3, lambda x: None)

        # 前N个（当Result_Method=2时使用）
        cv2.createTrackbar('Top_N', trackbar_window, 5, 50, lambda x: None)

        # NMS阈值 (0-100, 实际值除以100)
        cv2.createTrackbar('NMS_Thresh%', trackbar_window, 30, 100, lambda x: None)

        # 降低分辨率 (10-100%)
        cv2.createTrackbar('Resize%', trackbar_window, 100, 100, lambda x: None)

        # 超时时间 (100-5000ms, 实际值除以1000)
        cv2.createTrackbar('Timeout_ms', trackbar_window, 1000, 5000, lambda x: None)

        # ROI区域
        h, w = self.original_image.shape[:2]
        cv2.createTrackbar('ROI_X', trackbar_window, 0, w, lambda x: None)
        cv2.createTrackbar('ROI_Y', trackbar_window, 0, h, lambda x: None)
        cv2.createTrackbar('ROI_W', trackbar_window, w, w, lambda x: None)
        cv2.createTrackbar('ROI_H', trackbar_window, h, h, lambda x: None)

        # 显示缩放
        cv2.createTrackbar('Display%', trackbar_window, 100, 400, lambda x: None)

        # 视图模式 (0=结果, 1=原图+ROI, 2=热力图)
        cv2.createTrackbar('View', trackbar_window, 0, 2, lambda x: None)

        last_values = None
        combined = None
        process_time = 0
        last_method_idx = -1
        last_result_method = -1
        last_view_mode = -1

        while True:
            # 获取所有滑块值
            method_idx = cv2.getTrackbarPos('Method', trackbar_window)

            # 如果方法改变，更新窗口标题
            if method_idx != last_method_idx:
                method_name = self.match_methods[method_idx][0]
                cv2.setWindowTitle(trackbar_window, f'Trackbars - {method_name}')
                last_method_idx = method_idx

            threshold_percent = cv2.getTrackbarPos('Threshold%', trackbar_window)
            threshold = threshold_percent / 100.0

            scale_min = cv2.getTrackbarPos('Scale_Min%', trackbar_window) / 100.0
            scale_max = cv2.getTrackbarPos('Scale_Max%', trackbar_window) / 100.0
            scale_steps = max(1, cv2.getTrackbarPos('Scale_Steps', trackbar_window))

            result_method = cv2.getTrackbarPos('Result_Method', trackbar_window)
            top_n = max(1, cv2.getTrackbarPos('Top_N', trackbar_window))
            nms_threshold = cv2.getTrackbarPos('NMS_Thresh%', trackbar_window) / 100.0

            resize_percent = max(10, cv2.getTrackbarPos('Resize%', trackbar_window))
            resize_factor = resize_percent / 100.0

            timeout_ms = max(100, cv2.getTrackbarPos('Timeout_ms', trackbar_window))
            timeout_sec = timeout_ms / 1000.0

            roi_x = cv2.getTrackbarPos('ROI_X', trackbar_window)
            roi_y = cv2.getTrackbarPos('ROI_Y', trackbar_window)
            roi_w = cv2.getTrackbarPos('ROI_W', trackbar_window)
            roi_h = cv2.getTrackbarPos('ROI_H', trackbar_window)

            display_percent = max(10, cv2.getTrackbarPos('Display%', trackbar_window))
            view_mode = cv2.getTrackbarPos('View', trackbar_window)

            # 更新显示窗口标题
            if result_method != last_result_method or view_mode != last_view_mode:
                view_names = ['Result', 'Original+ROI', 'Heatmap']
                cv2.setWindowTitle(display_window,
                    f'Display - {view_names[view_mode]} | {self.result_methods[result_method]}')
                last_result_method = result_method
                last_view_mode = view_mode

            # 确保scale_min <= scale_max
            if scale_min > scale_max:
                scale_min, scale_max = scale_max, scale_min

            # 检查参数是否变化
            current_values = (method_idx, threshold, scale_min, scale_max, scale_steps,
                            result_method, top_n, nms_threshold, resize_factor, timeout_sec,
                            roi_x, roi_y, roi_w, roi_h, view_mode)

            if current_values != last_values:
                start_time = time.time()
                last_values = current_values

                # 确保ROI有效
                img_h, img_w = self.original_image.shape[:2]
                roi_x = max(0, min(roi_x, img_w - 10))
                roi_y = max(0, min(roi_y, img_h - 10))
                roi_w = max(10, min(roi_w, img_w - roi_x))
                roi_h = max(10, min(roi_h, img_h - roi_y))

                self.roi_rect = (roi_x, roi_y, roi_w, roi_h)

                # 提取ROI
                image = self.original_image[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()

                if image.size == 0:
                    continue

                # 执行模板匹配
                matches, is_timeout = self.match_template_multiscale(
                    image, self.template, method_idx,
                    scale_min, scale_max, scale_steps,
                    threshold, result_method, top_n, nms_threshold,
                    resize_factor, timeout_sec
                )

                # 绘制结果
                result_img = image.copy()

                # 获取匹配方法名称
                method_name, _, is_higher_better = self.match_methods[method_idx]

                # 绘制所有匹配
                for i, match in enumerate(matches, 1):
                    x, y, w, h = match['x'], match['y'], match['w'], match['h']
                    score = match['score']
                    scale = match['scale']

                    # 绘制矩形
                    color = (0, 255, 0) if i == 1 else (0, 255, 255)
                    thickness = 3 if i == 1 else 2
                    cv2.rectangle(result_img, (x, y), (x+w, y+h), color, thickness)

                    # 绘制角标记
                    corner_len = min(w, h) // 5
                    cv2.line(result_img, (x, y), (x + corner_len, y), color, thickness)
                    cv2.line(result_img, (x, y), (x, y + corner_len), color, thickness)
                    cv2.line(result_img, (x+w, y), (x+w - corner_len, y), color, thickness)
                    cv2.line(result_img, (x+w, y), (x+w, y + corner_len), color, thickness)
                    cv2.line(result_img, (x, y+h), (x + corner_len, y+h), color, thickness)
                    cv2.line(result_img, (x, y+h), (x, y+h - corner_len), color, thickness)
                    cv2.line(result_img, (x+w, y+h), (x+w - corner_len, y+h), color, thickness)
                    cv2.line(result_img, (x+w, y+h), (x+w, y+h - corner_len), color, thickness)

                    # 绘制中心点
                    center_x = x + w // 2
                    center_y = y + h // 2
                    cv2.circle(result_img, (center_x, center_y), 8, color, -1)
                    cv2.circle(result_img, (center_x, center_y), 11, (0, 0, 255), 2)

                    # 显示匹配信息
                    info = f"#{i} {score:.3f} s:{scale:.2f}"
                    (text_w, text_h), _ = cv2.getTextSize(info, cv2.FONT_HERSHEY_SIMPLEX, 0.8, 2)

                    # 绘制背景
                    cv2.rectangle(result_img, (x, y-text_h-10), (x+text_w+8, y-2), (0, 0, 0), -1)
                    cv2.putText(result_img, info, (x+4, y-6),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.8, color, 2)

                # 显示参数信息
                info_lines = [
                    f"Method: {method_name}",
                    f"Threshold: {threshold:.2f} ({'>' if is_higher_better else '<'})",
                    f"Scale: {scale_min:.2f}-{scale_max:.2f} ({scale_steps} steps)",
                    f"Result: {self.result_methods[result_method]}",
                    f"Resize: {resize_percent}% | Timeout: {timeout_ms}ms",
                    f"Found: {len(matches)} matches"
                ]

                # 如果超时，添加警告
                if is_timeout:
                    info_lines.append("WARNING: TIMEOUT!")

                y_pos = 30
                for i, line in enumerate(info_lines):
                    # 超时警告用红色显示
                    color = (0, 0, 255) if "TIMEOUT" in line else (0, 255, 0)
                    cv2.putText(result_img, line, (10, y_pos),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.8, color, 2)
                    y_pos += 30

                # 根据view_mode选择显示
                if view_mode == 0:  # 结果
                    combined = result_img
                elif view_mode == 1:  # 原图+ROI
                    display_original = self.original_image.copy()
                    cv2.rectangle(display_original, (roi_x, roi_y),
                                (roi_x+roi_w, roi_y+roi_h), (0, 255, 0), 2)
                    cv2.putText(display_original, f"ROI: {roi_w}x{roi_h}",
                               (roi_x, roi_y-10),
                               cv2.FONT_HERSHEY_SIMPLEX, 1, (0, 255, 0), 2)
                    combined = display_original
                else:  # 热力图 (view_mode == 2)
                    # 生成热力图（只用单尺度）
                    image_gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
                    template_gray = cv2.cvtColor(self.template, cv2.COLOR_BGR2GRAY)

                    method_cv = self.match_methods[method_idx][1]
                    try:
                        result_map = cv2.matchTemplate(image_gray, template_gray, method_cv)

                        # 归一化到0-255
                        result_normalized = cv2.normalize(result_map, None, 0, 255, cv2.NORM_MINMAX)
                        result_normalized = result_normalized.astype(np.uint8)

                        # 应用颜色映射
                        heatmap = cv2.applyColorMap(result_normalized, cv2.COLORMAP_JET)

                        # 调整大小到原图大小
                        heatmap = cv2.resize(heatmap, (image.shape[1], image.shape[0]))

                        # 叠加到原图
                        combined = cv2.addWeighted(image, 0.5, heatmap, 0.5, 0)
                    except:
                        combined = result_img

                # 计算处理时间
                process_time = (time.time() - start_time) * 1000

            # 显示图像
            if combined is not None:
                display_img = combined.copy()

                # 显示处理时间
                view_names = ['Result', 'Original+ROI', 'Heatmap']
                time_text = f"Process: {process_time:.1f}ms | View: {view_names[view_mode]}"
                cv2.putText(display_img, time_text, (10, display_img.shape[0] - 15),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 0), 2)

                # 显示鼠标位置信息
                if self.mouse_pos is not None:
                    mx, my = self.mouse_pos
                    if 0 <= mx < display_img.shape[1] and 0 <= my < display_img.shape[0]:
                        pixel = display_img[my, mx]

                        # 显示坐标和像素值
                        info_text = f"Pos: ({mx}, {my})"
                        cv2.rectangle(display_img, (mx + 15, my - 50), (mx + 250, my - 5), (0, 0, 0), -1)
                        cv2.rectangle(display_img, (mx + 15, my - 50), (mx + 250, my - 5), (0, 255, 0), 2)
                        cv2.putText(display_img, info_text, (mx + 20, my - 25),
                                   cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 0), 2)

                        # 绘制十字光标
                        cv2.line(display_img, (mx - 10, my), (mx + 10, my), (0, 255, 0), 2)
                        cv2.line(display_img, (mx, my - 10), (mx, my + 10), (0, 255, 0), 2)

                # 应用显示缩放
                if display_percent != 100:
                    scale_factor = display_percent / 100.0
                    new_w = int(display_img.shape[1] * scale_factor)
                    new_h = int(display_img.shape[0] * scale_factor)

                    if scale_factor > 1.0:
                        display_img = cv2.resize(display_img, (new_w, new_h),
                                               interpolation=cv2.INTER_LANCZOS4)
                    else:
                        display_img = cv2.resize(display_img, (new_w, new_h),
                                               interpolation=cv2.INTER_AREA)

                cv2.imshow(display_window, display_img)

            # 键盘控制
            key = cv2.waitKey(10) & 0xFF

            if key == ord('q'):
                print("\n" + "=" * 60)
                print("最终参数:")
                print("=" * 60)
                method_name = self.match_methods[method_idx][0]
                is_higher = self.match_methods[method_idx][2]

                print(f"\n# 匹配方法: {method_name}")
                print(f"method = cv2.{method_name}")

                print(f"\n# 匹配阈值 ({'值越大越好' if is_higher else '值越小越好'})")
                print(f"threshold = {threshold:.2f}")

                print(f"\n# 多尺度参数")
                print(f"scale_min = {scale_min:.2f}")
                print(f"scale_max = {scale_max:.2f}")
                print(f"scale_steps = {scale_steps}")
                print(f"scales = np.linspace({scale_min}, {scale_max}, {scale_steps})")

                print(f"\n# 结果处理: {self.result_methods[result_method]}")
                if result_method == 0:
                    print(f"# 使用 NMS 非极大值抑制")
                    print(f"nms_threshold = {nms_threshold:.2f}")
                elif result_method == 1:
                    print(f"# 只返回最佳匹配")
                elif result_method == 2:
                    print(f"# 返回前N个最佳匹配")
                    print(f"top_n = {top_n}")
                elif result_method == 3:
                    print(f"# 返回所有匹配（超过阈值的）")

                print(f"\n# 性能优化")
                print(f"resize_factor = {resize_factor:.2f}  # 图像缩放比例")
                print(f"timeout = {timeout_sec:.2f}  # 秒，超时会跳过剩余尺度")

                print(f"\n# ROI搜索区域 (x, y, width, height)")
                print(f"roi_region = ({roi_x}, {roi_y}, {roi_w}, {roi_h})")

                print(f"\n# 完整示例代码")
                print(f"# result = cv2.matchTemplate(image_gray, template_gray, cv2.{method_name})")
                print(f"# locations = np.where(result {'>' if is_higher else '<'}= {threshold:.2f})")

                print("=" * 60)
                break

            elif key == ord('s'):
                if combined is not None:
                    filename = f"template_match_{int(time.time())}.png"
                    cv2.imwrite(filename, combined)
                    print(f"\n✓ 已保存图像: {filename}")

        cv2.destroyAllWindows()


if __name__ == "__main__":
    debugger = TemplateMatchDebugger()
    debugger.run()
