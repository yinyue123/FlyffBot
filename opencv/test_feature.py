import cv2
import numpy as np
import time

class FeatureDetectorDebugger:
    def __init__(self):
        self.template = None  # 模板图（小图）
        self.scene = None     # 场景图（大图）
        self.mouse_pos = None

        # 特征检测器类型
        self.detector_types = {
            0: 'SIFT',
            1: 'ORB',
            2: 'AKAZE',
            3: 'BRISK'
        }

        # 匹配器类型
        self.matcher_types = {
            0: 'BF (暴力匹配)',
            1: 'FLANN (快速匹配)'
        }

    def mouse_callback(self, event, x, y, flags, param):
        """鼠标回调：记录鼠标位置"""
        if event == cv2.EVENT_MOUSEMOVE:
            self.mouse_pos = (x, y)

    def detect_and_compute(self, image, detector_type, **params):
        """
        检测特征点并计算描述符

        返回: (keypoints, descriptors, detector_name)
        """
        try:
            if detector_type == 0:  # SIFT
                detector = cv2.SIFT_create(
                    nfeatures=params.get('nfeatures', 0),
                    nOctaveLayers=params.get('nOctaveLayers', 3),
                    contrastThreshold=params.get('contrastThreshold', 0.04),
                    edgeThreshold=params.get('edgeThreshold', 10),
                    sigma=params.get('sigma', 1.6)
                )
            elif detector_type == 1:  # ORB
                detector = cv2.ORB_create(
                    nfeatures=params.get('nfeatures', 500),
                    scaleFactor=params.get('scaleFactor', 1.2),
                    nlevels=params.get('nlevels', 8),
                    edgeThreshold=params.get('edgeThreshold', 31),
                    firstLevel=params.get('firstLevel', 0),
                    WTA_K=params.get('WTA_K', 2),
                    patchSize=params.get('patchSize', 31)
                )
            elif detector_type == 2:  # AKAZE
                detector = cv2.AKAZE_create(
                    threshold=params.get('threshold', 0.001),
                    nOctaves=params.get('nOctaves', 4),
                    nOctaveLayers=params.get('nOctaveLayers', 4),
                    diffusivity=params.get('diffusivity', 1)
                )
            elif detector_type == 3:  # BRISK
                detector = cv2.BRISK_create(
                    thresh=params.get('thresh', 30),
                    octaves=params.get('octaves', 3),
                    patternScale=params.get('patternScale', 1.0)
                )
            else:
                return None, None, "Unknown"

            # 检测关键点和计算描述符
            kp, des = detector.detectAndCompute(image, None)
            return kp, des, self.detector_types[detector_type]

        except Exception as e:
            print(f"检测错误: {e}")
            return None, None, "Error"

    def match_features(self, des1, des2, matcher_type, detector_type, ratio_thresh=0.75):
        """
        匹配特征描述符

        返回: 好的匹配列表
        """
        if des1 is None or des2 is None or len(des1) < 2 or len(des2) < 2:
            return []

        try:
            # 根据检测器类型选择距离度量
            if detector_type == 0 or detector_type == 2:  # SIFT, AKAZE 使用 L2
                norm_type = cv2.NORM_L2
            else:  # ORB, BRISK 使用 HAMMING
                norm_type = cv2.NORM_HAMMING

            if matcher_type == 0:  # BF Matcher
                matcher = cv2.BFMatcher(norm_type, crossCheck=False)
            else:  # FLANN Matcher
                if norm_type == cv2.NORM_L2:
                    FLANN_INDEX_KDTREE = 1
                    index_params = dict(algorithm=FLANN_INDEX_KDTREE, trees=5)
                else:
                    FLANN_INDEX_LSH = 6
                    index_params = dict(algorithm=FLANN_INDEX_LSH,
                                       table_number=6, key_size=12, multi_probe_level=1)
                search_params = dict(checks=50)
                matcher = cv2.FlannBasedMatcher(index_params, search_params)

            # KNN 匹配
            matches = matcher.knnMatch(des1, des2, k=2)

            # Lowe's ratio test
            good_matches = []
            for match in matches:
                if len(match) == 2:
                    m, n = match
                    if m.distance < ratio_thresh * n.distance:
                        good_matches.append(m)

            return good_matches

        except Exception as e:
            print(f"匹配错误: {e}")
            return []

    def compute_homography(self, kp1, kp2, matches, ransac_thresh):
        """
        计算单应性矩阵

        返回: (homography_matrix, inliers_count)
        """
        if len(matches) < 4:
            return None, 0

        # 提取匹配点的坐标
        src_pts = np.float32([kp1[m.queryIdx].pt for m in matches]).reshape(-1, 1, 2)
        dst_pts = np.float32([kp2[m.trainIdx].pt for m in matches]).reshape(-1, 1, 2)

        # 使用 RANSAC 计算单应性矩阵
        M, mask = cv2.findHomography(src_pts, dst_pts, cv2.RANSAC, ransac_thresh)

        if M is None:
            return None, 0

        inliers = np.sum(mask)
        return M, int(inliers)

    def draw_detection_result(self, scene_img, template_img, kp_scene, kp_template,
                              matches, homography_matrix, detector_name,
                              show_keypoints=True, show_matches=True, max_matches=50):
        """
        并排显示两张图，并绘制匹配关系
        - 左边：模板图 + 关键点
        - 右边：场景图 + 检测框
        - 中间：连接线显示对应关系
        """
        # 复制图像
        img_template = template_img.copy()
        img_scene = scene_img.copy()

        # 1. 在模板图上绘制关键点
        if show_keypoints and kp_template:
            for kp in kp_template:
                pt = tuple(map(int, kp.pt))
                cv2.circle(img_template, pt, 3, (0, 255, 255), -1)
                cv2.circle(img_template, pt, 6, (0, 255, 255), 1)

        # 2. 在场景图上绘制检测框和关键点
        if homography_matrix is not None:
            h, w = template_img.shape[:2]
            corners = np.float32([[0, 0], [0, h], [w, h], [w, 0]]).reshape(-1, 1, 2)
            transformed_corners = cv2.perspectiveTransform(corners, homography_matrix)

            # 绘制绿色检测框
            cv2.polylines(img_scene, [np.int32(transformed_corners)], True, (0, 255, 0), 3)

            # 在框内绘制中心十字
            center = np.mean(transformed_corners, axis=0)[0]
            center = tuple(map(int, center))
            cv2.drawMarker(img_scene, center, (0, 255, 0), cv2.MARKER_CROSS, 20, 2)

        # 3. 在场景图上绘制关键点
        if show_keypoints and kp_scene:
            for kp in kp_scene[:100]:  # 限制显示数量
                pt = tuple(map(int, kp.pt))
                cv2.circle(img_scene, pt, 2, (255, 255, 0), -1)

        # 4. 将两张图并排放置
        h1, w1 = img_template.shape[:2]
        h2, w2 = img_scene.shape[:2]

        # 创建合并图像
        max_height = max(h1, h2)
        total_width = w1 + w2

        # 调整图像大小使高度一致
        if h1 < max_height:
            new_template = np.zeros((max_height, w1, 3), dtype=np.uint8)
            new_template[:h1, :] = img_template
            img_template = new_template

        if h2 < max_height:
            new_scene = np.zeros((max_height, w2, 3), dtype=np.uint8)
            new_scene[:h2, :] = img_scene
            img_scene = new_scene

        # 合并图像
        result = np.hstack([img_template, img_scene])

        # 5. 绘制匹配连接线
        if show_matches and matches and homography_matrix is not None:
            # 只绘制前N个匹配
            for i, match in enumerate(matches[:max_matches]):
                if i >= max_matches:
                    break

                # 模板点
                pt1 = tuple(map(int, kp_template[match.queryIdx].pt))

                # 场景点（需要偏移w1）
                pt2 = tuple(map(int, kp_scene[match.trainIdx].pt))
                pt2 = (pt2[0] + w1, pt2[1])

                # 绘制连接线
                cv2.line(result, pt1, pt2, (0, 255, 0), 1)

        return result

    def debug_with_trackbars(self):
        """使用滑动条进行调试"""
        trackbar_window = 'Trackbars'
        display_window = 'Detection Result'

        cv2.namedWindow(trackbar_window)
        cv2.namedWindow(display_window)
        cv2.setMouseCallback(display_window, self.mouse_callback)

        # 创建滑动条
        # 通用参数
        cv2.createTrackbar('Detector Type', trackbar_window, 0, 3, lambda x: None)
        cv2.createTrackbar('Matcher Type', trackbar_window, 0, 1, lambda x: None)
        cv2.createTrackbar('Ratio Thresh', trackbar_window, 75, 99, lambda x: None)
        cv2.createTrackbar('RANSAC Thresh', trackbar_window, 50, 100, lambda x: None)
        cv2.createTrackbar('Min Matches', trackbar_window, 10, 50, lambda x: None)
        cv2.createTrackbar('Max Show Matches', trackbar_window, 50, 200, lambda x: None)
        cv2.createTrackbar('Show Keypoints', trackbar_window, 1, 1, lambda x: None)
        cv2.createTrackbar('Show Matches', trackbar_window, 1, 1, lambda x: None)

        # SIFT 参数
        cv2.createTrackbar('SIFT_nFeatures', trackbar_window, 0, 10000, lambda x: None)
        cv2.createTrackbar('SIFT_Octaves', trackbar_window, 3, 10, lambda x: None)
        cv2.createTrackbar('SIFT_Contrast', trackbar_window, 40, 100, lambda x: None)
        cv2.createTrackbar('SIFT_Edge', trackbar_window, 10, 30, lambda x: None)

        # ORB 参数
        cv2.createTrackbar('ORB_nFeatures', trackbar_window, 500, 10000, lambda x: None)
        cv2.createTrackbar('ORB_Scale', trackbar_window, 12, 20, lambda x: None)
        cv2.createTrackbar('ORB_Levels', trackbar_window, 8, 16, lambda x: None)
        cv2.createTrackbar('ORB_EdgeThresh', trackbar_window, 31, 50, lambda x: None)
        cv2.createTrackbar('ORB_FirstLevel', trackbar_window, 0, 5, lambda x: None)
        cv2.createTrackbar('ORB_WTA_K', trackbar_window, 2, 4, lambda x: None)
        cv2.createTrackbar('ORB_PatchSize', trackbar_window, 31, 50, lambda x: None)

        # AKAZE 参数
        cv2.createTrackbar('AKAZE_Thresh', trackbar_window, 10, 100, lambda x: None)
        cv2.createTrackbar('AKAZE_Octaves', trackbar_window, 4, 10, lambda x: None)
        cv2.createTrackbar('AKAZE_Layers', trackbar_window, 4, 10, lambda x: None)
        cv2.createTrackbar('AKAZE_Diffusivity', trackbar_window, 1, 2, lambda x: None)

        # BRISK 参数
        cv2.createTrackbar('BRISK_Thresh', trackbar_window, 30, 100, lambda x: None)
        cv2.createTrackbar('BRISK_Octaves', trackbar_window, 3, 10, lambda x: None)
        cv2.createTrackbar('BRISK_Scale', trackbar_window, 10, 20, lambda x: None)

        print("\n" + "=" * 60)
        print("特征检测调试工具 - 开始运行")
        print("=" * 60)
        print("快捷键:")
        print("  q / ESC - 退出并生成代码")
        print("  s       - 保存当前结果图像")
        print("=" * 60)

        # 缓存变量
        last_params = None
        cached_result = None
        cached_info = None

        while True:
            # 读取所有参数
            detector_type = cv2.getTrackbarPos('Detector Type', trackbar_window)
            matcher_type = cv2.getTrackbarPos('Matcher Type', trackbar_window)
            ratio_test = cv2.getTrackbarPos('Ratio Thresh', trackbar_window) / 100.0
            ransac_thresh = cv2.getTrackbarPos('RANSAC Thresh', trackbar_window) / 10.0
            min_matches = cv2.getTrackbarPos('Min Matches', trackbar_window)
            max_show_matches = cv2.getTrackbarPos('Max Show Matches', trackbar_window)
            show_keypoints = cv2.getTrackbarPos('Show Keypoints', trackbar_window) == 1
            show_matches = cv2.getTrackbarPos('Show Matches', trackbar_window) == 1

            # 读取算法特定参数
            nfeatures = cv2.getTrackbarPos('SIFT_nFeatures', trackbar_window)
            sift_octaves = cv2.getTrackbarPos('SIFT_Octaves', trackbar_window)
            sift_contrast = cv2.getTrackbarPos('SIFT_Contrast', trackbar_window) / 1000.0
            sift_edge = cv2.getTrackbarPos('SIFT_Edge', trackbar_window)

            orb_nfeatures = cv2.getTrackbarPos('ORB_nFeatures', trackbar_window)
            orb_scale = cv2.getTrackbarPos('ORB_Scale', trackbar_window) / 10.0
            orb_levels = cv2.getTrackbarPos('ORB_Levels', trackbar_window)
            orb_edge = cv2.getTrackbarPos('ORB_EdgeThresh', trackbar_window)
            orb_first = cv2.getTrackbarPos('ORB_FirstLevel', trackbar_window)
            orb_wta = cv2.getTrackbarPos('ORB_WTA_K', trackbar_window)
            orb_patch = cv2.getTrackbarPos('ORB_PatchSize', trackbar_window)

            akaze_thresh = cv2.getTrackbarPos('AKAZE_Thresh', trackbar_window) / 10000.0
            akaze_octaves = cv2.getTrackbarPos('AKAZE_Octaves', trackbar_window)
            akaze_layers = cv2.getTrackbarPos('AKAZE_Layers', trackbar_window)
            akaze_diff = cv2.getTrackbarPos('AKAZE_Diffusivity', trackbar_window)

            brisk_thresh = cv2.getTrackbarPos('BRISK_Thresh', trackbar_window)
            brisk_octaves = cv2.getTrackbarPos('BRISK_Octaves', trackbar_window)
            brisk_scale = cv2.getTrackbarPos('BRISK_Scale', trackbar_window) / 10.0

            # 构建当前参数元组（用于比较）
            current_params = (
                detector_type, matcher_type, ratio_test, ransac_thresh, min_matches,
                max_show_matches, show_keypoints, show_matches,
                nfeatures, sift_octaves, sift_contrast, sift_edge,
                orb_nfeatures, orb_scale, orb_levels, orb_edge, orb_first, orb_wta, orb_patch,
                akaze_thresh, akaze_octaves, akaze_layers, akaze_diff,
                brisk_thresh, brisk_octaves, brisk_scale
            )

            # 只在参数改变时重新计算
            if current_params != last_params:
                start_time = time.time()

                print(f"\n参数已改变，重新计算... (Detector: {self.detector_types[detector_type]})")

                # 构建参数字典
                params = {
                    'nfeatures': nfeatures if detector_type == 0 else orb_nfeatures,
                    'nOctaveLayers': sift_octaves,
                    'contrastThreshold': sift_contrast,
                    'edgeThreshold': sift_edge if detector_type == 0 else orb_edge,
                    'sigma': 1.6,
                    'scaleFactor': orb_scale,
                    'nlevels': orb_levels,
                    'firstLevel': orb_first,
                    'WTA_K': orb_wta,
                    'patchSize': orb_patch,
                    'threshold': akaze_thresh,
                    'nOctaves': akaze_octaves,
                    'nOctaveLayers': akaze_layers,
                    'diffusivity': akaze_diff,
                    'thresh': brisk_thresh,
                    'octaves': brisk_octaves,
                    'patternScale': brisk_scale
                }

                # 检测模板特征
                kp_template, des_template, detector_name = self.detect_and_compute(
                    self.template, detector_type, **params)

                # 检测场景特征
                kp_scene, des_scene, _ = self.detect_and_compute(
                    self.scene, detector_type, **params)

                # 匹配特征
                matches = self.match_features(
                    des_template, des_scene, matcher_type, detector_type, ratio_test)

                # 计算单应性矩阵
                homography_matrix = None
                inliers = 0
                if len(matches) >= min_matches:
                    homography_matrix, inliers = self.compute_homography(
                        kp_template, kp_scene, matches, ransac_thresh)

                # 绘制结果
                result = self.draw_detection_result(
                    self.scene, self.template,
                    kp_scene, kp_template,
                    matches, homography_matrix,
                    detector_name,
                    show_keypoints, show_matches, max_show_matches
                )

                # 显示统计信息
                process_time = (time.time() - start_time) * 1000

                info_lines = [
                    f"Detector: {detector_name} | Matcher: {self.matcher_types[matcher_type]}",
                    f"Template KP: {len(kp_template) if kp_template else 0} | Scene KP: {len(kp_scene) if kp_scene else 0}",
                    f"Matches: {len(matches)} | Inliers: {inliers}",
                    f"Inlier Ratio: {inliers/len(matches)*100:.1f}%" if len(matches) > 0 else "Inlier Ratio: 0%",
                    f"Detection: {'Found' if homography_matrix is not None else 'Not Found'}",
                    f"Process Time: {process_time:.1f}ms"
                ]

                # 绘制信息背景
                info_height = len(info_lines) * 35 + 10
                overlay = result.copy()
                cv2.rectangle(overlay, (5, 5), (650, info_height), (0, 0, 0), -1)
                cv2.addWeighted(overlay, 0.6, result, 0.4, 0, result)

                # 绘制信息文本
                y_pos = 30
                for line in info_lines:
                    color = (0, 255, 0) if "Found" in line else (255, 255, 255)
                    cv2.putText(result, line, (10, y_pos),
                               cv2.FONT_HERSHEY_SIMPLEX, 0.6, color, 2)
                    y_pos += 35

                # 更新窗口标题
                cv2.setWindowTitle(trackbar_window, f'Trackbars - {detector_name}')
                cv2.setWindowTitle(display_window,
                                 f'Detection Result - {detector_name} | Matches: {len(matches)}')

                # 缓存结果
                cached_result = result
                cached_info = (detector_name, detector_type, matcher_type, params,
                              ratio_test, ransac_thresh, min_matches)
                last_params = current_params

                print(f"✓ 计算完成: {len(matches)} matches, {inliers} inliers, {process_time:.1f}ms")

            # 显示结果（使用缓存）
            if cached_result is not None:
                cv2.imshow(display_window, cached_result)

            # 等待按键
            key = cv2.waitKey(30) & 0xFF

            if key == ord('q') or key == 27:  # q 或 ESC
                if cached_info is not None:
                    print("\n退出并生成代码...")
                    detector_name, detector_type, matcher_type, params, ratio_test, ransac_thresh, min_matches = cached_info
                    self.generate_code(detector_type, matcher_type, params,
                                      ratio_test, ransac_thresh, min_matches)
                break
            elif key == ord('s'):  # 保存图像
                if cached_result is not None:
                    timestamp = time.strftime("%Y%m%d_%H%M%S")
                    filename = f"feature_detection_{timestamp}.png"
                    cv2.imwrite(filename, cached_result)
                    print(f"✓ 图像已保存: {filename}")

        cv2.destroyAllWindows()

    def generate_code(self, detector_type, matcher_type, params,
                     ratio_test, ransac_thresh, min_matches):
        """生成可执行的Python代码"""
        detector_name = self.detector_types[detector_type]

        code = f"""
import cv2
import numpy as np

# 读取图像
template = cv2.imread('template.png')
scene = cv2.imread('scene.png')

# 创建 {detector_name} 检测器
"""

        if detector_type == 0:  # SIFT
            code += f"""detector = cv2.SIFT_create(
    nfeatures={params['nfeatures']},
    nOctaveLayers={params['nOctaveLayers']},
    contrastThreshold={params['contrastThreshold']},
    edgeThreshold={params['edgeThreshold']},
    sigma={params['sigma']}
)
"""
        elif detector_type == 1:  # ORB
            code += f"""detector = cv2.ORB_create(
    nfeatures={params['nfeatures']},
    scaleFactor={params['scaleFactor']},
    nlevels={params['nlevels']},
    edgeThreshold={params['edgeThreshold']},
    firstLevel={params['firstLevel']},
    WTA_K={params['WTA_K']},
    patchSize={params['patchSize']}
)
"""
        elif detector_type == 2:  # AKAZE
            code += f"""detector = cv2.AKAZE_create(
    threshold={params['threshold']},
    nOctaves={params['nOctaves']},
    nOctaveLayers={params['nOctaveLayers']},
    diffusivity={params['diffusivity']}
)
"""
        elif detector_type == 3:  # BRISK
            code += f"""detector = cv2.BRISK_create(
    thresh={params['thresh']},
    octaves={params['octaves']},
    patternScale={params['patternScale']}
)
"""

        matcher_code = ""
        if matcher_type == 0:
            if detector_type in [0, 2]:
                matcher_code = "matcher = cv2.BFMatcher(cv2.NORM_L2, crossCheck=False)"
            else:
                matcher_code = "matcher = cv2.BFMatcher(cv2.NORM_HAMMING, crossCheck=False)"
        else:
            if detector_type in [0, 2]:
                matcher_code = """FLANN_INDEX_KDTREE = 1
index_params = dict(algorithm=FLANN_INDEX_KDTREE, trees=5)
search_params = dict(checks=50)
matcher = cv2.FlannBasedMatcher(index_params, search_params)"""
            else:
                matcher_code = """FLANN_INDEX_LSH = 6
index_params = dict(algorithm=FLANN_INDEX_LSH, table_number=6, key_size=12, multi_probe_level=1)
search_params = dict(checks=50)
matcher = cv2.FlannBasedMatcher(index_params, search_params)"""

        code += f"""
# 检测特征点
kp_template, des_template = detector.detectAndCompute(template, None)
kp_scene, des_scene = detector.detectAndCompute(scene, None)

# 创建匹配器
{matcher_code}

# 匹配特征
matches = matcher.knnMatch(des_template, des_scene, k=2)

# Lowe's ratio test
good_matches = []
for match in matches:
    if len(match) == 2:
        m, n = match
        if m.distance < {ratio_test} * n.distance:
            good_matches.append(m)

print(f"找到 {{len(good_matches)}} 个匹配")

# 计算单应性矩阵
if len(good_matches) >= {min_matches}:
    src_pts = np.float32([kp_template[m.queryIdx].pt for m in good_matches]).reshape(-1, 1, 2)
    dst_pts = np.float32([kp_scene[m.trainIdx].pt for m in good_matches]).reshape(-1, 1, 2)

    M, mask = cv2.findHomography(src_pts, dst_pts, cv2.RANSAC, {ransac_thresh})

    if M is not None:
        # 获取模板的四个角点
        h, w = template.shape[:2]
        corners = np.float32([[0, 0], [0, h], [w, h], [w, 0]]).reshape(-1, 1, 2)

        # 变换到场景图
        transformed_corners = cv2.perspectiveTransform(corners, M)

        # 绘制检测框
        result = scene.copy()
        cv2.polylines(result, [np.int32(transformed_corners)], True, (0, 255, 0), 3)

        # 显示结果
        cv2.imshow('Detection Result', result)
        cv2.waitKey(0)
        cv2.destroyAllWindows()

        print("✓ 检测成功！")
    else:
        print("✗ 单应性矩阵计算失败")
else:
    print(f"✗ 匹配点不足: {{len(good_matches)}}/{min_matches}")
"""

        print("\n" + "=" * 60)
        print("生成的代码:")
        print("=" * 60)
        print(code)
        print("=" * 60)

    def run(self):
        """运行调试工具"""
        print("=" * 60)
        print("特征检测调试工具 - 物体检测模式")
        print("=" * 60)

        # 获取模板图片路径
        template_path = input("\n请输入模板图片路径 (小图): ").strip().strip("'\"")
        self.template = cv2.imread(template_path)

        if self.template is None:
            print("❌ 无法读取模板图片！")
            return

        print(f"✓ 模板大小: {self.template.shape[1]}x{self.template.shape[0]}")

        # 获取场景图片路径
        scene_path = input("请输入场景图片路径 (大图): ").strip().strip("'\"")
        self.scene = cv2.imread(scene_path)

        if self.scene is None:
            print("❌ 无法读取场景图片！")
            return

        print(f"✓ 场景大小: {self.scene.shape[1]}x{self.scene.shape[0]}")

        # 开始调试
        self.debug_with_trackbars()

if __name__ == '__main__':
    debugger = FeatureDetectorDebugger()
    debugger.run()
