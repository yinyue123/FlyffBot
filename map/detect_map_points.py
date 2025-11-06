"""
地图点识别脚本
使用HSV颜色空间检测地图中的彩色小点
"""

import cv2
import numpy as np
from typing import List, Tuple, Dict
from dataclasses import dataclass


@dataclass
class Point:
    """检测到的点"""
    x: int
    y: int
    color_type: str
    radius: float


class MapPointDetector:
    """地图点检测器"""

    def __init__(self):
        # 定义不同颜色的HSV范围
        # HSV: Hue(0-180), Saturation(0-255), Value(0-255)
        self.color_ranges = {
            'orange': {
                'lower': np.array([10, 100, 100]),   # 橙色下界
                'upper': np.array([25, 255, 255])    # 橙色上界
            },
            'red1': {  # 红色在HSV中分两段
                'lower': np.array([0, 100, 100]),
                'upper': np.array([10, 255, 255])
            },
            'red2': {
                'lower': np.array([170, 100, 100]),
                'upper': np.array([180, 255, 255])
            },
            'yellow': {
                'lower': np.array([25, 100, 100]),
                'upper': np.array([35, 255, 255])
            }
        }

        # 点的大小范围（像素面积）
        self.min_area = 10
        self.max_area = 200

        # 圆形度阈值（越接近1越圆）
        self.min_circularity = 0.5

        # 重叠检测阈值（像素距离）
        self.merge_distance = 10

    def preprocess(self, image: np.ndarray) -> np.ndarray:
        """预处理图像"""
        # 高斯模糊去噪
        blurred = cv2.GaussianBlur(image, (5, 5), 0)
        # 转换到HSV色彩空间
        hsv = cv2.cvtColor(blurred, cv2.COLOR_BGR2HSV)
        return hsv

    def extract_color_mask(self, hsv_image: np.ndarray, color_name: str) -> np.ndarray:
        """提取指定颜色的掩码"""
        if color_name in ['red1', 'red2']:
            # 红色需要合并两个范围
            mask1 = cv2.inRange(hsv_image,
                               self.color_ranges['red1']['lower'],
                               self.color_ranges['red1']['upper'])
            mask2 = cv2.inRange(hsv_image,
                               self.color_ranges['red2']['lower'],
                               self.color_ranges['red2']['upper'])
            mask = cv2.bitwise_or(mask1, mask2)
            return mask, 'red'
        else:
            mask = cv2.inRange(hsv_image,
                             self.color_ranges[color_name]['lower'],
                             self.color_ranges[color_name]['upper'])
            return mask, color_name

    def morphological_operations(self, mask: np.ndarray) -> np.ndarray:
        """形态学操作优化掩码"""
        # 定义结构元素
        kernel_small = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (3, 3))
        kernel_medium = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5, 5))

        # 开运算去除小噪点
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel_small)
        # 闭运算填充内部
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel_medium)

        return mask

    def calculate_circularity(self, contour: np.ndarray) -> float:
        """计算轮廓的圆形度"""
        area = cv2.contourArea(contour)
        perimeter = cv2.arcLength(contour, True)
        if perimeter == 0:
            return 0
        circularity = 4 * np.pi * area / (perimeter * perimeter)
        return circularity

    def detect_points_from_mask(self, mask: np.ndarray, color_type: str) -> List[Point]:
        """从掩码中检测点"""
        points = []

        # 查找轮廓
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        for contour in contours:
            # 计算面积
            area = cv2.contourArea(contour)

            # 面积过滤
            if area < self.min_area or area > self.max_area:
                continue

            # 圆形度过滤
            circularity = self.calculate_circularity(contour)
            if circularity < self.min_circularity:
                continue

            # 计算中心点和半径
            M = cv2.moments(contour)
            if M['m00'] == 0:
                continue

            cx = int(M['m10'] / M['m00'])
            cy = int(M['m01'] / M['m00'])

            # 计算等效圆半径
            radius = np.sqrt(area / np.pi)

            points.append(Point(cx, cy, color_type, radius))

        return points

    def merge_overlapping_points(self, points: List[Point]) -> List[Point]:
        """合并重叠的点"""
        if len(points) <= 1:
            return points

        merged = []
        used = set()

        for i, point1 in enumerate(points):
            if i in used:
                continue

            # 查找与当前点距离很近的点
            cluster = [point1]
            for j, point2 in enumerate(points[i+1:], start=i+1):
                if j in used:
                    continue

                distance = np.sqrt((point1.x - point2.x)**2 + (point1.y - point2.y)**2)
                if distance < self.merge_distance:
                    cluster.append(point2)
                    used.add(j)

            # 计算聚类中心
            avg_x = int(np.mean([p.x for p in cluster]))
            avg_y = int(np.mean([p.y for p in cluster]))
            avg_radius = np.mean([p.radius for p in cluster])

            # 使用第一个点的颜色类型
            merged.append(Point(avg_x, avg_y, point1.color_type, avg_radius))

        return merged

    def detect(self, image_path: str) -> Tuple[List[Point], np.ndarray]:
        """
        检测地图中的所有点

        Args:
            image_path: 图片路径

        Returns:
            检测到的点列表和可视化图像
        """
        # 读取图像
        image = cv2.imread(image_path)
        if image is None:
            raise ValueError(f"无法读取图像: {image_path}")

        # 预处理
        hsv_image = self.preprocess(image)

        # 检测所有颜色的点
        all_points = []

        for color_name in ['orange', 'red1', 'yellow']:
            # 提取颜色掩码
            mask, actual_color = self.extract_color_mask(hsv_image, color_name)

            # 形态学优化
            mask = self.morphological_operations(mask)

            # 检测点
            points = self.detect_points_from_mask(mask, actual_color)
            all_points.extend(points)

        # 合并重叠的点
        all_points = self.merge_overlapping_points(all_points)

        # 可视化
        result_image = self.visualize(image, all_points)

        return all_points, result_image

    def visualize(self, image: np.ndarray, points: List[Point]) -> np.ndarray:
        """可视化检测结果"""
        result = image.copy()

        # 定义颜色映射（BGR格式）
        color_map = {
            'orange': (0, 165, 255),    # 橙色
            'red': (0, 0, 255),          # 红色
            'yellow': (0, 255, 255)      # 黄色
        }

        for point in points:
            color = color_map.get(point.color_type, (255, 255, 255))
            # 画圆圈标记
            cv2.circle(result, (point.x, point.y), int(point.radius) + 2, color, 2)
            # 画中心点
            cv2.circle(result, (point.x, point.y), 2, (255, 255, 255), -1)

        return result

    def save_results(self, points: List[Point], output_path: str):
        """保存检测结果到文本文件"""
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(f"检测到 {len(points)} 个点\n\n")

            # 按颜色分组统计
            color_counts = {}
            for point in points:
                color_counts[point.color_type] = color_counts.get(point.color_type, 0) + 1

            f.write("颜色统计:\n")
            for color, count in color_counts.items():
                f.write(f"  {color}: {count} 个\n")

            f.write("\n详细坐标:\n")
            f.write("X\tY\t颜色\t半径\n")
            for point in points:
                f.write(f"{point.x}\t{point.y}\t{point.color_type}\t{point.radius:.2f}\n")


def main():
    """主函数"""
    # 创建检测器
    detector = MapPointDetector()

    # 检测点
    print("开始检测地图点...")
    points, result_image = detector.detect('map.jpeg')

    # 打印统计信息
    print(f"\n检测到 {len(points)} 个点")

    color_counts = {}
    for point in points:
        color_counts[point.color_type] = color_counts.get(point.color_type, 0) + 1

    print("\n颜色分布:")
    for color, count in color_counts.items():
        print(f"  {color}: {count} 个")

    # 保存结果
    cv2.imwrite('result_visual.jpg', result_image)
    detector.save_results(points, 'result_data.txt')

    print("\n结果已保存:")
    print("  - result_visual.jpg (可视化图像)")
    print("  - result_data.txt (坐标数据)")

    # 显示图像（可选）
    # cv2.imshow('Original', cv2.imread('map.jpeg'))
    # cv2.imshow('Result', result_image)
    # cv2.waitKey(0)
    # cv2.destroyAllWindows()


if __name__ == '__main__':
    main()
