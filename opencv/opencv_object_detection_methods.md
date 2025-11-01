# OpenCV 物体识别常见思路总结

本文档总结了使用OpenCV进行物体识别的常见方法和完整工作流程。

## 一、基础工作流程

```
图像采集 → 预处理 → 特征提取/检测 → 匹配/识别 → 后处理 → 结果输出
```

## 二、图像预处理技术

### 1. 颜色空间转换
```python
# 转换到灰度图
gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

# 转换到HSV（便于颜色识别）
hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
```

**使用场景**：
- 灰度图：减少计算量，适用于形状检测
- HSV：颜色识别更准确，光照变化鲁棒性好

### 2. 降噪处理

#### 高斯模糊 `cv2.GaussianBlur(src, ksize, sigmaX)`
```python
# 参数说明：
# ksize: 核大小，必须是奇数，如(3,3), (5,5), (7,7)
# sigmaX: X方向标准差，通常设为0让OpenCV自动计算

# 轻度模糊 - 去除轻微噪声，保留更多细节
blurred_light = cv2.GaussianBlur(gray, (3, 3), 0)

# 中度模糊 - 平衡噪声和细节（常用）
blurred_medium = cv2.GaussianBlur(gray, (5, 5), 0)

# 重度模糊 - 去除强噪声，会损失较多细节
blurred_heavy = cv2.GaussianBlur(gray, (9, 9), 0)
```

**使用场景**：
- `(3,3)`: 图像质量好，噪声少，需要精细边缘（OCR文字识别）
- `(5,5)`: 一般场景，平衡性能和效果（推荐作为默认值）
- `(7,7)` 或更大: 噪声严重的图像（低光照、压缩图片）

#### 中值滤波 `cv2.medianBlur(src, ksize)`
```python
# 参数说明：
# ksize: 核大小，必须是正奇数

# 轻度滤波
median_light = cv2.medianBlur(gray, 3)

# 标准滤波（推荐）
median_standard = cv2.medianBlur(gray, 5)

# 强力滤波
median_strong = cv2.medianBlur(gray, 7)
```

**使用场景**：
- `3`: 椒盐噪声较少，保留边缘清晰
- `5`: 中等椒盐噪声（最常用）
- `7-9`: 严重椒盐噪声，但会模糊小物体

#### 双边滤波 `cv2.bilateralFilter(src, d, sigmaColor, sigmaSpace)`
```python
# 参数说明：
# d: 滤波直径，-1表示自动计算
# sigmaColor: 颜色空间标准差，值越大颜色差异越大也会被平滑
# sigmaSpace: 坐标空间标准差，值越大越远的像素也会影响当前像素

# 保留强边缘
bilateral_sharp = cv2.bilateralFilter(gray, 5, 50, 50)

# 平衡模式（推荐）
bilateral_balanced = cv2.bilateralFilter(gray, 9, 75, 75)

# 强力降噪
bilateral_strong = cv2.bilateralFilter(gray, 9, 150, 150)
```

**使用场景**：
- `sigmaColor=50`: 需要保留所有明显边缘（人脸美颜）
- `sigmaColor=75`: 一般场景，适度平滑
- `sigmaColor=150`: 强力平滑，只保留主要边缘

**方法对比**：
| 方法 | 速度 | 边缘保留 | 去噪效果 | 适用场景 |
|-----|------|---------|---------|---------|
| 高斯模糊 | 快 | 差 | 一般 | 一般图像预处理 |
| 中值滤波 | 中 | 好 | 椒盐噪声好 | 有斑点噪声的图像 |
| 双边滤波 | 慢 | 最好 | 很好 | 需要保留边缘的场景 |

### 3. 二值化

#### 固定阈值 `cv2.threshold(src, thresh, maxval, type)`
```python
# 参数说明：
# thresh: 阈值，0-255
# maxval: 最大值，通常为255
# type: 二值化类型

# 低阈值 - 保留更多暗部信息，可能有噪声
_, binary_low = cv2.threshold(gray, 80, 255, cv2.THRESH_BINARY)

# 中等阈值 - 平衡模式（最常用）
_, binary_mid = cv2.threshold(gray, 127, 255, cv2.THRESH_BINARY)

# 高阈值 - 只保留明亮区域
_, binary_high = cv2.threshold(gray, 180, 255, cv2.THRESH_BINARY)

# 反向二值化 - 暗部变白色，亮部变黑色
_, binary_inv = cv2.threshold(gray, 127, 255, cv2.THRESH_BINARY_INV)
```

**阈值选择指南**：
- `50-100`: 检测暗色物体（黑色、深蓝色）
- `127`: 通用值，适合对比度好的图像
- `150-200`: 检测亮色物体（白色、黄色）
- 使用直方图分析选择合适阈值：`hist = cv2.calcHist([gray], [0], None, [256], [0,256])`

#### 自适应阈值 `cv2.adaptiveThreshold()`
```python
# 参数说明：
# adaptiveMethod: ADAPTIVE_THRESH_MEAN_C 或 ADAPTIVE_THRESH_GAUSSIAN_C
# blockSize: 邻域大小，必须是奇数
# C: 常数，从平均值中减去

# 小块处理 - 适合小文字、细节多的图像
adaptive_small = cv2.adaptiveThreshold(gray, 255,
    cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 7, 2)

# 中等块（推荐）- 一般场景
adaptive_medium = cv2.adaptiveThreshold(gray, 255,
    cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 11, 2)

# 大块处理 - 适合大物体、光照变化剧烈
adaptive_large = cv2.adaptiveThreshold(gray, 255,
    cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 21, 2)

# 均值方法 - 速度更快，但效果略差
adaptive_mean = cv2.adaptiveThreshold(gray, 255,
    cv2.ADAPTIVE_THRESH_MEAN_C, cv2.THRESH_BINARY, 11, 2)
```

**参数调优**：
- `blockSize=7-11`: 文档扫描、文字识别
- `blockSize=15-21`: 光照不均的大场景
- `C=0-5`: C值越大，二值化越严格（越少白色区域）

#### Otsu自动阈值（适合双峰直方图）
```python
# 自动计算最佳阈值
_, otsu = cv2.threshold(gray, 0, 255,
    cv2.THRESH_BINARY + cv2.THRESH_OTSU)

# 结合高斯模糊效果更好
blurred = cv2.GaussianBlur(gray, (5, 5), 0)
_, otsu_better = cv2.threshold(blurred, 0, 255,
    cv2.THRESH_BINARY + cv2.THRESH_OTSU)
```

**使用场景对比**：
| 方法 | 适用场景 | 优点 | 缺点 |
|-----|---------|------|------|
| 固定阈值 | 光照均匀、对比度高 | 快速、简单 | 光照不均失效 |
| 自适应阈值 | 光照不均、阴影多 | 鲁棒性好 | 速度较慢 |
| Otsu | 双峰直方图（前景背景分明） | 自动、无需调参 | 多峰或单峰失效 |

### 4. 形态学操作

#### 核（Kernel）的选择
```python
# 矩形核 - 最常用，各方向均匀处理
kernel_rect = cv2.getStructuringElement(cv2.MORPH_RECT, (5, 5))

# 椭圆核 - 圆形物体，更平滑
kernel_ellipse = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (5, 5))

# 十字核 - 特定方向的连接
kernel_cross = cv2.getStructuringElement(cv2.MORPH_CROSS, (5, 5))

# 不同大小的核
kernel_small = cv2.getStructuringElement(cv2.MORPH_RECT, (3, 3))  # 细节处理
kernel_medium = cv2.getStructuringElement(cv2.MORPH_RECT, (5, 5)) # 常用
kernel_large = cv2.getStructuringElement(cv2.MORPH_RECT, (7, 7))  # 强力处理
```

**核大小选择**：
- `(3,3)`: 精细操作，保留细节（文字、小物体）
- `(5,5)`: 一般操作（推荐默认值）
- `(7,7)以上`: 处理大噪声或大间隙

#### 腐蚀 `cv2.erode()` - 缩小白色区域
```python
# 轻度腐蚀 - 去除细小噪点
eroded_light = cv2.erode(binary, kernel_small, iterations=1)

# 标准腐蚀 - 一般去噪
eroded_standard = cv2.erode(binary, kernel_medium, iterations=1)

# 强力腐蚀 - 分离粘连物体
eroded_strong = cv2.erode(binary, kernel_medium, iterations=2)

# 超强腐蚀 - 严重粘连
eroded_super = cv2.erode(binary, kernel_large, iterations=3)
```

