#!/usr/bin/env python3
"""
Map Monster Detection Tool
Detect monster positions (yellow dots) on game map
"""

import cv2
import numpy as np
import time
import os
import json
from datetime import datetime


class MonsterDetector:
    """Base class for monster detectors"""
    def __init__(self, image):
        self.original_image = image.copy()
        self.image = image
        self.method_name = "Base Detector"

    def detect(self, **params):
        """Detect monsters and return results"""
        raise NotImplementedError

    def draw_results(self, image, monsters, scale=1.0):
        """Draw detection results on image

        Args:
            image: Input image
            monsters: List of detected monsters
            scale: Scale factor for annotations
        """
        result = image.copy()

        for monster in monsters:
            x, y = monster.get('x', 0), monster.get('y', 0)
            area = monster.get('area', 0)
            monster_id = monster.get('id', 0)

            # Draw marker at position
            color = (0, 255, 255)  # Yellow in BGR

            # Draw crosshair
            size = max(5, int(8 * scale))
            thickness = max(1, int(2 * scale))
            cv2.line(result, (x - size, y), (x + size, y), color, thickness)
            cv2.line(result, (x, y - size), (x, y + size), color, thickness)

            # Draw circle around point
            radius = max(8, int(12 * scale))
            cv2.circle(result, (x, y), radius, color, max(1, int(2 * scale)))

            # Draw ID number
            font_scale = 0.35 * scale
            id_thickness = max(1, int(1 * scale))
            id_str = str(monster_id)
            (id_w, id_h), _ = cv2.getTextSize(id_str, cv2.FONT_HERSHEY_SIMPLEX,
                                              font_scale, id_thickness)

            # Draw ID above the point
            text_x = x - id_w // 2
            text_y = y - radius - 5
            if text_y < 15:
                text_y = y + radius + id_h + 5

            cv2.putText(result, id_str, (text_x, text_y),
                       cv2.FONT_HERSHEY_SIMPLEX, font_scale, (255, 255, 255),
                       id_thickness + 1, cv2.LINE_AA)
            cv2.putText(result, id_str, (text_x, text_y),
                       cv2.FONT_HERSHEY_SIMPLEX, font_scale, color,
                       id_thickness, cv2.LINE_AA)

        return result

    def get_trackbar_names(self):
        """Return list of trackbar parameters"""
        return []


