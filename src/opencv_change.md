# OpenCV HSV 颜色检测重构文档

## 概述

本文档记录了从 RGB 像素匹配到 OpenCV HSV 颜色检测的重构。

### 重构原因
- **原方法**：使用 RGB 颜色空间进行像素匹配，对光照变化敏感
- **新方法**：使用 HSV 颜色空间 + 形态学操作 + 轮廓检测，更加鲁棒

### 重构范围
- `stats.go` - 状态栏检测（HP/MP/FP/目标血量）
- `analyzer.go` - 图像分析（怪物检测、目标标记检测）

---

## 1. stats.go 的变化

### 1.1 原来的检测方法（RGB 像素匹配）

#### 原代码结构
```go
// 原来的方法：直接扫描像素，匹配 RGB 颜色
func (si *StatInfo) UpdateValue(img *image.RGBA) bool {
    config := GetStatusBarConfig(si.StatKind)

    // 检测像素匹配的颜色
    cloud := si.detectPixels(img, config)

    // 从点云计算边界
    bounds := cloud.ToBounds()

    // 计算百分比
    if si.MaxW > 0 {
        valueFrac := float64(bounds.W) / float64(si.MaxW)
        newValue = int(valueFrac * 100)
    }
}

// 原来的像素检测：逐像素扫描，使用容差匹配 RGB 颜色
func (si *StatInfo) detectPixels(img *image.RGBA, config StatusBarConfig) *PointCloud {
    cloud := NewPointCloud()
    tolerance := uint8(2) // RGB 容差

    for y := config.MinY; y < config.MaxY; y++ {
        for x := config.MinX; x < config.MaxX; x++ {
            r, g, b, a := img.At(x, y).RGBA()
            pixel := Color{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)}

            // 检查是否匹配任何参考颜色
            for _, refColor := range config.Colors {
                if pixel.Matches(refColor, tolerance) {
                    cloud.Add(Point{X: x, Y: y})
                    break
                }
            }
        }
    }
    return cloud
}
```

#### 原来的颜色配置（RGB）
```go
// HP Bar - 使用 4 个 RGB 颜色参考值
case StatusBarHP:
    return NewStatusBarConfig([][3]uint8{
        {174, 18, 55},   // RGB
        {188, 24, 62},
        {204, 30, 70},
        {220, 36, 78},
    })

// MP Bar - 使用 4 个 RGB 颜色参考值
case StatusBarMP:
    return NewStatusBarConfig([][3]uint8{
        {20, 84, 196},   // RGB
        {36, 132, 220},
        {44, 164, 228},
        {56, 188, 232},
    })
```

### 1.2 新的检测方法（OpenCV HSV + 轮廓）

#### 新代码结构
```go
// 新方法：使用 OpenCV HSV 颜色空间 + 形态学操作 + 轮廓检测
func (si *StatInfo) UpdateValueOpenCV(hsvMat *gocv.Mat) bool {
    config := GetStatusBarConfig(si.StatKind)

    // 1. 提取 ROI（感兴趣区域）
    roiMat := hsvMat.Region(image.Rect(config.MinX, config.MinY, config.MaxX, config.MaxY))
    defer roiMat.Close()

    // 2. 创建 HSV 颜色掩码
    mask := si.createHSVMask(&roiMat, config.HSVRange)
    defer mask.Close()

    // 3. 应用形态学操作（去噪）
    morphed := si.applyMorphology(&mask)
    defer morphed.Close()

    // 4. 查找轮廓
    contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
    defer contours.Close()

    // 5. 找到最大轮廓宽度（主血条）
    maxWidth := 0
    for i := 0; i < contours.Size(); i++ {
        contour := contours.At(i)
        rect := gocv.BoundingRect(contour)
        if rect.Dx() > maxWidth {
            maxWidth = rect.Dx()
        }
    }

    // 6. 计算百分比
    valueFrac := float64(maxWidth) / float64(roiWidth)
    newValue = int(valueFrac * 100)
}

// 新的 HSV 掩码创建
func (si *StatInfo) createHSVMask(hsvMat *gocv.Mat, colorRange HSVRange) gocv.Mat {
    // 创建上下界
    lower := gocv.NewScalar(float64(colorRange.LowerH), float64(colorRange.LowerS), float64(colorRange.LowerV), 0)
    upper := gocv.NewScalar(float64(colorRange.UpperH), float64(colorRange.UpperS), float64(colorRange.UpperV), 0)

    // 创建掩码
    mask := gocv.NewMat()
    gocv.InRangeWithScalar(*hsvMat, lower, upper, &mask)
    return mask
}

// 新的形态学操作（去噪 + 填洞）
func (si *StatInfo) applyMorphology(mask *gocv.Mat) gocv.Mat {
    kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
    defer kernel.Close()

    // 开运算：腐蚀 + 膨胀，去除小噪点
    temp := gocv.NewMat()
    gocv.Erode(*mask, &temp, kernel)
    opened := gocv.NewMat()
    gocv.Dilate(temp, &opened, kernel)
    temp.Close()

    // 闭运算：膨胀 + 腐蚀，填充小孔
    temp2 := gocv.NewMat()
    gocv.Dilate(opened, &temp2, kernel)
    result := gocv.NewMat()
    gocv.Erode(temp2, &result, kernel)
    temp2.Close()
    opened.Close()

    return result
}
```

