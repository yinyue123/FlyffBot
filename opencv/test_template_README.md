# 模板匹配调试工具使用说明

## 功能概述

`test_template.py` 是一个交互式的模板匹配调试工具，类似于 `test_hsv.py`，用于帮助你快速找到最佳的模板匹配参数。

## 主要功能

### ✅ 核心功能
- 📸 **双图像输入**：支持模板图像和待检测图像
- 🎯 **多尺度匹配**：自动在多个尺度下搜索模板
- 🔧 **6种匹配方法**：支持OpenCV所有模板匹配算法
- 📊 **4种结果处理**：NMS、最佳匹配、前N个、显示全部
- 🎛️ **实时参数调节**：通过滑块实时调整所有参数
- 👁️ **3种视图模式**：结果图、原图+ROI、热力图
- 🔍 **ROI区域选择**：只在指定区域搜索，提升性能
- ⚡ **降低分辨率**：可选择降低分辨率加速处理

## 使用方法

### 1. 运行程序

```bash
python test_template.py
```

### 2. 输入图像路径

程序会依次提示输入两个路径：

```
请输入模板图片路径: /path/to/template.png
请输入待检测图片路径: /path/to/image.png
```

**提示**：可以直接拖拽图片到终端窗口！

### 3. 调整参数

程序会打开两个窗口：
- **Trackbars**：滑块控制窗口
- **Display**：图像显示窗口

## 参数说明

### 🎯 匹配方法 (Method)

| 值 | 方法 | 说明 | 阈值方向 |
|----|------|------|---------|
| 0 | TM_CCOEFF_NORMED | 归一化相关系数（推荐） | 值越大越好 |
| 1 | TM_CCORR_NORMED | 归一化相关 | 值越大越好 |
| 2 | TM_SQDIFF_NORMED | 归一化平方差 | 值越小越好 |
| 3 | TM_CCOEFF | 相关系数 | 值越大越好 |
| 4 | TM_CCORR | 相关 | 值越大越好 |
| 5 | TM_SQDIFF | 平方差 | 值越小越好 |

**推荐**：一般使用 `Method = 0 (TM_CCOEFF_NORMED)`

### 📏 阈值 (Threshold%)

- **范围**：0-100 (实际值 = 滑块值/100)
- **说明**：匹配的置信度阈值
- **推荐值**：
  - 精确匹配：85-95
  - 一般匹配：75-85
  - 宽松匹配：65-75

### 🔍 多尺度参数

#### Scale_Min% / Scale_Max%
- **范围**：0-200 (实际值 = 滑块值/100)
- **说明**：模板缩放的最小和最大比例
- **示例**：
  - 80-120 = 模板从80%到120%大小搜索（推荐）
  - 50-150 = 从50%到150%（大范围搜索）
  - 90-110 = 只在接近原始大小附近搜索（快速）

#### Scale_Steps
- **范围**：1-30
- **说明**：在min和max之间尝试多少个尺度
- **推荐值**：
  - 快速搜索：3-5 步
  - 平衡：5-10 步
  - 精确搜索：15-20 步

**性能提示**：步数越多越慢，但更容易找到目标！

### 📊 结果处理 (Result_Method)

| 值 | 方法 | 说明 | 适用场景 |
|----|------|------|---------|
| 0 | NMS | 非极大值抑制，去除重叠检测 | 多目标检测（推荐） |
| 1 | 最佳匹配 | 只显示得分最高的一个 | 只需要找一个目标 |
| 2 | 前N个 | 显示得分最高的N个 | 需要找多个目标 |
| 3 | 显示全部 | 显示所有符合阈值的 | 调试用 |

### 🎛️ 其他参数

#### Top_N
- **范围**：1-50
- **说明**：当Result_Method=2时，显示前N个匹配
- **推荐**：5-10

#### NMS_Thresh%
- **范围**：0-100 (实际值 = 滑块值/100)
- **说明**：NMS的IoU阈值，越小去重越严格
- **推荐值**：
  - 严格去重：20-30
  - 平衡：30-40
  - 宽松：40-50

#### Resize%
- **范围**：10-100 (实际值 = 滑块值/100)
- **说明**：降低图像分辨率加速处理
- **推荐值**：
  - 不降低：100（默认）
  - 快速模式：50-70
  - 超快模式：30-50

**注意**：降低分辨率会影响小目标检测！

#### Timeout_ms ⚠️ 新功能
- **范围**：100-5000 (毫秒)
- **说明**：处理超时时间，超过该时间会自动跳过剩余尺度
- **推荐值**：
  - 快速响应：500-1000ms（默认1000ms）
  - 平衡模式：1000-2000ms
  - 精确搜索：2000-5000ms

**用途**：
- 防止某些参数组合导致处理时间过长卡住界面
- 当出现红色 "WARNING: TIMEOUT!" 提示时，说明参数组合太慢
- 建议降低 Scale_Steps 或 Resize% 来加速

**超时时的行为**：
- 停止处理剩余尺度
- 返回已找到的匹配结果
- 在界面显示红色超时警告