**使用场景**：
- `iterations=1`: 去除1-2像素的边缘噪声
- `iterations=2-3`: 分离轻微粘连的物体
- `iterations>3`: 分离严重粘连，但会显著缩小物体

#### 膨胀 `cv2.dilate()` - 扩大白色区域
```python
# 轻度膨胀 - 填充小孔
dilated_light = cv2.dilate(binary, kernel_small, iterations=1)

# 标准膨胀 - 连接断裂
dilated_standard = cv2.dilate(binary, kernel_medium, iterations=1)

# 强力膨胀 - 连接较大间隙
dilated_strong = cv2.dilate(binary, kernel_medium, iterations=2)
```

**使用场景**：
- `iterations=1`: 填充1-2像素的小孔
- `iterations=2-3`: 连接断裂的边缘
- `iterations>3`: 合并临近物体（需要时）

#### 开运算 `cv2.MORPH_OPEN` - 先腐蚀后膨胀
```python
# 去除小噪点，保持物体大小基本不变
opening_small = cv2.morphologyEx(binary, cv2.MORPH_OPEN, kernel_small)

# 去除中等噪声（推荐）
opening_medium = cv2.morphologyEx(binary, cv2.MORPH_OPEN, kernel_medium)

# 去除大噪声，也会去除小物体
opening_large = cv2.morphologyEx(binary, cv2.MORPH_OPEN, kernel_large)
```

**使用场景**：
- 二值化后有很多白色噪点
- 需要平滑物体边界
- 分离细微粘连的物体

#### 闭运算 `cv2.MORPH_CLOSE` - 先膨胀后腐蚀
```python
# 填充小孔
closing_small = cv2.morphologyEx(binary, cv2.MORPH_CLOSE, kernel_small)

# 填充中等孔洞（推荐）
closing_medium = cv2.morphologyEx(binary, cv2.MORPH_CLOSE, kernel_medium)

# 填充大孔洞，连接断裂
closing_large = cv2.morphologyEx(binary, cv2.MORPH_CLOSE, kernel_large)
```

**使用场景**：
- 物体内部有黑色孔洞
- 物体边缘有断裂
- 需要连接临近的物体

#### 其他形态学操作
```python
# 形态学梯度 - 突出边缘（膨胀-腐蚀）
gradient = cv2.morphologyEx(binary, cv2.MORPH_GRADIENT, kernel)

# 顶帽 - 突出亮细节（原图-开运算）
tophat = cv2.morphologyEx(gray, cv2.MORPH_TOPHAT, kernel)

# 黑帽 - 突出暗细节（闭运算-原图）
blackhat = cv2.morphologyEx(gray, cv2.MORPH_BLACKHAT, kernel)
```

**典型操作链**：
```python
# 去噪 + 填充孔洞（最常用）
kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (5, 5))
cleaned = cv2.morphologyEx(binary, cv2.MORPH_OPEN, kernel)   # 去噪
cleaned = cv2.morphologyEx(cleaned, cv2.MORPH_CLOSE, kernel) # 填充

# 分离粘连物体
kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (3, 3))
separated = cv2.erode(binary, kernel, iterations=2)
separated = cv2.dilate(separated, kernel, iterations=2)
```

## 三、物体检测方法

### 方法1：颜色识别

**原理**：基于HSV颜色空间进行颜色范围过滤

**步骤**：
```python
# 1. 转换到HSV
hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)

# 2. 定义颜色范围
lower_red = np.array([0, 100, 100])
upper_red = np.array([10, 255, 255])

# 3. 创建掩码
mask = cv2.inRange(hsv, lower_red, upper_red)

# 4. 形态学优化
kernel = np.ones((5, 5), np.uint8)
mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)

# 5. 找轮廓
contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL,
                                cv2.CHAIN_APPROX_SIMPLE)

# 6. 筛选轮廓
for contour in contours:
    area = cv2.contourArea(contour)
    if area > 500:  # 面积阈值
        x, y, w, h = cv2.boundingRect(contour)
        cv2.rectangle(image, (x, y), (x+w, y+h), (0, 255, 0), 2)
```

#### HSV颜色范围详解

**HSV空间说明**：
- **H (色调)**: 0-180，表示颜色类型
- **S (饱和度)**: 0-255，0是灰色，255是纯色
- **V (亮度)**: 0-255，0是黑色，255是最亮

**常见颜色HSV范围及使用场景**：

```python
# 红色（注意红色在HSV空间被分成两段）
red_lower1 = np.array([0, 100, 100])      # 鲜艳的红色
red_upper1 = np.array([10, 255, 255])
red_lower2 = np.array([160, 100, 100])    # 深红色
red_upper2 = np.array([180, 255, 255])

# 宽松红色范围（暗红、粉红也能检测）
red_loose_lower1 = np.array([0, 50, 50])
red_loose_upper1 = np.array([10, 255, 255])

# 严格红色范围（只检测纯红）
red_strict_lower = np.array([0, 150, 150])
red_strict_upper = np.array([10, 255, 255])

# 蓝色系
blue_light = ([90, 50, 50], [110, 255, 255])    # 浅蓝、天蓝
blue_standard = ([100, 100, 100], [130, 255, 255])  # 标准蓝色
blue_dark = ([100, 150, 50], [130, 255, 200])   # 深蓝

# 绿色系
green_light = ([35, 50, 50], [85, 255, 255])    # 黄绿到蓝绿
green_standard = ([40, 100, 100], [80, 255, 255])   # 标准绿
green_dark = ([40, 150, 50], [80, 255, 200])    # 深绿

# 黄色系
yellow_light = ([20, 50, 100], [35, 255, 255])  # 浅黄
yellow_standard = ([20, 100, 100], [30, 255, 255])  # 标准黄
yellow_gold = ([15, 150, 150], [30, 255, 255])  # 金黄色

# 橙色
orange = ([10, 100, 100], [25, 255, 255])

# 紫色/品红
purple = ([125, 50, 50], [160, 255, 255])

# 白色（高亮度，低饱和度）
white_bright = ([0, 0, 200], [180, 30, 255])    # 明亮的白色
white_loose = ([0, 0, 150], [180, 50, 255])     # 包含浅灰色

# 黑色（低亮度）
black_dark = ([0, 0, 0], [180, 255, 50])        # 深黑色
black_loose = ([0, 0, 0], [180, 255, 80])       # 包含深灰色

# 灰色（低饱和度，中等亮度）
gray = ([0, 0, 80], [180, 30, 200])
```

**参数调优指南**：

| 参数 | 调小效果 | 调大效果 | 典型场景 |
|-----|---------|---------|---------|
| S下限 | 包含更多浅色、灰色 | 只检测鲜艳颜色 | 游戏UI通常用100+，现实场景用50+ |
| V下限 | 包含暗色区域 | 只检测亮色区域 | 光照好用100+，暗处用50+ |
| H范围 | 颜色更精确 | 颜色容差更大 | 精确检测用±5，宽松检测用±10 |

**实用调色技巧**：
```python
def find_hsv_range(image_path, x, y):
    """点击图片获取该位置的HSV值及推荐范围"""
    img = cv2.imread(image_path)
    hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)
    pixel_hsv = hsv[y, x]

    print(f"点击位置的HSV值: {pixel_hsv}")

    # 推荐范围（上下浮动）
    h, s, v = pixel_hsv
    lower = np.array([max(0, h-10), max(0, s-50), max(0, v-50)])
    upper = np.array([min(180, h+10), min(255, s+50), min(255, v+50)])

    print(f"推荐下限: {lower}")
    print(f"推荐上限: {upper}")
    return lower, upper

# 使用滑动条实时调整
def create_hsv_trackbars():
    """创建HSV调整窗口"""
    cv2.namedWindow('HSV Adjuster')
    cv2.createTrackbar('H Lower', 'HSV Adjuster', 0, 180, lambda x: None)
    cv2.createTrackbar('H Upper', 'HSV Adjuster', 180, 180, lambda x: None)
    cv2.createTrackbar('S Lower', 'HSV Adjuster', 0, 255, lambda x: None)
    cv2.createTrackbar('S Upper', 'HSV Adjuster', 255, 255, lambda x: None)
    cv2.createTrackbar('V Lower', 'HSV Adjuster', 0, 255, lambda x: None)
    cv2.createTrackbar('V Upper', 'HSV Adjuster', 255, 255, lambda x: None)
```

