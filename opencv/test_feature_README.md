# 特征检测调试工具 (test_feature.py)

## 功能概述

这是一个交互式特征检测调试工具，用于**在大图中检测小图的位置**。类似于 test_template.py 的设计，支持多种特征检测算法，提供实时参数调整功能。

## 主要用途

**用一张模板图（小图）在场景图（大图）中找到匹配位置**

- 左侧窗口：显示检测结果（绿色框标出检测位置）
- 右侧窗口：调整各种参数

## 支持的算法

1. **SIFT** (Scale-Invariant Feature Transform)
   - 尺度不变特征变换
   - 最稳定，但速度较慢
   - 需要 opencv-contrib-python

2. **ORB** (Oriented FAST and Rotated BRIEF)
   - 旋转不变的快速特征
   - 速度快，资源消耗低
   - OpenCV 内置

3. **AKAZE** (Accelerated-KAZE)
   - 加速的 KAZE 算法
   - 平衡速度和质量
   - OpenCV 内置

4. **BRISK** (Binary Robust Invariant Scalable Keypoints)
   - 二进制鲁棒不变特征
   - 速度快，适合实时应用
   - OpenCV 内置

## 使用方法

### 基本用法

```bash
python test_feature.py
```

### 输入要求

1. **模板图片**（必需）
   - 输入模板图片路径（小图，你要找的目标）

2. **场景图片**（必需）
   - 输入场景图片路径（大图，包含目标的图片）

### 显示效果

程序会在场景图上同时显示：

1. **绿色检测框** - 标出找到的模板位置
2. **黄色关键点** - 场景图中的特征点（可关闭）
3. **青色匹配线** - 连接模板和场景的匹配点（可关闭）
4. **绿色十字** - 检测框的中心点
5. **统计信息** - 检测器、匹配数量、处理时间等

## 参数说明

### 通用参数

- **Detector Type** (0-3)
  - 0: SIFT
  - 1: ORB
  - 2: AKAZE
  - 3: BRISK

- **Matcher Type** (0-1)
  - 0: BF (暴力匹配)
  - 1: FLANN (快速匹配)

- **Ratio Thresh** (10-99, ÷100)
  - 默认: 0.75
  - Lowe's 比率测试阈值
  - 越小越严格，误匹配越少

- **RANSAC Thresh** (10-100, ÷10)
  - 默认: 5.0
  - RANSAC 重投影误差阈值
  - 越小越严格

- **Min Matches** (10-50)
  - 默认: 10
  - 计算单应性矩阵的最小匹配点数
  - 降低可以检测更难的目标

- **Max Show Matches** (50-200)
  - 默认: 50
  - 最多显示的匹配连接线数量

- **Show Keypoints** (0/1)
  - 是否显示场景图的关键点

- **Show Matches** (0/1)
  - 是否显示匹配连接线

### SIFT 参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| SIFT_nFeatures | 0-10000 | 0 | 最大特征点数量（0=不限制） |
| SIFT_Octaves | 3-10 | 3 | 金字塔层数 |
| SIFT_Contrast | 0-100 (÷1000) | 0.04 | 对比度阈值，越大特征点越少 |
| SIFT_Edge | 10-30 | 10 | 边缘阈值，越大保留边缘特征越多 |

**推荐配置：**
- 高质量检测：Contrast=0.03, Edge=10
- 快速检测：nFeatures=500, Contrast=0.08
- 纹理丰富图像：降低 Contrast
- 边缘明显图像：降低 Edge

### ORB 参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| ORB_nFeatures | 100-10000 | 500 | 最大特征点数量 |
| ORB_Scale | 10-20 (÷10) | 1.2 | 图像金字塔缩放因子 |
| ORB_Levels | 1-16 | 8 | 金字塔层数 |
| ORB_EdgeThresh | 5-50 | 31 | 边界大小 |
| ORB_FirstLevel | 0-5 | 0 | 金字塔起始层 |
| ORB_WTA_K | 2-4 | 2 | 生成描述符的点数 |
| ORB_PatchSize | 10-50 | 31 | 特征点邻域大小 |

**推荐配置：**
- 标准检测：nFeatures=500, Scale=1.2, Levels=8
- 多尺度检测：增加 Levels 到 12-16
- 快速检测：nFeatures=300, Levels=4
- 精细检测：PatchSize=51, EdgeThresh=15

### AKAZE 参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| AKAZE_Thresh | 1-100 (÷10000) | 0.001 | 检测阈值，越大特征点越少 |
| AKAZE_Octaves | 1-10 | 4 | 八度层数 |
| AKAZE_Layers | 1-10 | 4 | 每个八度的子层数 |
| AKAZE_Diffusivity | 0-2 | 1 | 扩散类型 |

**推荐配置：**
- 高质量检测：Thresh=0.0005, Octaves=4, Layers=4
- 快速检测：Thresh=0.003, Octaves=3, Layers=3
- 均匀分布特征：Diffusivity=1 (PM_G2)
- 保留细节：Diffusivity=2 (WEICKERT)