class HSVMonsterDetector(MonsterDetector):
    """Method 1: HSV Color Detection - Best for yellow dots"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 1: HSV Color Detection"

    def detect(self, **params):
        """Detect yellow monsters using HSV color space

        Parameters:
            h_min, h_max: Hue range (0-180)
            s_min, s_max: Saturation range (0-255)
            v_min, v_max: Value/Brightness range (0-255)
            morph_open: Opening kernel size (remove noise)
            morph_close: Closing kernel size (fill holes)
            min_area: Minimum area
            max_area: Maximum area
            circularity: Circularity threshold (0-100)
        """
        h_min = params.get('h_min', 20)
        h_max = params.get('h_max', 35)
        s_min = params.get('s_min', 100)
        s_max = params.get('s_max', 255)
        v_min = params.get('v_min', 100)
        v_max = params.get('v_max', 255)
        morph_open = params.get('morph_open', 2)
        morph_close = params.get('morph_close', 3)
        min_area = params.get('min_area', 20)
        max_area = params.get('max_area', 500)
        circularity = params.get('circularity', 50) / 100.0

        steps = {}

        # Convert to HSV
        if len(self.image.shape) == 3:
            hsv = cv2.cvtColor(self.image, cv2.COLOR_BGR2HSV)
        else:
            hsv = cv2.cvtColor(self.image, cv2.COLOR_GRAY2BGR)
            hsv = cv2.cvtColor(hsv, cv2.COLOR_BGR2HSV)

        steps['hsv'] = hsv

        # Create mask for yellow color
        lower = np.array([h_min, s_min, v_min])
        upper = np.array([h_max, s_max, v_max])
        mask = cv2.inRange(hsv, lower, upper)

        steps['mask'] = mask

        # Morphological operations to remove noise
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

        # Filter and extract monster positions
        monsters = []
        monster_id = 1

        for contour in contours:
            area = cv2.contourArea(contour)

            # Filter by area
            if area < min_area or area > max_area:
                continue

            # Calculate circularity: 4*pi*area / perimeter^2
            perimeter = cv2.arcLength(contour, True)
            if perimeter == 0:
                continue

            circ = 4 * np.pi * area / (perimeter * perimeter)

            # Filter by circularity
            if circ < circularity:
                continue

            # Get center position
            M = cv2.moments(contour)
            if M['m00'] == 0:
                continue

            cx = int(M['m10'] / M['m00'])
            cy = int(M['m01'] / M['m00'])

            monsters.append({
                'id': monster_id,
                'x': cx,
                'y': cy,
                'area': int(area),
                'circularity': float(circ),
                'contour': contour
            })

            monster_id += 1

        return monsters, steps

    def get_trackbar_names(self):
        return [
            ('H_min', 20, 180, 'Hue minimum (Yellow: 20-35)'),
            ('H_max', 35, 180, 'Hue maximum'),
            ('S_min', 100, 255, 'Saturation minimum (High: 100-255)'),
            ('S_max', 255, 255, 'Saturation maximum'),
            ('V_min', 100, 255, 'Value/Brightness minimum'),
            ('V_max', 255, 255, 'Value/Brightness maximum'),
            ('Morph_Open', 2, 10, 'Opening: remove noise'),
            ('Morph_Close', 3, 10, 'Closing: fill holes'),
            ('Min_Area', 20, 1000, 'Minimum area'),
            ('Max_Area', 500, 2000, 'Maximum area'),
            ('Circularity', 50, 100, 'Circularity threshold (0-100)')
        ]


class TemplateMonsterDetector(MonsterDetector):
    """Method 2: Template Matching"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 2: Template Matching"
        self.template = None

    def load_template(self, template_path):
        """Load template image"""
        if not os.path.exists(template_path):
            print(f"Template not found: {template_path}")
            return False

        self.template = cv2.imread(template_path)
        if self.template is None:
            print(f"Failed to load template: {template_path}")
            return False

        h, w = self.template.shape[:2]
        print(f"Loaded template: {w}x{h}")
        return True

    def detect(self, **params):
        """Detect using template matching

        Parameters:
            threshold: Matching threshold (0-100)
            scale_start: Starting scale (50-150%)
            scale_end: Ending scale (50-150%)
            scale_step: Scale step (1-20%)
            nms_threshold: NMS threshold (0-100)
        """
        if self.template is None:
            return [], {}

        threshold = params.get('threshold', 70) / 100.0
        scale_start = params.get('scale_start', 80) / 100.0
        scale_end = params.get('scale_end', 120) / 100.0
        scale_step = params.get('scale_step', 10) / 100.0
        nms_threshold = params.get('nms_threshold', 30) / 100.0

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

        # Multi-scale template matching
        all_matches = []
        th, tw = template_gray.shape[:2]

        scale = scale_start
        while scale <= scale_end:
            # Resize template
            new_w = int(tw * scale)
            new_h = int(th * scale)

            if new_w < 5 or new_h < 5 or new_w > gray.shape[1] or new_h > gray.shape[0]:
                scale += scale_step
                continue

            resized_template = cv2.resize(template_gray, (new_w, new_h))

            # Template matching
            result = cv2.matchTemplate(gray, resized_template, cv2.TM_CCOEFF_NORMED)

            # Find matches above threshold
            locations = np.where(result >= threshold)

            for pt in zip(*locations[::-1]):
                all_matches.append({
                    'x': pt[0] + new_w // 2,
                    'y': pt[1] + new_h // 2,
                    'w': new_w,
                    'h': new_h,
                    'score': result[pt[1], pt[0]],
                    'scale': scale
                })

            scale += scale_step

        steps['matches'] = len(all_matches)

        # Non-maximum suppression
        monsters = self._nms(all_matches, nms_threshold)

        # Assign IDs
        for i, m in enumerate(monsters):
            m['id'] = i + 1
            m['area'] = m['w'] * m['h']

        return monsters, steps

    def _nms(self, matches, threshold):
        """Non-maximum suppression"""
        if not matches:
            return []

        # Sort by score
        matches = sorted(matches, key=lambda x: x['score'], reverse=True)

        kept = []
        while matches:
            current = matches.pop(0)
            kept.append(current)

            # Remove overlapping matches
            matches = [m for m in matches if not self._is_overlapping(current, m, threshold)]

        return kept

    def _is_overlapping(self, m1, m2, threshold):
        """Check if two matches overlap"""
        dx = abs(m1['x'] - m2['x'])
        dy = abs(m1['y'] - m2['y'])
        dist = (dx**2 + dy**2)**0.5

        avg_size = (m1['w'] + m2['w']) / 2
        return dist < avg_size * threshold

    def get_trackbar_names(self):
        return [
            ('Threshold', 70, 100, 'Match threshold (higher=stricter)'),
            ('Scale_Start', 80, 150, 'Start scale (%)'),
            ('Scale_End', 120, 150, 'End scale (%)'),
            ('Scale_Step', 10, 50, 'Scale step (%)'),
            ('NMS_Threshold', 30, 100, 'NMS overlap threshold (%)')
        ]


