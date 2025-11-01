import cv2
import numpy as np
from PIL import ImageGrab
import time

class AdvancedColorDebugger:
    def __init__(self):
        self.original_image = None
        self.roi_mode = False  # ROI选择模式
        self.roi_start = None
        self.roi_end = None
        self.roi_rect = None  # (x, y, w, h)
        
    def mouse_callback(self, event, x, y, flags, param):
        """鼠标回调：选择检测区域"""
        if event == cv2.EVENT_LBUTTONDOWN:
            self.roi_start = (x, y)
            self.roi_end = None
            
        elif event == cv2.EVENT_MOUSEMOVE:
            if self.roi_start is not None:
                self.roi_end = (x, y)
                
        elif event == cv2.EVENT_LBUTTONUP:
            if self.roi_start is not None:
                self.roi_end = (x, y)
                x1, y1 = self.roi_start
                x2, y2 = self.roi_end
                
                # 确保坐标正确
                x_min, x_max = min(x1, x2), max(x1, x2)
                y_min, y_max = min(y1, y2), max(y1, y2)
                
                self.roi_rect = (x_min, y_min, x_max - x_min, y_max - y_min)
                print(f"\n✓ 已选择区域: x={x_min}, y={y_min}, width={x_max-x_min}, height={y_max-y_min}")
    
    def capture_screen(self):
        """截取屏幕"""
        print("\n准备截图...")
        for i in range(3, 0, -1):
            print(f"{i}...")
            time.sleep(1)
        print("截图！")
        
        screenshot = ImageGrab.grab()
        return cv2.cvtColor(np.array(screenshot), cv2.COLOR_RGB2BGR)
    
    def run(self):
        """运行调试工具"""
        print("=" * 60)
        print("高级颜色识别调试工具")
        print("=" * 60)
        print("\n图片来源:")
        print("1. 从文件加载")
        print("2. 实时截屏")
        
        choice = input("\n选择 (1/2): ").strip()
        
        if choice == '1':
            image_path = input("请输入图片路径: ")
            self.original_image = cv2.imread(image_path)
            if self.original_image is None:
                print("❌ 无法读取图片！")
                return
        else:
            self.original_image = self.capture_screen()
        
        print(f"\n✓ 图片大小: {self.original_image.shape[1]}x{self.original_image.shape[0]}")
        
        # 询问是否需要选择区域
        print("\n是否需要选择检测区域？")
        print("1. 是（用鼠标框选初始区域）")
        print("2. 否（检测全图）")
        
        roi_choice = input("\n选择 (1/2): ").strip()
        
        if roi_choice == '1':
            print("\n✓ 请在图片上拖动鼠标框选检测区域")
            print("✓ 框选完成后按任意键继续")
            print("✓ 进入调试界面后可以用滑块精确调整")
            
            # 显示图片让用户选择区域
            cv2.namedWindow('Select ROI')
            cv2.setMouseCallback('Select ROI', self.mouse_callback)
            
            temp_img = self.original_image.copy()
            while True:
                display = temp_img.copy()
                
                # 绘制正在选择的矩形
                if self.roi_start and self.roi_end:
                    cv2.rectangle(display, self.roi_start, self.roi_end, (0, 255, 0), 2)
                    x1, y1 = self.roi_start
                    x2, y2 = self.roi_end
                    info = f"Size: {abs(x2-x1)}x{abs(y2-y1)}"
                    cv2.putText(display, info, (min(x1,x2), min(y1,y2)-10),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 255, 0), 2)
                
                # 显示已确定的ROI
                if self.roi_rect:
                    x, y, w, h = self.roi_rect
                    cv2.rectangle(display, (x, y), (x+w, y+h), (0, 255, 0), 3)
                    cv2.putText(display, "ROI Selected - Press any key to continue", (10, 30),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 0), 2)
                    cv2.putText(display, f"Position: ({x}, {y}) Size: {w}x{h}", (10, 60),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 255, 0), 2)
                
                cv2.imshow('Select ROI', display)
                
                key = cv2.waitKey(1)
                if key != -1 and self.roi_rect:
                    break
            
            cv2.destroyWindow('Select ROI')
        else:
            # 使用全图，也设置默认ROI以便后续调整
            h, w = self.original_image.shape[:2]
            self.roi_rect = (0, 0, w, h)
        
        # 开始调试
        self.debug_with_trackbars()
    
    def debug_with_trackbars(self):
        """带滑块的调试界面"""
        print("\n" + "=" * 60)
        print("开始调试")
        print("=" * 60)
        print("\n操作说明:")
        print("- 调整滑块找到最佳参数")
        print("- 按 'q' 退出并显示最终参数")
        print("- 按 's' 保存当前Mask图像")
        print("- 按 'r' 重新选择ROI区域")
        print("=" * 60)
        
        # 创建窗口
        cv2.namedWindow('Trackbars')
        cv2.namedWindow('Original')
        cv2.namedWindow('Mask')
        cv2.namedWindow('Result')
        cv2.namedWindow('Contours')
        
        # HSV 滑块
        cv2.createTrackbar('H_min', 'Trackbars', 0, 180, lambda x: None)
        cv2.createTrackbar('H_max', 'Trackbars', 10, 180, lambda x: None)
        cv2.createTrackbar('S_min', 'Trackbars', 100, 255, lambda x: None)
        cv2.createTrackbar('S_max', 'Trackbars', 255, 255, lambda x: None)
        cv2.createTrackbar('V_min', 'Trackbars', 100, 255, lambda x: None)
        cv2.createTrackbar('V_max', 'Trackbars', 255, 255, lambda x: None)
        
        # 形态学操作滑块（去除文字）
        cv2.createTrackbar('Close_Size', 'Trackbars', 7, 30, lambda x: None)  # 闭运算核大小
        cv2.createTrackbar('Close_Iter', 'Trackbars', 2, 10, lambda x: None)  # 闭运算迭代次数
        cv2.createTrackbar('Open_Size', 'Trackbars', 5, 30, lambda x: None)   # 开运算核大小
        
        # 轮廓过滤滑块
        cv2.createTrackbar('Min_Area', 'Trackbars', 500, 10000, lambda x: None)      # 最小面积
        cv2.createTrackbar('Min_Width', 'Trackbars', 50, 500, lambda x: None)        # 最小宽度
        cv2.createTrackbar('Min_Height', 'Trackbars', 10, 200, lambda x: None)       # 最小高度
        cv2.createTrackbar('Min_Aspect', 'Trackbars', 20, 100, lambda x: None)       # 最小宽高比*10
        
        # ROI 坐标滑块（如果有ROI）
        if self.roi_rect:
            x, y, w, h = self.roi_rect
            cv2.createTrackbar('ROI_X', 'Trackbars', x, self.original_image.shape[1], lambda x: None)
            cv2.createTrackbar('ROI_Y', 'Trackbars', y, self.original_image.shape[0], lambda x: None)
            cv2.createTrackbar('ROI_W', 'Trackbars', w, self.original_image.shape[1], lambda x: None)
            cv2.createTrackbar('ROI_H', 'Trackbars', h, self.original_image.shape[0], lambda x: None)
        
        while True:
            # 获取所有滑块值
            h_min = cv2.getTrackbarPos('H_min', 'Trackbars')
            h_max = cv2.getTrackbarPos('H_max', 'Trackbars')
            s_min = cv2.getTrackbarPos('S_min', 'Trackbars')
            s_max = cv2.getTrackbarPos('S_max', 'Trackbars')
            v_min = cv2.getTrackbarPos('V_min', 'Trackbars')
            v_max = cv2.getTrackbarPos('V_max', 'Trackbars')
            
            close_size = cv2.getTrackbarPos('Close_Size', 'Trackbars')
            close_iter = cv2.getTrackbarPos('Close_Iter', 'Trackbars')
            open_size = cv2.getTrackbarPos('Open_Size', 'Trackbars')
            
            min_area = cv2.getTrackbarPos('Min_Area', 'Trackbars')
            min_width = cv2.getTrackbarPos('Min_Width', 'Trackbars')
            min_height = cv2.getTrackbarPos('Min_Height', 'Trackbars')
            min_aspect = cv2.getTrackbarPos('Min_Aspect', 'Trackbars') / 10.0
            
            # 获取ROI区域
            if self.roi_rect:
                roi_x = cv2.getTrackbarPos('ROI_X', 'Trackbars')
                roi_y = cv2.getTrackbarPos('ROI_Y', 'Trackbars')
                roi_w = cv2.getTrackbarPos('ROI_W', 'Trackbars')
                roi_h = cv2.getTrackbarPos('ROI_H', 'Trackbars')
                
                # 确保ROI在图像范围内
                roi_w = min(roi_w, self.original_image.shape[1] - roi_x)
                roi_h = min(roi_h, self.original_image.shape[0] - roi_y)
                
                if roi_w <= 0 or roi_h <= 0:
                    roi_w = 100
                    roi_h = 100
                
                self.roi_rect = (roi_x, roi_y, roi_w, roi_h)
            
            # 提取ROI或使用全图
            if self.roi_rect:
                x, y, w, h = self.roi_rect
                image = self.original_image[y:y+h, x:x+w].copy()
                display_original = self.original_image.copy()
                cv2.rectangle(display_original, (x, y), (x+w, y+h), (0, 255, 0), 2)
                cv2.putText(display_original, f"ROI: {w}x{h}", (x, y-10),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 255, 0), 2)
            else:
                image = self.original_image.copy()
                display_original = self.original_image.copy()
            
            # 转换为HSV
            hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
            
            # 创建掩码
            lower = np.array([h_min, s_min, v_min])
            upper = np.array([h_max, s_max, v_max])
            mask = cv2.inRange(hsv, lower, upper)
            
            # 形态学操作（去除文字）
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
            
            # 过滤轮廓并绘制
            result = cv2.bitwise_and(image, image, mask=mask)
            contour_img = image.copy()
            
            valid_contours = 0
            for i, contour in enumerate(contours):
                area = cv2.contourArea(contour)
                x, y, w, h = cv2.boundingRect(contour)
                aspect_ratio = w / h if h > 0 else 0
                
                # 应用过滤条件
                if (area >= min_area and 
                    w >= min_width and 
                    h >= min_height and 
                    aspect_ratio >= min_aspect):
                    
                    valid_contours += 1
                    
                    # 绘制轮廓（绿色）
                    cv2.drawContours(contour_img, [contour], -1, (0, 255, 0), 2)
                    
                    # 绘制边界框（红色粗线，更明显）
                    cv2.rectangle(contour_img, (x, y), (x+w, y+h), (0, 0, 255), 3)
                    
                    # 绘制角标记（增强视觉效果）
                    corner_length = min(w, h) // 4
                    corner_thickness = 3
                    
                    # 左上角
                    cv2.line(contour_img, (x, y), (x + corner_length, y), (0, 255, 255), corner_thickness)
                    cv2.line(contour_img, (x, y), (x, y + corner_length), (0, 255, 255), corner_thickness)
                    
                    # 右上角
                    cv2.line(contour_img, (x+w, y), (x+w - corner_length, y), (0, 255, 255), corner_thickness)
                    cv2.line(contour_img, (x+w, y), (x+w, y + corner_length), (0, 255, 255), corner_thickness)
                    
                    # 左下角
                    cv2.line(contour_img, (x, y+h), (x + corner_length, y+h), (0, 255, 255), corner_thickness)
                    cv2.line(contour_img, (x, y+h), (x, y+h - corner_length), (0, 255, 255), corner_thickness)
                    
                    # 右下角
                    cv2.line(contour_img, (x+w, y+h), (x+w - corner_length, y+h), (0, 255, 255), corner_thickness)
                    cv2.line(contour_img, (x+w, y+h), (x+w, y+h - corner_length), (0, 255, 255), corner_thickness)
                    
                    # 显示信息（白色背景，黑色文字，更清晰）
                    info = f"#{valid_contours} A:{int(area)} {w}x{h} R:{aspect_ratio:.1f}"
                    
                    # 计算文本大小
                    (text_w, text_h), _ = cv2.getTextSize(info, cv2.FONT_HERSHEY_SIMPLEX, 0.5, 1)
                    
                    # 绘制白色背景
                    cv2.rectangle(contour_img, (x, y-text_h-8), (x+text_w+4, y-2), (255, 255, 255), -1)
                    
                    # 绘制黑色文字
                    cv2.putText(contour_img, info, (x+2, y-5),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 0), 1)
                    
                    # 显示中心点（更大更明显）
                    center_x = x + w // 2
                    center_y = y + h // 2
                    cv2.circle(contour_img, (center_x, center_y), 8, (255, 255, 0), -1)
                    cv2.circle(contour_img, (center_x, center_y), 10, (0, 0, 255), 2)
                    
                    # 在中心显示编号
                    cv2.putText(contour_img, str(valid_contours), (center_x-5, center_y+5),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 0, 0), 2)
            
            # 显示参数信息
            info_text = [
                f"HSV: [{h_min},{s_min},{v_min}] - [{h_max},{s_max},{v_max}]",
                f"Morph: Close={close_size}x{close_iter}, Open={open_size}",
                f"Filter: Area>={min_area}, Size>={min_width}x{min_height}, Aspect>={min_aspect}",
                f"Found: {valid_contours} valid contours"
            ]
            
            y_pos = 25
            for text in info_text:
                cv2.putText(contour_img, text, (10, y_pos),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 255), 1)
                y_pos += 20
            
            # 显示所有窗口
            cv2.imshow('Original', display_original)
            cv2.imshow('Mask', mask)
            cv2.imshow('Result', result)
            cv2.imshow('Contours', contour_img)
            
            # 键盘控制
            key = cv2.waitKey(1) & 0xFF
            
            if key == ord('q'):
                # 退出并打印参数
                print("\n" + "=" * 60)
                print("最终参数:")
                print("=" * 60)
                print("\n# HSV 范围")
                print(f"lower_bound = np.array([{h_min}, {s_min}, {v_min}])")
                print(f"upper_bound = np.array([{h_max}, {s_max}, {v_max}])")
                
                print("\n# 形态学操作")
                print(f"close_kernel_size = {close_size}")
                print(f"close_iterations = {close_iter}")
                print(f"open_kernel_size = {open_size}")
                
                print("\n# 轮廓过滤")
                print(f"min_area = {min_area}")
                print(f"min_width = {min_width}")
                print(f"min_height = {min_height}")
                print(f"min_aspect_ratio = {min_aspect}")
                
                if self.roi_rect:
                    print("\n# 检测区域 (ROI)")
                    x, y, w, h = self.roi_rect
                    print(f"roi_region = ({x}, {y}, {w}, {h})")
                
                print("\n# 完整代码示例:")
                print("```python")
                print("import cv2")
                print("import numpy as np")
                print()
                print(f"lower = np.array([{h_min}, {s_min}, {v_min}])")
                print(f"upper = np.array([{h_max}, {s_max}, {v_max}])")
                print()
                print("hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)")
                print("mask = cv2.inRange(hsv, lower, upper)")
                print()
                if close_size > 0:
                    print(f"kernel_close = np.ones(({close_size}, {close_size}), np.uint8)")
                    print(f"mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel_close, iterations={close_iter})")
                if open_size > 0:
                    print(f"kernel_open = np.ones(({open_size}, {open_size}), np.uint8)")
                    print(f"mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel_open)")
                print()
                print("contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)")
                print("for contour in contours:")
                print("    area = cv2.contourArea(contour)")
                print("    x, y, w, h = cv2.boundingRect(contour)")
                print("    aspect_ratio = w / h if h > 0 else 0")
                print(f"    if area >= {min_area} and w >= {min_width} and h >= {min_height} and aspect_ratio >= {min_aspect}:")
                print("        # 这是有效的目标")
                print("        pass")
                print("```")
                print("=" * 60)
                break
                
            elif key == ord('s'):
                # 保存Mask图像
                filename = f"mask_{int(time.time())}.png"
                cv2.imwrite(filename, mask)
                print(f"\n✓ 已保存 Mask 图像: {filename}")
                
            elif key == ord('r'):
                # 重新选择ROI
                print("\n重新选择ROI区域...")
                self.roi_start = None
                self.roi_end = None
                self.roi_rect = None
                
                cv2.namedWindow('Select ROI')
                cv2.setMouseCallback('Select ROI', self.mouse_callback)
                
                temp_img = self.original_image.copy()
                while True:
                    display = temp_img.copy()
                    
                    if self.roi_start and self.roi_end:
                        cv2.rectangle(display, self.roi_start, self.roi_end, (0, 255, 0), 2)
                    
                    if self.roi_rect:
                        x, y, w, h = self.roi_rect
                        cv2.rectangle(display, (x, y), (x+w, y+h), (0, 255, 0), 2)
                        cv2.putText(display, "ROI Selected - Press any key", (10, 30),
                                   cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 0), 2)
                    
                    cv2.imshow('Select ROI', display)
                    
                    key = cv2.waitKey(1)
                    if key != -1 and self.roi_rect:
                        break
                
                cv2.destroyWindow('Select ROI')
        
        cv2.destroyAllWindows()


if __name__ == "__main__":
    debugger = AdvancedColorDebugger()
    debugger.run()