#!/usr/bin/env python3
"""
Player Arrow Detection Tool
Detect player arrow position and direction on game map
"""

import cv2
import numpy as np
import time
import os
import json
from datetime import datetime


class ArrowDetector:
    """Base class for arrow detectors"""
    def __init__(self, image):
        self.original_image = image.copy()
        self.image = image
        self.method_name = "Base Detector"

    def detect(self, **params):
        """Detect arrow and return results"""
        raise NotImplementedError

    def draw_results(self, image, arrow, scale=1.0):
        """Draw arrow detection results on image

        Args:
            image: Input image
            arrow: Detected arrow dict
            scale: Scale factor for annotations
        """
        if arrow is None or not arrow.get('found', False):
            return image

        result = image.copy()

        x = arrow['position']['x']
        y = arrow['position']['y']
        angle = arrow['direction']['angle']
        cardinal = arrow['direction']['cardinal']

        # Draw crosshair at arrow position
        color_pos = (0, 255, 0)  # Green
        size = max(10, int(15 * scale))
        thickness = max(2, int(3 * scale))

        cv2.line(result, (x - size, y), (x + size, y), color_pos, thickness)
        cv2.line(result, (x, y - size), (x, y + size), color_pos, thickness)
        cv2.circle(result, (x, y), max(5, int(8 * scale)), color_pos, thickness)

        # Draw direction arrow
        color_dir = (0, 0, 255)  # Red
        arrow_len = max(30, int(50 * scale))

        # Convert game angle (0=up) to radians
        rad = np.radians(angle)
        end_x = int(x + arrow_len * np.sin(rad))
        end_y = int(y - arrow_len * np.cos(rad))  # Y axis inverted

        cv2.arrowedLine(result, (x, y), (end_x, end_y), color_dir,
                       max(2, int(3 * scale)), tipLength=0.3)

        # Draw angle text
        font_scale = 0.5 * scale
        text_thickness = max(1, int(2 * scale))

        angle_text = f"{angle:.1f}deg ({cardinal})"
        (text_w, text_h), _ = cv2.getTextSize(angle_text, cv2.FONT_HERSHEY_SIMPLEX,
                                              font_scale, text_thickness)

        # Position text below the arrow
        text_x = x - text_w // 2
        text_y = y + size + text_h + 10

        # Ensure text is within image bounds
        if text_y > result.shape[0] - 5:
            text_y = y - size - 10

        # Draw text background
        cv2.rectangle(result, (text_x - 5, text_y - text_h - 5),
                     (text_x + text_w + 5, text_y + 5), (0, 0, 0), -1)

        # Draw text
        cv2.putText(result, angle_text, (text_x, text_y),
                   cv2.FONT_HERSHEY_SIMPLEX, font_scale, (255, 255, 255),
                   text_thickness, cv2.LINE_AA)

        # Draw position text
        pos_text = f"({x}, {y})"
        (pos_w, pos_h), _ = cv2.getTextSize(pos_text, cv2.FONT_HERSHEY_SIMPLEX,
                                            font_scale, text_thickness)

        pos_y = text_y + pos_h + 15
        pos_x = x - pos_w // 2

        cv2.rectangle(result, (pos_x - 5, pos_y - pos_h - 5),
                     (pos_x + pos_w + 5, pos_y + 5), (0, 0, 0), -1)

        cv2.putText(result, pos_text, (pos_x, pos_y),
                   cv2.FONT_HERSHEY_SIMPLEX, font_scale, (255, 255, 255),
                   text_thickness, cv2.LINE_AA)

        return result

    def get_trackbar_names(self):
        """Return list of trackbar parameters"""
        return []

    @staticmethod
    def angle_to_cardinal(angle):
        """Convert angle (0-360) to cardinal direction

        Args:
            angle: Angle in degrees (0=North, 90=East, 180=South, 270=West)

        Returns:
            Cardinal direction string (N, NE, E, SE, S, SW, W, NW)
        """
        directions = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
        index = int((angle + 22.5) / 45) % 8
        return directions[index]