#### 新的颜色配置（HSV 范围）
```go
// HP Bar - 使用 HSV 颜色范围（占位符值，需要更新）
case StatusBarHP:
    return StatusBarConfig{
        MinX: 105, MinY: 30, MaxX: 225, MaxY: 110,
        HSVRange: HSVRange{
            LowerH: 0,   // 色调下界 (0-180)
            LowerS: 100, // 饱和度下界 (0-255)
            LowerV: 100, // 明度下界 (0-255)
            UpperH: 10,  // 色调上界 (红色)
            UpperS: 255, // 饱和度上界
            UpperV: 255, // 明度上界
        },
    }

// MP Bar - 使用 HSV 颜色范围（占位符值，需要更新）
case StatusBarMP:
    return StatusBarConfig{
        MinX: 105, MinY: 30, MaxX: 225, MaxY: 110,
        HSVRange: HSVRange{
            LowerH: 100, // 蓝色色调下界
            LowerS: 100,
            LowerV: 150,
            UpperH: 130, // 蓝色色调上界
            UpperS: 255,
            UpperV: 255,
        },
    }
```

### 1.3 关键变化对比表

| 方面 | 原方法（RGB） | 新方法（HSV + OpenCV） |
|------|--------------|----------------------|
| **颜色空间** | RGB | HSV |
| **颜色定义** | 4 个精确 RGB 值 + 容差 2 | HSV 范围（H/S/V 上下界） |
| **检测方式** | 逐像素扫描匹配 | 颜色掩码 + 轮廓检测 |
| **噪点处理** | 无 | 形态学操作（开运算 + 闭运算） |
| **形状识别** | 点云 -> 边界框 | 轮廓检测 -> 边界框 |
| **对光照敏感度** | 高（RGB 受光照影响大） | 低（HSV 分离颜色和亮度） |
| **准确性** | 中等 | 高 |

---

## 2. analyzer.go 的变化

### 2.1 原来的怪物检测方法（RGB 像素扫描）

#### 原代码结构
```go
// 原来的方法：扫描像素匹配颜色，然后聚类
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target {
    img := ia.GetImage()

    // 定义扫描区域
    region := Bounds{X: 0, Y: 0, W: ia.screenInfo.Width, H: ia.screenInfo.Height - 100}

    // 检测被动怪（黄色名字）- RGB 颜色
    passiveColors := []Color{config.PassiveColor}
    passivePoints := ia.scanPixelsForColors(img, region, passiveColors, config.PassiveTolerance)

    // 检测攻击性怪（红色名字）- RGB 颜色
    aggressiveColors := []Color{config.AggressiveColor}
    aggressivePoints := ia.scanPixelsForColors(img, region, aggressiveColors, config.AggressiveTolerance)

    // 聚类点形成怪物边界
    passiveClusters := clusterPoints(passivePoints, 50, 3)
    aggressiveClusters := clusterPoints(aggressivePoints, 50, 3)

    // 过滤并创建目标
    for _, bounds := range passiveClusters {
        if bounds.W > config.MinMobNameWidth && bounds.W < config.MaxMobNameWidth && bounds.Y >= 110 {
            mobs = append(mobs, Target{Type: MobPassive, Bounds: bounds})
        }
    }
}

// 原来的像素扫描：逐像素检查 RGB 颜色匹配
func (ia *ImageAnalyzer) scanPixelsForColors(img *image.RGBA, region Bounds, colors []Color, tolerance uint8) []Point {
    var points []Point
    for y := minY; y < maxY; y++ {
        for x := minX; x < maxX; x++ {
            // 跳过 HP 栏区域
            if x <= 250 && y <= 110 {
                continue
            }

            c := img.RGBAAt(x, y)

            // 检查像素是否匹配目标颜色（RGB）
            for _, targetColor := range colors {
                if colorMatches(c, targetColor, tolerance) {
                    points = append(points, Point{X: x, Y: y})
                    break
                }
            }
        }
    }
    return points
}

// RGB 颜色匹配
func colorMatches(c color.RGBA, target Color, tolerance uint8) bool {
    if c.A < 250 {
        return false
    }
    rDiff := abs(int(c.R) - int(target.R))
    gDiff := abs(int(c.G) - int(target.G))
    bDiff := abs(int(c.B) - int(target.B))
    return rDiff <= int(tolerance) && gDiff <= int(tolerance) && bDiff <= int(tolerance)
}
```