### BRISK 参数

| 参数 | 范围 | 默认值 | 说明 |
|------|------|--------|------|
| BRISK_Thresh | 1-100 | 30 | AGAST 检测阈值 |
| BRISK_Octaves | 0-10 | 3 | 八度层数 |
| BRISK_Scale | 5-20 (÷10) | 1.0 | 采样模式的缩放因子 |

**推荐配置：**
- 标准检测：Thresh=30, Octaves=3, Scale=1.0
- 高灵敏度：Thresh=10-20（更多特征点）
- 低灵敏度：Thresh=50-80（更少但更稳定的特征点）
- 大目标检测：Scale=1.5-2.0

## 快捷键

- **q** - 退出程序并生成代码
- **s** - 保存当前显示图像
- **ESC** - 退出程序

## 输出文件

### 保存的图像

按 's' 键保存当前视图：
- `feature_detection_<timestamp>.png` - 包含检测框和匹配信息的结果图

### 生成的代码

按 'q' 退出时，会生成完整的 Python 代码，包含当前所有参数设置。

## 使用场景

### 场景 1：游戏图标识别

**需求**：在游戏截图中找到特定图标/按钮

**推荐配置**：
```
Detector: ORB (速度快)
ORB_nFeatures: 500
Ratio Thresh: 0.75
RANSAC Thresh: 5.0
Min Matches: 10
```

**步骤**：
1. 模板图：截取单个图标
2. 场景图：游戏完整截图
3. 调整参数直到绿框准确标出图标位置

### 场景 2：物体定位

**需求**：在照片中找到特定物体

**推荐配置**：
```
Detector: SIFT (高质量)
SIFT_Contrast: 0.04
Ratio Thresh: 0.7
RANSAC Thresh: 5.0
Matcher: FLANN
```

**步骤**：
1. 模板图：清晰的物体照片
2. 场景图：包含该物体的完整场景
3. 如果检测不到，降低 Ratio Thresh

### 场景 3：快速实时检测

**需求**：需要快速处理多张图片

**推荐配置**：
```
Detector: ORB 或 BRISK
ORB_nFeatures: 300
ORB_Levels: 4
Matcher: BF
```

### 场景 4：精确匹配

**需求**：需要极高的准确度

**推荐配置**：
```
Detector: SIFT
SIFT_nFeatures: 1000
Ratio Thresh: 0.6 (更严格)
RANSAC Thresh: 3.0
Min Matches: 20
Matcher: FLANN
```

## 常见问题

### 1. 检测不到目标（找不到绿框）

**可能原因**：
- 特征点太少
- 匹配点不足
- 阈值太严格

**解决方案**：
1. 降低 Min Matches (减到 4-6)
2. 提高 Ratio Thresh (0.8-0.85)
3. 增加 RANSAC Thresh (6.0-8.0)
4. 根据检测器调整参数：
   - SIFT: 降低 SIFT_Contrast (0.02-0.03)
   - ORB: 增加 ORB_nFeatures (1000-2000)
   - AKAZE: 降低 AKAZE_Thresh (0.0005)
   - BRISK: 降低 BRISK_Thresh (15-20)

### 2. 检测框位置不准确

**原因**：误匹配太多

**解决方案**：
1. 降低 Ratio Thresh (0.6-0.7)
2. 降低 RANSAC Thresh (3.0-4.0)
3. 增加 Min Matches (15-20)
4. 切换到更稳定的检测器 (SIFT)

### 3. 特征点太多，处理太慢

**解决方案**：
- 限制特征点数量：
  - SIFT: 设置 nFeatures=500
  - ORB: 设置 nFeatures=300
- 减少金字塔层数：
  - ORB_Levels: 4-6
  - AKAZE_Octaves: 3
- 使用更快的检测器 (ORB/BRISK)
- 使用 BF 匹配器替代 FLANN

### 4. 检测到多个位置（误检）

**解决方案**：
1. 降低 Ratio Thresh (0.65)
2. 增加 Min Matches (20+)
3. 降低 RANSAC Thresh (2.0-3.0)
4. 关闭 Show Matches 查看实际匹配质量

### 5. 旋转或缩放的目标检测不到

**原因**：尺度不变性不足

**解决方案**：
1. 使用 SIFT 或 AKAZE (尺度不变性好)
2. 增加金字塔层数：
   - SIFT_Octaves: 5-6
   - ORB_Levels: 12-16
   - AKAZE_Octaves: 5-6
3. 调整缩放因子：
   - ORB_Scale: 1.5-1.8 (更大的尺度范围)

### 6. 光照变化导致检测失败

**解决方案**：
1. 使用 SIFT 或 ORB (光照不变性好)
2. 预处理图像：
   - 使用直方图均衡化
   - 转换为灰度图
3. 降低对比度阈值：
   - SIFT_Contrast: 0.03

## 性能对比