class HSVArrowDetector(ArrowDetector):
    """Method 1: HSV Color Detection + Contour Analysis"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 1: HSV Color Detection"

    def detect(self, **params):
        """Detect white/gray arrow using HSV color space

        Parameters:
            h_min, h_max: Hue range (usually 0-180 for white)
            s_min, s_max: Saturation range (low for white, 0-50)
            v_min, v_max: Value/Brightness range (high for white, 150-255)
            morph_open: Opening kernel size
            morph_close: Closing kernel size
            min_area: Minimum area
            max_area: Maximum area
            direction_method: 0=MinAreaRect, 1=ConvexHull, 2=PCA
        """
        h_min = params.get('h_min', 0)
        h_max = params.get('h_max', 180)
        s_min = params.get('s_min', 0)
        s_max = params.get('s_max', 50)
        v_min = params.get('v_min', 150)
        v_max = params.get('v_max', 255)
        morph_open = params.get('morph_open', 2)
        morph_close = params.get('morph_close', 3)
        min_area = params.get('min_area', 200)
        max_area = params.get('max_area', 5000)
        direction_method = params.get('direction_method', 1)

        steps = {}

        # Convert to HSV
        if len(self.image.shape) == 3:
            hsv = cv2.cvtColor(self.image, cv2.COLOR_BGR2HSV)
        else:
            hsv = cv2.cvtColor(self.image, cv2.COLOR_GRAY2BGR)
            hsv = cv2.cvtColor(hsv, cv2.COLOR_BGR2HSV)

        steps['hsv'] = hsv

        # Create mask for white/gray color
        lower = np.array([h_min, s_min, v_min])
        upper = np.array([h_max, s_max, v_max])
        mask = cv2.inRange(hsv, lower, upper)

        steps['mask'] = mask

        # Morphological operations
        if morph_open > 0:
            kernel_open = cv2.getStructuringElement(cv2.MORPH_ELLIPSE,
                                                    (morph_open*2+1, morph_open*2+1))
            mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel_open)

        if morph_close > 0:
            kernel_close = cv2.getStructuringElement(cv2.MORPH_ELLIPSE,
                                                     (morph_close*2+1, morph_close*2+1))
            mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel_close)

        steps['morph'] = mask

        # Find contours
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        # Find best arrow candidate
        best_arrow = None
        max_score = 0

        for contour in contours:
            area = cv2.contourArea(contour)

            # Filter by area
            if area < min_area or area > max_area:
                continue

            # Get bounding rectangle
            x, y, w, h = cv2.boundingRect(contour)

            # Calculate center position
            M = cv2.moments(contour)
            if M['m00'] == 0:
                continue

            cx = int(M['m10'] / M['m00'])
            cy = int(M['m01'] / M['m00'])

            # Calculate direction based on method
            angle = self._calculate_direction(contour, direction_method)

            if angle is None:
                continue

            # Convert to 0-360 range
            if angle < 0:
                angle += 360

            # Score based on area (prefer larger arrows)
            score = area

            if score > max_score:
                max_score = score
                best_arrow = {
                    'found': True,
                    'position': {'x': cx, 'y': cy},
                    'direction': {
                        'angle': angle,
                        'cardinal': self.angle_to_cardinal(angle),
                        'method': ['MinAreaRect', 'ConvexHull', 'PCA'][direction_method]
                    },
                    'properties': {
                        'area': int(area),
                        'width': w,
                        'height': h,
                        'aspect_ratio': w / h if h > 0 else 0,
                        'confidence': min(1.0, score / 2000)
                    },
                    'contour': contour
                }

        if best_arrow is None:
            best_arrow = {'found': False}

        return best_arrow, steps

    def _calculate_direction(self, contour, method):
        """Calculate arrow direction using specified method

        Args:
            contour: OpenCV contour
            method: 0=MinAreaRect, 1=ConvexHull, 2=PCA

        Returns:
            Angle in degrees (0=North, 90=East, etc.) or None
        """
        if method == 0:
            return self._direction_min_area_rect(contour)
        elif method == 1:
            return self._direction_convex_hull(contour)
        elif method == 2:
            return self._direction_pca(contour)
        return None

    def _direction_min_area_rect(self, contour):
        """Calculate direction using minimum area rectangle

        Note: Has 180-degree ambiguity
        """
        rect = cv2.minAreaRect(contour)
        angle = rect[2]

        # Adjust angle based on rectangle orientation
        w, h = rect[1]
        if w < h:
            angle = angle + 90

        # Convert to game coordinate system (0=North)
        game_angle = (90 - angle) % 360

        return game_angle

    def _direction_convex_hull(self, contour):
        """Calculate direction using convex hull tip detection

        More accurate - finds arrow tip point
        """
        # Calculate contour centroid
        M = cv2.moments(contour)
        if M['m00'] == 0:
            return None

        cx = M['m10'] / M['m00']
        cy = M['m01'] / M['m00']

        # Get convex hull with indices
        hull = cv2.convexHull(contour, returnPoints=False)

        if hull is None or len(hull) < 3:
            return None

        # Get defects to find the sharpest point (arrow tip)
        try:
            defects = cv2.convexityDefects(contour, hull)
        except:
            # Fallback to simple furthest point method
            return self._direction_furthest_point(contour, cx, cy)

        # Find the point with minimum angle (sharpest corner = arrow tip)
        min_angle = float('inf')
        tip_point = None

        hull_points = []
        for i in range(len(hull)):
            idx = hull[i][0]
            hull_points.append(tuple(contour[idx][0]))

        # Calculate angle at each hull point
        for i in range(len(hull_points)):
            p1 = hull_points[i - 1]
            p2 = hull_points[i]
            p3 = hull_points[(i + 1) % len(hull_points)]

            # Vectors
            v1 = np.array([p1[0] - p2[0], p1[1] - p2[1]])
            v2 = np.array([p3[0] - p2[0], p3[1] - p2[1]])

            # Calculate angle between vectors
            v1_norm = np.linalg.norm(v1)
            v2_norm = np.linalg.norm(v2)

            if v1_norm == 0 or v2_norm == 0:
                continue

            cos_angle = np.dot(v1, v2) / (v1_norm * v2_norm)
            cos_angle = np.clip(cos_angle, -1.0, 1.0)
            angle = np.degrees(np.arccos(cos_angle))

            # The arrow tip should have the smallest angle (sharpest corner)
            if angle < min_angle:
                min_angle = angle
                tip_point = p2

        # If no sharp corner found, use furthest point
        if tip_point is None or min_angle > 120:
            return self._direction_furthest_point(contour, cx, cy)

        # Calculate angle from centroid to tip
        dx = tip_point[0] - cx
        dy = tip_point[1] - cy

        # Math angle (0=East, counter-clockwise)
        math_angle = np.degrees(np.arctan2(dy, dx))

        # Convert to game angle (0=North, clockwise)
        game_angle = (90 - math_angle) % 360

        return game_angle

    def _direction_furthest_point(self, contour, cx, cy):
        """Fallback method: find furthest point from centroid"""
        hull = cv2.convexHull(contour)

        max_dist = 0
        tip_point = None

        for point in hull:
            px, py = point[0]
            dist = np.sqrt((px - cx)**2 + (py - cy)**2)

            if dist > max_dist:
                max_dist = dist
                tip_point = (px, py)

        if tip_point is None:
            return None

        # Calculate angle from centroid to tip
        dx = tip_point[0] - cx
        dy = tip_point[1] - cy

        # Math angle (0=East, counter-clockwise)
        math_angle = np.degrees(np.arctan2(dy, dx))

        # Convert to game angle (0=North, clockwise)
        game_angle = (90 - math_angle) % 360

        return game_angle

    def _direction_pca(self, contour):
        """Calculate direction using PCA (Principal Component Analysis)

        Finds main axis direction
        """
        # Get all points
        points = contour.reshape(-1, 2).astype(np.float32)

        # Calculate mean
        mean = np.mean(points, axis=0)

        # Calculate covariance matrix
        cov = np.cov(points.T)

        # Get eigenvalues and eigenvectors
        eigenvalues, eigenvectors = np.linalg.eig(cov)

        # Principal axis is eigenvector with largest eigenvalue
        main_axis = eigenvectors[:, np.argmax(eigenvalues)]

        # Calculate angle
        math_angle = np.degrees(np.arctan2(main_axis[1], main_axis[0]))

        # Convert to game angle (0=North, clockwise)
        game_angle = (90 - math_angle) % 360

        # Note: PCA has 180-degree ambiguity
        # May need additional logic to determine correct direction

        return game_angle

    def get_trackbar_names(self):
        return [
            ('H_min', 0, 180, 'Hue minimum'),
            ('H_max', 180, 180, 'Hue maximum'),
            ('S_min', 0, 255, 'Saturation minimum (low for white)'),
            ('S_max', 50, 255, 'Saturation maximum'),
            ('V_min', 150, 255, 'Value/Brightness minimum (high for white)'),
            ('V_max', 255, 255, 'Value/Brightness maximum'),
            ('Morph_Open', 2, 10, 'Opening: remove noise'),
            ('Morph_Close', 3, 10, 'Closing: fill holes'),
            ('Min_Area', 200, 5000, 'Minimum arrow area'),
            ('Max_Area', 5000, 10000, 'Maximum arrow area'),
            ('Direction_Method', 1, 2, '0=MinRect 1=ConvexHull 2=PCA')
        ]


class TemplateArrowDetector(ArrowDetector):
    """Method 2: Template Matching with Rotation"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 2: Template Matching"
        self.template = None

    def load_template(self, template_path):
        """Load arrow template image"""
        if not os.path.exists(template_path):
            print(f"Template not found: {template_path}")
            return False

        self.template = cv2.imread(template_path, cv2.IMREAD_UNCHANGED)
        if self.template is None:
            print(f"Failed to load template: {template_path}")
            return False

        h, w = self.template.shape[:2]
        print(f"Loaded template: {w}x{h}")
        return True

    def detect(self, **params):
        """Detect using template matching with rotation

        Parameters:
            threshold: Matching threshold (0-100)
            angle_start: Starting angle (0-360)
            angle_end: Ending angle (0-360)
            angle_step: Angle step (1-45)
            scale_min: Minimum scale (50-150%)
            scale_max: Maximum scale (50-150%)
        """
        if self.template is None:
            return {'found': False}, {}

        threshold = params.get('threshold', 70) / 100.0
        angle_start = params.get('angle_start', 0)
        angle_end = params.get('angle_end', 360)
        angle_step = params.get('angle_step', 15)
        scale_min = params.get('scale_min', 80) / 100.0
        scale_max = params.get('scale_max', 120) / 100.0

        steps = {}

        # Convert to grayscale
        if len(self.image.shape) == 3:
            gray = cv2.cvtColor(self.image, cv2.COLOR_BGR2GRAY)
        else:
            gray = self.image

        if len(self.template.shape) == 3:
            template_gray = cv2.cvtColor(self.template, cv2.COLOR_BGR2GRAY)
        else:
            template_gray = self.template

        steps['gray'] = gray

        best_match = None
        best_score = threshold

        # Multi-angle, multi-scale matching
        th, tw = template_gray.shape[:2]
        center = (tw // 2, th // 2)

        for angle in range(angle_start, angle_end, angle_step):
            # Rotate template
            M_rot = cv2.getRotationMatrix2D(center, angle, 1.0)
            rotated = cv2.warpAffine(template_gray, M_rot, (tw, th),
                                     flags=cv2.INTER_LINEAR,
                                     borderMode=cv2.BORDER_CONSTANT,
                                     borderValue=0)

            for scale in np.arange(scale_min, scale_max + 0.1, 0.1):
                new_w = int(tw * scale)
                new_h = int(th * scale)

                if new_w < 5 or new_h < 5 or new_w > gray.shape[1] or new_h > gray.shape[0]:
                    continue

                # Resize template
                resized = cv2.resize(rotated, (new_w, new_h))

                # Match template
                result = cv2.matchTemplate(gray, resized, cv2.TM_CCOEFF_NORMED)

                min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)

                if max_val > best_score:
                    best_score = max_val
                    best_match = {
                        'found': True,
                        'position': {
                            'x': max_loc[0] + new_w // 2,
                            'y': max_loc[1] + new_h // 2
                        },
                        'direction': {
                            'angle': float(angle),
                            'cardinal': self.angle_to_cardinal(angle),
                            'method': 'Template'
                        },
                        'properties': {
                            'score': float(max_val),
                            'scale': float(scale),
                            'width': new_w,
                            'height': new_h,
                            'confidence': float(max_val)
                        }
                    }

        if best_match is None:
            best_match = {'found': False}

        steps['matches'] = 1 if best_match['found'] else 0

        return best_match, steps

    def get_trackbar_names(self):
        return [
            ('Threshold', 70, 100, 'Match threshold (higher=stricter)'),
            ('Angle_Start', 0, 360, 'Start angle (degrees)'),
            ('Angle_End', 360, 360, 'End angle (degrees)'),
            ('Angle_Step', 15, 45, 'Angle step (degrees)'),
            ('Scale_Min', 80, 150, 'Minimum scale (%)'),
            ('Scale_Max', 120, 150, 'Maximum scale (%)')
        ]