### 2.2 新的怪物检测方法（OpenCV HSV + 轮廓）

#### 新代码结构
```go
// 新方法：使用 OpenCV HSV + 轮廓检测
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target {
    img := ia.GetImage()

    // 1. 转换为 Mat
    mat := ia.imageToMat(img)
    defer mat.Close()

    // 2. 转换为 HSV 颜色空间
    hsvMat := gocv.NewMat()
    defer hsvMat.Close()
    gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

    // 3. 定义搜索 ROI
    searchROI := ROI{X: 0, Y: 0, Width: ia.screenInfo.Width, Height: ia.screenInfo.Height - 100}

    var mobs []Target

    // 4. 使用 HSV 检测被动怪
    passiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.PassiveMobRange, config)
    for _, bounds := range passiveBounds {
        if bounds.Y >= 110 {  // 过滤顶部 HP 栏区域
            mobs = append(mobs, Target{Type: MobPassive, Bounds: bounds})
        }
    }

    // 5. 使用 HSV 检测攻击性怪
    aggressiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.AggressiveMobRange, config)
    for _, bounds := range aggressiveBounds {
        if bounds.Y >= 110 {
            mobs = append(mobs, Target{Type: MobAggressive, Bounds: bounds})
        }
    }

    return mobs
}

// 新的 HSV 怪物检测：掩码 + 形态学 + 轮廓
func (ia *ImageAnalyzer) detectMobsByHSV(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange, config *Config) []Bounds {
    // 1. 提取 ROI
    roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
    defer roiMat.Close()

    // 2. 创建 HSV 颜色掩码
    mask := ia.createHSVMask(&roiMat, colorRange)
    defer mask.Close()

    // 3. 应用形态学操作去噪
    morphed := ia.applyMorphology(&mask)
    defer morphed.Close()

    // 4. 查找轮廓
    contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
    defer contours.Close()

    // 5. 转换轮廓为边界并过滤
    var bounds []Bounds
    for i := 0; i < contours.Size(); i++ {
        contour := contours.At(i)
        rect := gocv.BoundingRect(contour)

        // 按宽度过滤（怪物名字宽度约束）
        if rect.Dx() > config.MinMobNameWidth && rect.Dx() < config.MaxMobNameWidth {
            screenBounds := Bounds{
                X: roi.X + rect.Min.X,
                Y: roi.Y + rect.Min.Y,
                W: rect.Dx(),
                H: rect.Dy(),
            }

            // 跳过 HP 栏区域
            if screenBounds.X <= 250 && screenBounds.Y <= 110 {
                continue
            }

            bounds = append(bounds, screenBounds)
        }
    }

    return bounds
}
```

### 2.3 目标标记检测的变化

#### 原方法（RGB）
```go
// 原来：扫描像素，计数匹配像素
func (ia *ImageAnalyzer) DetectTargetMarker() bool {
    img := ia.GetImage()
    region := Bounds{...}

    // 蓝色标记 - RGB 颜色
    blueMarkerColors := []Color{NewColor(131, 148, 205)}
    bluePoints := ia.scanPixelsForColors(img, region, blueMarkerColors, 5)
    if len(bluePoints) > 20 {
        return true
    }

    // 红色标记 - RGB 颜色
    redMarkerColors := []Color{NewColor(246, 90, 106)}
    redPoints := ia.scanPixelsForColors(img, region, redMarkerColors, 5)
    if len(redPoints) > 20 {
        return true
    }

    return false
}
```

