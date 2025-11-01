import cv2
import numpy as np
import time

class ColorDebugger:
    def __init__(self):
        self.original_image = None
        self.mouse_pos = None
        self.display_image = None

    def mouse_callback(self, event, x, y, flags, param):
        """鼠标回调：记录鼠标位置"""
        if event == cv2.EVENT_MOUSEMOVE:
            self.mouse_pos = (x, y)

    def run(self):
        """运行调试工具"""
        print("=" * 60)
        print("颜色识别调试工具")
        print("=" * 60)

        # 获取图片路径并去除单引号
        image_path = input("\n请输入图片路径（可直接拖拽图片到终端）: ").strip().strip("'\"")

        self.original_image = cv2.imread(image_path)
        if self.original_image is None:
            print("❌ 无法读取图片！")
            return

        print(f"\n✓ 图片大小: {self.original_image.shape[1]}x{self.original_image.shape[0]}")

        # 初始化ROI为左上角300x300
        h, w = self.original_image.shape[:2]
        roi_w = min(300, w)
        roi_h = min(300, h)
        self.roi_rect = (0, 0, roi_w, roi_h)

        # 开始调试
        self.debug_with_trackbars()

    def debug_with_trackbars(self):
        """带滑块的调试界面"""
        print("\n" + "=" * 60)
        print("操作说明:")
        print("- 调整滑块找到最佳参数")
        print("- 调整 'View' 滑块切换视图: 0=Original, 1=Mask, 2=Result, 3=Contours")
        print("- 鼠标移动到图像上可查看HSV值")
        print("- 按 'q' 退出并显示最终参数")
        print("- 按 's' 保存当前图像")
        print("=" * 60)

        # 创建两个窗口：滑块窗口和图像显示窗口
        trackbar_window = 'Trackbars'
        display_window = 'Display'

        # 创建滑块窗口
        cv2.namedWindow(trackbar_window)

        # 创建图像显示窗口
        cv2.namedWindow(display_window)
        cv2.setMouseCallback(display_window, self.mouse_callback)

        # 创建所有滑块（绑定到滑块窗口）
        # HSV 滑块
        cv2.createTrackbar('H_min', trackbar_window, 0, 180, lambda x: None)
        cv2.createTrackbar('H_max', trackbar_window, 10, 180, lambda x: None)
        cv2.createTrackbar('S_min', trackbar_window, 100, 255, lambda x: None)
        cv2.createTrackbar('S_max', trackbar_window, 255, 255, lambda x: None)
        cv2.createTrackbar('V_min', trackbar_window, 100, 255, lambda x: None)
        cv2.createTrackbar('V_max', trackbar_window, 255, 255, lambda x: None)

        # 形态学操作滑块
        cv2.createTrackbar('Close_Size', trackbar_window, 7, 30, lambda x: None)
        cv2.createTrackbar('Close_Iter', trackbar_window, 2, 10, lambda x: None)
        cv2.createTrackbar('Open_Size', trackbar_window, 5, 30, lambda x: None)

        # 轮廓过滤滑块
        cv2.createTrackbar('Min_Area', trackbar_window, 500, 10000, lambda x: None)
        cv2.createTrackbar('Min_Width', trackbar_window, 50, 500, lambda x: None)
        cv2.createTrackbar('Max_Width', trackbar_window, 500, 2000, lambda x: None)
        cv2.createTrackbar('Min_Height', trackbar_window, 10, 200, lambda x: None)
        cv2.createTrackbar('Max_Height', trackbar_window, 200, 2000, lambda x: None)
        cv2.createTrackbar('Min_Aspect', trackbar_window, 20, 100, lambda x: None)

        # ROI 坐标滑块
        x, y, w, h = self.roi_rect
        cv2.createTrackbar('ROI_X', trackbar_window, x, self.original_image.shape[1], lambda x: None)
        cv2.createTrackbar('ROI_Y', trackbar_window, y, self.original_image.shape[0], lambda x: None)
        cv2.createTrackbar('ROI_W', trackbar_window, w, self.original_image.shape[1], lambda x: None)
        cv2.createTrackbar('ROI_H', trackbar_window, h, self.original_image.shape[0], lambda x: None)

        # 显示缩放滑块（10%-400%）
        cv2.createTrackbar('Scale%', trackbar_window, 100, 400, lambda x: None)

        # 视图切换滑块（0=Original, 1=Mask, 2=Result, 3=Contours）
        cv2.createTrackbar('View', trackbar_window, 0, 3, lambda x: None)

        # 视图模式名称
        view_names = ["Original", "Mask", "Result", "Contours"]

        # 记录上一次的滑块值
        last_values = None
        combined = None
        process_time = 0

        while True:
            # 获取所有滑块值
            h_min = cv2.getTrackbarPos('H_min', trackbar_window)
            h_max = cv2.getTrackbarPos('H_max', trackbar_window)
            s_min = cv2.getTrackbarPos('S_min', trackbar_window)
            s_max = cv2.getTrackbarPos('S_max', trackbar_window)
            v_min = cv2.getTrackbarPos('V_min', trackbar_window)
            v_max = cv2.getTrackbarPos('V_max', trackbar_window)

            close_size = cv2.getTrackbarPos('Close_Size', trackbar_window)
            close_iter = cv2.getTrackbarPos('Close_Iter', trackbar_window)
            open_size = cv2.getTrackbarPos('Open_Size', trackbar_window)

            min_area = cv2.getTrackbarPos('Min_Area', trackbar_window)
            min_width = cv2.getTrackbarPos('Min_Width', trackbar_window)
            max_width = cv2.getTrackbarPos('Max_Width', trackbar_window)
            min_height = cv2.getTrackbarPos('Min_Height', trackbar_window)
            max_height = cv2.getTrackbarPos('Max_Height', trackbar_window)
            min_aspect = cv2.getTrackbarPos('Min_Aspect', trackbar_window) / 10.0

            # 获取ROI区域
            roi_x = cv2.getTrackbarPos('ROI_X', trackbar_window)
            roi_y = cv2.getTrackbarPos('ROI_Y', trackbar_window)
            roi_w = cv2.getTrackbarPos('ROI_W', trackbar_window)
            roi_h = cv2.getTrackbarPos('ROI_H', trackbar_window)

            # 获取缩放比例
            scale_percent = max(10, cv2.getTrackbarPos('Scale%', trackbar_window))

            # 获取视图模式（从滑块）
            current_view = cv2.getTrackbarPos('View', trackbar_window)

            # 检查滑块是否变化
            current_values = (h_min, h_max, s_min, s_max, v_min, v_max,
                            close_size, close_iter, open_size,
                            min_area, min_width, max_width, min_height, max_height, min_aspect,
                            roi_x, roi_y, roi_w, roi_h, scale_percent, current_view)

            # 只有滑块变化时才重新处理
            if current_values != last_values:
                start_time = time.time()
                last_values = current_values

                # 确保ROI在图像范围内
                img_h, img_w = self.original_image.shape[:2]
                roi_x = max(0, min(roi_x, img_w - 10))
                roi_y = max(0, min(roi_y, img_h - 10))
                roi_w = max(10, min(roi_w, img_w - roi_x))
                roi_h = max(10, min(roi_h, img_h - roi_y))

                self.roi_rect = (roi_x, roi_y, roi_w, roi_h)

                # 提取ROI
                x, y, w, h = self.roi_rect
                image = self.original_image[y:y+h, x:x+w].copy()

                # 确保提取的图像不为空
                if image.size == 0:
                    continue

                # 显示原图（带ROI框）
                display_original = self.original_image.copy()
                cv2.rectangle(display_original, (x, y), (x+w, y+h), (0, 255, 0), 3)
                cv2.putText(display_original, f"ROI: {w}x{h}", (x, y-15),
                           cv2.FONT_HERSHEY_SIMPLEX, 1.5, (0, 255, 0), 3)

                # 转换为HSV并创建掩码
                hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
                lower = np.array([h_min, s_min, v_min])
                upper = np.array([h_max, s_max, v_max])
                mask = cv2.inRange(hsv, lower, upper)

                # 形态学操作
                if close_size > 0:
                    close_size = close_size if close_size % 2 == 1 else close_size + 1
                    kernel_close = np.ones((close_size, close_size), np.uint8)
                    mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel_close, iterations=close_iter)

                if open_size > 0:
                    open_size = open_size if open_size % 2 == 1 else open_size + 1
                    kernel_open = np.ones((open_size, open_size), np.uint8)
                    mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel_open)

                # 查找轮廓
                contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

                # 生成结果图像
                result = cv2.bitwise_and(image, image, mask=mask)
                contour_img = image.copy()

                valid_contours = 0
                for contour in contours:
                    area = cv2.contourArea(contour)
                    cx, cy, cw, ch = cv2.boundingRect(contour)
                    aspect_ratio = cw / ch if ch > 0 else 0

                    # 应用过滤条件
                    if (area >= min_area and
                        cw >= min_width and
                        cw <= max_width and
                        ch >= min_height and
                        ch <= max_height and
                        aspect_ratio >= min_aspect):

                        valid_contours += 1

                        # 绘制轮廓和边界框
                        cv2.drawContours(contour_img, [contour], -1, (0, 255, 0), 2)
                        cv2.rectangle(contour_img, (cx, cy), (cx+cw, cy+ch), (0, 0, 255), 3)

                        # 绘制角标记
                        corner_len = min(cw, ch) // 4
                        cv2.line(contour_img, (cx, cy), (cx + corner_len, cy), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx, cy), (cx, cy + corner_len), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx+cw, cy), (cx+cw - corner_len, cy), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx+cw, cy), (cx+cw, cy + corner_len), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx, cy+ch), (cx + corner_len, cy+ch), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx, cy+ch), (cx, cy+ch - corner_len), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx+cw, cy+ch), (cx+cw - corner_len, cy+ch), (0, 255, 0), 3)
                        cv2.line(contour_img, (cx+cw, cy+ch), (cx+cw, cy+ch - corner_len), (0, 255, 0), 3)

                        # 显示信息
                        info = f"#{valid_contours} A:{int(area)} {cw}x{ch} R:{aspect_ratio:.1f}"
                        (text_w, text_h), _ = cv2.getTextSize(info, cv2.FONT_HERSHEY_SIMPLEX, 1.2, 3)
                        cv2.rectangle(contour_img, (cx, cy-text_h-12), (cx+text_w+8, cy-2), (0, 0, 0), -1)
                        cv2.putText(contour_img, info, (cx+4, cy-8),
                                   cv2.FONT_HERSHEY_SIMPLEX, 1.2, (0, 255, 0), 3)

                        # 显示中心点
                        center_x = cx + cw // 2
                        center_y = cy + ch // 2
                        cv2.circle(contour_img, (center_x, center_y), 10, (0, 255, 0), -1)
                        cv2.circle(contour_img, (center_x, center_y), 13, (0, 0, 255), 2)
                        cv2.putText(contour_img, str(valid_contours), (center_x-10, center_y+10),
                                   cv2.FONT_HERSHEY_SIMPLEX, 1.5, (0, 0, 0), 4)

                # 显示参数信息
                info_text = [
                    f"HSV: [{h_min},{s_min},{v_min}] - [{h_max},{s_max},{v_max}]",
                    f"Morph: Close={close_size}x{close_iter}, Open={open_size}",
                    f"Filter: Area>={min_area}, W:{min_width}-{max_width}, H:{min_height}-{max_height}, Aspect>={min_aspect}",
                    f"Found: {valid_contours} valid contours"
                ]

                y_pos = 40
                for text in info_text:
                    cv2.putText(contour_img, text, (10, y_pos),
                               cv2.FONT_HERSHEY_SIMPLEX, 1.2, (0, 255, 0), 3)
                    y_pos += 45

                # 根据current_view选择要显示的图像
                # 0=Original, 1=Mask, 2=Result, 3=Contours
                if current_view == 0:
                    combined = display_original.copy()
                elif current_view == 1:
                    combined = cv2.cvtColor(mask, cv2.COLOR_GRAY2BGR)
                elif current_view == 2:
                    combined = result.copy()
                else:  # current_view == 3
                    combined = contour_img.copy()

                # 应用缩放（使用高质量插值）
                scale_factor = scale_percent / 100.0
                if scale_percent != 100:
                    new_w = int(combined.shape[1] * scale_factor)
                    new_h = int(combined.shape[0] * scale_factor)
                    # 放大用LANCZOS4，缩小用AREA
                    if scale_factor > 1.0:
                        combined = cv2.resize(combined, (new_w, new_h), interpolation=cv2.INTER_LANCZOS4)
                    else:
                        combined = cv2.resize(combined, (new_w, new_h), interpolation=cv2.INTER_AREA)

                # 计算处理耗时
                process_time = (time.time() - start_time) * 1000  # 转换为毫秒

            # 显示处理耗时（在图像外面，每帧都显示）
            if combined is not None:
                display_img = combined.copy()

                # 显示处理耗时和当前视图模式
                time_text = f"Process time: {process_time:.1f}ms | View: {view_names[current_view]}"
                cv2.putText(display_img, time_text, (10, display_img.shape[0] - 20),
                           cv2.FONT_HERSHEY_SIMPLEX, 1.5, (0, 255, 0), 3)

                # 在图像上显示鼠标位置的HSV值
                if self.mouse_pos is not None:
                    mx, my = self.mouse_pos
                    # 检查鼠标位置是否在图像范围内
                    if 0 <= mx < display_img.shape[1] and 0 <= my < display_img.shape[0]:
                        # 获取当前像素的BGR值
                        pixel_bgr = display_img[my, mx]
                        # 转换为HSV
                        pixel_bgr_array = np.uint8([[pixel_bgr]])
                        pixel_hsv = cv2.cvtColor(pixel_bgr_array, cv2.COLOR_BGR2HSV)[0][0]

                        # 显示信息
                        info_text = f"HSV: [{pixel_hsv[0]}, {pixel_hsv[1]}, {pixel_hsv[2]}]"
                        info_bg = f"BGR: [{pixel_bgr[0]}, {pixel_bgr[1]}, {pixel_bgr[2]}]"

                        # 绘制背景框
                        cv2.rectangle(display_img, (mx + 15, my - 85), (mx + 450, my - 5), (0, 0, 0), -1)
                        cv2.rectangle(display_img, (mx + 15, my - 85), (mx + 450, my - 5), (0, 255, 0), 2)

                        # 绘制文字
                        cv2.putText(display_img, info_text, (mx + 20, my - 50),
                                   cv2.FONT_HERSHEY_SIMPLEX, 1.2, (0, 255, 0), 3)
                        cv2.putText(display_img, info_bg, (mx + 20, my - 18),
                                   cv2.FONT_HERSHEY_SIMPLEX, 1.2, (0, 255, 0), 3)

                        # 绘制十字光标
                        cv2.line(display_img, (mx - 15, my), (mx + 15, my), (0, 255, 0), 2)
                        cv2.line(display_img, (mx, my - 15), (mx, my + 15), (0, 255, 0), 2)
                        cv2.circle(display_img, (mx, my), 5, (0, 255, 0), -1)

                # 显示合并后的窗口
                cv2.imshow(display_window, display_img)

            # 键盘控制 (每10ms检测一次)
            key = cv2.waitKey(10) & 0xFF

            if key == ord('q'):
                # 退出并打印参数
                print("\n" + "=" * 60)
                print("最终参数:")
                print("=" * 60)
                print(f"\nlower_bound = np.array([{h_min}, {s_min}, {v_min}])")
                print(f"upper_bound = np.array([{h_max}, {s_max}, {v_max}])")
                print(f"\nclose_kernel_size = {close_size}")
                print(f"close_iterations = {close_iter}")
                print(f"open_kernel_size = {open_size}")
                print(f"\nmin_area = {min_area}")
                print(f"min_width = {min_width}")
                print(f"max_width = {max_width}")
                print(f"min_height = {min_height}")
                print(f"max_height = {max_height}")
                print(f"min_aspect_ratio = {min_aspect}")
                print(f"\nroi_region = ({roi_x}, {roi_y}, {roi_w}, {roi_h})")
                print("=" * 60)
                break

            elif key == ord('s'):
                if combined is not None:
                    filename = f"mask_{int(time.time())}.png"
                    # 保存的是mask，需要从当前值重新获取
                    cv2.imwrite(filename, combined)
                    print(f"\n✓ 已保存图像: {filename}")

        cv2.destroyAllWindows()


if __name__ == "__main__":
    debugger = ColorDebugger()
    debugger.run()