class HoughMonsterDetector(MonsterDetector):
    """Method 3: Hough Circle Detection"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 3: Hough Circle Detection"

    def detect(self, **params):
        """Detect circles using Hough transform

        Parameters:
            blur_size: Blur kernel size
            dp: Inverse ratio of accumulator
            min_dist: Minimum distance between centers
            param1: Canny high threshold
            param2: Accumulator threshold
            min_radius: Minimum radius
            max_radius: Maximum radius
        """
        blur_size = params.get('blur_size', 5)
        dp = params.get('dp', 1)
        min_dist = params.get('min_dist', 15)
        param1 = params.get('param1', 50)
        param2 = params.get('param2', 20)
        min_radius = params.get('min_radius', 3)
        max_radius = params.get('max_radius', 15)

        # Ensure positive parameters
        dp = max(1, dp)
        min_dist = max(1, min_dist)
        param1 = max(1, param1)
        param2 = max(1, param2)
        min_radius = max(1, min_radius)
        max_radius = max(min_radius + 1, max_radius)
        blur_size = max(1, blur_size)

        steps = {}

        # Convert to grayscale
        if len(self.image.shape) == 3:
            gray = cv2.cvtColor(self.image, cv2.COLOR_BGR2GRAY)
        else:
            gray = self.image

        steps['gray'] = gray

        # Apply Gaussian blur
        if blur_size % 2 == 0:
            blur_size += 1
        blurred = cv2.GaussianBlur(gray, (blur_size, blur_size), 0)

        steps['blur'] = blurred

        # Hough circle detection
        circles = cv2.HoughCircles(
            blurred,
            cv2.HOUGH_GRADIENT,
            dp=dp,
            minDist=min_dist,
            param1=param1,
            param2=param2,
            minRadius=min_radius,
            maxRadius=max_radius
        )

        monsters = []

        if circles is not None:
            circles = np.uint16(np.around(circles))
            for i, (x, y, r) in enumerate(circles[0, :]):
                monsters.append({
                    'id': i + 1,
                    'x': int(x),
                    'y': int(y),
                    'radius': int(r),
                    'area': int(np.pi * r * r)
                })

        steps['circles'] = len(monsters)

        return monsters, steps

    def get_trackbar_names(self):
        return [
            ('Blur_Size', 5, 25, 'Blur kernel size'),
            ('DP', 1, 3, 'Inverse ratio of accumulator'),
            ('Min_Dist', 15, 100, 'Min distance between centers'),
            ('Param1', 50, 200, 'Canny high threshold'),
            ('Param2', 20, 100, 'Accumulator threshold'),
            ('Min_Radius', 3, 50, 'Minimum radius'),
            ('Max_Radius', 15, 50, 'Maximum radius')
        ]


class MapDetectionApp:
    """Main application for map monster detection"""
    def __init__(self):
        self.original_image = None
        self.detector = None
        self.method_choice = None

    def run(self):
        """Main entry point"""
        print("=" * 60)
        print("Map Monster Detection Tool")
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
        print("   - Best for yellow dots on map")
        print("   - Fast and reliable")
        print("")
        print("2. Template Matching")
        print("   - Uses point.png as template")
        print("   - Good for specific shapes")
        print("")
        print("3. Hough Circle Detection")
        print("   - Detects circular shapes")
        print("   - Good for perfect circles")
        print("=" * 60)

        while True:
            choice = input("\nEnter method number (1/2/3): ").strip()
            if choice in ['1', '2', '3']:
                return int(choice)
            print("Invalid input, please enter 1, 2 or 3")

    def create_detector(self, method, image):
        """Create detector based on method"""
        if method == 1:
            return HSVMonsterDetector(image)
        elif method == 2:
            detector = TemplateMonsterDetector(image)
            template_path = input("\nEnter template path (point.png): ").strip().strip("'\"")
            if not detector.load_template(template_path):
                print("Failed to load template")
                return None
            return detector
        elif method == 3:
            return HoughMonsterDetector(image)
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
        cv2.resizeWindow(large_window, 1000, 800)

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

            params['roi_x'] = roi_x
            params['roi_y'] = roi_y
            params['roi_w'] = roi_w
            params['roi_h'] = roi_h

            # Only reprocess if parameters changed
            current_params = str(params)
            if current_params != last_params:
                start_time = time.time()

                # Extract ROI
                roi_image = self.original_image[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()
                self.detector.image = roi_image

                # Detect monsters
                monsters, steps = self.detector.detect(**params)

                # Adjust coordinates to original image space
                for m in monsters:
                    m['x'] += roi_x
                    m['y'] += roi_y

                # Draw results
                if len(self.original_image.shape) == 2:
                    result_img = cv2.cvtColor(self.original_image, cv2.COLOR_GRAY2BGR)
                else:
                    result_img = self.original_image.copy()

                result = self.detector.draw_results(result_img, monsters)

                # Draw ROI rectangle
                cv2.rectangle(result, (roi_x, roi_y), (roi_x + roi_w, roi_y + roi_h),
                             (0, 255, 0), 2)
                cv2.putText(result, f"ROI: {roi_w}x{roi_h}", (roi_x, roi_y - 10),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 255, 0), 2)

                process_time = (time.time() - start_time) * 1000

                # Create display
                display = self._create_display(steps, result, monsters, process_time,
                                               roi_x, roi_y, roi_w, roi_h)
                cached_display = display

                # Create large view (ROI only, enlarged)
                target_size = 800
                roi_scale = max(target_size / roi_w, target_size / roi_h, 1.0)
                new_roi_w = int(roi_w * roi_scale)
                new_roi_h = int(roi_h * roi_scale)

                roi_enlarged = cv2.resize(roi_image, (new_roi_w, new_roi_h),
                                         interpolation=cv2.INTER_LINEAR)

                # Scale monsters to ROI coordinate system
                roi_monsters = []
                for m in monsters:
                    rm = m.copy()
                    rm['x'] = int((m['x'] - roi_x) * roi_scale)
                    rm['y'] = int((m['y'] - roi_y) * roi_scale)
                    roi_monsters.append(rm)

                if len(roi_enlarged.shape) == 2:
                    roi_enlarged = cv2.cvtColor(roi_enlarged, cv2.COLOR_GRAY2BGR)

                large_view = self.detector.draw_results(roi_enlarged, roi_monsters, scale=1.0)
                cached_large = large_view
                cached_monsters = monsters

                last_params = current_params

                # Print results
                print(f"\n{'='*60}")
                print(f"Detected {len(monsters)} monsters in ROI({roi_w}x{roi_h}), Time: {process_time:.1f}ms")
                print(f"{'='*60}")

                if len(monsters) > 0:
                    print("Monster Positions:")
                    for m in monsters[:20]:  # Show first 20
                        print(f"  #{m['id']}: ({m['x']}, {m['y']}) - Area: {m['area']}")
                    if len(monsters) > 20:
                        print(f"  ... and {len(monsters)-20} more monsters")
                    print(f"{'='*60}")
                    print(f"✓ Total monsters found: {len(monsters)}")
                    print(f"✓ Check '{large_window}' for enlarged ROI view")
                    print(f"✓ ROI scaled {roi_scale:.1f}x for clarity")
                else:
                    print("No monsters detected. Try adjusting parameters:")
                    if self.method_choice == 1:
                        print("  - Adjust HSV ranges (H, S, V)")
                        print("  - Decrease Min_Area")
                        print("  - Decrease Circularity")
                    elif self.method_choice == 2:
                        print("  - Decrease Threshold")
                        print("  - Adjust scale range")
                    else:
                        print("  - Adjust Param1/Param2")
                        print("  - Adjust radius range")
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
                    self._save_results(cached_display, cached_large, cached_monsters)
            elif key == ord('r'):
                # Reset ROI
                cv2.setTrackbarPos('ROI_X', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Y', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Width', trackbar_window, w)
                cv2.setTrackbarPos('ROI_Height', trackbar_window, h)
                print("\n✓ ROI reset to full image")

        cv2.destroyAllWindows()

    def _create_display(self, steps, result, monsters, process_time,
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
        cv2.putText(original_roi, f"ROI: {roi_w}x{roi_h}", (roi_x + 5, roi_y + 25),
                   cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 0), 2)
        step_images.append(original_roi)
        step_titles.append("1. Original + ROI")

        # 2. Processing step (method-specific)
        if 'mask' in steps:
            mask_bgr = cv2.cvtColor(steps['mask'], cv2.COLOR_GRAY2BGR)
            step_images.append(mask_bgr)
            step_titles.append("2. HSV Mask")
        elif 'gray' in steps:
            gray_bgr = cv2.cvtColor(steps['gray'], cv2.COLOR_GRAY2BGR)
            step_images.append(gray_bgr)
            step_titles.append("2. Grayscale")
        else:
            step_images.append(original_roi.copy())
            step_titles.append("2. Processing")

        # 3. Morphology or intermediate step
        if 'morph' in steps:
            morph_bgr = cv2.cvtColor(steps['morph'], cv2.COLOR_GRAY2BGR)
            step_images.append(morph_bgr)
            step_titles.append("3. Morphology")
        elif 'blur' in steps:
            blur_bgr = cv2.cvtColor(steps['blur'], cv2.COLOR_GRAY2BGR)
            step_images.append(blur_bgr)
            step_titles.append("3. Blurred")
        else:
            step_images.append(step_images[-1].copy())
            step_titles.append("3. Intermediate")

        # 4. Final result
        step_images.append(result)
        step_titles.append(f"4. Result ({len(monsters)} monsters)")

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
            f"Detected Monsters: {len(monsters)}",
            f"ROI: ({roi_x}, {roi_y}) {roi_w}x{roi_h}",
            "",
            "Hotkeys: q-Exit  s-Save  r-Reset ROI"
        ]

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

    def _save_results(self, display, large_view, monsters):
        """Save detection results"""
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        # Save display image
        display_file = f"map_detection_{timestamp}.png"
        cv2.imwrite(display_file, display)
        print(f"\n✓ Display saved: {display_file}")

        # Save large view
        large_file = f"map_large_view_{timestamp}.png"
        cv2.imwrite(large_file, large_view)
        print(f"✓ Large view saved: {large_file}")

        # Save coordinates as JSON
        coords_file = f"map_coordinates_{timestamp}.json"
        data = {
            'timestamp': timestamp,
            'total_monsters': len(monsters),
            'monsters': [
                {
                    'id': m['id'],
                    'x': m['x'],
                    'y': m['y'],
                    'area': m.get('area', 0)
                }
                for m in monsters
            ]
        }

        with open(coords_file, 'w') as f:
            json.dump(data, f, indent=2)

        print(f"✓ Coordinates saved: {coords_file}")
        print(f"✓ Total monsters: {len(monsters)}\n")


if __name__ == '__main__':
    app = MapDetectionApp()
    app.run()