**轮廓筛选参数**：
```python
# 面积过滤
if area > 500:          # 小物体（游戏小图标）
if area > 2000:         # 中等物体（游戏角色）
if area > 10000:        # 大物体（游戏Boss）

# 长宽比过滤
ratio = w / h
if 0.8 < ratio < 1.2:   # 近似正方形
if ratio > 2:           # 横向矩形（血条）
if ratio < 0.5:         # 纵向矩形（技能条）

# 轮廓完整性（周长平方/面积）
circularity = 4 * np.pi * area / (perimeter ** 2)
if circularity > 0.7:   # 近似圆形
```

**优点**：简单快速，实时性好，适合颜色鲜明的场景
**缺点**：受光照影响大，需要调参，不同光照条件下范围可能变化

### 方法2：模板匹配

**原理**：将模板图像在目标图像上滑动，计算相似度

**步骤**：
```python
# 读取模板
template = cv2.imread('template.png', 0)
h, w = template.shape

# 模板匹配
result = cv2.matchTemplate(gray, template, cv2.TM_CCOEFF_NORMED)

# 设置阈值
threshold = 0.8
locations = np.where(result >= threshold)

# 绘制结果
for pt in zip(*locations[::-1]):
    cv2.rectangle(image, pt, (pt[0] + w, pt[1] + h), (0, 255, 0), 2)
```

#### 匹配方法详解
```python
# 1. TM_CCOEFF_NORMED - 归一化相关系数（最推荐）
# 返回值: -1到1，1表示完美匹配
# 特点: 对光照变化鲁棒，最常用
result = cv2.matchTemplate(gray, template, cv2.TM_CCOEFF_NORMED)
threshold = 0.8  # 推荐阈值：0.7-0.9

# 2. TM_CCORR_NORMED - 归一化相关
# 返回值: 0到1，1表示完美匹配
# 特点: 受亮度影响较大，不推荐
result = cv2.matchTemplate(gray, template, cv2.TM_CCORR_NORMED)
threshold = 0.9  # 推荐阈值：0.85-0.95

# 3. TM_SQDIFF_NORMED - 归一化平方差
# 返回值: 0到1，0表示完美匹配（注意是相反的！）
# 特点: 对噪声敏感
result = cv2.matchTemplate(gray, template, cv2.TM_SQDIFF_NORMED)
threshold = 0.2  # 推荐阈值：0.1-0.3（小于阈值为匹配）
locations = np.where(result <= threshold)  # 注意是 <=

# 4. TM_CCOEFF - 相关系数（未归一化）
# 返回值: 可能为负数或很大的正数
# 特点: 需要动态阈值，不推荐
result = cv2.matchTemplate(gray, template, cv2.TM_CCOEFF)
min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)
threshold = max_val * 0.8  # 动态阈值
```

**阈值设置指南**：
| 场景 | TM_CCOEFF_NORMED阈值 | 说明 |
|-----|---------------------|------|
| 精确匹配（UI图标） | 0.85-0.95 | 要求高相似度 |
| 一般匹配（游戏元素） | 0.75-0.85 | 平衡准确率和召回率 |
| 宽松匹配（允许变化） | 0.65-0.75 | 容许一定差异 |
| 找所有相似物体 | 0.60-0.70 | 可能有误检 |

#### 多尺度匹配详解
```python
# 场景1: 物体大小变化不大（游戏UI）
# 尺度范围小，步数少，速度快
for scale in np.linspace(0.9, 1.1, 5):  # 90%到110%，5个尺度
    resized = cv2.resize(template, None, fx=scale, fy=scale)
    result = cv2.matchTemplate(image, resized, cv2.TM_CCOEFF_NORMED)
    # 处理...

# 场景2: 物体大小中等变化（一般场景）
# 平衡速度和精度
for scale in np.linspace(0.7, 1.3, 10):  # 70%到130%，10个尺度
    resized = cv2.resize(template, None, fx=scale, fy=scale)
    if resized.shape[0] > image.shape[0] or resized.shape[1] > image.shape[1]:
        continue
    result = cv2.matchTemplate(image, resized, cv2.TM_CCOEFF_NORMED)
    # 处理...

# 场景3: 物体大小变化很大
# 更大范围，更多步数
for scale in np.linspace(0.5, 2.0, 20):  # 50%到200%，20个尺度
    resized = cv2.resize(template, None, fx=scale, fy=scale)
    h, w = resized.shape[:2]
    if h > image.shape[0] or w > image.shape[1] or h < 10 or w < 10:
        continue
    result = cv2.matchTemplate(image, resized, cv2.TM_CCOEFF_NORMED)
    # 处理...

# 场景4: 金字塔加速搜索（先粗后精）
# 先用大步长找大致范围，再用小步长精确匹配
scales_coarse = np.linspace(0.5, 2.0, 8)   # 粗搜索
scales_fine = np.linspace(0.9, 1.1, 10)     # 精搜索
```

#### 模板预处理提高鲁棒性
```python
# 1. 边缘匹配 - 对光照不敏感
template_edge = cv2.Canny(template, 50, 150)
image_edge = cv2.Canny(gray, 50, 150)
result = cv2.matchTemplate(image_edge, template_edge, cv2.TM_CCOEFF_NORMED)

# 2. 直方图均衡化 - 应对光照变化
template_eq = cv2.equalizeHist(template)
image_eq = cv2.equalizeHist(gray)
result = cv2.matchTemplate(image_eq, template_eq, cv2.TM_CCOEFF_NORMED)

# 3. 模板归一化 - 减少亮度影响
template_norm = cv2.normalize(template, None, 0, 255, cv2.NORM_MINMAX)
image_norm = cv2.normalize(gray, None, 0, 255, cv2.NORM_MINMAX)
result = cv2.matchTemplate(image_norm, template_norm, cv2.TM_CCOEFF_NORMED)
```

#### 处理多个匹配结果
```python
# 方法1: 非极大值抑制（推荐）
threshold = 0.8
locations = np.where(result >= threshold)
boxes = []
for pt in zip(*locations[::-1]):
    boxes.append([pt[0], pt[1], w, h])

# NMS去重
def nms(boxes, overlap_thresh=0.3):
    # ... (见完整示例代码)
    pass

filtered_boxes = nms(boxes, overlap_thresh=0.3)

# 方法2: 只取最佳匹配
min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)
if max_val >= threshold:
    top_left = max_loc
    bottom_right = (top_left[0] + w, top_left[1] + h)
    cv2.rectangle(image, top_left, bottom_right, (0, 255, 0), 2)

# 方法3: 取前N个最佳匹配
n_matches = 5
flat_result = result.flatten()
top_indices = np.argsort(flat_result)[-n_matches:][::-1]
for idx in top_indices:
    y, x = divmod(idx, result.shape[1])
    if result[y, x] >= threshold:
        cv2.rectangle(image, (x, y), (x+w, y+h), (0, 255, 0), 2)
```

#### 性能优化技巧
```python
# 1. ROI区域搜索 - 只在感兴趣区域匹配
roi = image[100:500, 200:800]
result = cv2.matchTemplate(roi, template, cv2.TM_CCOEFF_NORMED)
# 记得把坐标加上ROI偏移：actual_x = roi_x + 200, actual_y = roi_y + 100

# 2. 降低分辨率 - 快速粗定位
image_small = cv2.resize(image, None, fx=0.5, fy=0.5)
template_small = cv2.resize(template, None, fx=0.5, fy=0.5)
result = cv2.matchTemplate(image_small, template_small, cv2.TM_CCOEFF_NORMED)
# 找到后在原图精确匹配

# 3. 缓存模板 - 避免重复读取
templates_cache = {}
def get_template(path):
    if path not in templates_cache:
        templates_cache[path] = cv2.imread(path, 0)
    return templates_cache[path]
```

**优点**：简单直观，精确度高，不需要训练
**缺点**：对旋转敏感（>5度失效），对尺度变化敏感，计算量较大，不能处理形变

### 方法3：边缘检测 + 轮廓分析

**原理**：检测边缘后提取轮廓，根据形状特征识别物体