#### 新方法（HSV）
```go
// 新方法：HSV 掩码 + 计数非零像素
func (ia *ImageAnalyzer) DetectTargetMarkerOpenCV(hsvMat *gocv.Mat) bool {
    markerROI := ROI{...}

    // 蓝色标记 - HSV 范围
    blueMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.BlueMarkerRange)
    if blueMarkerDetected {
        return true
    }

    // 红色标记 - HSV 范围
    redMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.RedMarkerRange)
    return redMarkerDetected
}

// HSV 标记检测
func (ia *ImageAnalyzer) detectMarker(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) bool {
    // 提取 ROI
    roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
    defer roiMat.Close()

    // 创建 HSV 颜色掩码
    mask := ia.createHSVMask(&roiMat, colorRange)
    defer mask.Close()

    // 计数非零像素
    nonZero := gocv.CountNonZero(mask)

    // 阈值：至少 20 个像素
    return nonZero > 20
}
```

### 2.4 关键变化对比表

| 方面 | 原方法（RGB） | 新方法（HSV + OpenCV） |
|------|--------------|----------------------|
| **颜色空间** | RGB | HSV |
| **检测流程** | 像素扫描 -> 聚类 -> 边界框 | HSV 掩码 -> 形态学 -> 轮廓 -> 边界框 |
| **怪物颜色** | 单个 RGB 值 + 容差 | HSV 范围 |
| **标记检测** | 像素计数 | 掩码非零像素计数 |
| **噪点抑制** | 聚类时处理 | 形态学操作（开运算/闭运算） |
| **性能** | 中等（逐像素扫描） | 较好（OpenCV 优化） |
| **准确性** | 中等 | 高（轮廓提供更好的形状信息） |

---

## 3. HSV 颜色范围配置（占位符）

所有 HSV 颜色范围目前使用占位符值，需要根据实际游戏截图更新。

### 3.1 状态栏颜色（stats.go）

```go
// HP Bar - 红色（占位符）
HSVRange{
    LowerH: 0,   // 色调：0-10 为红色
    LowerS: 100, // 饱和度：100-255
    LowerV: 100, // 明度：100-255
    UpperH: 10,
    UpperS: 255,
    UpperV: 255,
}

// MP Bar - 蓝色（占位符）
HSVRange{
    LowerH: 100, // 色调：100-130 为蓝色
    LowerS: 100,
    LowerV: 150,
    UpperH: 130,
    UpperS: 255,
    UpperV: 255,
}

// FP Bar - 绿色（占位符）
HSVRange{
    LowerH: 40,  // 色调：40-80 为绿色
    LowerS: 100,
    LowerV: 100,
    UpperH: 80,
    UpperS: 255,
    UpperV: 255,
}
```

### 3.2 怪物颜色（analyzer.go）

```go
// 被动怪 - 黄色名字（占位符）
PassiveMobRange: HSVRange{
    LowerH: 20,  // 色调：20-35 为黄色
    LowerS: 100,
    LowerV: 150,
    UpperH: 35,
    UpperS: 255,
    UpperV: 255,
}

// 攻击性怪 - 红色名字（占位符）
AggressiveMobRange: HSVRange{
    LowerH: 0,   // 色调：0-10 为红色
    LowerS: 150,
    LowerV: 150,
    UpperH: 10,
    UpperS: 255,
    UpperV: 255,
}

// 紫色怪 - 紫色名字（占位符）
VioletMobRange: HSVRange{
    LowerH: 130, // 色调：130-160 为紫色
    LowerS: 100,
    LowerV: 100,
    UpperH: 160,
    UpperS: 255,
    UpperV: 255,
}
```

### 3.3 目标标记颜色（analyzer.go）

```go
// 红色目标标记（占位符）
RedMarkerRange: HSVRange{
    LowerH: 0,
    LowerS: 100,
    LowerV: 200,
    UpperH: 10,
    UpperS: 255,
    UpperV: 255,
}

// 蓝色目标标记（占位符）
BlueMarkerRange: HSVRange{
    LowerH: 100,
    LowerS: 80,
    LowerV: 180,
    UpperH: 130,
    UpperS: 255,
    UpperV: 255,
}
```