class ArrowDetectionApp:
    """Main application for arrow detection"""
    def __init__(self):
        self.original_image = None
        self.detector = None
        self.method_choice = None

    def run(self):
        """Main entry point"""
        print("=" * 60)
        print("Player Arrow Detection Tool")
        print("=" * 60)

        # Load image
        image_path = input("\nEnter map image path: ").strip().strip("'\"")
        if not self.load_image(image_path):
            return

        # Choose detection method
        self.method_choice = self.choose_method()

        # Create detector
        self.detector = self.create_detector(self.method_choice, self.original_image)
        if self.detector is None:
            return

        # Run detection with trackbars
        self.run_detector_with_trackbars()

    def load_image(self, path):
        """Load map image"""
        if not os.path.exists(path):
            print(f"Image not found: {path}")
            return False

        self.original_image = cv2.imread(path)
        if self.original_image is None:
            print(f"Failed to load image: {path}")
            return False

        h, w = self.original_image.shape[:2]
        print(f"✓ Image loaded: {w}x{h}")
        return True

    def choose_method(self):
        """Choose detection method"""
        print("\n" + "=" * 60)
        print("Select Detection Method:")
        print("=" * 60)
        print("1. HSV Color Detection (Recommended)")
        print("   - Best for white/gray arrow")
        print("   - Fast and accurate")
        print("")
        print("2. Template Matching")
        print("   - Uses arrow image as template")
        print("   - Accurate direction detection")
        print("=" * 60)

        while True:
            choice = input("\nEnter method number (1/2): ").strip()
            if choice in ['1', '2']:
                return int(choice)
            print("Invalid input, please enter 1 or 2")

    def create_detector(self, method, image):
        """Create detector based on method"""
        if method == 1:
            return HSVArrowDetector(image)
        elif method == 2:
            detector = TemplateArrowDetector(image)
            template_path = input("\nEnter arrow template path: ").strip().strip("'\"")
            if not detector.load_template(template_path):
                print("Failed to load template")
                return None
            return detector
        return None

    def run_detector_with_trackbars(self):
        """Run detector with interactive trackbars"""
        trackbar_window = f'{self.detector.method_name} - Controls'
        display_window = f'{self.detector.method_name} - Detection Result'
        large_window = f'{self.detector.method_name} - Large View'

        cv2.namedWindow(trackbar_window)
        cv2.namedWindow(display_window, cv2.WINDOW_NORMAL)
        cv2.resizeWindow(display_window, 1200, 800)
        cv2.namedWindow(large_window, cv2.WINDOW_NORMAL)
        cv2.resizeWindow(large_window, 800, 800)

        h, w = self.original_image.shape[:2]

        # Create ROI trackbars
        cv2.createTrackbar('ROI_X', trackbar_window, 0, w-1, lambda _: None)
        cv2.createTrackbar('ROI_Y', trackbar_window, 0, h-1, lambda _: None)
        cv2.createTrackbar('ROI_Width', trackbar_window, w, w, lambda _: None)
        cv2.createTrackbar('ROI_Height', trackbar_window, h, h, lambda _: None)

        # Create detection method trackbars
        trackbars = self.detector.get_trackbar_names()
        for name, default, max_val, description in trackbars:
            cv2.createTrackbar(name, trackbar_window, default, max_val, lambda _: None)
            print(f"  {name}: {description}")

        print("\n" + "=" * 60)
        print(f"{self.detector.method_name} - Running")
        print("=" * 60)
        print("Hotkeys:")
        print("  q / ESC - Exit")
        print("  s       - Save result and coordinates")
        print("  r       - Reset ROI to full image")
        print("=" * 60)

        last_params = None
        cached_display = None
        cached_large = None
        cached_arrow = None

        while True:
            # Read ROI parameters
            roi_x = cv2.getTrackbarPos('ROI_X', trackbar_window)
            roi_y = cv2.getTrackbarPos('ROI_Y', trackbar_window)
            roi_w = cv2.getTrackbarPos('ROI_Width', trackbar_window)
            roi_h = cv2.getTrackbarPos('ROI_Height', trackbar_window)

            # Validate ROI
            roi_x = max(0, min(roi_x, w - 10))
            roi_y = max(0, min(roi_y, h - 10))
            roi_w = max(10, min(roi_w, w - roi_x))
            roi_h = max(10, min(roi_h, h - roi_y))

            # Read detection parameters
            params = {}
            for name, default, max_val, _ in trackbars:
                param_name = name.lower().replace(' ', '_')
                params[param_name] = cv2.getTrackbarPos(name, trackbar_window)

            # Only reprocess if parameters changed
            current_params = str(params) + str((roi_x, roi_y, roi_w, roi_h))
            if current_params != last_params:
                start_time = time.time()

                # Extract ROI
                roi_image = self.original_image[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()
                self.detector.image = roi_image

                # Detect arrow
                arrow, steps = self.detector.detect(**params)

                # Adjust coordinates to original image space
                if arrow.get('found', False):
                    arrow['position']['x'] += roi_x
                    arrow['position']['y'] += roi_y

                # Draw results
                if len(self.original_image.shape) == 2:
                    result_img = cv2.cvtColor(self.original_image, cv2.COLOR_GRAY2BGR)
                else:
                    result_img = self.original_image.copy()

                result = self.detector.draw_results(result_img, arrow)

                # Draw ROI rectangle
                cv2.rectangle(result, (roi_x, roi_y), (roi_x + roi_w, roi_y + roi_h),
                             (0, 255, 0), 2)

                process_time = (time.time() - start_time) * 1000

                # Create display
                display = self._create_display(steps, result, arrow, process_time,
                                               roi_x, roi_y, roi_w, roi_h)
                cached_display = display

                # Create large view
                target_size = 800
                roi_scale = max(target_size / roi_w, target_size / roi_h, 1.0)
                new_roi_w = int(roi_w * roi_scale)
                new_roi_h = int(roi_h * roi_scale)

                roi_enlarged = cv2.resize(roi_image, (new_roi_w, new_roi_h),
                                         interpolation=cv2.INTER_LINEAR)

                if len(roi_enlarged.shape) == 2:
                    roi_enlarged = cv2.cvtColor(roi_enlarged, cv2.COLOR_GRAY2BGR)

                # Scale arrow to ROI coordinate system
                if arrow.get('found', False):
                    arrow_scaled = arrow.copy()
                    arrow_scaled['position']['x'] = int((arrow['position']['x'] - roi_x) * roi_scale)
                    arrow_scaled['position']['y'] = int((arrow['position']['y'] - roi_y) * roi_scale)
                    large_view = self.detector.draw_results(roi_enlarged, arrow_scaled, scale=1.0)
                else:
                    large_view = roi_enlarged

                cached_large = large_view
                cached_arrow = arrow
                last_params = current_params

                # Print results
                print(f"\n{'='*60}")
                print(f"Arrow Detection Result")
                print(f"{'='*60}")

                if arrow.get('found', False):
                    pos = arrow['position']
                    direction = arrow['direction']
                    props = arrow.get('properties', {})

                    print(f"✓ Arrow found at position: ({pos['x']}, {pos['y']})")
                    print(f"✓ Direction angle: {direction['angle']:.1f}°")
                    print(f"✓ Cardinal direction: {direction['cardinal']}")
                    print(f"✓ Detection method: {direction.get('method', 'Unknown')}")
                    print(f"✓ Confidence: {props.get('confidence', 0):.2f}")
                    print(f"{'='*60}")
                    print(f"Details:")
                    print(f"  - Area: {props.get('area', 0)} pixels")
                    print(f"  - Width x Height: {props.get('width', 0)} x {props.get('height', 0)}")
                    print(f"  - Aspect Ratio: {props.get('aspect_ratio', 0):.2f}")
                    print(f"  - Processing time: {process_time:.1f}ms")
                    print(f"{'='*60}")
                    print(f"✓ Check '{large_window}' for enlarged view")
                else:
                    print("✗ Arrow not detected. Try adjusting parameters:")
                    if self.method_choice == 1:
                        print("  - Adjust S_max (saturation) for white arrow")
                        print("  - Adjust V_min (brightness) threshold")
                        print("  - Decrease Min_Area")
                        print("  - Try different Direction_Method (0/1/2)")
                    else:
                        print("  - Decrease Threshold")
                        print("  - Adjust angle range")
                        print("  - Adjust scale range")
                print(f"{'='*60}\n")

            # Display results
            if cached_display is not None:
                cv2.imshow(display_window, cached_display)

            if cached_large is not None:
                cv2.imshow(large_window, cached_large)

            # Key handling
            key = cv2.waitKey(30) & 0xFF
            if key == ord('q') or key == 27:
                print("\n\nExiting...")
                break
            elif key == ord('s'):
                if cached_display is not None:
                    self._save_results(cached_display, cached_large, cached_arrow)
            elif key == ord('r'):
                # Reset ROI
                cv2.setTrackbarPos('ROI_X', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Y', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Width', trackbar_window, w)
                cv2.setTrackbarPos('ROI_Height', trackbar_window, h)
                print("\n✓ ROI reset to full image")

        cv2.destroyAllWindows()

    def _create_display(self, steps, result, arrow, process_time,
                       roi_x, roi_y, roi_w, roi_h):
        """Create 2x2 display grid"""
        target_w, target_h = 600, 450

        step_images = []
        step_titles = []

        # 1. Original with ROI
        original_roi = self.original_image.copy()
        if len(original_roi.shape) == 2:
            original_roi = cv2.cvtColor(original_roi, cv2.COLOR_GRAY2BGR)
        cv2.rectangle(original_roi, (roi_x, roi_y), (roi_x + roi_w, roi_y + roi_h),
                     (0, 255, 0), 3)
        step_images.append(original_roi)
        step_titles.append("1. Original + ROI")

        # 2. HSV or Gray
        if 'hsv' in steps:
            hsv_vis = steps['hsv']
            step_images.append(hsv_vis)
            step_titles.append("2. HSV Image")
        elif 'gray' in steps:
            gray_bgr = cv2.cvtColor(steps['gray'], cv2.COLOR_GRAY2BGR)
            step_images.append(gray_bgr)
            step_titles.append("2. Grayscale")
        else:
            step_images.append(original_roi.copy())
            step_titles.append("2. Processing")

        # 3. Mask or intermediate
        if 'mask' in steps:
            mask_bgr = cv2.cvtColor(steps['mask'], cv2.COLOR_GRAY2BGR)
            step_images.append(mask_bgr)
            step_titles.append("3. Color Mask")
        elif 'morph' in steps:
            morph_bgr = cv2.cvtColor(steps['morph'], cv2.COLOR_GRAY2BGR)
            step_images.append(morph_bgr)
            step_titles.append("3. Morphology")
        else:
            step_images.append(step_images[-1].copy())
            step_titles.append("3. Intermediate")

        # 4. Final result
        found_text = "Found" if arrow.get('found', False) else "Not Found"
        step_images.append(result)
        step_titles.append(f"4. Result ({found_text})")

        # Resize and combine
        display_images = []
        for img, title in zip(step_images, step_titles):
            img_resized = cv2.resize(img, (target_w, target_h))

            # Add title
            cv2.rectangle(img_resized, (0, 0), (target_w, 35), (0, 0, 0), -1)
            cv2.putText(img_resized, title, (10, 28),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.8, (255, 255, 255), 2)

            display_images.append(img_resized)

        # Create 2x2 grid
        top_row = np.hstack(display_images[0:2])
        bottom_row = np.hstack(display_images[2:4])
        display = np.vstack([top_row, bottom_row])

        # Add info text
        info_y = display.shape[0] - 150
        info_lines = [
            f"Method: {self.detector.method_name}",
            f"Processing Time: {process_time:.1f}ms",
            f"Arrow: {found_text}",
        ]

        if arrow.get('found', False):
            info_lines.append(f"Position: ({arrow['position']['x']}, {arrow['position']['y']})")
            info_lines.append(f"Direction: {arrow['direction']['angle']:.1f}° ({arrow['direction']['cardinal']})")
        else:
            info_lines.append("")

        info_lines.append("Hotkeys: q-Exit  s-Save  r-Reset ROI")

        y_offset = info_y
        for line in info_lines:
            (text_w, text_h), _ = cv2.getTextSize(line, cv2.FONT_HERSHEY_SIMPLEX, 0.6, 2)
            overlay = display.copy()
            cv2.rectangle(overlay, (10, y_offset - text_h - 5),
                         (20 + text_w, y_offset + 5), (0, 0, 0), -1)
            cv2.addWeighted(overlay, 0.7, display, 0.3, 0, display)

            cv2.putText(display, line, (10, y_offset),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 2)
            y_offset += text_h + 10

        return display

    def _save_results(self, display, large_view, arrow):
        """Save detection results"""
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        # Save display image
        display_file = f"arrow_detection_{timestamp}.png"
        cv2.imwrite(display_file, display)
        print(f"\n✓ Display saved: {display_file}")

        # Save large view
        large_file = f"arrow_large_view_{timestamp}.png"
        cv2.imwrite(large_file, large_view)
        print(f"✓ Large view saved: {large_file}")

        # Save arrow data as JSON
        result_file = f"arrow_result_{timestamp}.json"

        # Create serializable copy
        save_data = {
            'timestamp': timestamp,
            'arrow': {
                'found': arrow.get('found', False)
            }
        }

        if arrow.get('found', False):
            save_data['arrow'].update({
                'position': arrow['position'],
                'direction': arrow['direction'],
                'properties': {
                    k: v for k, v in arrow.get('properties', {}).items()
                    if not isinstance(v, np.ndarray)
                }
            })

        with open(result_file, 'w') as f:
            json.dump(save_data, f, indent=2)

        print(f"✓ Result data saved: {result_file}")

        if arrow.get('found', False):
            print(f"✓ Arrow at ({arrow['position']['x']}, {arrow['position']['y']})")
            print(f"✓ Direction: {arrow['direction']['angle']:.1f}° ({arrow['direction']['cardinal']})\n")
        else:
            print("✓ No arrow detected\n")


if __name__ == '__main__':
    app = ArrowDetectionApp()
    app.run()