#### Canny边缘检测参数详解
```python
# cv2.Canny(image, threshold1, threshold2, apertureSize, L2gradient)
# threshold1: 低阈值，threshold2: 高阈值
# 推荐比例: threshold2 = 2~3 * threshold1

# 场景1: 边缘清晰、噪声少（游戏界面、UI）
edges_clean = cv2.Canny(gray, 100, 200)

# 场景2: 一般场景（推荐）
edges_standard = cv2.Canny(gray, 50, 150)

# 场景3: 边缘模糊、需要检测弱边缘
edges_sensitive = cv2.Canny(gray, 30, 90)

# 场景4: 噪声多、只要强边缘
edges_strong = cv2.Canny(gray, 150, 300)

# 场景5: 自动阈值（基于图像中值）
median_val = np.median(gray)
lower = int(max(0, 0.7 * median_val))
upper = int(min(255, 1.3 * median_val))
edges_auto = cv2.Canny(gray, lower, upper)
```

**Canny参数调优指南**：
| 阈值设置 | 适用场景 | 边缘数量 | 噪声程度 |
|---------|---------|---------|---------|
| 30, 90 | 模糊图像、需要检测所有边缘 | 很多 | 可能多 |
| 50, 150 | 一般图像（推荐默认值） | 适中 | 较少 |
| 100, 200 | 清晰图像、只要主要边缘 | 较少 | 很少 |
| 150, 300 | 高噪声、只要强边缘 | 少 | 极少 |

#### 轮廓检测模式 `cv2.findContours()`
```python
# 参数1: retrieval mode - 轮廓检索模式
# 参数2: approximation method - 轮廓近似方法

# 模式1: RETR_EXTERNAL - 只检测最外层轮廓
# 适用: 只关心外轮廓，不关心内部孔洞（最常用）
contours, _ = cv2.findContours(binary, cv2.RETR_EXTERNAL,
                                cv2.CHAIN_APPROX_SIMPLE)

# 模式2: RETR_LIST - 检测所有轮廓，不建立层次关系
# 适用: 需要所有轮廓，但不关心包含关系
contours, _ = cv2.findContours(binary, cv2.RETR_LIST,
                                cv2.CHAIN_APPROX_SIMPLE)

# 模式3: RETR_TREE - 建立完整层次树
# 适用: 需要知道轮廓的父子关系
contours, hierarchy = cv2.findContours(binary, cv2.RETR_TREE,
                                        cv2.CHAIN_APPROX_SIMPLE)

# 模式4: RETR_CCOMP - 两层层次结构
# 适用: 区分外边界和内孔洞
contours, hierarchy = cv2.findContours(binary, cv2.RETR_CCOMP,
                                        cv2.CHAIN_APPROX_SIMPLE)

# 近似方法1: CHAIN_APPROX_SIMPLE - 压缩轮廓（推荐）
# 只保存端点，如矩形只保存4个角点
contours, _ = cv2.findContours(binary, cv2.RETR_EXTERNAL,
                                cv2.CHAIN_APPROX_SIMPLE)

# 近似方法2: CHAIN_APPROX_NONE - 保存所有点
# 保存轮廓上所有点，占用更多内存
contours, _ = cv2.findContours(binary, cv2.RETR_EXTERNAL,
                                cv2.CHAIN_APPROX_NONE)
```

#### 轮廓筛选参数详解
```python
for contour in contours:
    # 1. 面积过滤 - 最常用的过滤方法
    area = cv2.contourArea(contour)

    # 小物体（游戏小图标、按钮）
    if 100 < area < 5000:
        pass

    # 中等物体（游戏角色、道具）
    if 1000 < area < 50000:
        pass

    # 大物体（游戏Boss、大型UI）
    if area > 10000:
        pass

    # 2. 周长过滤
    perimeter = cv2.arcLength(contour, True)
    if perimeter < 50:  # 过滤太小的轮廓
        continue

    # 3. 长宽比过滤
    x, y, w, h = cv2.boundingRect(contour)
    aspect_ratio = float(w) / h

    if 0.9 < aspect_ratio < 1.1:  # 正方形
        shape = "正方形"
    elif aspect_ratio > 2:         # 横向长条（血条、进度条）
        shape = "横条"
    elif aspect_ratio < 0.5:       # 纵向长条
        shape = "竖条"
    else:                          # 一般矩形
        shape = "矩形"

    # 4. 圆形度过滤（4π*面积/周长²）
    circularity = 4 * np.pi * area / (perimeter * perimeter)
    if circularity > 0.85:    # 接近圆形
        shape = "圆形"
    elif circularity > 0.7:   # 椭圆或圆角矩形
        shape = "椭圆"

    # 5. 凸性检测
    hull = cv2.convexHull(contour)
    hull_area = cv2.contourArea(hull)
    solidity = area / hull_area if hull_area > 0 else 0

    if solidity > 0.95:       # 凸形状（圆、矩形）
        is_convex = True
    elif solidity < 0.8:      # 凹形状（星形、月牙形）
        is_convex = False

    # 6. 范围过滤
    extent = area / (w * h)   # 轮廓面积占外接矩形的比例
    if extent > 0.9:          # 几乎填满矩形（矩形）
        pass
    elif extent > 0.75:       # 大部分填充（圆、椭圆）
        pass
```

#### 轮廓近似详解
```python
# cv2.approxPolyDP(contour, epsilon, closed)
# epsilon: 近似精度，越小越接近原轮廓

perimeter = cv2.arcLength(contour, True)

# 精细近似 - 保留更多细节
approx_fine = cv2.approxPolyDP(contour, 0.01 * perimeter, True)

# 标准近似（推荐）
approx_standard = cv2.approxPolyDP(contour, 0.02 * perimeter, True)

# 粗略近似 - 简化形状
approx_coarse = cv2.approxPolyDP(contour, 0.04 * perimeter, True)

# 非常粗略 - 只保留主要角点
approx_very_coarse = cv2.approxPolyDP(contour, 0.1 * perimeter, True)

# 根据顶点数识别形状
vertices = len(approx_standard)
if vertices == 3:
    shape = "三角形"
elif vertices == 4:
    # 进一步判断是矩形还是正方形
    x, y, w, h = cv2.boundingRect(approx_standard)
    ratio = float(w) / h
    if 0.95 <= ratio <= 1.05:
        shape = "正方形"
    else:
        shape = "矩形"
elif vertices == 5:
    shape = "五边形"
elif vertices == 6:
    shape = "六边形"
elif vertices > 6:
    # 可能是圆形或复杂形状
    circularity = 4 * np.pi * area / (perimeter * perimeter)
    if circularity > 0.8:
        shape = "圆形"
    else:
        shape = "多边形"
```

#### 实用形状检测函数
```python
def detect_shape(contour):
    """综合多个特征检测形状"""
    area = cv2.contourArea(contour)
    perimeter = cv2.arcLength(contour, True)

    # 过滤太小的轮廓
    if area < 100:
        return None

    # 轮廓近似
    approx = cv2.approxPolyDP(contour, 0.04 * perimeter, True)
    vertices = len(approx)

    # 获取外接矩形
    x, y, w, h = cv2.boundingRect(contour)
    aspect_ratio = float(w) / h

    # 圆形度
    circularity = 4 * np.pi * area / (perimeter * perimeter)

    # 形状判断
    if vertices == 3:
        return "三角形"
    elif vertices == 4:
        if 0.95 <= aspect_ratio <= 1.05:
            return "正方形"
        else:
            return "矩形"
    elif circularity > 0.8:
        return "圆形"
    elif 5 <= vertices <= 6:
        return f"{vertices}边形"
    else:
        return "复杂形状"

# 完整的轮廓筛选流程
def filter_contours(contours, min_area=500, max_area=50000,
                    aspect_ratio_range=(0.5, 2.0)):
    """筛选符合条件的轮廓"""
    filtered = []
    for contour in contours:
        area = cv2.contourArea(contour)

        # 面积过滤
        if not (min_area < area < max_area):
            continue

        # 长宽比过滤
        x, y, w, h = cv2.boundingRect(contour)
        aspect_ratio = float(w) / h
        if not (aspect_ratio_range[0] < aspect_ratio < aspect_ratio_range[1]):
            continue

        filtered.append(contour)

    return filtered
```