---

## 4. 如何更新 HSV 颜色值

### 4.1 使用 Python + OpenCV 测试脚本

创建一个测试脚本来调整 HSV 值：

```python
import cv2
import numpy as np

# 读取游戏截图
img = cv2.imread('screenshot.png')
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

# 定义 HSV 范围（需要调整）
lower_red = np.array([0, 100, 100])
upper_red = np.array([10, 255, 255])

# 创建掩码
mask = cv2.inRange(hsv, lower_red, upper_red)

# 显示结果
cv2.imshow('Original', img)
cv2.imshow('Mask', mask)
cv2.waitKey(0)
```

### 4.2 HSV 颜色参考

| 颜色 | H (色调) | S (饱和度) | V (明度) |
|------|---------|-----------|---------|
| 红色 | 0-10, 160-180 | 100-255 | 100-255 |
| 黄色 | 20-35 | 100-255 | 150-255 |
| 绿色 | 40-80 | 100-255 | 100-255 |
| 蓝色 | 100-130 | 100-255 | 150-255 |
| 紫色 | 130-160 | 100-255 | 100-255 |

**注意**：OpenCV 中 H 的范围是 0-180（而不是 0-360）

### 4.3 更新位置

- **stats.go**: `GetStatusBarConfig()` 函数中的 `HSVRange` 字段
- **analyzer.go**: `GetDefaultMobColorConfig()` 函数中的各个 `HSVRange` 字段

---

## 5. 形态学操作说明

### 5.1 开运算（Opening）= 腐蚀 + 膨胀

**作用**：去除小的噪点和细小突起

```
原图:  ████  ·  ████
腐蚀:  ███      ███
膨胀:  ████     ████
结果:  ████     ████  (小噪点 · 被去除)
```

### 5.2 闭运算（Closing）= 膨胀 + 腐蚀

**作用**：填充小孔和缝隙

```
原图:  ████  ████
膨胀:  █████████
腐蚀:  ████████
结果:  ████████  (中间的间隙被填充)
```

### 5.3 代码实现

```go
func applyMorphology(mask *gocv.Mat) gocv.Mat {
    kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))

    // 开运算：去除噪点
    gocv.Erode(*mask, &temp, kernel)
    gocv.Dilate(temp, &opened, kernel)

    // 闭运算：填充小孔
    gocv.Dilate(opened, &temp2, kernel)
    gocv.Erode(temp2, &result, kernel)

    return result
}
```

---

## 6. 优势总结

### 6.1 新方法的优势

1. **更鲁棒**：HSV 对光照变化不敏感
2. **更准确**：轮廓检测提供精确的形状信息
3. **更少噪点**：形态学操作有效去除噪声
4. **更易调整**：只需调整 HSV 范围，而不是多个精确 RGB 值
5. **更好的性能**：OpenCV 高度优化，C++ 实现

### 6.2 需要注意的地方

1. **依赖 gocv**：需要安装 `gocv.io/x/gocv` 和 OpenCV
2. **颜色调优**：HSV 范围需要根据实际游戏画面调整
3. **内存管理**：需要正确 `defer Close()` 释放 Mat 资源

---

## 7. 下一步行动

1. **安装依赖**：
   ```bash
   go get -u gocv.io/x/gocv
   ```

2. **更新 HSV 颜色值**：
   - 截取游戏画面
   - 使用 Python/OpenCV 或在线工具找到正确的 HSV 范围
   - 更新 `GetStatusBarConfig()` 和 `GetDefaultMobColorConfig()` 中的值

3. **测试**：
   - 测试 HP/MP/FP 检测
   - 测试怪物检测
   - 测试目标标记检测

4. **微调**：
   - 调整形态学操作的内核大小
   - 调整轮廓过滤条件
   - 优化 ROI 区域

---

## 附录：备份文件

原始文件已备份：
- `stats.go.back` - 原始 stats.go（RGB 方法）
- `analyzer.go.back` - 原始 analyzer.go（RGB 方法）

如需恢复原始版本：
```bash
mv stats.go.back stats.go
mv analyzer.go.back analyzer.go
```
