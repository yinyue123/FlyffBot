"""
识别地图圆形区域
"""

import cv2
import numpy as np


def main():
    # 读取图像
    image = cv2.imread('map.jpeg')
    if image is None:
        print("无法读取图像")
        return

    # 转灰度
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

    # 二值化 - 使用自适应阈值或固定阈值
    _, binary = cv2.threshold(gray, 50, 255, cv2.THRESH_BINARY)

    # 找轮廓
    contours, _ = cv2.findContours(binary, cv2.RETR_TREE, cv2.CHAIN_APPROX_SIMPLE)

    print(f"找到 {len(contours)} 个轮廓")

    # 找到所有圆形轮廓
    circles = []
    for contour in contours:
        area = cv2.contourArea(contour)

        # 面积太小的忽略
        if area < 10000:
            continue

        # 计算圆形度
        perimeter = cv2.arcLength(contour, True)
        if perimeter == 0:
            continue

        circularity = 4 * np.pi * area / (perimeter * perimeter)

        # 圆形度要足够高
        if circularity > 0.7:
            # 拟合最小外接圆
            (x, y), radius = cv2.minEnclosingCircle(contour)
            circles.append({
                'center': (int(x), int(y)),
                'radius': int(radius),
                'area': area,
                'circularity': circularity
            })
            print(f"圆形: 中心({int(x)}, {int(y)}), 半径={int(radius)}, 面积={int(area)}, 圆形度={circularity:.3f}")

    # 按面积排序
    circles.sort(key=lambda c: c['area'])

    # 找到大于某个值(比如10000)的最小圆
    min_area_threshold = 10000
    target_circle = None
    for circle in circles:
        if circle['area'] > min_area_threshold:
            target_circle = circle
            break

    if target_circle:
        print(f"\n选中的圆: 中心{target_circle['center']}, 半径={target_circle['radius']}")

        # 画出这个圆
        result = image.copy()
        cv2.circle(result, target_circle['center'], target_circle['radius'], (0, 255, 0), 3)
        cv2.circle(result, target_circle['center'], 5, (0, 0, 255), -1)

        # 保存结果
        cv2.imwrite('circle_detected.jpg', result)
        print("结果已保存到 circle_detected.jpg")
    else:
        print("没有找到符合条件的圆")


if __name__ == '__main__':
    main()
