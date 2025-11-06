"""
颜色调试工具
用于调整HSV颜色范围，找到最佳的检测参数
"""

import cv2
import numpy as np


def nothing(x):
    pass


def main():
    """交互式调整HSV范围"""
    # 读取图像
    image = cv2.imread('map.jpeg')
    if image is None:
        print("无法读取图像")
        return

    # 创建窗口和滑动条
    cv2.namedWindow('HSV Tuner')

    # 创建滑动条
    cv2.createTrackbar('H_min', 'HSV Tuner', 10, 180, nothing)
    cv2.createTrackbar('H_max', 'HSV Tuner', 25, 180, nothing)
    cv2.createTrackbar('S_min', 'HSV Tuner', 100, 255, nothing)
    cv2.createTrackbar('S_max', 'HSV Tuner', 255, 255, nothing)
    cv2.createTrackbar('V_min', 'HSV Tuner', 100, 255, nothing)
    cv2.createTrackbar('V_max', 'HSV Tuner', 255, 255, nothing)

    print("使用滑动条调整HSV范围")
    print("按 'q' 退出")
    print("按 's' 保存当前参数")

    while True:
        # 获取当前滑动条的值
        h_min = cv2.getTrackbarPos('H_min', 'HSV Tuner')
        h_max = cv2.getTrackbarPos('H_max', 'HSV Tuner')
        s_min = cv2.getTrackbarPos('S_min', 'HSV Tuner')
        s_max = cv2.getTrackbarPos('S_max', 'HSV Tuner')
        v_min = cv2.getTrackbarPos('V_min', 'HSV Tuner')
        v_max = cv2.getTrackbarPos('V_max', 'HSV Tuner')

        # 转换到HSV
        hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)

        # 创建掩码
        lower = np.array([h_min, s_min, v_min])
        upper = np.array([h_max, s_max, v_max])
        mask = cv2.inRange(hsv, lower, upper)

        # 形态学操作
        kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5, 5))
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)

        # 应用掩码到原图
        result = cv2.bitwise_and(image, image, mask=mask)

        # 显示
        cv2.imshow('HSV Tuner', np.hstack([image, result]))
        cv2.imshow('Mask', mask)

        key = cv2.waitKey(1) & 0xFF
        if key == ord('q'):
            break
        elif key == ord('s'):
            print(f"\n当前参数:")
            print(f"lower = np.array([{h_min}, {s_min}, {v_min}])")
            print(f"upper = np.array([{h_max}, {s_max}, {v_max}])")

    cv2.destroyAllWindows()


if __name__ == '__main__':
    main()