**优点**：对形状识别效果好，不依赖颜色，可以检测复杂形状
**缺点**：对噪声敏感，需要良好的预处理，参数调节相对复杂

### 方法4：特征点匹配（SIFT/ORB）

**原理**：提取图像关键点和特征描述符，进行匹配

#### ORB（专利免费，速度快）
```python
# cv2.ORB_create(nfeatures, scaleFactor, nlevels, edgeThreshold,
#                firstLevel, WTA_K, scoreType, patchSize, fastThreshold)

# 场景1: 高速检测（游戏实时检测）
orb_fast = cv2.ORB_create(
    nfeatures=500,      # 特征点数量：500个足够快速匹配
    scaleFactor=1.2,    # 金字塔缩放因子
    nlevels=8           # 金字塔层数
)

# 场景2: 一般检测（推荐）
orb_standard = cv2.ORB_create(
    nfeatures=1000,     # 1000个特征点，平衡速度和精度
    scaleFactor=1.2,
    nlevels=8,
    edgeThreshold=31,   # 边缘阈值，忽略边缘附近的特征
    patchSize=31        # 特征patch大小
)

# 场景3: 高精度检测
orb_accurate = cv2.ORB_create(
    nfeatures=2000,     # 更多特征点
    scaleFactor=1.1,    # 更密集的金字塔（更小的缩放）
    nlevels=12,         # 更多层
    edgeThreshold=15,   # 更低阈值，检测更多边缘特征
    fastThreshold=10    # 更敏感的FAST检测
)

# 检测关键点和描述符
kp1, des1 = orb_standard.detectAndCompute(template_gray, None)
kp2, des2 = orb_standard.detectAndCompute(scene_gray, None)

print(f"模板特征点数: {len(kp1)}, 场景特征点数: {len(kp2)}")
```

**ORB参数说明**：
| 参数 | 小值效果 | 大值效果 | 推荐值 |
|-----|---------|---------|--------|
| nfeatures | 速度快，可能漏检 | 精度高，速度慢 | 500-2000 |
| scaleFactor | 尺度不变性差，精度高 | 速度快，鲁棒性好 | 1.2 |
| nlevels | 尺度不变性差 | 处理更大尺度变化 | 8-12 |
| edgeThreshold | 检测更多边缘特征 | 更稳定的特征 | 15-31 |

#### 特征匹配器详解
```python
# 方法1: BFMatcher（暴力匹配）- 精确但慢
# NORM_HAMMING: 用于ORB、BRIEF等二进制描述符
# NORM_L2: 用于SIFT、SURF等浮点描述符
# crossCheck=True: 双向匹配，更可靠但匹配数减少

# ORB匹配（推荐crossCheck）
bf = cv2.BFMatcher(cv2.NORM_HAMMING, crossCheck=True)
matches = bf.match(des1, des2)
matches = sorted(matches, key=lambda x: x.distance)

# 不使用crossCheck，用KNN匹配
bf_knn = cv2.BFMatcher(cv2.NORM_HAMMING, crossCheck=False)
matches_knn = bf_knn.knnMatch(des1, des2, k=2)

# Lowe's Ratio Test（比率测试）- 筛选好的匹配
good_matches = []
for m, n in matches_knn:
    # 最近距离/次近距离 < 0.75，说明匹配可靠
    if m.distance < 0.75 * n.distance:  # 0.75是典型值
        good_matches.append(m)

# 方法2: FLANN（快速近似匹配）- 适合大量特征点
FLANN_INDEX_LSH = 6
index_params = dict(
    algorithm=FLANN_INDEX_LSH,
    table_number=12,    # 12-20，越大越精确但越慢
    key_size=20,        # 20-25
    multi_probe_level=2 # 1-2，越大越精确
)
search_params = dict(checks=50)  # 50-100，检查的叶子数
flann = cv2.FlannBasedMatcher(index_params, search_params)
matches = flann.knnMatch(des1, des2, k=2)
```

**匹配筛选参数**：
```python
# 距离阈值筛选
good_matches = [m for m in matches if m.distance < 30]  # ORB距离通常0-256

# 取前N个最佳匹配
n_best = 50
good_matches = sorted(matches, key=lambda x: x.distance)[:n_best]

# Ratio Test阈值选择
ratio_strict = 0.7      # 严格，匹配数少但更可靠
ratio_standard = 0.75   # 标准（推荐）
ratio_loose = 0.8       # 宽松，更多匹配但可能有误匹配
```

#### 单应性矩阵和物体定位
```python
# 需要足够的匹配点（至少4个，推荐10+）
MIN_MATCH_COUNT = 10

if len(good_matches) > MIN_MATCH_COUNT:
    # 提取匹配点坐标
    src_pts = np.float32([kp1[m.queryIdx].pt for m in good_matches]).reshape(-1, 1, 2)
    dst_pts = np.float32([kp2[m.trainIdx].pt for m in good_matches]).reshape(-1, 1, 2)

    # 计算单应性矩阵
    # method: RANSAC（推荐）或 LMEDS
    # ransacReprojThreshold: 点被认为是内点的最大距离

    # 标准RANSAC
    M, mask = cv2.findHomography(src_pts, dst_pts, cv2.RANSAC, 5.0)

    # 宽松RANSAC - 更多内点，但可能不太精确
    M, mask = cv2.findHomography(src_pts, dst_pts, cv2.RANSAC, 8.0)

    # 严格RANSAC - 更精确，但可能内点少
    M, mask = cv2.findHomography(src_pts, dst_pts, cv2.RANSAC, 3.0)

    # 统计内点数量
    inliers = mask.ravel().tolist()
    n_inliers = sum(inliers)
    inlier_ratio = n_inliers / len(good_matches)

    print(f"内点数: {n_inliers}/{len(good_matches)}, 比例: {inlier_ratio:.2f}")

    # 只有足够的内点才认为找到了物体
    if inlier_ratio > 0.5:  # 至少50%是内点
        # 获取模板四个角点
        h, w = template_gray.shape
        pts = np.float32([[0, 0], [0, h], [w, h], [w, 0]]).reshape(-1, 1, 2)

        # 变换到场景图像
        dst = cv2.perspectiveTransform(pts, M)

        # 绘制边界框
        scene_with_box = scene.copy()
        cv2.polylines(scene_with_box, [np.int32(dst)], True, (0, 255, 0), 3)

        # 计算中心点
        center = np.mean(dst, axis=0)[0]
        print(f"物体中心: ({int(center[0])}, {int(center[1])})")
else:
    print(f"匹配点不足: {len(good_matches)}/{MIN_MATCH_COUNT}")
```

#### SIFT（更精确，需要opencv-contrib）
```python
# cv2.SIFT_create(nfeatures, nOctaveLayers, contrastThreshold,
#                 edgeThreshold, sigma)

# 场景1: 快速检测
sift_fast = cv2.SIFT_create(
    nfeatures=500,          # 特征点数量
    nOctaveLayers=3,        # 每组层数
    contrastThreshold=0.04, # 对比度阈值（越大特征点越少）
    edgeThreshold=10        # 边缘阈值
)

# 场景2: 标准检测（推荐）
sift_standard = cv2.SIFT_create(
    nfeatures=0,            # 0表示不限制数量
    nOctaveLayers=3,
    contrastThreshold=0.04,
    edgeThreshold=10,
    sigma=1.6               # 高斯模糊参数
)

# 场景3: 高精度检测
sift_accurate = cv2.SIFT_create(
    nfeatures=0,
    nOctaveLayers=4,        # 更多层，检测更多尺度
    contrastThreshold=0.03, # 更低阈值，更多特征点
    edgeThreshold=15,       # 更高值，过滤更多边缘
    sigma=1.6
)

# 检测关键点和描述符
kp1, des1 = sift_standard.detectAndCompute(template, None)
kp2, des2 = sift_standard.detectAndCompute(scene, None)

# FLANN匹配器（SIFT推荐使用FLANN）
FLANN_INDEX_KDTREE = 1
index_params = dict(
    algorithm=FLANN_INDEX_KDTREE,
    trees=5                 # 5-10，越大越精确但越慢
)
search_params = dict(
    checks=50              # 50-100，检查的叶子数
)
flann = cv2.FlannBasedMatcher(index_params, search_params)

matches = flann.knnMatch(des1, des2, k=2)

# Lowe's ratio test
good_matches = []
for m, n in matches:
    if m.distance < 0.7 * n.distance:  # 0.7是SIFT推荐值
        good_matches.append(m)
```