### 📍 ROI区域 (Region of Interest)

- **ROI_X, ROI_Y**：搜索区域左上角坐标
- **ROI_W, ROI_H**：搜索区域宽度和高度

**使用场景**：
- 只在屏幕特定区域查找（如游戏中的技能栏）
- 提升性能，减少误检

### 🖼️ 视图模式 (View)

| 值 | 模式 | 说明 |
|----|------|------|
| 0 | Result | 在ROI区域显示匹配结果（默认） |
| 1 | Original+ROI | 在完整原图上显示ROI框 |
| 2 | Heatmap | 显示匹配热力图（红色=高匹配） |

### 📺 显示缩放 (Display%)

- **范围**：10-400
- **说明**：只影响显示，不影响检测
- **用途**：放大查看细节或缩小查看全图

## 快捷键

| 按键 | 功能 |
|-----|------|
| `q` | 退出并打印最终参数代码 |
| `s` | 保存当前显示的图像 |

## 使用示例

### 场景1：游戏UI元素检测

假设你要在游戏中找技能图标：

1. **输入图像**：
   - 模板：skill_icon.png (技能图标截图)
   - 目标：game_screenshot.png (游戏截图)

2. **参数设置**：
   ```
   Method = 0 (TM_CCOEFF_NORMED)
   Threshold% = 85
   Scale_Min% = 90
   Scale_Max% = 110
   Scale_Steps = 5
   Result_Method = 0 (NMS)
   NMS_Thresh% = 30
   Resize% = 100
   ```

3. **ROI设置**：
   - 只在技能栏区域搜索
   - 例如：ROI_X=0, ROI_Y=800, ROI_W=1920, ROI_H=200

### 场景2：物品检测

在复杂场景中找物品：

1. **参数设置**：
   ```
   Method = 0
   Threshold% = 75  # 较低阈值，允许一定变化
   Scale_Min% = 70
   Scale_Max% = 130
   Scale_Steps = 10  # 更多步数
   Result_Method = 2 (前N个)
   Top_N = 10
   Resize% = 100
   ```

2. **使用热力图**：
   - 切换到 View=2 查看哪些区域匹配度高

### 场景3：快速搜索

需要快速处理大量图像：

1. **性能优化**：
   ```
   Scale_Steps = 3  # 少量步数
   Result_Method = 1 (最佳匹配)
   Resize% = 50  # 降低分辨率
   ```

2. **使用ROI**：
   - 缩小搜索范围

## 输出示例

按 `q` 退出后，程序会输出可直接使用的Python代码：

```python
============================================================
最终参数:
============================================================

# 匹配方法: TM_CCOEFF_NORMED
method = cv2.TM_CCOEFF_NORMED

# 匹配阈值 (值越大越好)
threshold = 0.85

# 多尺度参数
scale_min = 0.90
scale_max = 1.10
scale_steps = 5
scales = np.linspace(0.9, 1.1, 5)

# 结果处理: NMS (非极大值抑制)
# 使用 NMS 非极大值抑制
nms_threshold = 0.30

# 性能优化
resize_factor = 1.00  # 图像缩放比例
timeout = 1.00  # 秒，超时会跳过剩余尺度

# ROI搜索区域 (x, y, width, height)
roi_region = (0, 800, 1920, 200)

# 完整示例代码
# result = cv2.matchTemplate(image_gray, template_gray, cv2.TM_CCOEFF_NORMED)
# locations = np.where(result >= 0.85)
============================================================
```

## 调试技巧

### 1. 找不到目标？

**检查清单**：
- ✅ 阈值是否太高？尝试降低到70-75
- ✅ 尺度范围是否够大？扩大Scale_Min和Scale_Max
- ✅ 增加Scale_Steps，尝试更多尺度
- ✅ 切换到热力图(View=2)看看匹配分布
- ✅ 确认模板图像和目标图像的颜色/光照是否一致

### 2. 太多误检？

**解决方案**：
- ✅ 提高阈值到85-90
- ✅ 使用NMS方法(Result_Method=0)
- ✅ 降低NMS_Thresh%到20-30
- ✅ 使用ROI限制搜索区域
- ✅ 减少Scale范围，只在目标可能的尺度搜索

### 3. 速度太慢？

**优化方法**：
- ✅ 降低Resize%到50-70
- ✅ 减少Scale_Steps到3-5
- ✅ 缩小Scale范围
- ✅ 使用ROI减小搜索区域
- ✅ 如果只需要一个目标，用Result_Method=1
- ✅ 降低Timeout_ms到500-800ms，强制跳过慢速处理

### 3.5. 出现超时警告？⚠️

**问题**：界面显示红色 "WARNING: TIMEOUT!"

**原因**：当前参数组合导致处理时间超过设定的超时时间

**解决方案**：
1. **增加超时时间**（临时方案）
   - 调高 Timeout_ms 到 2000-5000ms

