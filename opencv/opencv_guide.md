# OpenCV Usage Guide

## Table of Contents
- [Basic Operations](#basic-operations)
- [Color Space Conversion](#color-space-conversion)
- [Image Filtering](#image-filtering)
- [Edge Detection](#edge-detection)
- [Morphological Operations](#morphological-operations)
- [Feature Detection](#feature-detection)
- [Object Detection](#object-detection)
- [Algorithm Comparisons](#algorithm-comparisons)

---

## Basic Operations

### Read, Display, and Save Images

```python
import cv2

# Read image
img = cv2.imread('image.png')           # Default: BGR color
img_gray = cv2.imread('image.png', 0)   # Grayscale

# Display image
cv2.imshow('Window Title', img)
cv2.waitKey(0)                          # Wait for key press (0 = infinite)
cv2.destroyAllWindows()

# Save image
cv2.imwrite('output.png', img)
```

### Image Properties and Manipulation

```python
# Get image properties
height, width, channels = img.shape
total_pixels = img.size
data_type = img.dtype

# Crop image
crop = img[y1:y2, x1:x2]

# Resize image
resized = cv2.resize(img, (new_width, new_height))
resized = cv2.resize(img, None, fx=0.5, fy=0.5)  # Scale by factor

# Rotate image
(h, w) = img.shape[:2]
center = (w // 2, h // 2)
M = cv2.getRotationMatrix2D(center, angle=45, scale=1.0)
rotated = cv2.warpAffine(img, M, (w, h))

# Flip image
flipped_h = cv2.flip(img, 1)   # Horizontal
flipped_v = cv2.flip(img, 0)   # Vertical
flipped_both = cv2.flip(img, -1)
```

### Drawing Functions

```python
# Rectangle
cv2.rectangle(img, (x1, y1), (x2, y2), color=(0,255,0), thickness=2)

# Circle
cv2.circle(img, center=(x, y), radius=50, color=(255,0,0), thickness=-1)  # -1 = filled

# Line
cv2.line(img, (x1, y1), (x2, y2), color=(0,0,255), thickness=2)

# Text
cv2.putText(img, 'Text', (x, y), cv2.FONT_HERSHEY_SIMPLEX,
            fontScale=1, color=(255,255,255), thickness=2)
```

---

## Color Space Conversion

### Common Color Spaces

```python
# BGR to other color spaces
gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
rgb = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
lab = cv2.cvtColor(img, cv2.COLOR_BGR2LAB)

# HSV color range detection
lower = np.array([h_min, s_min, v_min])
upper = np.array([h_max, s_max, v_max])
mask = cv2.inRange(hsv, lower, upper)
```

### HSV Color Ranges (Hue: 0-180 in OpenCV)

| Color  | Hue Range | Example Values |
|--------|-----------|----------------|
| Red    | 0-10, 170-180 | `[0,100,100]` to `[10,255,255]` |
| Yellow | 20-40 | `[20,100,100]` to `[40,255,255]` |
| Green  | 40-80 | `[40,100,100]` to `[80,255,255]` |
| Cyan   | 80-100 | `[80,100,100]` to `[100,255,255]` |
| Blue   | 100-130 | `[100,100,100]` to `[130,255,255]` |

---

## Image Filtering

### Blur and Smoothing

```python
# Gaussian Blur - Best for general noise reduction
blur = cv2.GaussianBlur(img, (5, 5), 0)

# Median Blur - Best for salt-and-pepper noise
median = cv2.medianBlur(img, 5)

# Bilateral Filter - Preserves edges while smoothing
bilateral = cv2.bilateralFilter(img, 9, 75, 75)

# Average Blur
avg = cv2.blur(img, (5, 5))
```

### Sharpening

```python
# Kernel-based sharpening
kernel = np.array([[-1,-1,-1],
                   [-1, 9,-1],
                   [-1,-1,-1]])
sharpened = cv2.filter2D(img, -1, kernel)

# Unsharp masking
gaussian = cv2.GaussianBlur(img, (9, 9), 10.0)
sharpened = cv2.addWeighted(img, 1.5, gaussian, -0.5, 0)
```

---

## Edge Detection

### Common Edge Detection Algorithms

```python
# Canny Edge Detection (most popular)
edges = cv2.Canny(img, threshold1=100, threshold2=200)

# Sobel Edge Detection
sobelx = cv2.Sobel(gray, cv2.CV_64F, 1, 0, ksize=5)  # X direction
sobely = cv2.Sobel(gray, cv2.CV_64F, 0, 1, ksize=5)  # Y direction
sobel = cv2.magnitude(sobelx, sobely)

# Laplacian Edge Detection
laplacian = cv2.Laplacian(gray, cv2.CV_64F)

# Scharr Edge Detection (more accurate than Sobel)
scharrx = cv2.Scharr(gray, cv2.CV_64F, 1, 0)
scharry = cv2.Scharr(gray, cv2.CV_64F, 0, 1)
```

---

## Morphological Operations

### Basic Operations

```python
# Create kernel
kernel = np.ones((5,5), np.uint8)
kernel_ellipse = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5,5))
kernel_cross = cv2.getStructuringElement(cv2.MORPH_CROSS, (5,5))

# Erosion - Removes small white noise
erosion = cv2.erode(img, kernel, iterations=1)

# Dilation - Enlarges white regions
dilation = cv2.dilate(img, kernel, iterations=1)

# Opening - Erosion followed by Dilation (removes noise)
opening = cv2.morphologyEx(img, cv2.MORPH_OPEN, kernel)

# Closing - Dilation followed by Erosion (fills small holes)
closing = cv2.morphologyEx(img, cv2.MORPH_CLOSE, kernel)

# Gradient - Difference between dilation and erosion
gradient = cv2.morphologyEx(img, cv2.MORPH_GRADIENT, kernel)

# Top Hat - Difference between input and opening
tophat = cv2.morphologyEx(img, cv2.MORPH_TOPHAT, kernel)

# Black Hat - Difference between closing and input
blackhat = cv2.morphologyEx(img, cv2.MORPH_BLACKHAT, kernel)
```

---

## Feature Detection

### Corner Detection

```python
# Harris Corner Detection
gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
corners = cv2.cornerHarris(gray, blockSize=2, ksize=3, k=0.04)

# Shi-Tomasi Corner Detection (Good Features to Track)
corners = cv2.goodFeaturesToTrack(gray, maxCorners=100, qualityLevel=0.01, minDistance=10)

# FAST Corner Detection (fastest)
fast = cv2.FastFeatureDetector_create()
keypoints = fast.detect(gray, None)
```

### Feature Descriptors

```python
# SIFT (Scale-Invariant Feature Transform) - Most robust, patented
sift = cv2.SIFT_create()
keypoints, descriptors = sift.detectAndCompute(gray, None)

# ORB (Oriented FAST and Rotated BRIEF) - Fast, free
orb = cv2.ORB_create()
keypoints, descriptors = orb.detectAndCompute(gray, None)

# AKAZE - Good balance of speed and accuracy
akaze = cv2.AKAZE_create()
keypoints, descriptors = akaze.detectAndCompute(gray, None)

# Draw keypoints
img_keypoints = cv2.drawKeypoints(img, keypoints, None, color=(0,255,0))
```

---

## Object Detection

### Contour Detection

```python
# Find contours
contours, hierarchy = cv2.findContours(binary_img, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

# Draw contours
cv2.drawContours(img, contours, -1, (0,255,0), 2)  # -1 = all contours

# Get contour properties
for cnt in contours:
    area = cv2.contourArea(cnt)
    perimeter = cv2.arcLength(cnt, closed=True)
    x, y, w, h = cv2.boundingRect(cnt)

    # Approximate contour
    epsilon = 0.01 * perimeter
    approx = cv2.approxPolyDP(cnt, epsilon, closed=True)
```

### Template Matching

```python
# Find template in image
result = cv2.matchTemplate(img, template, cv2.TM_CCOEFF_NORMED)
min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)

# TM_CCOEFF_NORMED: Use max_loc for best match
# TM_SQDIFF_NORMED: Use min_loc for best match

# Draw rectangle around match
top_left = max_loc
bottom_right = (top_left[0] + template.shape[1], top_left[1] + template.shape[0])
cv2.rectangle(img, top_left, bottom_right, (0,255,0), 2)
```

### Cascade Classifiers

```python
# Load pre-trained cascade
face_cascade = cv2.CascadeClassifier(cv2.data.haarcascades + 'haarcascade_frontalface_default.xml')

# Detect objects
gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
faces = face_cascade.detectMultiScale(gray, scaleFactor=1.1, minNeighbors=5, minSize=(30,30))

# Draw rectangles
for (x, y, w, h) in faces:
    cv2.rectangle(img, (x, y), (x+w, y+h), (255,0,0), 2)
```

---

## Algorithm Comparisons

### Blur/Smoothing Algorithms

| Algorithm | Speed | Edge Preservation | Best Use Case | Pros | Cons |
|-----------|-------|-------------------|---------------|------|------|
| **Gaussian Blur** | Fast | Poor | General noise reduction | Fast, simple, effective | Blurs edges |
| **Median Blur** | Medium | Good | Salt-and-pepper noise | Preserves edges, removes outliers | Slower than Gaussian |
| **Bilateral Filter** | Slow | Excellent | Denoise while keeping edges | Best edge preservation | Computationally expensive |
| **Average Blur** | Fastest | Poor | Quick smoothing | Very fast | Poorest quality |

### Edge Detection Algorithms

| Algorithm | Accuracy | Speed | Noise Sensitivity | Best Use Case | Pros | Cons |
|-----------|----------|-------|-------------------|---------------|------|------|
| **Canny** | Excellent | Medium | Low (has denoising) | General edge detection | Multi-stage, thin edges | Requires parameter tuning |
| **Sobel** | Good | Fast | Medium | Gradient calculation | Simple, fast | Thick edges, sensitive to noise |
| **Laplacian** | Good | Fast | High | Finding rapid intensity changes | Detects all directions | Very noise-sensitive |
| **Scharr** | Excellent | Fast | Medium | Precise gradient | More accurate than Sobel | Limited to 3x3 kernel |

### Feature Detection Algorithms

| Algorithm | Speed | Rotation Invariant | Scale Invariant | Patent Free | Best Use Case | Pros | Cons |
|-----------|-------|-------------------|-----------------|-------------|---------------|------|------|
| **SIFT** | Slow | Yes | Yes | No | Accurate matching | Most robust | Slow, patented |
| **SURF** | Medium | Yes | Yes | No | Faster than SIFT | Good accuracy, faster than SIFT | Patented |
| **ORB** | Fast | Yes | Partial | Yes | Real-time applications | Fast, free | Less accurate than SIFT |
| **AKAZE** | Medium | Yes | Yes | Yes | Good balance | Fast and accurate | Less features than SIFT |
| **FAST** | Fastest | No | No | Yes | Corner detection | Extremely fast | Not scale/rotation invariant |

### Color Spaces Comparison

| Color Space | Use Case | Advantages | Disadvantages |
|-------------|----------|------------|---------------|
| **BGR/RGB** | General display | Intuitive, hardware standard | Not perceptually uniform, poor for color detection |
| **HSV** | Color detection, segmentation | Separates color from intensity, easy thresholding | Hue wraps around (red at both ends) |
| **LAB** | Color correction, skin detection | Perceptually uniform, device-independent | Less intuitive, slower conversion |
| **Grayscale** | Edge/feature detection | Faster processing, simpler algorithms | Loses color information |
| **YCrCb** | JPEG compression, skin detection | Good for video compression | Less intuitive than HSV |

### Template Matching Methods

| Method | Formula | Match Location | Best For |
|--------|---------|----------------|----------|
| **TM_CCOEFF_NORMED** | Correlation coefficient | max_loc | General matching (recommended) |
| **TM_CCORR_NORMED** | Cross correlation | max_loc | Bright objects |
| **TM_SQDIFF_NORMED** | Squared difference | min_loc | Fast matching, lower quality |

### Morphological Operations

| Operation | Effect | Use Case | Formula |
|-----------|--------|----------|---------|
| **Erosion** | Shrinks white regions | Remove small noise | Minimum filter |
| **Dilation** | Expands white regions | Fill small holes | Maximum filter |
| **Opening** | Remove small white noise | Clean binary images | Erosion → Dilation |
| **Closing** | Fill small black holes | Connect components | Dilation → Erosion |
| **Gradient** | Find object boundaries | Edge detection | Dilation - Erosion |
| **Top Hat** | Find small bright spots | Enhance features | Image - Opening |
| **Black Hat** | Find small dark spots | Enhance features | Closing - Image |

---

## Best Practices

### Performance Optimization

```python
# 1. Work with smaller images when possible
resized = cv2.resize(img, None, fx=0.5, fy=0.5)

# 2. Convert to grayscale early if color not needed
gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)

# 3. Use appropriate data types
img_float = img.astype(np.float32)  # For precise calculations
img_uint8 = img.astype(np.uint8)    # For display and storage

# 4. ROI (Region of Interest) processing
roi = img[y1:y2, x1:x2]
processed_roi = cv2.GaussianBlur(roi, (5,5), 0)
img[y1:y2, x1:x2] = processed_roi
```

### Common Pitfalls

1. **Color Space**: OpenCV uses BGR, not RGB by default
2. **Coordinate System**: (x, y) vs [row, col] - img[y, x] not img[x, y]
3. **HSV Range**: Hue is 0-180 in OpenCV (not 0-360)
4. **Data Types**: Operations may require specific types (uint8, float32)
5. **Memory**: Release resources with `cv2.destroyAllWindows()`

---

## Quick Reference

### Common Parameters

| Function | Key Parameter | Typical Values | Effect |
|----------|---------------|----------------|--------|
| `GaussianBlur()` | kernel size | (3,3) to (9,9) | Larger = more blur |
| `Canny()` | threshold1, threshold2 | 50, 150 or 100, 200 | Lower = more edges |
| `inRange()` | lower, upper | HSV arrays | Defines color range |
| `threshold()` | thresh | 127 (mid) | Binary threshold value |
| `dilate()/erode()` | iterations | 1-5 | More = stronger effect |

### Installation

```bash
pip install opencv-python
pip install opencv-contrib-python  # Includes extra modules (SIFT, SURF, etc.)
```

### Import

```python
import cv2
import numpy as np
```