**SIFT参数说明**：
| 参数 | 小值效果 | 大值效果 | 推荐值 |
|-----|---------|---------|--------|
| contrastThreshold | 更多特征点，可能有噪声 | 更少但更稳定的特征 | 0.03-0.04 |
| edgeThreshold | 更多边缘特征 | 过滤边缘，更稳定 | 10 |
| nOctaveLayers | 速度快，尺度覆盖少 | 更多尺度，更鲁棒 | 3-4 |

**ORB vs SIFT 对比**：
| 特性 | ORB | SIFT |
|-----|-----|------|
| 速度 | 快（10-100倍） | 慢 |
| 精度 | 良好 | 优秀 |
| 旋转不变性 | 有 | 有 |
| 尺度不变性 | 有 | 更好 |
| 专利 | 免费 | 专利过期（可免费使用） |
| 适用场景 | 实时检测、游戏辅助 | 高精度要求、离线处理 |

**优点**：对旋转、缩放、视角变化、光照变化鲁棒性强，精度高
**缺点**：计算量大，速度较慢（SIFT尤其慢），ORB对模糊图像效果较差

### 方法5：级联分类器（Haar/LBP）

**原理**：使用预训练或自训练的级联分类器

#### 使用预训练分类器
```python
# 常见的预训练分类器
face_cascade = cv2.CascadeClassifier(
    cv2.data.haarcascades + 'haarcascade_frontalface_default.xml')
eye_cascade = cv2.CascadeClassifier(
    cv2.data.haarcascades + 'haarcascade_eye.xml')
smile_cascade = cv2.CascadeClassifier(
    cv2.data.haarcascades + 'haarcascade_smile.xml')

# detectMultiScale参数详解
# scaleFactor: 图像缩放比例，每次搜索窗口的缩放系数
# minNeighbors: 每个候选矩形应该保留的邻近矩形数量
# minSize/maxSize: 检测目标的最小/最大尺寸

# 场景1: 高召回率（检测尽可能多，可能有误检）
faces_high_recall = face_cascade.detectMultiScale(
    gray,
    scaleFactor=1.05,    # 小步长，更密集搜索
    minNeighbors=3,      # 低阈值，更容易检测到
    minSize=(20, 20),    # 较小的最小尺寸
    maxSize=(300, 300)
)

# 场景2: 平衡模式（推荐）
faces_balanced = face_cascade.detectMultiScale(
    gray,
    scaleFactor=1.1,     # 标准步长
    minNeighbors=5,      # 标准阈值
    minSize=(30, 30),
    maxSize=(200, 200)
)

# 场景3: 高精确率（减少误检，可能漏检）
faces_high_precision = face_cascade.detectMultiScale(
    gray,
    scaleFactor=1.2,     # 大步长，更快但可能漏检
    minNeighbors=7,      # 高阈值，更严格
    minSize=(50, 50),    # 较大的最小尺寸
    flags=cv2.CASCADE_SCALE_IMAGE
)

# 场景4: 快速检测（牺牲精度）
faces_fast = face_cascade.detectMultiScale(
    gray,
    scaleFactor=1.3,     # 很大的步长
    minNeighbors=4,
    minSize=(40, 40)
)

# 绘制结果
for (x, y, w, h) in faces_balanced:
    cv2.rectangle(image, (x, y), (x+w, y+h), (255, 0, 0), 2)

    # 在人脸区域内检测眼睛
    roi_gray = gray[y:y+h, x:x+w]
    eyes = eye_cascade.detectMultiScale(roi_gray, 1.1, 10)
    for (ex, ey, ew, eh) in eyes:
        cv2.rectangle(image, (x+ex, y+ey), (x+ex+ew, y+ey+eh), (0, 255, 0), 2)
```

**参数调优指南**：
| 参数 | 小值效果 | 大值效果 | 典型场景 |
|-----|---------|---------|---------|
| scaleFactor | 精度高，速度慢（1.05-1.1） | 速度快，可能漏检（1.3-1.5） | 人脸检测: 1.1 |
| minNeighbors | 更多检测，误检多（3-4） | 更精确，可能漏检（6-8） | 人脸检测: 5 |
| minSize | 检测小物体，计算量大 | 忽略小物体，更快 | 根据实际物体大小 |

#### 训练自定义分类器

**准备数据**：
```bash
# 需要准备：
# 1. 正样本：包含目标物体的图像（至少1000张）
# 2. 负样本：不包含目标物体的图像（至少2000张）

# 创建正样本描述文件 positive.txt
# 格式: 图片路径 目标数量 x y w h
positive_images/img1.jpg 1 50 50 100 100
positive_images/img2.jpg 1 30 40 80 90

# 创建负样本描述文件 negative.txt
# 格式: 图片路径
negative_images/img1.jpg
negative_images/img2.jpg

# 生成样本向量文件
opencv_createsamples -info positive.txt -vec positive.vec \
  -num 1000 -w 24 -h 24
```

**训练分类器**：
```bash
# opencv_traincascade参数说明
opencv_traincascade \
  -data classifier_output \      # 输出目录
  -vec positive.vec \             # 正样本向量文件
  -bg negative.txt \              # 负样本列表
  -numPos 900 \                   # 每阶段使用的正样本数（略少于总数）
  -numNeg 500 \                   # 每阶段使用的负样本数
  -numStages 20 \                 # 级联阶段数（10-25）
  -w 24 -h 24 \                   # 样本宽高
  -featureType HAAR \             # 特征类型: HAAR或LBP
  -minHitRate 0.999 \            # 每阶段最小检测率（0.99-0.999）
  -maxFalseAlarmRate 0.5 \       # 每阶段最大误检率（0.4-0.5）
  -mode ALL                       # 特征模式

# 快速训练（LBP特征，速度快但精度稍低）
opencv_traincascade \
  -data classifier_lbp \
  -vec positive.vec \
  -bg negative.txt \
  -numPos 900 -numNeg 500 \
  -numStages 15 \
  -w 24 -h 24 \
  -featureType LBP \
  -minHitRate 0.995 \
  -maxFalseAlarmRate 0.5

# 高精度训练（HAAR特征，较慢但精度高）
opencv_traincascade \
  -data classifier_haar \
  -vec positive.vec \
  -bg negative.txt \
  -numPos 900 -numNeg 500 \
  -numStages 20 \
  -w 24 -h 24 \
  -featureType HAAR \
  -minHitRate 0.999 \
  -maxFalseAlarmRate 0.4
```

**训练参数说明**：
| 参数 | 说明 | 推荐值 |
|-----|------|--------|
| numStages | 级联阶段数，越多越精确但越慢 | 15-20 |
| minHitRate | 每阶段最小检测率，越高越不容易漏检 | 0.995-0.999 |
| maxFalseAlarmRate | 每阶段最大误检率，越低误检越少 | 0.4-0.5 |
| featureType | HAAR精度高但慢，LBP快但精度稍低 | 看需求 |

**使用自定义分类器**：
```python
# 加载训练好的分类器
custom_cascade = cv2.CascadeClassifier('classifier_output/cascade.xml')

# 检测
objects = custom_cascade.detectMultiScale(
    gray,
    scaleFactor=1.1,
    minNeighbors=5,
    minSize=(30, 30)
)

for (x, y, w, h) in objects:
    cv2.rectangle(image, (x, y), (x+w, y+h), (0, 255, 0), 2)
```

**优点**：速度快（实时性好），CPU友好，适合特定物体检测（人脸、车牌等）
**缺点**：训练复杂耗时，需要大量样本，泛化能力有限，对姿态变化敏感

### 方法6：深度学习方法

**使用预训练模型（YOLO/SSD/Faster R-CNN）**：

