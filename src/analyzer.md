# analyzer.go 实现逻辑详解

## 目录

1. [整体架构](#1-整体架构)
2. [核心数据结构](#2-核心数据结构)
3. [怪物检测流程](#3-怪物检测流程)
4. [目标标记检测](#4-目标标记检测)
5. [目标距离计算](#5-目标距离计算)
6. [找最近的怪物](#6-找最近的怪物)
7. [避免区域管理](#7-避免区域管理)
8. [辅助方法](#8-辅助方法)
9. [使用示例](#9-使用示例)
10. [调优指南](#10-调优指南)

---

## 1. 整体架构

### 1.1 设计目标

`analyzer.go` 是游戏 bot 的**图像分析核心模块**，负责：
- 🎮 **屏幕捕获**：获取游戏画面
- 👾 **怪物识别**：检测被动怪、攻击性怪、紫色怪
- 🎯 **目标标记检测**：检测选中目标的标记（红色/蓝色）
- 📏 **距离计算**：计算到目标的距离
- 🔍 **智能选择**：找离玩家最近的怪物
- 🚫 **区域回避**：管理需要避开的区域

### 1.2 架构层次

```
┌─────────────────────────────────────────────────────────┐
│                   ImageAnalyzer                          │
│  (主控制器：管理图像分析的所有功能)                        │
└─────────────────────────────────────────────────────────┘
                         │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│ 怪物检测      │ │ 目标标记检测  │ │ 状态栏检测    │
│ IdentifyMobs │ │ DetectMarker │ │ UpdateStats  │
└──────────────┘ └──────────────┘ └──────────────┘
        │               │               │
        └───────────────┴───────────────┘
                    │
                    ▼
        ┌───────────────────────────┐
        │   OpenCV HSV 检测流程      │
        │  HSV掩码 → 形态学 → 轮廓   │
        └───────────────────────────┘
```

### 1.3 主要功能模块

| 功能 | 函数 | 用途 |
|------|------|------|
| **屏幕捕获** | `Capture()` | 截取游戏画面 |
| **怪物识别** | `IdentifyMobs()` | 找出所有怪物 |
| **目标标记** | `DetectTargetMarker()` | 检测是否选中了目标 |
| **距离计算** | `DetectTargetDistance()` | 计算到目标的距离 |
| **最近怪物** | `FindClosestMob()` | 找离玩家最近的怪 |
| **状态更新** | `UpdateStats()` | 更新 HP/MP/FP 等状态 |

---

## 2. 核心数据结构

### 2.1 ImageAnalyzer - 图像分析器

```go
type ImageAnalyzer struct {
    browser        *Browser         // 浏览器接口（用于截图）
    screenInfo     *ScreenInfo      // 屏幕信息（宽高）
    lastImage      *image.RGBA      // 最后一次截图
    stats          *ClientStats     // 客户端状态（HP/MP等）
    mobColorConfig *MobColorConfig  // 怪物颜色配置
    mu             sync.RWMutex     // 线程安全锁
}
```

**字段说明**：

| 字段 | 类型 | 作用 |
|------|------|------|
| `browser` | `*Browser` | 浏览器对象，负责截图 |
| `screenInfo` | `*ScreenInfo` | 屏幕分辨率信息 |
| `lastImage` | `*image.RGBA` | 缓存的最新截图，避免重复截图 |
| `stats` | `*ClientStats` | 玩家状态（HP/MP/FP/目标HP等） |
| `mobColorConfig` | `*MobColorConfig` | 怪物颜色的 HSV 配置 |
| `mu` | `sync.RWMutex` | 保护 `lastImage` 的并发访问 |

**创建方式**：
```go
analyzer := NewImageAnalyzer(browser)
```

---

### 2.2 ROI - 感兴趣区域

```go
type ROI struct {
    X      int  // 左上角 X 坐标
    Y      int  // 左上角 Y 坐标
    Width  int  // 宽度
    Height int  // 高度
}
```

**作用**：定义图像中需要检测的矩形区域。

**示例**：
```
游戏画面 (1920x1080)
┌─────────────────────────────────────┐
│                                     │
│  ┌─────────────────────┐           │
│  │  目标标记搜索区域    │           │
│  │  ROI{480, 180,      │           │
│  │      960, 360}      │           │
│  └─────────────────────┘           │
│                                     │
│                                     │
│                                     │
│  ████████████████████████████████  │ ← 底部 UI（不搜索）
└─────────────────────────────────────┘
   ↑                                 ↑
   (0,0)                     (1920,1080)
```

---

### 2.3 MobColorConfig - 怪物颜色配置

```go
type MobColorConfig struct {
    PassiveMobRange    HSVRange // 被动怪（黄色名字）
    AggressiveMobRange HSVRange // 攻击性怪（红色名字）
    VioletMobRange     HSVRange // 紫色怪（紫色名字）
    RedMarkerRange     HSVRange // 红色目标标记
    BlueMarkerRange    HSVRange // 蓝色目标标记
}
```

**作用**：存储所有怪物和标记的 HSV 颜色范围。

**默认配置**（占位符）：
```go
func GetDefaultMobColorConfig() *MobColorConfig {
    return &MobColorConfig{
        // 被动怪 - 黄色名字
        PassiveMobRange: HSVRange{
            LowerH: 20, LowerS: 100, LowerV: 150,
            UpperH: 35, UpperS: 255, UpperV: 255,
        },

        // 攻击性怪 - 红色名字
        AggressiveMobRange: HSVRange{
            LowerH: 0, LowerS: 150, LowerV: 150,
            UpperH: 10, UpperS: 255, UpperV: 255,
        },

        // 紫色怪 - 紫色名字
        VioletMobRange: HSVRange{
            LowerH: 130, LowerS: 100, LowerV: 100,
            UpperH: 160, UpperS: 255, UpperV: 255,
        },

        // 红色标记
        RedMarkerRange: HSVRange{
            LowerH: 0, LowerS: 100, LowerV: 200,
            UpperH: 10, UpperS: 255, UpperV: 255,
        },

        // 蓝色标记
        BlueMarkerRange: HSVRange{
            LowerH: 100, LowerS: 80, LowerV: 180,
            UpperH: 130, UpperS: 255, UpperV: 255,
        },
    }
}
```

**颜色对照表**：

| 怪物类型 | 名字颜色 | HSV H 范围 | 行为 |
|---------|---------|-----------|------|
| 被动怪 | 黄色 | 20-35 | 不主动攻击 |
| 攻击性怪 | 红色 | 0-10 | 主动攻击 |
| 紫色怪 | 紫色 | 130-160 | 特殊怪物（通常过滤） |

---

### 2.4 AvoidedArea & AvoidanceList - 避免区域

```go
// 单个避免区域
type AvoidedArea struct {
    Bounds    Bounds        // 区域边界
    CreatedAt time.Time     // 创建时间
    Duration  time.Duration // 持续时间
}

// 避免区域列表
type AvoidanceList struct {
    areas []AvoidedArea  // 所有避免区域
    mu    sync.RWMutex   // 线程安全锁
}
```

**作用**：管理需要避开的区域，例如：
- 🚫 死过的地方（避免重复送死）
- 🚫 卡住的位置（避免重复卡住）
- 🚫 危险区域（高级怪聚集地）

**使用场景**：
```go
// 添加避免区域：在 (100, 200) 附近，避免 30 秒
avoidList.Add(Bounds{X: 100, Y: 200, W: 50, H: 50}, 30*time.Second)

// 检查怪物是否在避免区域内
if avoidList.IsAvoided(mob.Bounds) {
    // 跳过这个怪物
    continue
}

// 清理过期的避免区域
avoidList.CleanExpired()
```

---

## 3. 怪物检测流程

### 3.1 整体流程图

```
┌─────────────────┐
│ 1. 捕获游戏画面  │
│    Capture()    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. 转换为 HSV   │
│    CvtColor()   │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│ 3. 定义搜索 ROI                          │
│    避免底部 UI：Height - 100             │
└────────┬────────────────────────────────┘
         │
    ┌────┴────┬──────────────┐
    │         │              │
    ▼         ▼              ▼
┌────────┐ ┌────────┐  ┌────────┐
│被动怪  │ │攻击性怪│  │紫色怪  │
│黄色    │ │红色    │  │紫色    │
└────┬───┘ └───┬────┘  └───┬────┘
     │         │           │
     └────┬────┴─────┬─────┘
          │          │
          ▼          ▼
    ┌──────────────────────┐
    │ 4. HSV 颜色掩码       │
    │    inRange()         │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │ 5. 形态学操作         │
    │    开运算 + 闭运算    │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │ 6. 查找轮廓           │
    │    FindContours()    │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │ 7. 过滤轮廓           │
    │  - 宽度约束           │
    │  - 位置过滤           │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │ 8. 返回怪物列表       │
    │    []Target          │
    └──────────────────────┘
```

### 3.2 IdentifyMobs() - 识别所有怪物

**函数签名**：
```go
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target
```

**输入**：
- `config`：配置对象，包含 `MinMobNameWidth` 和 `MaxMobNameWidth`

**输出**：
- `[]Target`：检测到的怪物列表

**执行步骤**：

```go
func (ia *ImageAnalyzer) IdentifyMobs(config *Config) []Target {
    // 1. 获取截图
    img := ia.GetImage()
    if img == nil {
        return nil
    }

    // 2. 转换为 OpenCV Mat 格式
    mat := ia.imageToMat(img)
    defer mat.Close()

    // 3. 转换为 HSV 色彩空间
    hsvMat := gocv.NewMat()
    defer hsvMat.Close()
    gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

    // 4. 定义搜索区域（避免底部 UI）
    searchROI := ROI{
        X:      0,
        Y:      0,
        Width:  ia.screenInfo.Width,
        Height: ia.screenInfo.Height - 100, // 底部 100 像素是 UI
    }

    var mobs []Target

    // 5. 检测被动怪（黄色名字）
    passiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.PassiveMobRange, config)
    for _, bounds := range passiveBounds {
        // 过滤：Y >= 110（避免顶部 HP 栏区域）
        if bounds.Y >= 110 {
            mobs = append(mobs, Target{
                Type:   MobPassive,
                Bounds: bounds,
            })
        }
    }

    // 6. 检测攻击性怪（红色名字）
    aggressiveBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.AggressiveMobRange, config)
    for _, bounds := range aggressiveBounds {
        if bounds.Y >= 110 {
            mobs = append(mobs, Target{
                Type:   MobAggressive,
                Bounds: bounds,
            })
        }
    }

    // 7. 检测紫色怪（仅日志，不添加到列表）
    violetBounds := ia.detectMobsByHSV(&hsvMat, searchROI, ia.mobColorConfig.VioletMobRange, config)
    if len(violetBounds) > 0 {
        LogDebug("Detected %d violet mobs (filtered out)", len(violetBounds))
    }

    return mobs
}
```

### 3.3 detectMobsByHSV() - HSV 怪物检测

**函数签名**：
```go
func (ia *ImageAnalyzer) detectMobsByHSV(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange, config *Config) []Bounds
```

**输入**：
- `hsvMat`：HSV 格式的图像
- `roi`：搜索区域
- `colorRange`：颜色范围（如黄色、红色）
- `config`：配置（包含宽度约束）

**输出**：
- `[]Bounds`：检测到的边界框列表

**详细步骤**：

```go
func (ia *ImageAnalyzer) detectMobsByHSV(...) []Bounds {
    // 1. 提取 ROI
    roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
    defer roiMat.Close()

    // 2. 创建 HSV 颜色掩码
    mask := ia.createHSVMask(&roiMat, colorRange)
    defer mask.Close()

    // 3. 形态学操作（去噪 + 填洞）
    morphed := ia.applyMorphology(&mask)
    defer morphed.Close()

    // 4. 查找轮廓
    contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
    defer contours.Close()

    // 5. 转换轮廓为边界框并过滤
    var bounds []Bounds
    for i := 0; i < contours.Size(); i++ {
        contour := contours.At(i)
        rect := gocv.BoundingRect(contour)

        // 过滤 1：宽度约束（怪物名字的典型宽度）
        if rect.Dx() > config.MinMobNameWidth && rect.Dx() < config.MaxMobNameWidth {
            // 转换回屏幕坐标
            screenBounds := Bounds{
                X: roi.X + rect.Min.X,
                Y: roi.Y + rect.Min.Y,
                W: rect.Dx(),
                H: rect.Dy(),
            }

            // 过滤 2：跳过左上角 HP 栏区域
            if screenBounds.X <= 250 && screenBounds.Y <= 110 {
                continue
            }

            bounds = append(bounds, screenBounds)
        }
    }

    return bounds
}
```

### 3.4 过滤逻辑详解

#### 过滤 1：宽度约束

怪物名字的宽度通常在一定范围内：

```
MinMobNameWidth = 30 像素
MaxMobNameWidth = 150 像素

太小:                       合适:                   太大:
"·"  (噪点)              "Lawolf"               "═══════════════"
宽度 < 30               30 < 宽度 < 150         宽度 > 150
❌ 拒绝                 ✅ 接受                  ❌ 拒绝
```

**配置示例**：
```go
type Config struct {
    MinMobNameWidth int // 默认 30
    MaxMobNameWidth int // 默认 150
}
```

#### 过滤 2：位置约束

避免误检测 HP 栏和底部 UI：

```
游戏画面布局
┌─────────────────────────────────┐
│ HP/MP/FP 栏                     │ ← Y < 110, X < 250
│ (避免区域)                       │    ❌ 拒绝
├─────────────────────────────────┤
│                                 │
│        怪物检测区域              │ ← Y >= 110
│        ✅ 接受                   │
│                                 │
│                                 │
├─────────────────────────────────┤
│ 底部 UI                          │ ← Height - 100
│ (搜索 ROI 不包括这里)            │    自动排除
└─────────────────────────────────┘
```

**代码**：
```go
// 跳过左上角 HP 栏区域
if screenBounds.X <= 250 && screenBounds.Y <= 110 {
    continue
}

// 跳过顶部状态栏区域（第二层过滤）
if bounds.Y < 110 {
    continue
}
```

---

## 4. 目标标记检测

### 4.1 目标标记的作用

当玩家选中一个目标（怪物或 NPC）时，目标头顶会出现一个**标记**：
- 🔴 **红色标记**：普通区域
- 🔵 **蓝色标记**：特殊区域（如 Azria）

**可视化**：
```
游戏画面
┌─────────────────────────────────┐
│                                 │
│         ▲                       │
│        ╱ ╲  ← 蓝色/红色标记     │
│       ╱   ╲                     │
│      ┌─────┐                    │
│      │怪物 │                     │
│      └─────┘                    │
│                                 │
└─────────────────────────────────┘
```

### 4.2 DetectTargetMarkerOpenCV() - 检测目标标记

**函数签名**：
```go
func (ia *ImageAnalyzer) DetectTargetMarkerOpenCV(hsvMat *gocv.Mat) bool
```

**输入**：
- `hsvMat`：HSV 格式的游戏画面

**输出**：
- `bool`：是否检测到目标标记

**执行流程**：

```go
func (ia *ImageAnalyzer) DetectTargetMarkerOpenCV(hsvMat *gocv.Mat) bool {
    // 1. 定义标记搜索区域（屏幕中上部分）
    markerROI := ROI{
        X:      ia.screenInfo.Width / 4,      // 左边界：1/4 屏幕宽度
        Y:      ia.screenInfo.Height / 6,     // 上边界：1/6 屏幕高度
        Width:  ia.screenInfo.Width / 2,      // 宽度：1/2 屏幕宽度
        Height: ia.screenInfo.Height / 3,     // 高度：1/3 屏幕高度
    }

    // 2. 先尝试蓝色标记（某些区域使用蓝色）
    blueMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.BlueMarkerRange)
    if blueMarkerDetected {
        LogDebug("Blue target marker detected")
        return true
    }

    // 3. 再尝试红色标记（普通区域）
    redMarkerDetected := ia.detectMarker(hsvMat, markerROI, ia.mobColorConfig.RedMarkerRange)
    if redMarkerDetected {
        LogDebug("Red target marker detected")
        return true
    }

    return false
}
```

**搜索区域可视化**：

```
屏幕 (1920x1080)
┌─────────────────────────────────────┐
│                                     │ ← Y = 180 (1/6 高度)
│  ┌───────────────────────────┐     │
│  │   标记搜索区域 (ROI)       │     │
│  │   X: 480 (1/4 宽度)       │     │
│  │   Y: 180 (1/6 高度)       │     │
│  │   Width: 960 (1/2 宽度)   │     │
│  │   Height: 360 (1/3 高度)  │     │
│  │                           │     │
│  └───────────────────────────┘     │
│                                     │
│                                     │
└─────────────────────────────────────┘
   ↑                             ↑
   X=480                         X=1440
```

### 4.3 detectMarker() - 检测单个颜色的标记

**函数签名**：
```go
func (ia *ImageAnalyzer) detectMarker(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) bool
```

**执行流程**：

```go
func (ia *ImageAnalyzer) detectMarker(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) bool {
    // 1. 提取 ROI
    roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
    defer roiMat.Close()

    // 2. 创建 HSV 颜色掩码
    mask := ia.createHSVMask(&roiMat, colorRange)
    defer mask.Close()

    // 3. 统计非零像素（白色像素数量）
    nonZero := gocv.CountNonZero(mask)

    // 4. 阈值判断：至少 20 个像素才认为检测到标记
    return nonZero > 20
}
```

**原理图解**：

```
原始 ROI              HSV 掩码              像素计数
┌─────────────┐      ┌─────────────┐      nonZero = 42
│             │      │             │
│    ▲        │ HSV  │    ███      │      42 > 20 ✅
│   ╱ ╲       │ →    │   █████     │      检测到标记！
│  ╱   ╲      │ 掩码 │    ███      │
│ (蓝色标记)  │      │             │
│             │      │             │
└─────────────┘      └─────────────┘
```

**为什么是 20 个像素？**

- 太小（如 5）：噪点也会触发
- 太大（如 100）：小的标记可能漏掉
- **20 个像素**：经验值，平衡准确性和召回率

---

## 5. 目标距离计算

### 5.1 DetectTargetDistance() - 计算目标距离

**作用**：计算目标标记到屏幕中心的距离，用于判断是否需要移动。

**函数签名**：
```go
func (ia *ImageAnalyzer) DetectTargetDistance() int
```

**输出**：
- `int`：像素距离（如果没有目标，返回 9999）

**执行流程**：

```go
func (ia *ImageAnalyzer) DetectTargetDistance() int {
    // 1. 获取图像并转换为 HSV
    img := ia.GetImage()
    mat := ia.imageToMat(img)
    defer mat.Close()

    hsvMat := gocv.NewMat()
    defer hsvMat.Close()
    gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)

    // 2. 定义搜索区域
    markerROI := ROI{
        X:      ia.screenInfo.Width / 4,
        Y:      ia.screenInfo.Height / 6,
        Width:  ia.screenInfo.Width / 2,
        Height: ia.screenInfo.Height / 3,
    }

    // 3. 查找标记中心点
    markerCenter := ia.findMarkerCenter(&hsvMat, markerROI)
    if markerCenter == nil {
        return 9999  // 未找到标记
    }

    // 4. 计算到屏幕中心的距离
    centerX := ia.screenInfo.Width / 2
    centerY := ia.screenInfo.Height / 2

    dx := float64(markerCenter.X - centerX)
    dy := float64(markerCenter.Y - centerY)
    distance := int(math.Sqrt(dx*dx + dy*dy))

    return distance
}
```

### 5.2 findMarkerCenter() - 查找标记中心

**函数签名**：
```go
func (ia *ImageAnalyzer) findMarkerCenter(hsvMat *gocv.Mat, roi ROI) *Point
```

**执行流程**：

```go
func (ia *ImageAnalyzer) findMarkerCenter(hsvMat *gocv.Mat, roi ROI) *Point {
    // 先尝试蓝色标记
    blueCenter := ia.findMarkerCenterByColor(hsvMat, roi, ia.mobColorConfig.BlueMarkerRange)
    if blueCenter != nil {
        return blueCenter
    }

    // 再尝试红色标记
    redCenter := ia.findMarkerCenterByColor(hsvMat, roi, ia.mobColorConfig.RedMarkerRange)
    return redCenter
}
```

### 5.3 findMarkerCenterByColor() - 查找特定颜色标记的中心

**执行流程**：

```go
func (ia *ImageAnalyzer) findMarkerCenterByColor(hsvMat *gocv.Mat, roi ROI, colorRange HSVRange) *Point {
    // 1. 提取 ROI
    roiMat := hsvMat.Region(image.Rect(roi.X, roi.Y, roi.X+roi.Width, roi.Y+roi.Height))
    defer roiMat.Close()

    // 2. 创建掩码
    mask := ia.createHSVMask(&roiMat, colorRange)
    defer mask.Close()

    // 3. 查找轮廓
    contours := gocv.FindContours(mask, gocv.RetrievalExternal, gocv.ChainApproxSimple)
    defer contours.Close()

    if contours.Size() == 0 {
        return nil  // 未找到轮廓
    }

    // 4. 找最大轮廓（主标记）
    maxArea := 0.0
    var maxRect image.Rectangle
    for i := 0; i < contours.Size(); i++ {
        contour := contours.At(i)
        area := gocv.ContourArea(contour)
        if area > maxArea {
            maxArea = area
            maxRect = gocv.BoundingRect(contour)
        }
    }

    // 5. 计算中心点（转换回屏幕坐标）
    centerX := roi.X + maxRect.Min.X + maxRect.Dx()/2
    centerY := roi.Y + maxRect.Min.Y + maxRect.Dy()/2

    return &Point{X: centerX, Y: centerY}
}
```

**可视化**：

```
标记 ROI              掩码                 找最大轮廓            计算中心
┌─────────────┐      ┌─────────────┐     ┌─────────────┐      ┌─────────────┐
│             │      │             │     │  ┌───────┐  │      │      ●      │
│    ▲        │ HSV  │    ███      │ 轮廓│  │███████│  │ 中心 │   (中心点)  │
│   ╱ ╲       │ →    │   █████     │  →  │  │███████│  │  →   │   centerX   │
│  ╱   ╲      │ 掩码 │  ███████    │     │  │███████│  │      │   centerY   │
│             │      │   █████     │     │  └───────┘  │      │             │
│             │      │    ███      │     │  最大轮廓   │      │             │
└─────────────┘      └─────────────┘     └─────────────┘      └─────────────┘
```

### 5.4 距离计算示例

```
屏幕 (1920x1080)
┌─────────────────────────────────────┐
│                                     │
│         ● (标记)                    │
│        (800, 300)                   │
│           ╲                         │
│            ╲ distance               │
│             ╲                       │
│              ╲                      │
│               ● (屏幕中心)          │
│              (960, 540)             │
│                                     │
└─────────────────────────────────────┘

计算：
dx = 800 - 960 = -160
dy = 300 - 540 = -240
distance = √(160² + 240²)
         = √(25600 + 57600)
         = √83200
         ≈ 288 像素
```

---

## 6. 找最近的怪物

### 6.1 FindClosestMob() - 找最近的怪物

**函数签名**：
```go
func (ia *ImageAnalyzer) FindClosestMob(mobs []Target) *Target
```

**输入**：
- `mobs`：检测到的所有怪物列表

**输出**：
- `*Target`：离玩家最近的怪物（如果没有返回 `nil`）

**执行流程**：

```go
func (ia *ImageAnalyzer) FindClosestMob(mobs []Target) *Target {
    if len(mobs) == 0 {
        return nil
    }

    // 屏幕中心（玩家位置）
    centerX := ia.screenInfo.Width / 2
    centerY := ia.screenInfo.Height / 2

    var closest *Target
    minDistance := float64(99999)

    // 最大距离阈值：325 像素
    // 超过此距离的怪物被认为太远，无法攻击
    maxDistance := 325.0

    // 遍历所有怪物
    for i := range mobs {
        // 计算怪物中心点
        mobX := mobs[i].Bounds.X + mobs[i].Bounds.W/2
        mobY := mobs[i].Bounds.Y + mobs[i].Bounds.H/2

        // 计算到屏幕中心的距离
        dx := float64(mobX - centerX)
        dy := float64(mobY - centerY)
        distance := math.Sqrt(dx*dx + dy*dy)

        // 过滤：跳过太远的怪物
        if distance > maxDistance {
            continue
        }

        // 更新最近的怪物
        if distance < minDistance {
            minDistance = distance
            closest = &mobs[i]
        }
    }

    return closest
}
```

### 6.2 距离阈值说明

**为什么是 325 像素？**

```
攻击范围示意图（俯视图）

             ┌───────────────┐
             │               │
             │               │
             │   ┌───────┐   │ ← 325px 半径圆
             │   │       │   │
             │   │  玩家 │   │
             │   │   ●   │   │
             │   └───────┘   │
             │               │
             │               │
             └───────────────┘

超出 325px 的怪物：
- 技能打不到
- 走过去太远，不值得
- 可能脱离当前区域
```

**可调整**：
```go
maxDistance := 325.0  // 正常刷怪
maxDistance := 500.0  // 大范围搜索（circle 模式）
maxDistance := 200.0  // 小范围刷怪（密集区域）
```

### 6.3 算法示例

```
怪物列表：
Mob1: (500, 400) - 被动怪
Mob2: (700, 500) - 被动怪
Mob3: (1200, 300) - 攻击性怪
Mob4: (950, 550) - 被动怪

屏幕中心: (960, 540)

计算距离：
Mob1: √((500-960)² + (400-540)²) = √(211600 + 19600) = √231200 ≈ 481px  ❌ 太远 (> 325)
Mob2: √((700-960)² + (500-540)²) = √(67600 + 1600)  = √69200  ≈ 263px  ✅
Mob3: √((1200-960)² + (300-540)²) = √(57600 + 57600) = √115200 ≈ 339px  ❌ 太远
Mob4: √((950-960)² + (550-540)²) = √(100 + 100)     = √200    ≈ 14px   ✅

过滤后：
Mob2: 263px ✅
Mob4: 14px  ✅ ← 最近！

返回: Mob4
```

---

## 7. 避免区域管理

### 7.1 使用场景

**避免区域（AvoidanceList）** 用于记录需要暂时避开的地方：

| 场景 | 原因 | 持续时间 |
|------|------|---------|
| **死亡位置** | 防止重复送死 | 60 秒 |
| **卡住位置** | 防止重复卡住 | 30 秒 |
| **PK 区域** | 避开其他玩家 | 120 秒 |
| **高级怪区** | 打不过的怪 | 300 秒 |

### 7.2 AvoidanceList API

#### 添加避免区域

```go
func (al *AvoidanceList) Add(bounds Bounds, duration time.Duration)
```

**示例**：
```go
// 在 (500, 300) 附近死了，避免 60 秒
deathArea := Bounds{X: 450, Y: 250, W: 100, H: 100}
avoidList.Add(deathArea, 60*time.Second)
```

#### 检查是否在避免区域

```go
func (al *AvoidanceList) IsAvoided(bounds Bounds) bool
```

**示例**：
```go
mob := Target{Bounds: Bounds{X: 480, Y: 280, W: 50, H: 20}}
if avoidList.IsAvoided(mob.Bounds) {
    // 这个怪物在避免区域内，跳过
    continue
}
```

#### 清理过期区域

```go
func (al *AvoidanceList) CleanExpired()
```

**示例**：
```go
// 每 10 秒清理一次
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    avoidList.CleanExpired()
}
```

### 7.3 边界重叠检测

**boundsOverlap() - 检测两个矩形是否重叠**

```go
func boundsOverlap(a, b Bounds) bool {
    return a.X < b.X+b.W &&
           a.X+a.W > b.X &&
           a.Y < b.Y+b.H &&
           a.Y+a.H > b.Y
}
```

**可视化**：

```
情况 1：重叠 ✅
┌─────────┐
│  A      │
│    ┌────┼────┐
│    │重叠│    │
└────┼────┘    │
     │    B    │
     └─────────┘
返回: true

情况 2：不重叠 ❌
┌─────────┐       ┌─────────┐
│  A      │       │    B    │
└─────────┘       └─────────┘
返回: false

情况 3：包含 ✅
┌───────────────┐
│  A            │
│  ┌─────────┐  │
│  │    B    │  │
│  └─────────┘  │
└───────────────┘
返回: true
```

**算法原理**（AABB 碰撞检测）：

两个矩形重叠的条件：
```
A.left < B.right   AND
A.right > B.left   AND
A.top < B.bottom   AND
A.bottom > B.top
```

代码对应：
```go
a.X < b.X+b.W      // A 左边 < B 右边
a.X+a.W > b.X      // A 右边 > B 左边
a.Y < b.Y+b.H      // A 上边 < B 下边
a.Y+a.H > b.Y      // A 下边 > B 上边
```

### 7.4 完整使用示例

```go
// 创建避免列表
avoidList := NewAvoidanceList()

// 游戏循环
for {
    // 1. 识别怪物
    mobs := analyzer.IdentifyMobs(config)

    // 2. 过滤在避免区域内的怪物
    var validMobs []Target
    for _, mob := range mobs {
        if !avoidList.IsAvoided(mob.Bounds) {
            validMobs = append(validMobs, mob)
        }
    }

    // 3. 找最近的怪物
    closestMob := analyzer.FindClosestMob(validMobs)
    if closestMob != nil {
        // 攻击这个怪物
        attack(closestMob)
    }

    // 4. 如果死亡，添加避免区域
    if stats.IsAlive == AliveStateDead {
        deathPos := Bounds{X: playerX-50, Y: playerY-50, W: 100, H: 100}
        avoidList.Add(deathPos, 60*time.Second)
    }

    // 5. 定期清理过期区域
    avoidList.CleanExpired()
}
```

---

## 8. 辅助方法

### 8.1 imageToMat() - 图像格式转换

**作用**：将 Go 的 `image.RGBA` 转换为 OpenCV 的 `gocv.Mat`。

```go
func (ia *ImageAnalyzer) imageToMat(img *image.RGBA) gocv.Mat {
    if img == nil {
        return gocv.NewMat()
    }

    // RGBA → BGR（OpenCV 使用 BGR 顺序）
    mat, err := gocv.ImageToMatRGB(img)
    if err != nil {
        LogError("Failed to convert image to mat: %v", err)
        return gocv.NewMat()
    }

    return mat
}
```

**注意**：OpenCV 使用 **BGR** 而不是 RGB！

### 8.2 createHSVMask() - 创建 HSV 掩码

**作用**：根据 HSV 颜色范围创建二值掩码。

```go
func (ia *ImageAnalyzer) createHSVMask(hsvMat *gocv.Mat, colorRange HSVRange) gocv.Mat {
    // 创建上下界
    lower := gocv.NewScalar(float64(colorRange.LowerH), float64(colorRange.LowerS), float64(colorRange.LowerV), 0)
    upper := gocv.NewScalar(float64(colorRange.UpperH), float64(colorRange.UpperS), float64(colorRange.UpperV), 0)

    // 创建掩码
    mask := gocv.NewMat()
    gocv.InRangeWithScalar(*hsvMat, lower, upper, &mask)

    return mask
}
```

**原理**：对每个像素 `(h, s, v)`，如果在范围内则设为 255，否则设为 0。

### 8.3 applyMorphology() - 形态学操作

**作用**：去除噪点，填充孔洞。

```go
func (ia *ImageAnalyzer) applyMorphology(mask *gocv.Mat) gocv.Mat {
    kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
    defer kernel.Close()

    // 开运算：腐蚀 + 膨胀 → 去噪
    temp := gocv.NewMat()
    gocv.Erode(*mask, &temp, kernel)
    opened := gocv.NewMat()
    gocv.Dilate(temp, &opened, kernel)
    temp.Close()

    // 闭运算：膨胀 + 腐蚀 → 填洞
    temp2 := gocv.NewMat()
    gocv.Dilate(opened, &temp2, kernel)
    result := gocv.NewMat()
    gocv.Erode(temp2, &result, kernel)
    temp2.Close()
    opened.Close()

    return result
}
```

**效果**：
```
原始掩码        开运算          闭运算
███·███        ████████        ████████
███ ███   →    ████████   →    ████████
██· ███        ████████        ████████

去除小噪点(·)    连接断裂         平滑结果
```

---

## 9. 使用示例

### 9.1 完整游戏循环

```go
package main

import (
    "time"
)

func main() {
    // 1. 初始化
    browser := NewBrowser()
    analyzer := NewImageAnalyzer(browser)
    config := LoadConfig()
    avoidList := NewAvoidanceList()

    // 2. 游戏主循环
    ticker := time.NewTicker(100 * time.Millisecond)
    for range ticker.C {
        // 3. 捕获屏幕
        analyzer.Capture()

        // 4. 更新状态（HP/MP/FP）
        analyzer.UpdateStats()
        stats := analyzer.GetStats()

        // 5. 检查存活状态
        if stats.IsAlive == AliveStateDead {
            // 死亡，记录位置并复活
            playerPos := getPlayerPosition()
            avoidList.Add(Bounds{X: playerPos.X-50, Y: playerPos.Y-50, W: 100, H: 100}, 60*time.Second)
            respawn()
            continue
        }

        // 6. 检查是否有目标
        hasTarget := analyzer.DetectTargetMarker()
        if hasTarget {
            // 有目标，检查距离
            distance := analyzer.DetectTargetDistance()
            if distance > 50 {
                // 太远，移动靠近
                moveToTarget()
            } else {
                // 够近，攻击
                attack()
            }
            continue
        }

        // 7. 没有目标，寻找新怪物
        mobs := analyzer.IdentifyMobs(config)

        // 8. 过滤避免区域内的怪物
        var validMobs []Target
        for _, mob := range mobs {
            if !avoidList.IsAvoided(mob.Bounds) {
                validMobs = append(validMobs, mob)
            }
        }

        // 9. 找最近的怪物
        closestMob := analyzer.FindClosestMob(validMobs)
        if closestMob != nil {
            // 点击怪物
            clickMob(closestMob)
        } else {
            // 没有怪物，移动到下一个位置
            moveToNextSpot()
        }

        // 10. 清理过期的避免区域
        avoidList.CleanExpired()

        // 11. 检查血量，必要时喝药
        if stats.GetHPPercent() < 30 {
            useHPPotion()
        }
        if stats.GetMPPercent() < 20 {
            useMPPotion()
        }
    }
}
```

### 9.2 简单的怪物检测示例

```go
// 只检测怪物并显示
func detectMobs() {
    analyzer := NewImageAnalyzer(browser)
    config := &Config{
        MinMobNameWidth: 30,
        MaxMobNameWidth: 150,
    }

    for {
        // 捕获并检测
        analyzer.Capture()
        mobs := analyzer.IdentifyMobs(config)

        // 输出结果
        fmt.Printf("Found %d mobs:\n", len(mobs))
        for i, mob := range mobs {
            fmt.Printf("  Mob %d: Type=%s, Pos=(%d,%d), Size=%dx%d\n",
                i+1, mob.Type, mob.Bounds.X, mob.Bounds.Y, mob.Bounds.W, mob.Bounds.H)
        }

        time.Sleep(1 * time.Second)
    }
}
```

---

## 10. 调优指南

### 10.1 调整怪物颜色范围

如果检测不准确，需要调整 HSV 颜色范围。

#### 方法 1：使用 Python 工具

```python
import cv2
import numpy as np

# 读取游戏截图
img = cv2.imread('screenshot.png')
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

# 调整这些值
lower_yellow = np.array([20, 100, 150])  # 被动怪（黄色）
upper_yellow = np.array([35, 255, 255])

mask = cv2.inRange(hsv, lower_yellow, upper_yellow)
cv2.imshow('Mask', mask)
cv2.waitKey(0)
```

#### 方法 2：在线工具

使用 [https://colorizer.org/](https://colorizer.org/) 拾取颜色并转换为 HSV。

### 10.2 调整宽度约束

如果怪物名字太长或太短被过滤掉：

```go
config := &Config{
    MinMobNameWidth: 20,   // 降低下限（原 30）
    MaxMobNameWidth: 200,  // 提高上限（原 150）
}
```

**如何确定合适的值**：
1. 截图怪物名字
2. 使用画图工具测量宽度（像素）
3. 设置 `MinMobNameWidth = 测量值 - 10`
4. 设置 `MaxMobNameWidth = 测量值 + 50`

### 10.3 调整搜索区域

如果底部怪物检测不到：

```go
searchROI := ROI{
    X:      0,
    Y:      0,
    Width:  ia.screenInfo.Width,
    Height: ia.screenInfo.Height - 50,  // 减少排除区域（原 100）
}
```

### 10.4 调整最大距离阈值

如果怪物太远打不到：

```go
// FindClosestMob() 中
maxDistance := 200.0  // 减小范围（原 325）
```

如果想搜索更大范围：

```go
maxDistance := 500.0  // 扩大范围
```

### 10.5 调试技巧

#### 技巧 1：保存掩码图像

```go
func (ia *ImageAnalyzer) detectMobsByHSV(...) []Bounds {
    // ... 省略前面的代码

    // 保存掩码用于调试
    gocv.IMWrite("debug_mask_passive.png", mask)
    gocv.IMWrite("debug_morphed_passive.png", morphed)

    // ... 继续执行
}
```

#### 技巧 2：可视化检测结果

```go
// 在检测到的怪物上绘制矩形
func visualizeMobs(img *image.RGBA, mobs []Target) {
    mat, _ := gocv.ImageToMatRGB(img)
    defer mat.Close()

    for _, mob := range mobs {
        // 绘制绿色矩形
        rect := image.Rect(mob.Bounds.X, mob.Bounds.Y,
                          mob.Bounds.X+mob.Bounds.W,
                          mob.Bounds.Y+mob.Bounds.H)
        gocv.Rectangle(&mat, rect, color.RGBA{0, 255, 0, 255}, 2)
    }

    gocv.IMWrite("debug_detected_mobs.png", mat)
}
```

#### 技巧 3：输出详细日志

```go
LogDebug("Passive mobs: found %d candidates, accepted %d",
         len(passiveBounds), acceptedCount)
LogDebug("Mob at (%d,%d) width=%d, minW=%d, maxW=%d",
         bounds.X, bounds.Y, bounds.W, config.MinMobNameWidth, config.MaxMobNameWidth)
```

---

## 总结

### 核心流程回顾

```
游戏截图 → RGB → HSV → 定义 ROI → HSV 掩码 → 形态学 → 轮廓检测 → 过滤 → 怪物列表
```

### 主要功能

| 功能 | 输入 | 输出 | 用途 |
|------|------|------|------|
| `IdentifyMobs` | Config | []Target | 识别所有怪物 |
| `DetectTargetMarker` | HSV图像 | bool | 检测是否选中目标 |
| `DetectTargetDistance` | - | int | 计算到目标距离 |
| `FindClosestMob` | []Target | *Target | 找最近的怪物 |
| `UpdateStats` | - | - | 更新 HP/MP/FP |

### 关键优势

1. **HSV 颜色空间**：对光照不敏感
2. **形态学操作**：去除噪点，连接断裂
3. **轮廓检测**：精确提取形状
4. **智能过滤**：宽度约束 + 位置约束
5. **区域回避**：避免重复送死/卡住
6. **线程安全**：支持并发访问

### 需要调优的部分

1. **HSV 颜色范围**：根据游戏截图调整
2. **宽度约束**：根据怪物名字长度调整
3. **搜索区域**：根据 UI 布局调整
4. **距离阈值**：根据攻击范围调整

### 与 stats.go 的协作

```
analyzer.go          stats.go
     │                  │
     ├─ Capture()       │
     ├─ RGB → HSV       │
     │                  │
     ├─ IdentifyMobs()  │  ← 怪物检测
     │                  │
     └─ UpdateStats() ──┼→ UpdateOpenCV()  ← 状态栏检测
                        │
                        └─ HP/MP/FP 值
```

### 下一步

1. 安装 gocv：`go get -u gocv.io/x/gocv`
2. 截取游戏画面
3. 使用 Python 工具调整 HSV 值
4. 更新 `GetDefaultMobColorConfig()` 中的颜色
5. 测试并微调宽度约束
6. 集成到游戏 bot 主循环中