2. **优化参数**（推荐）
   - 减少 Scale_Steps（如从20降到5-10）
   - 降低 Resize%（如从100降到50-70）
   - 使用ROI缩小搜索范围
   - 提高阈值减少匹配数量

3. **调整策略**
   - 如果找到了目标但超时，说明参数可能过于精细
   - 如果没找到目标且超时，可能需要改变搜索策略

### 4. 热力图的使用

热力图(View=2)可以帮助你：
- 🔥 **红色区域**：高匹配度
- 🧊 **蓝色区域**：低匹配度
- 用途：查看为什么某些区域被误检或漏检

## 匹配方法选择指南

### TM_CCOEFF_NORMED (推荐)
- ✅ 对光照变化较鲁棒
- ✅ 归一化后结果在-1到1之间
- ✅ 最常用的方法
- 📝 阈值推荐：0.75-0.9

### TM_SQDIFF_NORMED
- ⚠️ 值越小越好（注意是相反的！）
- ✅ 计算简单，速度快
- 📝 阈值推荐：0.1-0.3（小于阈值为匹配）

### TM_CCORR_NORMED
- ⚠️ 受亮度影响较大
- 📝 阈值推荐：0.85-0.95
- 💡 不推荐作为首选

## 常见问题

### Q1: 为什么模板需要和目标图像颜色一致？
A: 模板匹配是基于像素相似度的方法，颜色差异会影响匹配结果。如果颜色可能变化，可以尝试：
- 转换为灰度图再匹配
- 使用特征点匹配方法（SIFT/ORB）

### Q2: 多尺度匹配会很慢吗？
A: 是的。时间复杂度 ≈ O(尺度数 × 图像大小)。优化方法：
- 减少Scale_Steps
- 使用Resize%降低分辨率
- 使用ROI减小搜索区域

### Q3: NMS是什么？
A: 非极大值抑制（Non-Maximum Suppression），用于去除重叠的检测框。当同一个目标被多次检测时，保留得分最高的，抑制其他重叠的检测。

### Q4: 如何处理旋转的目标？
A: 模板匹配对旋转不鲁棒。建议：
- 准备多个旋转角度的模板
- 使用特征点匹配（ORB/SIFT）
- 使用深度学习方法（YOLO等）

## 实际应用代码示例

调试完参数后，可以这样使用：

```python
import cv2
import numpy as np

def match_template_multiscale(image, template, threshold=0.85,
                               scale_min=0.9, scale_max=1.1, scale_steps=5):
    """多尺度模板匹配"""
    image_gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    template_gray = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY)

    h, w = template_gray.shape
    all_matches = []

    for scale in np.linspace(scale_min, scale_max, scale_steps):
        scaled_w = int(w * scale)
        scaled_h = int(h * scale)

        if scaled_w > image_gray.shape[1] or scaled_h > image_gray.shape[0]:
            continue

        resized_template = cv2.resize(template_gray, (scaled_w, scaled_h))
        result = cv2.matchTemplate(image_gray, resized_template,
                                   cv2.TM_CCOEFF_NORMED)

        locations = np.where(result >= threshold)
        for pt in zip(*locations[::-1]):
            all_matches.append({
                'x': pt[0],
                'y': pt[1],
                'w': scaled_w,
                'h': scaled_h,
                'score': result[pt[1], pt[0]]
            })

    return all_matches

# 使用
template = cv2.imread('template.png')
image = cv2.imread('screenshot.png')

matches = match_template_multiscale(image, template,
                                   threshold=0.85,
                                   scale_min=0.9,
                                   scale_max=1.1,
                                   scale_steps=5)

for match in matches:
    print(f"找到目标: 位置({match['x']}, {match['y']}), "
          f"大小({match['w']}x{match['h']}), "
          f"置信度{match['score']:.2f}")
```

## 性能对比

不同参数组合的处理时间参考（1920x1080图像）：

| 配置 | Scale_Steps | Resize% | ROI | 处理时间 |
|-----|-------------|---------|-----|---------|
| 快速 | 3 | 50 | 是 | ~50ms |
| 平衡 | 5 | 100 | 是 | ~200ms |
| 精确 | 10 | 100 | 否 | ~800ms |
| 极致 | 20 | 100 | 否 | ~1500ms |

## 总结

### 推荐工作流程

1. **初始设置**：使用默认参数快速测试
2. **调整阈值**：确保能找到目标
3. **优化尺度**：调整scale范围覆盖目标大小
4. **去除误检**：使用NMS或提高阈值
5. **性能优化**：设置ROI和降低分辨率
6. **导出代码**：按`q`保存参数

### 参数速查表

```
快速搜索：
  Method=0, Threshold=80, Scale=90-110(3步), NMS, Resize=50, Timeout=500ms

平衡搜索：
  Method=0, Threshold=85, Scale=80-120(5步), NMS, Resize=100, Timeout=1000ms

精确搜索：
  Method=0, Threshold=75, Scale=70-130(10步), NMS, Resize=100, Timeout=2000ms
```

祝你调试顺利！🎯