```python
# YOLOv3示例
net = cv2.dnn.readNet("yolov3.weights", "yolov3.cfg")
layer_names = net.getLayerNames()
output_layers = [layer_names[i - 1] for i in net.getUnconnectedOutLayers()]

# 加载类别
with open("coco.names", "r") as f:
    classes = [line.strip() for line in f.readlines()]

# 检测
blob = cv2.dnn.blobFromImage(image, 0.00392, (416, 416), (0, 0, 0), True)
net.setInput(blob)
outputs = net.forward(output_layers)

# 解析结果
boxes = []
confidences = []
class_ids = []

for output in outputs:
    for detection in output:
        scores = detection[5:]
        class_id = np.argmax(scores)
        confidence = scores[class_id]

        if confidence > 0.5:
            center_x = int(detection[0] * width)
            center_y = int(detection[1] * height)
            w = int(detection[2] * width)
            h = int(detection[3] * height)

            x = int(center_x - w / 2)
            y = int(center_y - h / 2)

            boxes.append([x, y, w, h])
            confidences.append(float(confidence))
            class_ids.append(class_id)

# 非极大值抑制（NMS）参数详解
# confThreshold: 置信度阈值，低于此值的检测被忽略
# nmsThreshold: NMS阈值，IoU大于此值的框会被抑制

# 场景1: 高召回率（检测更多物体）
indices = cv2.dnn.NMSBoxes(boxes, confidences, 0.3, 0.5)

# 场景2: 平衡模式（推荐）
indices = cv2.dnn.NMSBoxes(boxes, confidences, 0.5, 0.4)

# 场景3: 高精确率（只要高置信度的检测）
indices = cv2.dnn.NMSBoxes(boxes, confidences, 0.7, 0.3)

for i in indices:
    box = boxes[i]
    x, y, w, h = box
    label = str(classes[class_ids[i]])
    confidence = confidences[i]

    # 绘制边框
    cv2.rectangle(image, (x, y), (x+w, y+h), (0, 255, 0), 2)

    # 添加标签和置信度
    text = f"{label}: {confidence:.2f}"
    cv2.putText(image, text, (x, y-10), cv2.FONT_HERSHEY_SIMPLEX,
                0.5, (0, 255, 0), 2)
```

#### YOLO参数详解

**输入图像处理**：
```python
# blobFromImage参数说明：
# scalefactor: 缩放因子，将像素值归一化
# size: 网络输入尺寸
# mean: 减去的均值
# swapRB: 是否交换R和B通道

# 场景1: YOLOv3标准配置
blob = cv2.dnn.blobFromImage(image, 1/255.0, (416, 416), (0, 0, 0), True, crop=False)

# 场景2: 高精度（更大输入，更慢）
blob = cv2.dnn.blobFromImage(image, 1/255.0, (608, 608), (0, 0, 0), True, crop=False)

# 场景3: 快速检测（较小输入，更快）
blob = cv2.dnn.blobFromImage(image, 1/255.0, (320, 320), (0, 0, 0), True, crop=False)

# 场景4: YOLOv4/v5（可能需要不同的归一化）
blob = cv2.dnn.blobFromImage(image, 1/255.0, (640, 640), (0, 0, 0), True, crop=False)
```

**阈值设置指南**：
| 阈值类型 | 低值 | 中值（推荐） | 高值 | 适用场景 |
|---------|------|------------|------|---------|
| 置信度阈值 | 0.3 | 0.5 | 0.7 | 低：检测所有可能；高：只要确定的 |
| NMS阈值 | 0.3 | 0.4 | 0.5 | 低：去重更严格；高：保留更多框 |

#### 使用其他深度学习模型

**SSD (Single Shot Detector)**：
```python
# 加载SSD模型
net = cv2.dnn.readNetFromCaffe(
    'MobileNetSSD_deploy.prototxt',
    'MobileNetSSD_deploy.caffemodel'
)

# 预处理
blob = cv2.dnn.blobFromImage(
    cv2.resize(image, (300, 300)),  # SSD通常使用300x300
    0.007843,                        # 缩放因子：1/127.5
    (300, 300),
    127.5                            # 均值
)

net.setInput(blob)
detections = net.forward()

# 解析结果
for i in range(detections.shape[2]):
    confidence = detections[0, 0, i, 2]

    if confidence > 0.5:  # 置信度阈值
        class_id = int(detections[0, 0, i, 1])
        box = detections[0, 0, i, 3:7] * np.array([w, h, w, h])
        (x1, y1, x2, y2) = box.astype("int")

        cv2.rectangle(image, (x1, y1), (x2, y2), (0, 255, 0), 2)
```

**Faster R-CNN**：
```python
# 加载Faster R-CNN模型（TensorFlow）
net = cv2.dnn.readNetFromTensorflow(
    'frozen_inference_graph.pb',
    'graph.pbtxt'
)

blob = cv2.dnn.blobFromImage(image, 1.0, (300, 300), (0, 0, 0), True)
net.setInput(blob)
output = net.forward()

# 处理输出...
```

#### 深度学习模型对比

| 模型 | 速度 | 精度 | 输入尺寸 | 适用场景 |
|-----|------|------|---------|---------|
| YOLOv3 | 中 | 高 | 320-608 | 实时通用检测 |
| YOLOv4 | 中快 | 很高 | 416-640 | 高精度实时检测 |
| YOLOv5 | 快 | 高 | 640 | 工业应用、边缘设备 |
| SSD | 快 | 中 | 300 | 移动设备、快速检测 |
| Faster R-CNN | 慢 | 很高 | 可变 | 离线高精度检测 |

#### 性能优化

```python
# 1. 使用GPU加速
net.setPreferableBackend(cv2.dnn.DNN_BACKEND_CUDA)
net.setPreferableTarget(cv2.dnn.DNN_TARGET_CUDA)

# 2. 使用OpenVINO（Intel CPU优化）
net.setPreferableBackend(cv2.dnn.DNN_BACKEND_INFERENCE_ENGINE)
net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)

# 3. 降低输入分辨率
blob = cv2.dnn.blobFromImage(image, 1/255.0, (320, 320), (0, 0, 0), True)

# 4. 批处理（处理多张图片）
blobs = [cv2.dnn.blobFromImage(img, 1/255.0, (416, 416), (0, 0, 0), True)
         for img in images]
# 处理每个blob...

# 5. 只检测特定类别
CLASSES_TO_DETECT = ['person', 'car', 'dog']
for i in indices:
    label = classes[class_ids[i]]
    if label in CLASSES_TO_DETECT:
        # 处理检测结果
        pass
```

#### 模型下载和使用

```python
# 下载YOLO模型文件：
# yolov3.weights: https://pjreddie.com/media/files/yolov3.weights
# yolov3.cfg: https://github.com/pjreddie/darknet/blob/master/cfg/yolov3.cfg
# coco.names: https://github.com/pjreddie/darknet/blob/master/data/coco.names

# 或使用opencv自带的模型下载工具
import urllib.request

def download_model(url, filename):
    """下载模型文件"""
    print(f"正在下载 {filename}...")
    urllib.request.urlretrieve(url, filename)
    print(f"下载完成: {filename}")

# 使用示例
# download_model('https://pjreddie.com/media/files/yolov3.weights', 'yolov3.weights')
```

**优点**：精度最高，可识别多种物体（80+类别），泛化能力强，不需要特征工程
**缺点**：需要下载大模型文件（100MB+），计算资源要求高，推理速度较慢（CPU上），需要GPU加速才能实时

## 四、完整示例：游戏物体识别

