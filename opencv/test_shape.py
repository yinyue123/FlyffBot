import cv2
import numpy as np
import time
import os


class ShapeDetector:
    """Base class for shape detectors"""
    def __init__(self, image):
        self.original_image = image.copy()
        self.image = image
        self.method_name = "Base Detector"

    def detect(self, **params):
        """Detect shapes and return results"""
        raise NotImplementedError

    def draw_results(self, image, shapes, scale=1.0):
        """Draw detection results with clear labels and bounding boxes

        Args:
            image: Input image
            shapes: List of detected shapes
            scale: Scale factor for annotations (default 1.0)
        """
        result = image.copy()

        for shape in shapes:
            color = shape.get('color', (0, 255, 0))
            x, y = shape.get('x', 10), shape.get('y', 10)
            w, h = shape.get('w', 0), shape.get('h', 0)

            # Draw contour or circle
            if 'contour' in shape:
                # Draw contour
                cv2.drawContours(result, [shape['contour']], -1, color, max(1, int(3 * scale)))

                # Draw bounding box
                cv2.rectangle(result, (x, y), (x + w, y + h), color, max(1, int(3 * scale)))

                # Draw corner markers for better visibility
                corner_len = int(min(w, h) // 6)
                line_thick = max(1, int(3 * scale))
                # Top-left
                cv2.line(result, (x, y), (x + corner_len, y), color, line_thick)
                cv2.line(result, (x, y), (x, y + corner_len), color, line_thick)
                # Top-right
                cv2.line(result, (x+w, y), (x+w - corner_len, y), color, line_thick)
                cv2.line(result, (x+w, y), (x+w, y + corner_len), color, line_thick)
                # Bottom-left
                cv2.line(result, (x, y+h), (x + corner_len, y+h), color, line_thick)
                cv2.line(result, (x, y+h), (x, y+h - corner_len), color, line_thick)
                # Bottom-right
                cv2.line(result, (x+w, y+h), (x+w - corner_len, y+h), color, line_thick)
                cv2.line(result, (x+w, y+h), (x+w, y+h - corner_len), color, line_thick)

                # Draw vertices
                if 'approx' in shape:
                    for point in shape['approx']:
                        cv2.circle(result, tuple(point[0]), max(1, int(8 * scale)), (0, 0, 255), -1)

                # Draw center point with ID
                center_x = x + w // 2
                center_y = y + h // 2
                cv2.circle(result, (center_x, center_y), max(1, int(15 * scale)), color, -1)
                cv2.circle(result, (center_x, center_y), max(1, int(18 * scale)), (0, 0, 0), max(1, int(3 * scale)))

                # Draw shape ID at center
                if 'shape_id' in shape:
                    id_str = str(shape['shape_id'])
                    id_font_scale = 0.8 * scale
                    id_thickness = max(1, int(2 * scale))
                    (id_w, id_h), _ = cv2.getTextSize(id_str, cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, id_thickness)
                    cv2.putText(result, id_str, (center_x - id_w//2, center_y + id_h//2),
                               cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, (255, 255, 255), id_thickness)

            elif 'center' in shape and 'radius' in shape:
                # Hough circle
                center = shape['center']
                radius = shape['radius']

                # Draw circle
                cv2.circle(result, center, radius, color, max(1, int(2 * scale)))

                # Draw bounding box
                cv2.rectangle(result, (x, y), (x + w, y + h), color, max(1, int(2 * scale)))

                # Draw corner markers
                corner_len = int(min(w, h) // 6)
                line_thick = max(1, int(3 * scale))
                # Top-left
                cv2.line(result, (x, y), (x + corner_len, y), color, line_thick)
                cv2.line(result, (x, y), (x, y + corner_len), color, line_thick)
                # Top-right
                cv2.line(result, (x+w, y), (x+w - corner_len, y), color, line_thick)
                cv2.line(result, (x+w, y), (x+w, y + corner_len), color, line_thick)
                # Bottom-left
                cv2.line(result, (x, y+h), (x + corner_len, y+h), color, line_thick)
                cv2.line(result, (x, y+h), (x, y+h - corner_len), color, line_thick)
                # Bottom-right
                cv2.line(result, (x+w, y+h), (x+w - corner_len, y+h), color, line_thick)
                cv2.line(result, (x+w, y+h), (x+w, y+h - corner_len), color, line_thick)

                # Draw center point with ID
                cv2.circle(result, center, max(1, int(15 * scale)), color, -1)
                cv2.circle(result, center, max(1, int(18 * scale)), (0, 0, 0), max(1, int(3 * scale)))

                # Draw shape ID at center
                if 'shape_id' in shape:
                    id_str = str(shape['shape_id'])
                    id_font_scale = 0.8 * scale
                    id_thickness = max(1, int(2 * scale))
                    (id_w, id_h), _ = cv2.getTextSize(id_str, cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, id_thickness)
                    cv2.putText(result, id_str, (center[0] - id_w//2, center[1] + id_h//2),
                               cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, (255, 255, 255), id_thickness)
            elif 'bbox' in shape:
                # Template matching bbox
                bx, by, bw, bh = shape['bbox']
                cv2.rectangle(result, (bx, by), (bx + bw, by + bh), color, max(1, int(2 * scale)))

                # Draw corner markers
                corner_len = int(min(bw, bh) // 6)
                line_thick = max(1, int(3 * scale))
                # Top-left
                cv2.line(result, (bx, by), (bx + corner_len, by), color, line_thick)
                cv2.line(result, (bx, by), (bx, by + corner_len), color, line_thick)
                # Top-right
                cv2.line(result, (bx+bw, by), (bx+bw - corner_len, by), color, line_thick)
                cv2.line(result, (bx+bw, by), (bx+bw, by + corner_len), color, line_thick)
                # Bottom-left
                cv2.line(result, (bx, by+bh), (bx + corner_len, by+bh), color, line_thick)
                cv2.line(result, (bx, by+bh), (bx, by+bh - corner_len), color, line_thick)
                # Bottom-right
                cv2.line(result, (bx+bw, by+bh), (bx+bw - corner_len, by+bh), color, line_thick)
                cv2.line(result, (bx+bw, by+bh), (bx+bw, by+bh - corner_len), color, line_thick)

                # Draw center point
                center_x = bx + bw // 2
                center_y = by + bh // 2
                cv2.circle(result, (center_x, center_y), max(1, int(15 * scale)), color, -1)
                cv2.circle(result, (center_x, center_y), max(1, int(18 * scale)), (0, 0, 0), max(1, int(3 * scale)))

                # Draw shape ID at center
                if 'shape_id' in shape:
                    id_str = str(shape['shape_id'])
                    id_font_scale = 0.8 * scale
                    id_thickness = max(1, int(2 * scale))
                    (id_w, id_h), _ = cv2.getTextSize(id_str, cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, id_thickness)
                    cv2.putText(result, id_str, (center_x - id_w//2, center_y + id_h//2),
                               cv2.FONT_HERSHEY_SIMPLEX, id_font_scale, (255, 255, 255), id_thickness)

            # Draw label with background
            label = shape.get('label', 'Unknown')
            detail = shape.get('detail', '')

            font = cv2.FONT_HERSHEY_SIMPLEX
            font_scale = 0.4 * scale  # Scale font size based on image scale
            thickness = max(1, int(1 * scale))  # Scale thickness

            # Calculate text sizes
            (_, label_h), _ = cv2.getTextSize(label, font, font_scale, thickness)
            (_, detail_h), _ = cv2.getTextSize(detail, font, font_scale, thickness)

            # Determine label position (above the shape)
            label_x = x
            label_y = y - 10

            # Adjust if label is out of bounds
            if label_y - label_h - detail_h - 20 < 0:
                label_y = y + h + label_h + 10

            # Draw label text directly without background
            cv2.putText(result, label, (label_x, label_y - detail_h - 10),
                       font, font_scale, color, thickness)

            # Draw detail text if available
            if detail:
                cv2.putText(result, detail, (label_x, label_y - 5),
                           font, font_scale, (255, 255, 255), thickness)

        return result

    def get_trackbar_names(self):
        """Return list of trackbar parameters for this detection method"""
        return []


class ContourShapeDetector(ShapeDetector):
    """Method 1: Contour Detection"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 1: Contour Detection"

    def detect(self, **params):
        """
        Parameters:
            blur_type: Denoise type (0-3)
            kernel_size: Kernel size (1-25)
            edge_type: Edge detection type (0-3)
            threshold1: Edge threshold 1 (0-255)
            threshold2: Edge threshold 2 (0-255)
            min_area: Minimum area (0-2000)
            max_area: Maximum area (0-20000)
            epsilon: Contour precision (1-20)
        """
        # Extract parameters
        blur_type = params.get('blur_type', 1)
        kernel_size = params.get('kernel_size', 5)
        edge_type = params.get('edge_type', 0)
        threshold1 = params.get('threshold1', 50)
        threshold2 = params.get('threshold2', 150)
        min_area = params.get('min_area', 10)  # Small value for detecting tiny shapes
        max_area = params.get('max_area', 5000)  # Suitable for small map points
        epsilon_factor = params.get('epsilon', 4)

        # Convert to grayscale
        if len(self.image.shape) == 3:
            gray = cv2.cvtColor(self.image, cv2.COLOR_BGR2GRAY)
        else:
            gray = self.image

        # Apply denoising
        blurred = self._apply_blur(gray, blur_type, kernel_size)

        # Apply edge detection
        edges = self._apply_edge_detection(blurred, edge_type, threshold1, threshold2)

        # Find contours
        contours, _ = cv2.findContours(edges, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        # Detect shapes
        shapes = []
        shape_count = 0
        for contour in contours:
            area = cv2.contourArea(contour)
            # Filter by area (min and max)
            if area < min_area or area > max_area:
                continue

            # Contour approximation
            epsilon = epsilon_factor / 100.0 * cv2.arcLength(contour, True)
            approx = cv2.approxPolyDP(contour, epsilon, True)

            # Get bounding box
            x, y, w, h = cv2.boundingRect(approx)

            # Classify shape
            vertices = len(approx)
            shape_name, color = self._classify_shape(vertices, w, h, contour, area)

            shape_count += 1
            shapes.append({
                'contour': contour,
                'approx': approx,
                'label': f"#{shape_count} {shape_name}",
                'detail': f"V={vertices} A={int(area)} {w}x{h}",
                'name': shape_name,
                'vertices': vertices,
                'area': area,
                'x': x, 'y': y, 'w': w, 'h': h,
                'color': color,
                'shape_id': shape_count
            })

        # Return intermediate step images
        steps = {
            'gray': gray,
            'blurred': blurred,
            'edges': edges
        }

        return shapes, steps

    def _apply_blur(self, image, blur_type, kernel_size):
        """Apply denoising"""
        if kernel_size % 2 == 0:
            kernel_size += 1
        kernel_size = max(1, kernel_size)

        if blur_type == 0:
            return image
        elif blur_type == 1:
            return cv2.GaussianBlur(image, (kernel_size, kernel_size), 0)
        elif blur_type == 2:
            return cv2.medianBlur(image, kernel_size)
        elif blur_type == 3:
            return cv2.bilateralFilter(image, kernel_size, 75, 75)
        return image

    def _apply_edge_detection(self, image, edge_type, threshold1, threshold2):
        """Apply edge detection"""
        if edge_type == 0:  # Canny
            return cv2.Canny(image, threshold1, threshold2)
        elif edge_type == 1:  # Sobel
            sobelx = cv2.Sobel(image, cv2.CV_64F, 1, 0, ksize=3)
            sobely = cv2.Sobel(image, cv2.CV_64F, 0, 1, ksize=3)
            sobel = np.sqrt(sobelx**2 + sobely**2)
            sobel = np.uint8(sobel / sobel.max() * 255)
            _, edges = cv2.threshold(sobel, threshold1, 255, cv2.THRESH_BINARY)
            return edges
        elif edge_type == 2:  # Laplacian
            laplacian = cv2.Laplacian(image, cv2.CV_64F)
            laplacian = np.uint8(np.absolute(laplacian))
            _, edges = cv2.threshold(laplacian, threshold1, 255, cv2.THRESH_BINARY)
            return edges
        elif edge_type == 3:  # Scharr
            scharrx = cv2.Scharr(image, cv2.CV_64F, 1, 0)
            scharry = cv2.Scharr(image, cv2.CV_64F, 0, 1)
            scharr = np.sqrt(scharrx**2 + scharry**2)
            scharr = np.uint8(scharr / scharr.max() * 255)
            _, edges = cv2.threshold(scharr, threshold1, 255, cv2.THRESH_BINARY)
            return edges
        return image

    def _classify_shape(self, vertices, w, h, contour, area):
        """Classify shape based on vertices"""
        if vertices == 3:
            return "Triangle", (255, 0, 0)
        elif vertices == 4:
            aspect_ratio = float(w) / h if h > 0 else 0
            if 0.95 <= aspect_ratio <= 1.05:
                return "Square", (0, 255, 255)
            else:
                return "Rectangle", (255, 255, 0)
        elif vertices == 5:
            return "Pentagon", (255, 0, 255)
        elif vertices > 5 and vertices < 12:
            return f"{vertices}-gon", (128, 0, 128)
        else:
            # Check if circle
            perimeter = cv2.arcLength(contour, True)
            circularity = 4 * np.pi * area / (perimeter ** 2) if perimeter > 0 else 0
            if circularity > 0.8:
                return "Circle", (0, 255, 0)
            else:
                return "Polygon", (200, 200, 200)

    def get_trackbar_names(self):
        return [
            ('Blur Type', 1, 3, 'Denoise: 0=None 1=Gaussian 2=Median 3=Bilateral'),
            ('Kernel Size', 5, 25, 'Kernel size (odd number)'),
            ('Edge Type', 0, 3, 'Edge: 0=Canny 1=Sobel 2=Laplacian 3=Scharr'),
            ('Threshold1', 50, 255, 'Edge threshold 1'),
            ('Threshold2', 150, 255, 'Edge threshold 2'),
            ('Min Area', 10, 2000, 'Minimum area (filter small noise)'),
            ('Max Area', 5000, 20000, 'Maximum area'),
            ('Epsilon', 4, 20, 'Contour precision (%)')
        ]


class HoughCircleDetector(ShapeDetector):
    """Method 2: Hough Circle Detection"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 2: Hough Circle Detection"

    def detect(self, **params):
        """
        Parameters:
            blur_size: Blur kernel size
            dp: Inverse ratio of accumulator resolution
            min_dist: Minimum distance between circle centers
            param1: Canny edge detection high threshold
            param2: Accumulator threshold
            min_radius: Minimum radius
            max_radius: Maximum radius
        """
        blur_size = params.get('blur_size', 5)
        dp = params.get('dp', 1)
        min_dist = params.get('min_dist', 50)
        param1 = params.get('param1', 100)
        param2 = params.get('param2', 30)
        min_radius = params.get('min_radius', 10)
        max_radius = params.get('max_radius', 100)

        # Ensure all parameters are positive (HoughCircles requirement)
        dp = max(1, dp)
        min_dist = max(1, min_dist)
        param1 = max(1, param1)
        param2 = max(1, param2)
        min_radius = max(1, min_radius)
        max_radius = max(min_radius + 1, max_radius)

        # Convert to grayscale
        if len(self.image.shape) == 3:
            gray = cv2.cvtColor(self.image, cv2.COLOR_BGR2GRAY)
        else:
            gray = self.image

        # Apply Gaussian blur
        blur_size = max(1, blur_size)  # Ensure positive
        if blur_size % 2 == 0:
            blur_size += 1
        blurred = cv2.GaussianBlur(gray, (blur_size, blur_size), 0)

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

        shapes = []
        if circles is not None:
            circles = np.uint16(np.around(circles))
            for i, circle in enumerate(circles[0, :]):
                x, y, r = circle
                area = np.pi * r * r
                shapes.append({
                    'center': (x, y),
                    'radius': r,
                    'label': f"#{i+1} Circle",
                    'detail': f"R={r} A={int(area)}",
                    'name': 'Circle',
                    'area': area,
                    'x': x - r,
                    'y': y - r,
                    'w': r * 2,
                    'h': r * 2,
                    'color': (0, 255, 0),
                    'shape_id': i + 1
                })

        # Intermediate steps
        steps = {
            'gray': gray,
            'blurred': blurred,
            'edges': cv2.Canny(blurred, param1 // 2, param1)
        }

        return shapes, steps

    def get_trackbar_names(self):
        return [
            ('Blur Size', 5, 25, 'Blur kernel size'),
            ('DP', 1, 3, 'Inverse ratio of accumulator'),
            ('Min Dist', 50, 200, 'Min distance between centers'),
            ('Param1', 100, 300, 'Canny high threshold'),
            ('Param2', 30, 100, 'Accumulator threshold'),
            ('Min Radius', 10, 200, 'Minimum radius'),
            ('Max Radius', 100, 500, 'Maximum radius')
        ]


class TemplateMatchDetector(ShapeDetector):
    """Method 3: Template Matching Detection"""
    def __init__(self, image):
        super().__init__(image)
        self.method_name = "Method 3: Template Matching"
        self.templates = []
        self.template_names = []

    def load_templates(self, template_path):
        """Load template images from directory or single file"""
        if not os.path.exists(template_path):
            print(f"Template path not found: {template_path}")
            return False

        # Check if input is a file or directory
        if os.path.isfile(template_path):
            # Single template file
            if not template_path.lower().endswith(('.png', '.jpg', '.jpeg', '.bmp')):
                print(f"Invalid image file format: {template_path}")
                return False

            template = cv2.imread(template_path)
            if template is not None:
                self.templates.append(template)
                name = os.path.splitext(os.path.basename(template_path))[0]
                self.template_names.append(name)
                print(f"Loaded template: {name} ({template.shape[1]}x{template.shape[0]})")
                return True
            else:
                print(f"Failed to load template image: {template_path}")
                return False

        elif os.path.isdir(template_path):
            # Directory of templates
            template_files = [f for f in os.listdir(template_path)
                             if f.lower().endswith(('.png', '.jpg', '.jpeg', '.bmp'))]

            if not template_files:
                print(f"No template images found in {template_path}")
                return False

            for filename in template_files:
                path = os.path.join(template_path, filename)
                template = cv2.imread(path)
                if template is not None:
                    self.templates.append(template)
                    name = os.path.splitext(filename)[0]
                    self.template_names.append(name)
                    print(f"Loaded template: {name} ({template.shape[1]}x{template.shape[0]})")

            return len(self.templates) > 0

        else:
            print(f"Invalid path (not a file or directory): {template_path}")
            return False

    def detect(self, **params):
        """
        Parameters:
            method: Matching method (0-5)
            threshold: Match threshold (0-100)
            scale_start: Start scale ratio (50-150)
            scale_end: End scale ratio (50-150)
            scale_step: Scale step (1-20)
        """
        method_idx = params.get('method', 0)
        threshold = params.get('threshold', 80) / 100.0
        scale_start = params.get('scale_start', 80) / 100.0
        scale_end = params.get('scale_end', 120) / 100.0
        scale_step = params.get('scale_step', 10) / 100.0

        # Matching method mapping
        methods = [
            cv2.TM_CCOEFF_NORMED,
            cv2.TM_CCORR_NORMED,
            cv2.TM_SQDIFF_NORMED,
            cv2.TM_CCOEFF,
            cv2.TM_CCORR,
            cv2.TM_SQDIFF
        ]
        method = methods[method_idx]

        # Convert to grayscale
        if len(self.image.shape) == 3:
            gray = cv2.cvtColor(self.image, cv2.COLOR_BGR2GRAY)
        else:
            gray = self.image

        shapes = []
        all_matches = gray.copy()

        # Match each template
        for template_idx, template in enumerate(self.templates):
            # Convert template to grayscale
            if len(template.shape) == 3:
                template_gray = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY)
            else:
                template_gray = template

            th, tw = template_gray.shape[:2]

            # Multi-scale matching
            for scale in np.arange(scale_start, scale_end, scale_step):
                # Resize template
                new_w = int(tw * scale)
                new_h = int(th * scale)
                if new_w <= 0 or new_h <= 0 or new_w > gray.shape[1] or new_h > gray.shape[0]:
                    continue

                resized_template = cv2.resize(template_gray, (new_w, new_h))

                # Template matching
                result = cv2.matchTemplate(gray, resized_template, method)

                # Process results based on method type
                if method in [cv2.TM_SQDIFF, cv2.TM_SQDIFF_NORMED]:
                    # For SQDIFF, lower is better
                    locations = np.where(result <= (1 - threshold))
                else:
                    # For other methods, higher is better
                    locations = np.where(result >= threshold)

                # Extract matching locations
                for pt in zip(*locations[::-1]):
                    x, y = pt
                    confidence = result[y, x]

                    # Invert confidence for SQDIFF
                    if method in [cv2.TM_SQDIFF, cv2.TM_SQDIFF_NORMED]:
                        confidence = 1 - confidence

                    shapes.append({
                        'bbox': (x, y, new_w, new_h),
                        'label': f"{self.template_names[template_idx]}",
                        'detail': f"Conf={confidence:.2f} Scale={scale:.2f}",
                        'name': self.template_names[template_idx],
                        'confidence': confidence,
                        'scale': scale,
                        'x': x,
                        'y': y,
                        'w': new_w,
                        'h': new_h,
                        'color': (255, 0, 255)
                    })

        # Non-maximum suppression (remove overlapping detections)
        shapes = self._non_max_suppression(shapes, 0.5)

        # Add shape IDs after NMS
        for i, shape in enumerate(shapes):
            shape['shape_id'] = i + 1
            shape['label'] = f"#{i+1} {shape['name']}"

        steps = {
            'gray': gray,
            'matches': all_matches
        }

        return shapes, steps

    def _non_max_suppression(self, shapes, overlap_thresh):
        """Non-maximum suppression to remove overlapping detections"""
        if len(shapes) == 0:
            return []

        boxes = np.array([[s['bbox'][0], s['bbox'][1],
                          s['bbox'][0] + s['bbox'][2],
                          s['bbox'][1] + s['bbox'][3]] for s in shapes])

        scores = np.array([s.get('confidence', 1.0) for s in shapes])

        x1 = boxes[:, 0]
        y1 = boxes[:, 1]
        x2 = boxes[:, 2]
        y2 = boxes[:, 3]

        areas = (x2 - x1 + 1) * (y2 - y1 + 1)
        indices = np.argsort(scores)

        keep = []
        while len(indices) > 0:
            last = len(indices) - 1
            i = indices[last]
            keep.append(i)

            xx1 = np.maximum(x1[i], x1[indices[:last]])
            yy1 = np.maximum(y1[i], y1[indices[:last]])
            xx2 = np.minimum(x2[i], x2[indices[:last]])
            yy2 = np.minimum(y2[i], y2[indices[:last]])

            w = np.maximum(0, xx2 - xx1 + 1)
            h = np.maximum(0, yy2 - yy1 + 1)

            overlap = (w * h) / areas[indices[:last]]

            indices = np.delete(indices, np.concatenate(([last],
                              np.where(overlap > overlap_thresh)[0])))

        return [shapes[i] for i in keep]

    def get_trackbar_names(self):
        return [
            ('Method', 0, 5, 'Match method: 0=CCOEFF_N 1=CCORR_N 2=SQDIFF_N'),
            ('Threshold', 80, 100, 'Match threshold (%)'),
            ('Scale Start', 80, 150, 'Start scale (%)'),
            ('Scale End', 120, 150, 'End scale (%)'),
            ('Scale Step', 10, 20, 'Scale step (%)')
        ]


class ShapeDetectorApp:
    """Shape Detector Application"""
    def __init__(self):
        self.detector = None
        self.original_image = None
        self.method_choice = None

    def select_method(self):
        """Select detection method"""
        print("\n" + "=" * 60)
        print("Shape Detection Tool - Multi-Method System")
        print("=" * 60)
        print("\nPlease select detection method:")
        print("  1 - Method 1: Contour Detection (Denoise + Edge + Contour)")
        print("  2 - Method 2: Hough Circle Detection (Circle only)")
        print("  3 - Method 3: Template Matching (Template-based)")
        print("=" * 60)

        while True:
            choice = input("\nEnter method number (1/2/3): ").strip()
            if choice in ['1', '2', '3']:
                return int(choice)
            print("Invalid input, please enter 1, 2 or 3")

    def create_detector(self, method, image):
        """Create detector"""
        if method == 1:
            return ContourShapeDetector(image)
        elif method == 2:
            return HoughCircleDetector(image)
        elif method == 3:
            detector = TemplateMatchDetector(image)
            # Load templates
            template_dir = input("\nEnter template path (single file or directory): ").strip().strip("'\"")
            if not detector.load_templates(template_dir):
                print("Failed to load templates, returning to main menu")
                return None
            return detector
        return None

    def run_detector_with_trackbars(self):
        """Run detector with trackbars"""
        trackbar_window = f'{self.detector.method_name} - Controls'
        display_window = f'{self.detector.method_name} - Detection Result'
        result_window = f'{self.detector.method_name} - Full Result (Large)'

        cv2.namedWindow(trackbar_window)
        cv2.namedWindow(display_window, cv2.WINDOW_NORMAL)
        cv2.resizeWindow(display_window, 1600, 900)  # Larger display window
        cv2.namedWindow(result_window, cv2.WINDOW_NORMAL)
        cv2.resizeWindow(result_window, 1200, 800)  # Large result window

        # Initialize ROI to full image
        h, w = self.original_image.shape[:2]

        # Create ROI trackbars first
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
        print("  s       - Save current result")
        print("  r       - Reset ROI to full image")
        print("=" * 60)

        last_params = None
        cached_display = None
        cached_result = None  # Full result image for large window

        while True:
            # Read ROI parameters
            roi_x = cv2.getTrackbarPos('ROI_X', trackbar_window)
            roi_y = cv2.getTrackbarPos('ROI_Y', trackbar_window)
            roi_w = cv2.getTrackbarPos('ROI_Width', trackbar_window)
            roi_h = cv2.getTrackbarPos('ROI_Height', trackbar_window)

            # Ensure ROI is valid
            roi_x = max(0, min(roi_x, w - 10))
            roi_y = max(0, min(roi_y, h - 10))
            roi_w = max(10, min(roi_w, w - roi_x))
            roi_h = max(10, min(roi_h, h - roi_y))

            # Read detection parameters
            params = {}
            for name, default, max_val, _ in trackbars:
                value = cv2.getTrackbarPos(name, trackbar_window)
                # Convert trackbar name to parameter name
                param_name = name.lower().replace(' ', '_')
                params[param_name] = value

            # Convert parameters to tuple for comparison
            current_params = (roi_x, roi_y, roi_w, roi_h, tuple(params.items()))

            # Only reprocess when parameters change
            if current_params != last_params:
                start_time = time.time()

                # Extract ROI region
                roi_image = self.original_image[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()

                # Update detector image to ROI
                self.detector.image = roi_image

                # Execute detection on ROI
                shapes, steps = self.detector.detect(**params)

                # Adjust shape coordinates to original image coordinate system
                for shape in shapes:
                    shape['x'] += roi_x
                    shape['y'] += roi_y
                    if 'contour' in shape:
                        shape['contour'] = shape['contour'] + np.array([roi_x, roi_y])
                    if 'approx' in shape:
                        shape['approx'] = shape['approx'] + np.array([roi_x, roi_y])
                    if 'center' in shape:
                        cx, cy = shape['center']
                        shape['center'] = (cx + roi_x, cy + roi_y)
                    if 'bbox' in shape:
                        bx, by, bw, bh = shape['bbox']
                        shape['bbox'] = (bx + roi_x, by + roi_y, bw, bh)

                # Draw results on original image
                if len(self.original_image.shape) == 2:
                    result_img = cv2.cvtColor(self.original_image, cv2.COLOR_GRAY2BGR)
                else:
                    result_img = self.original_image.copy()

                result = self.detector.draw_results(result_img, shapes)

                # Draw ROI rectangle on result
                cv2.rectangle(result, (roi_x, roi_y), (roi_x + roi_w, roi_y + roi_h),
                             (0, 255, 0), 2)
                cv2.putText(result, f"ROI: {roi_w}x{roi_h}", (roi_x, roi_y - 10),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.6, (0, 255, 0), 2)

                process_time = (time.time() - start_time) * 1000

                # Create display panel with ROI info
                display = self._create_display(steps, result, shapes, process_time,
                                               roi_x, roi_y, roi_w, roi_h)

                cached_display = display

                # Create enlarged ROI result for large window
                # Extract ROI from original image
                roi_original = self.original_image[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()
                if len(roi_original.shape) == 2:
                    roi_original = cv2.cvtColor(roi_original, cv2.COLOR_GRAY2BGR)

                # Calculate scale factor to enlarge small ROIs (target min size: 800px)
                target_size = 800
                roi_scale = max(target_size / roi_w, target_size / roi_h, 1.0)

                # Resize ROI
                new_roi_w = int(roi_w * roi_scale)
                new_roi_h = int(roi_h * roi_scale)
                roi_enlarged = cv2.resize(roi_original, (new_roi_w, new_roi_h), interpolation=cv2.INTER_LINEAR)

                # Convert shapes to ROI coordinate system and scale them
                roi_shapes = []
                for shape in shapes:
                    roi_shape = shape.copy()
                    # Translate to ROI coordinates
                    roi_shape['x'] = int((shape['x'] - roi_x) * roi_scale)
                    roi_shape['y'] = int((shape['y'] - roi_y) * roi_scale)
                    roi_shape['w'] = int(shape['w'] * roi_scale)
                    roi_shape['h'] = int(shape['h'] * roi_scale)

                    if 'contour' in shape:
                        roi_shape['contour'] = ((shape['contour'] - np.array([roi_x, roi_y])) * roi_scale).astype(np.int32)
                    if 'approx' in shape:
                        roi_shape['approx'] = ((shape['approx'] - np.array([roi_x, roi_y])) * roi_scale).astype(np.int32)
                    if 'center' in shape:
                        cx, cy = shape['center']
                        roi_shape['center'] = (int((cx - roi_x) * roi_scale), int((cy - roi_y) * roi_scale))
                    if 'bbox' in shape:
                        bx, by, bw, bh = shape['bbox']
                        roi_shape['bbox'] = (int((bx - roi_x) * roi_scale), int((by - roi_y) * roi_scale),
                                            int(bw * roi_scale), int(bh * roi_scale))
                    if 'radius' in shape:
                        roi_shape['radius'] = int(shape['radius'] * roi_scale)

                    roi_shapes.append(roi_shape)

                # Draw results on enlarged ROI with fixed annotation size (don't scale annotations)
                cached_result = self.detector.draw_results(roi_enlarged, roi_shapes, scale=1.0)

                last_params = current_params

                # Print detection details to console
                print(f"\n{'='*60}")
                print(f"Detected {len(shapes)} shapes in ROI({roi_w}x{roi_h}), Time: {process_time:.1f}ms")
                print(f"{'='*60}")
                if len(shapes) > 0:
                    print("Shape Details:")
                    # Only show first 10 to avoid cluttering console
                    for _, shape in enumerate(shapes[:10]):
                        label = shape.get('label', 'Unknown')
                        detail = shape.get('detail', '')
                        pos_x = shape.get('x', 0)
                        pos_y = shape.get('y', 0)
                        print(f"  {label}: {detail} at ({pos_x},{pos_y})")
                    if len(shapes) > 10:
                        print(f"  ... and {len(shapes)-10} more shapes")
                    print(f"{'='*60}")
                    print(f"✓✓✓ CHECK THE LARGE WINDOW: 'Full Result (Large)' ✓✓✓")
                    print(f"    ROI enlarged {roi_scale:.1f}x ({roi_w}x{roi_h} → {new_roi_w}x{new_roi_h})")
                    print("    This window shows ONLY the ROI region, automatically enlarged!")
                    print("    Annotations remain at fixed size for clarity")
                    print("")
                    print("Annotations on Large Window:")
                    print("  - Colored bounding boxes with L-shaped corner markers")
                    print("  - Colored circle at center with white ID number")
                    print("  - Label text above/below shape with name and details")
                    print("  - Red dots marking the vertices")
                    print("")
                    print("If annotations are too small:")
                    print("  1. Increase Min Area slider to filter out small noise")
                    print("  2. Press 's' to save image and view it zoomed in")
                    print("  3. Try adjusting ROI to focus on specific area")
                else:
                    print("  No shapes detected. Try adjusting parameters:")
                    print("  - Decrease Min Area")
                    print("  - Increase Max Area")
                    print("  - Adjust Threshold1/Threshold2")
                print(f"{'='*60}\n")

            # Display result
            if cached_display is not None:
                cv2.imshow(display_window, cached_display)

            # Display full result in large window
            if cached_result is not None:
                cv2.imshow(result_window, cached_result)

            # Key handling
            key = cv2.waitKey(30) & 0xFF
            if key == ord('q') or key == 27:
                print("\n\nExiting...")
                break
            elif key == ord('s'):
                if cached_display is not None:
                    timestamp = time.strftime("%Y%m%d_%H%M%S")
                    filename = f"shape_detection_{timestamp}.png"
                    cv2.imwrite(filename, cached_display)
                    print(f"\n✓ Saved: {filename}")
            elif key == ord('r'):
                # Reset ROI to full image
                cv2.setTrackbarPos('ROI_X', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Y', trackbar_window, 0)
                cv2.setTrackbarPos('ROI_Width', trackbar_window, w)
                cv2.setTrackbarPos('ROI_Height', trackbar_window, h)
                print("\n✓ ROI reset to full image")

        cv2.destroyAllWindows()

    def _create_display(self, steps, result, shapes, process_time,
                       roi_x, roi_y, roi_w, roi_h):
        """Create display panel with ROI visualization"""
        # Get intermediate step images
        step_images = []
        step_titles = []

        # Add original image with ROI rectangle
        original_with_roi = self.original_image.copy()
        if len(original_with_roi.shape) == 2:
            original_with_roi = cv2.cvtColor(original_with_roi, cv2.COLOR_GRAY2BGR)
        cv2.rectangle(original_with_roi, (roi_x, roi_y),
                     (roi_x + roi_w, roi_y + roi_h), (0, 255, 0), 3)
        cv2.putText(original_with_roi, f"ROI: {roi_w}x{roi_h}",
                   (roi_x + 5, roi_y + 25),
                   cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 0), 2)
        step_images.append(original_with_roi)
        step_titles.append("1. Original + ROI")

        # Add ROI closeup with annotations
        roi_result = result[roi_y:roi_y+roi_h, roi_x:roi_x+roi_w].copy()
        step_images.append(roi_result)
        step_titles.append(f"2. ★ ROI RESULT ({len(shapes)} shapes) ★")

        if 'edges' in steps:
            step_images.append(steps['edges'])
            step_titles.append(f"3. Edge Detection (ROI)")

        # Add full result with annotations
        step_images.append(result)
        step_titles.append(f"4. Full Image Result")

        # Resize all images to same size
        target_h, target_w = 400, 600  # Larger display size

        display_images = []
        for img, title in zip(step_images, step_titles):
            # Convert to color
            if len(img.shape) == 2:
                img_color = cv2.cvtColor(img, cv2.COLOR_GRAY2BGR)
            else:
                img_color = img.copy()

            # Resize
            img_resized = cv2.resize(img_color, (target_w, target_h))

            # Add title
            overlay = img_resized.copy()
            cv2.rectangle(overlay, (0, 0), (target_w, 40), (0, 0, 0), -1)
            cv2.addWeighted(overlay, 0.7, img_resized, 0.3, 0, img_resized)
            cv2.putText(img_resized, title, (10, 28),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.8, (255, 255, 255), 2)

            display_images.append(img_resized)

        # Create 2x2 grid layout
        if len(display_images) >= 4:
            top_row = np.hstack(display_images[0:2])
            bottom_row = np.hstack(display_images[2:4])
            display = np.vstack([top_row, bottom_row])
        elif len(display_images) == 3:
            top_row = np.hstack(display_images[0:2])
            bottom_row = np.hstack([display_images[2], np.zeros((target_h, target_w, 3), dtype=np.uint8)])
            display = np.vstack([top_row, bottom_row])
        else:
            # Less than 3 images, arrange horizontally
            display = np.hstack(display_images)

        # Add info text
        info_y = display.shape[0] - 180
        info_lines = [
            f"Detection Method: {self.detector.method_name}",
            f"Processing Time: {process_time:.1f}ms",
            f"Detected Shapes: {len(shapes)}",
            f"ROI Region: ({roi_x}, {roi_y}) {roi_w}x{roi_h}",
            "",
            "Hotkeys: q-Exit  s-Save  r-Reset ROI"
        ]

        y_offset = info_y
        for line in info_lines:
            # Add semi-transparent background
            (text_w, text_h), _ = cv2.getTextSize(line, cv2.FONT_HERSHEY_SIMPLEX, 0.6, 2)
            overlay = display.copy()
            cv2.rectangle(overlay, (10, y_offset - text_h - 5),
                         (20 + text_w, y_offset + 5), (0, 0, 0), -1)
            cv2.addWeighted(overlay, 0.6, display, 0.4, 0, display)

            cv2.putText(display, line, (15, y_offset),
                       cv2.FONT_HERSHEY_SIMPLEX, 0.6, (255, 255, 255), 2)
            y_offset += 30

        return display

    def run(self):
        """Main run function"""
        # Select method
        self.method_choice = self.select_method()

        # Read image
        image_path = input("\nEnter image path: ").strip().strip("'\"")
        self.original_image = cv2.imread(image_path)

        if self.original_image is None:
            print("❌ Cannot read image!")
            return

        h, w = self.original_image.shape[:2]
        print(f"✓ Image size: {w}x{h}")

        # Create detector
        self.detector = self.create_detector(self.method_choice, self.original_image)
        if self.detector is None:
            return

        print(f"\nUsing detection method: {self.detector.method_name}")

        # Run detection
        self.run_detector_with_trackbars()


if __name__ == '__main__':
    app = ShapeDetectorApp()
    app.run()