| 算法 | 速度 | 质量 | 旋转不变 | 尺度不变 | 光照不变 | 推荐场景 |
|------|------|------|----------|----------|----------|----------|
| SIFT | ★★☆☆☆ | ★★★★★ | ✓ | ✓ | ✓ | 精确匹配、离线处理 |
| ORB  | ★★★★★ | ★★★☆☆ | ✓ | ✓ | ✓ | 实时检测、游戏辅助 |
| AKAZE| ★★★★☆ | ★★★★☆ | ✓ | ✓ | ✓ | 平衡选择 |
| BRISK| ★★★★★ | ★★★☆☆ | ✓ | ✓ | ✓ | 快速检测 |

## 技术说明

### 检测流程

1. **特征检测** - 在模板和场景中检测关键点
2. **描述符计算** - 为每个关键点生成特征向量
3. **特征匹配** - 使用 KNN 找到最相似的特征对
4. **Lowe's 比率测试** - 过滤不可靠的匹配
5. **RANSAC 验证** - 计算单应性矩阵并过滤离群点
6. **结果可视化** - 在场景图上绘制检测框

### Lowe's 比率测试

对于每个特征点，找到最近的两个匹配：
```python
if distance(match1) < ratio * distance(match2):
    accept match1  # 认为是好的匹配
```

ratio 越小，匹配越严格，误匹配越少。

### RANSAC 算法

随机抽样一致性算法，用于过滤离群点：
1. 随机选择最小样本集（4个点）
2. 计算单应性矩阵
3. 统计符合模型的内点数量
4. 重复多次，找到最佳模型

RANSAC Thresh 控制点到模型的最大距离。

### 单应性矩阵

描述两个平面之间的透视变换关系：
- 用于计算模板在场景中的位置和形状
- 支持旋转、缩放、透视变换
- 需要至少 4 个匹配点

## 示例命令

### 示例 1：检测游戏图标

```bash
python test_feature.py
# 输入模板: button_icon.png
# 输入场景: game_screenshot.png
# 调整参数直到出现绿色检测框
# 按 's' 保存结果
# 按 'q' 生成代码
```

### 示例 2：多尺度检测

```bash
python test_feature.py
# 输入模板: logo_small.png
# 输入场景: poster.png
# 设置 Detector Type = 0 (SIFT)
# 增加 SIFT_Octaves = 5
# 调整其他参数
```

### 示例 3：快速检测模式

```bash
python test_feature.py
# 输入模板: target.png
# 输入场景: scene.png
# 设置 Detector Type = 1 (ORB)
# 设置 ORB_nFeatures = 300
# 设置 ORB_Levels = 4
# 设置 Matcher Type = 0 (BF)
```

## 注意事项

### 1. 图片质量

- 使用清晰、对比度好的图片
- 避免过度模糊或噪声过多的图片
- 模板图应包含足够的纹理特征

### 2. 模板选择

- 模板应该是独特的、易识别的
- 避免选择纯色或简单几何图形
- 模板不应太小（建议至少 50x50 像素）

### 3. 匹配限制

- 模板和场景应有相似的光照条件
- 透视变形不应过大
- 尺度变化不应超过 5 倍

### 4. 性能优化

- ORB 和 BRISK 适合实时应用
- SIFT 适合离线高质量处理
- 限制特征点数量可以加速处理
- BF 匹配器适合小特征集，FLANN 适合大特征集

### 5. 调试技巧

- 先关闭 Show Keypoints 和 Show Matches 查看整体效果
- 观察 Inlier Ratio，应该 > 50%
- 如果 Matches 很多但 Inliers 很少，说明误匹配多
- 处理时间过长时，降低特征点数量

## 进阶技巧

### 1. 参数快速调优

**步骤**：
1. 先用默认参数运行
2. 如果检测不到，按优先级调整：
   - Min Matches (减少)
   - Ratio Thresh (增加)
   - 算法特定阈值（降低）
3. 如果检测不准，按优先级调整：
   - Ratio Thresh (减少)
   - RANSAC Thresh (减少)
   - Min Matches (增加)

### 2. 多目标检测

虽然此工具一次只检测一个目标，但可以：
1. 记录当前最佳参数
2. 用代码实现批量检测
3. 使用 NMS（非极大值抑制）去重

### 3. 与 test_template.py 对比

| 特性 | test_template.py | test_feature.py |
|------|------------------|-----------------|
| 方法 | 模板匹配 | 特征匹配 |
| 旋转不变 | ✗ | ✓ |
| 尺度不变 | 多尺度搜索 | ✓ |
| 速度 | 快 | 中等 |
| 精度 | 高（完全匹配） | 中等（特征匹配） |
| 适用场景 | 固定角度、固定大小 | 任意角度、任意大小 |

**选择建议**：
- 如果目标没有旋转和缩放：用 test_template.py
- 如果目标有旋转或缩放：用 test_feature.py
- 如果需要极快速度：用 test_template.py
- 如果需要鲁棒性：用 test_feature.py

---

**作者**: Claude Code
**版本**: 2.0
**日期**: 2025-11-02
**更新**: 简化为物体检测模式，移除多视图切换