```python
import cv2
import numpy as np

class GameObjectDetector:
    def __init__(self):
        self.hsv_ranges = {
            'red_item': ([0, 100, 100], [10, 255, 255]),
            'blue_enemy': ([100, 100, 100], [130, 255, 255])
        }

    def preprocess(self, image):
        """预处理"""
        # 降噪
        blurred = cv2.GaussianBlur(image, (5, 5), 0)
        # 转HSV
        hsv = cv2.cvtColor(blurred, cv2.COLOR_BGR2HSV)
        return hsv

    def detect_by_color(self, image, color_name):
        """颜色检测"""
        hsv = self.preprocess(image)

        # 获取颜色范围
        lower, upper = self.hsv_ranges[color_name]
        lower = np.array(lower)
        upper = np.array(upper)

        # 创建掩码
        mask = cv2.inRange(hsv, lower, upper)

        # 形态学处理
        kernel = np.ones((5, 5), np.uint8)
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)

        # 找轮廓
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL,
                                        cv2.CHAIN_APPROX_SIMPLE)

        results = []
        for contour in contours:
            area = cv2.contourArea(contour)
            if area > 500:  # 面积阈值
                x, y, w, h = cv2.boundingRect(contour)
                results.append({
                    'x': x, 'y': y, 'w': w, 'h': h,
                    'center': (x + w//2, y + h//2),
                    'area': area
                })

        return results

    def detect_by_template(self, image, template, threshold=0.8):
        """模板匹配"""
        gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
        template_gray = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY)

        h, w = template_gray.shape

        # 多尺度匹配
        results = []
        for scale in np.linspace(0.8, 1.2, 10):
            resized = cv2.resize(template_gray, None, fx=scale, fy=scale)
            if resized.shape[0] > gray.shape[0] or resized.shape[1] > gray.shape[1]:
                continue

            result = cv2.matchTemplate(gray, resized, cv2.TM_CCOEFF_NORMED)
            locations = np.where(result >= threshold)

            for pt in zip(*locations[::-1]):
                results.append({
                    'x': pt[0], 'y': pt[1],
                    'w': int(w * scale), 'h': int(h * scale),
                    'confidence': result[pt[1], pt[0]]
                })

        # 非极大值抑制
        results = self.nms(results)
        return results

    def nms(self, boxes, overlap_thresh=0.3):
        """非极大值抑制"""
        if len(boxes) == 0:
            return []

        # 转换为numpy数组
        boxes_array = np.array([[b['x'], b['y'],
                                 b['x'] + b['w'], b['y'] + b['h'],
                                 b.get('confidence', 1.0)] for b in boxes])

        x1 = boxes_array[:, 0]
        y1 = boxes_array[:, 1]
        x2 = boxes_array[:, 2]
        y2 = boxes_array[:, 3]
        scores = boxes_array[:, 4]

        areas = (x2 - x1 + 1) * (y2 - y1 + 1)
        indices = np.argsort(scores)[::-1]

        keep = []
        while len(indices) > 0:
            i = indices[0]
            keep.append(i)

            xx1 = np.maximum(x1[i], x1[indices[1:]])
            yy1 = np.maximum(y1[i], y1[indices[1:]])
            xx2 = np.minimum(x2[i], x2[indices[1:]])
            yy2 = np.minimum(y2[i], y2[indices[1:]])

            w = np.maximum(0, xx2 - xx1 + 1)
            h = np.maximum(0, yy2 - yy1 + 1)

            overlap = (w * h) / areas[indices[1:]]

            indices = indices[np.where(overlap <= overlap_thresh)[0] + 1]

        return [boxes[i] for i in keep]

# 使用示例
detector = GameObjectDetector()

# 捕获屏幕
screenshot = capture_screen()

# 检测红色物品
red_items = detector.detect_by_color(screenshot, 'red_item')
for item in red_items:
    print(f"发现红色物品: 位置({item['x']}, {item['y']})")

# 检测敌人（使用模板）
enemy_template = cv2.imread('enemy_template.png')
enemies = detector.detect_by_template(screenshot, enemy_template)
for enemy in enemies:
    print(f"发现敌人: 位置({enemy['x']}, {enemy['y']}) 置信度{enemy['confidence']:.2f}")
```

## 五、方法选择指南

| 场景 | 推荐方法 | 理由 |
|-----|---------|------|
| 颜色鲜明的物体 | 颜色识别 | 快速、简单、实时性好 |
| 固定外观的物体 | 模板匹配 | 精确、可多尺度 |
| 不同颜色的同形状物体 | 轮廓+形状分析 | 不依赖颜色 |
| 旋转缩放的物体 | 特征点匹配 | 鲁棒性强 |
| 特定类别（如人脸） | 级联分类器 | 速度快、效果好 |
| 多种类物体识别 | 深度学习 | 精度最高、泛化好 |
| 游戏UI元素 | 模板匹配 + 颜色 | 平衡速度和准确度 |

## 六、性能优化建议

### 1. ROI（感兴趣区域）
```python
# 只在特定区域检测
roi = image[100:500, 200:800]
results = detect_objects(roi)
```

### 2. 降低分辨率
```python
# 缩小图像加速处理
small = cv2.resize(image, None, fx=0.5, fy=0.5)
results = detect_objects(small)
# 结果坐标 * 2还原
```

### 3. 帧跳过
```python
# 每N帧检测一次
frame_count = 0
if frame_count % 3 == 0:
    results = detect_objects(frame)
frame_count += 1
```

### 4. 多线程/多进程
```python
from concurrent.futures import ThreadPoolExecutor

with ThreadPoolExecutor(max_workers=4) as executor:
    futures = [executor.submit(detect_in_region, region)
               for region in regions]
    results = [f.result() for f in futures]
```

### 5. GPU加速
```python
# 使用CUDA加速（需要GPU版本OpenCV）
gpu_frame = cv2.cuda_GpuMat()
gpu_frame.upload(frame)
# GPU处理...
result = gpu_frame.download()
```

## 七、常见问题和解决方案

### 1. 光照变化
- 使用HSV空间
- 直方图均衡化：`cv2.equalizeHist()`
- 自适应阈值

### 2. 遮挡问题
- 使用特征点匹配
- 增加多个模板
- 降低匹配阈值

### 3. 噪声干扰
- 加强预处理（降噪、形态学）
- 增加面积/形状过滤条件
- 使用中值滤波

### 4. 尺度变化
- 多尺度模板匹配
- 使用特征点（SIFT/ORB）
- 金字塔搜索

### 5. 误检测
- 增加多个过滤条件（面积、形状、颜色）
- 使用非极大值抑制
- 时间连续性验证（跟踪）

## 八、调试技巧

```python
# 1. 可视化每个步骤
cv2.imshow('Original', image)
cv2.imshow('Gray', gray)
cv2.imshow('Binary', binary)
cv2.imshow('Contours', contour_image)

# 2. 打印关键信息
print(f"找到{len(contours)}个轮廓")
print(f"面积: {cv2.contourArea(contour)}")

# 3. 参数调节滑动条
def on_trackbar(val):
    threshold = cv2.getTrackbarPos('Threshold', 'Settings')
    # 重新处理...

cv2.createTrackbar('Threshold', 'Settings', 127, 255, on_trackbar)

# 4. 保存中间结果
cv2.imwrite('debug_binary.png', binary)
```

## 九、实用工具函数

```python
def find_color_range(image, x, y, window=10):
    """点击图像获取HSV范围"""
    hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
    region = hsv[y-window:y+window, x-window:x+window]

    lower = np.array([region[:,:,i].min() for i in range(3)])
    upper = np.array([region[:,:,i].max() for i in range(3)])

    return lower, upper

def calculate_iou(box1, box2):
    """计算两个框的IoU"""
    x1 = max(box1[0], box2[0])
    y1 = max(box1[1], box2[1])
    x2 = min(box1[0] + box1[2], box2[0] + box2[2])
    y2 = min(box1[1] + box1[3], box2[1] + box2[3])

    intersection = max(0, x2 - x1) * max(0, y2 - y1)
    area1 = box1[2] * box1[3]
    area2 = box2[2] * box2[3]
    union = area1 + area2 - intersection

    return intersection / union if union > 0 else 0

def draw_detection_result(image, results, label="Object"):
    """绘制检测结果"""
    output = image.copy()
    for result in results:
        x, y, w, h = result['x'], result['y'], result['w'], result['h']

        # 绘制矩形
        cv2.rectangle(output, (x, y), (x+w, y+h), (0, 255, 0), 2)

        # 添加标签
        text = f"{label}: {result.get('confidence', 1.0):.2f}"
        cv2.putText(output, text, (x, y-10),
                    cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 0), 2)

        # 绘制中心点
        center = result.get('center', (x + w//2, y + h//2))
        cv2.circle(output, center, 5, (0, 0, 255), -1)

    return output
```

## 总结

选择合适的方法需要考虑：
1. **准确度要求**：深度学习 > 特征匹配 > 模板匹配 > 颜色检测
2. **速度要求**：颜色检测 > 模板匹配 > 级联分类器 > 特征匹配 > 深度学习
3. **环境复杂度**：简单场景用传统方法，复杂场景用深度学习
4. **开发成本**：颜色/模板最简单，深度学习需要更多资源

**实际项目中常用组合**：
- **游戏辅助**：颜色识别 + 模板匹配 + ROI优化
- **工业检测**：轮廓分析 + 形状匹配
- **通用物体识别**：YOLO/SSD等深度学习模型
- **实时追踪**：颜色检测 + 卡尔曼滤波跟踪
