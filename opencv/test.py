import cv2
import numpy as np
from PIL import ImageGrab
import time

class HealthBarDetector:
    def __init__(self):
        # 红色血条的HSV范围（两个范围，因为红色跨越0度）
        self.lower_red1 = np.array([0, 100, 100])
        self.upper_red1 = np.array([10, 255, 255])
        self.lower_red2 = np.array([170, 100, 100])
        self.upper_red2 = np.array([180, 255, 255])
        
        # 绿色血条的HSV范围
        self.lower_green = np.array([35, 100, 100])
        self.upper_green = np.array([85, 255, 255])
        
        # 最小血条面积（过滤噪点）
        self.min_area = 500
        
    def capture_screen(self, region=None):
        """截取屏幕
        region: (x, y, width, height) 指定区域，None表示全屏
        """
        if region:
            x, y, w, h = region
            screenshot = ImageGrab.grab(bbox=(x, y, x+w, y+h))
        else:
            screenshot = ImageGrab.grab()
        
        return cv2.cvtColor(np.array(screenshot), cv2.COLOR_RGB2BGR)
    
    def detect_color(self, image, color='red'):
        """检测指定颜色"""
        hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
        
        if color == 'red':
            # 红色需要两个范围
            mask1 = cv2.inRange(hsv, self.lower_red1, self.upper_red1)
            mask2 = cv2.inRange(hsv, self.lower_red2, self.upper_red2)
            mask = cv2.bitwise_or(mask1, mask2)
        elif color == 'green':
            mask = cv2.inRange(hsv, self.lower_green, self.upper_green)
        else:
            raise ValueError("颜色只支持 'red' 或 'green'")
        
        # 形态学操作：去除噪点，填充空洞
        kernel = np.ones((5, 5), np.uint8)
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)
        
        return mask
    
    def find_health_bars(self, image, color='red'):
        """查找血条位置和百分比"""
        mask = self.detect_color(image, color)
        
        # 查找轮廓
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        
        health_bars = []
        
        for contour in contours:
            area = cv2.contourArea(contour)
            
            # 过滤小区域
            if area < self.min_area:
                continue
            
            # 获取边界框
            x, y, w, h = cv2.boundingRect(contour)
            
            # 血条通常是横向的，宽高比应该大于2
            aspect_ratio = w / h if h > 0 else 0
            if aspect_ratio < 2:
                continue
            
            health_bars.append({
                'x': x,
                'y': y,
                'width': w,
                'height': h,
                'area': area,
                'center': (x + w//2, y + h//2)
            })
        
        return health_bars
    
    def calculate_health_percentage(self, image, bar_region, color='red'):
        """计算血条百分比
        bar_region: 血条的完整区域 (x, y, width, height)
        """
        x, y, w, h = bar_region
        roi = image[y:y+h, x:x+w]
        
        mask = self.detect_color(roi, color)
        
        # 计算有颜色的像素数量
        colored_pixels = cv2.countNonZero(mask)
        total_pixels = w * h
        
        percentage = (colored_pixels / total_pixels) * 100
        
        return percentage
    
    def draw_results(self, image, health_bars, color='red'):
        """在图像上绘制检测结果"""
        result = image.copy()
        
        for i, bar in enumerate(health_bars):
            x, y, w, h = bar['x'], bar['y'], bar['width'], bar['height']
            
            # 绘制边界框
            box_color = (0, 0, 255) if color == 'red' else (0, 255, 0)
            cv2.rectangle(result, (x, y), (x+w, y+h), box_color, 2)
            
            # 计算血量百分比
            percentage = self.calculate_health_percentage(image, (x, y, w, h), color)
            
            # 显示信息
            text = f"HP: {percentage:.1f}%"
            cv2.putText(result, text, (x, y-10), 
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, box_color, 2)
            
            # 显示中心点
            center = bar['center']
            cv2.circle(result, center, 5, (255, 255, 0), -1)
        
        return result


def main():
    detector = HealthBarDetector()
    
    print("血条识别系统")
    print("=" * 50)
    print("模式选择:")
    print("1. 从图片文件识别")
    print("2. 实时屏幕截图识别")
    print("3. 调试模式（调整HSV参数）")
    
    mode = input("\n请选择模式 (1/2/3): ")
    
    if mode == '1':
        # 从文件识别
        image_path = input("请输入图片路径: ")
        image = cv2.imread(image_path)
        
        if image is None:
            print("无法读取图片！")
            return
        
        color = input("血条颜色 (red/green): ").lower()
        
        # 检测血条
        health_bars = detector.find_health_bars(image, color)
        
        print(f"\n找到 {len(health_bars)} 个血条:")
        for i, bar in enumerate(health_bars):
            percentage = detector.calculate_health_percentage(
                image, (bar['x'], bar['y'], bar['width'], bar['height']), color
            )
            print(f"血条 {i+1}: 位置({bar['x']}, {bar['y']}), "
                  f"大小({bar['width']}x{bar['height']}), "
                  f"血量: {percentage:.1f}%")
        
        # 显示结果
        result = detector.draw_results(image, health_bars, color)
        cv2.imshow('Original', image)
        cv2.imshow('Result', result)
        cv2.waitKey(0)
        cv2.destroyAllWindows()
    
    elif mode == '2':
        # 实时截图识别
        color = input("血条颜色 (red/green): ").lower()
        region_input = input("指定区域 (格式: x,y,width,height) 或按回车使用全屏: ")
        
        region = None
        if region_input.strip():
            region = tuple(map(int, region_input.split(',')))
        
        print("\n开始实时识别，按 'q' 退出...")
        
        while True:
            # 截取屏幕
            image = detector.capture_screen(region)
            
            # 检测血条
            health_bars = detector.find_health_bars(image, color)
            
            # 绘制结果
            result = detector.draw_results(image, health_bars, color)
            
            cv2.imshow('Health Bar Detection', result)
            
            # 按 'q' 退出
            if cv2.waitKey(1) & 0xFF == ord('q'):
                break
            
            time.sleep(0.1)  # 降低CPU占用
        
        cv2.destroyAllWindows()
    
    elif mode == '3':
        # 调试模式
        debug_mode(detector)
    
    else:
        print("无效的选择！")


def debug_mode(detector):
    """调试模式：实时调整HSV参数"""
    print("\n=== 调试模式 ===")
    image_path = input("请输入图片路径（或按回车使用实时截图）: ")
    
    if image_path.strip():
        image = cv2.imread(image_path)
    else:
        print("3秒后截图...")
        time.sleep(3)
        image = detector.capture_screen()
    
    if image is None:
        print("无法获取图片！")
        return
    
    # 创建窗口和滑块
    cv2.namedWindow('Trackbars')
    cv2.createTrackbar('H_min', 'Trackbars', 0, 180, lambda x: None)
    cv2.createTrackbar('H_max', 'Trackbars', 10, 180, lambda x: None)
    cv2.createTrackbar('S_min', 'Trackbars', 100, 255, lambda x: None)
    cv2.createTrackbar('S_max', 'Trackbars', 255, 255, lambda x: None)
    cv2.createTrackbar('V_min', 'Trackbars', 100, 255, lambda x: None)
    cv2.createTrackbar('V_max', 'Trackbars', 255, 255, lambda x: None)
    
    print("调整滑块找到最佳参数，按 'q' 退出")
    
    while True:
        # 获取滑块值
        h_min = cv2.getTrackbarPos('H_min', 'Trackbars')
        h_max = cv2.getTrackbarPos('H_max', 'Trackbars')
        s_min = cv2.getTrackbarPos('S_min', 'Trackbars')
        s_max = cv2.getTrackbarPos('S_max', 'Trackbars')
        v_min = cv2.getTrackbarPos('V_min', 'Trackbars')
        v_max = cv2.getTrackbarPos('V_max', 'Trackbars')
        
        # 转换为HSV
        hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
        
        # 创建掩码
        lower = np.array([h_min, s_min, v_min])
        upper = np.array([h_max, s_max, v_max])
        mask = cv2.inRange(hsv, lower, upper)
        
        # 应用掩码
        result = cv2.bitwise_and(image, image, mask=mask)
        
        # 显示参数
        info_text = f"HSV: [{h_min},{s_min},{v_min}] - [{h_max},{s_max},{v_max}]"
        cv2.putText(result, info_text, (10, 30), 
                   cv2.FONT_HERSHEY_SIMPLEX, 0.7, (0, 255, 0), 2)
        
        cv2.imshow('Original', image)
        cv2.imshow('Mask', mask)
        cv2.imshow('Result', result)
        
        if cv2.waitKey(1) & 0xFF == ord('q'):
            print(f"\n最佳参数:")
            print(f"lower = np.array([{h_min}, {s_min}, {v_min}])")
            print(f"upper = np.array([{h_max}, {s_max}, {v_max}])")
            break
    
    cv2.destroyAllWindows()


if __name__ == "__main__":
    main()