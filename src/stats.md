# stats.go 实现逻辑详解

## 目录

1. [整体架构](#1-整体架构)
2. [核心数据结构](#2-核心数据结构)
3. [检测流程详解](#3-检测流程详解)
4. [关键函数实现](#4-关键函数实现)
5. [HSV 颜色空间](#5-hsv-颜色空间)
6. [形态学操作](#6-形态学操作)
7. [线程安全机制](#7-线程安全机制)
8. [使用示例](#8-使用示例)
9. [调优指南](#9-调优指南)

---

## 1. 整体架构

### 1.1 设计目标

`stats.go` 负责检测游戏中的玩家和目标状态栏（HP/MP/FP/目标血量等），使用 **OpenCV HSV 颜色检测** 来实现鲁棒的识别。

### 1.2 架构层次

```
┌─────────────────────────────────────────────────────────┐
│                    ClientStats                           │
│  (管理所有状态：HP/MP/FP/TargetHP/TargetMP/IsAlive)     │
└─────────────────────────────────────────────────────────┘
                         │
                         │ 包含 5 个 StatInfo
                         ▼
┌─────────────────────────────────────────────────────────┐
│                     StatInfo                             │
│  (单个状态栏的检测逻辑：HP、MP、FP、TargetHP、TargetMP)  │
└─────────────────────────────────────────────────────────┘
                         │
                         │ 使用
                         ▼
┌─────────────────────────────────────────────────────────┐
│              OpenCV HSV 检测流程                         │
│  ROI提取 → HSV掩码 → 形态学 → 轮廓检测 → 百分比计算     │
└─────────────────────────────────────────────────────────┘
```

### 1.3 主要组件

| 组件 | 职责 | 位置 |
|------|------|------|
| `StatusBarKind` | 定义状态栏类型（HP/MP/FP等） | Line 38-47 |
| `HSVRange` | 定义 HSV 颜色范围 | Line 77-85 |
| `StatusBarConfig` | 状态栏检测配置（ROI + 颜色） | Line 87-94 |
| `StatInfo` | 单个状态栏的数据和检测逻辑 | Line 194-354 |
| `ClientStats` | 所有状态的聚合管理 | Line 356-491 |

---

## 2. 核心数据结构

### 2.1 StatusBarKind - 状态栏类型

```go
type StatusBarKind int

const (
    StatusBarHP       StatusBarKind = iota  // 玩家 HP
    StatusBarMP                              // 玩家 MP
    StatusBarFP                              // 玩家 FP (Fatigue Point)
    StatusBarTargetHP                        // 目标 HP
    StatusBarTargetMP                        // 目标 MP
)
```

**作用**：枚举类型，标识要检测的状态栏种类。

**使用场景**：
- 获取对应的检测配置
- 日志输出时显示状态栏名称
- 区分不同状态栏的检测区域

---

### 2.2 HSVRange - HSV 颜色范围

```go
type HSVRange struct {
    LowerH uint8 // 色调下界 (0-180)
    LowerS uint8 // 饱和度下界 (0-255)
    LowerV uint8 // 明度下界 (0-255)
    UpperH uint8 // 色调上界 (0-180)
    UpperS uint8 // 饱和度上界 (0-255)
    UpperV uint8 // 明度上界 (0-255)
}
```

**作用**：定义 HSV 颜色空间的匹配范围。

**HSV 三要素**：
- **H (Hue / 色调)**：颜色的种类（红、黄、绿、蓝等），范围 0-180
- **S (Saturation / 饱和度)**：颜色的纯度，范围 0-255
- **V (Value / 明度)**：颜色的亮度，范围 0-255

**示例**：红色 HP 栏
```go
HSVRange{
    LowerH: 0,   // 红色色调开始
    LowerS: 100, // 饱和度至少 100（避免灰色）
    LowerV: 100, // 明度至少 100（避免黑色）
    UpperH: 10,  // 红色色调结束
    UpperS: 255, // 饱和度最大
    UpperV: 255, // 明度最大
}
```

**可视化**：
```
HSV 色彩空间（圆柱体）

         V (明度)
         ▲
         │     饱和度 (S)
         │   ◄─────────►
         │
    255  ├───────●─────────  鲜艳的红色 (H=0, S=255, V=255)
         │      ╱ ╲
         │     ╱   ╲
         │    ╱     ╲
    100  ├───●───────●────  暗红色 (H=0, S=100, V=100)
         │  ╱         ╲
         │ ╱           ╲
      0  ├●─────────────●  黑色
         └─────────────────► H (色调)
           0   60  120 180
           红  黄  绿  蓝
```

---

### 2.3 StatusBarConfig - 状态栏配置

```go
type StatusBarConfig struct {
    MinX     int      // ROI 左上角 X 坐标
    MinY     int      // ROI 左上角 Y 坐标
    MaxX     int      // ROI 右下角 X 坐标
    MaxY     int      // ROI 右下角 Y 坐标
    HSVRange HSVRange // 颜色匹配范围
}
```

**作用**：定义每个状态栏的检测区域和颜色。

**ROI（Region of Interest）概念**：
```
完整游戏画面 (800x600)
┌─────────────────────────────────────┐
│                                     │
│  ┌─────────┐← HP/MP/FP 检测区域    │
│  │ HP: 红  │  (105,30) - (225,110) │
│  │ MP: 蓝  │                        │
│  │ FP: 绿  │                        │
│  └─────────┘                        │
│                                     │
│           ┌──────────┐              │
│           │ 目标 HP  │← 目标检测区域│
│           └──────────┘ (300,30)-(550,60)│
│                                     │
│                                     │
└─────────────────────────────────────┘
```

**示例配置**：
```go
// 玩家 HP 栏配置
StatusBarConfig{
    MinX: 105,  // 左边界
    MinY: 30,   // 上边界
    MaxX: 225,  // 右边界 (宽度 = 225-105 = 120 像素)
    MaxY: 110,  // 下边界 (高度 = 110-30 = 80 像素)
    HSVRange: HSVRange{...}, // 红色范围
}
```

---

### 2.4 StatInfo - 单个状态栏数据

```go
type StatInfo struct {
    MaxW           int           // 检测到的最大宽度（满血时的宽度）
    Value          int           // 当前百分比值 (0-100)
    StatKind       StatusBarKind // 状态栏类型
    LastValue      int           // 上一次的值（用于变化检测）
    LastUpdateTime time.Time     // 最后更新时间
    mu             sync.RWMutex  // 读写锁（线程安全）
}
```

**关键字段说明**：

1. **MaxW (最大宽度)**：
   - 自适应学习的概念
   - 记录检测到的最大血条宽度
   - 用于计算百分比：`percentage = (当前宽度 / MaxW) * 100`
   - 随着检测不断更新，适应不同分辨率

2. **Value (当前值)**：
   - 当前状态栏的百分比 (0-100)
   - 通过轮廓宽度计算得出

3. **StatKind (类型)**：
   - 标识这是哪个状态栏（HP/MP/FP等）
   - 用于获取对应的检测配置

**示例数据**：
```go
// HP 栏检测到 80% 血量
StatInfo{
    MaxW: 120,           // 满血时宽度 120 像素
    Value: 80,           // 当前 80%
    StatKind: StatusBarHP,
    LastValue: 85,       // 上次是 85%（血量下降了）
    LastUpdateTime: ..., // 更新时间
}
```

---

### 2.5 ClientStats - 客户端状态管理

```go
type ClientStats struct {
    // 玩家状态栏
    HP *StatInfo
    MP *StatInfo
    FP *StatInfo

    // 目标状态栏
    TargetHP *StatInfo
    TargetMP *StatInfo

    // 状态信息
    HasTrayOpen  bool       // 状态栏是否打开
    IsAlive      AliveState // 玩家存活状态

    // 目标信息
    TargetIsNPC    bool // 目标是 NPC（HP=100%, MP=0%）
    TargetIsMover  bool // 目标是怪物（MP > 0%）
    TargetIsAlive  bool // 目标存活（HP > 0%）
    TargetOnScreen bool // 目标在屏幕上

    // 其他
    StatTryNotDetectedCount int // 连续检测失败次数

    mu sync.RWMutex // 线程安全锁
}
```

**作用**：聚合管理所有状态信息，提供统一的接口。

**状态推断逻辑**：
- `TargetIsNPC = (TargetHP == 100% && TargetMP == 0%)`
  - NPC 通常显示满血但无蓝条
- `TargetIsMover = (TargetMP > 0%)`
  - 怪物有蓝条
- `IsAlive = (HP > 0 && HasTrayOpen)`
  - 玩家存活需要状态栏打开且 HP > 0

---

## 3. 检测流程详解

### 3.1 整体流程图

```
┌─────────────────┐
│  输入：HSV 图像 │
│  (gocv.Mat)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 1. 提取 ROI     │  根据 StatusBarConfig 定义的区域
│    Region()     │  例如：(105,30)-(225,110)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. 创建 HSV 掩码│  使用 HSV 范围创建二值掩码
│    inRange()    │  匹配的像素 = 白色(255)，不匹配 = 黑色(0)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 3. 形态学操作   │  开运算：去除小噪点
│    开运算+闭运算│  闭运算：填充小孔洞
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 4. 查找轮廓     │  FindContours() 找到所有连续区域
│    FindContours │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 5. 找最大轮廓   │  遍历所有轮廓，找宽度最大的
│    Max Width    │  这是主血条的宽度
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 6. 计算百分比   │  percentage = (width / roiWidth) * 100
│    Percentage   │  或 percentage = (width / MaxW) * 100
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  输出：0-100%   │
└─────────────────┘
```

### 3.2 步骤详解

#### 步骤 1: 提取 ROI (Region of Interest)

**代码位置**：`UpdateValueOpenCV()` Line 234

```go
roiMat := hsvMat.Region(image.Rect(config.MinX, config.MinY, config.MaxX, config.MaxY))
defer roiMat.Close()
```

**作用**：
- 从完整的 HSV 图像中裁剪出状态栏区域
- 减少后续处理的计算量
- 避免其他区域的干扰

**可视化**：
```
原图 (800x600)                ROI 提取后 (120x80)
┌──────────────────┐         ┌─────────────┐
│                  │         │█████████░░░░│  HP 栏区域
│ ┌────────────┐   │  提取   │█████████░░░░│
│ │████████░░░░│   │ ─────>  │░░░░░░░░░░░░│
│ │████████░░░░│   │         │░░░░░░░░░░░░│
│ └────────────┘   │         └─────────────┘
│                  │         只包含状态栏的小图
└──────────────────┘
```

---

#### 步骤 2: 创建 HSV 颜色掩码

**代码位置**：`createHSVMask()` Line 304-313

```go
func (si *StatInfo) createHSVMask(hsvMat *gocv.Mat, colorRange HSVRange) gocv.Mat {
    // 定义 HSV 范围的上下界
    lower := gocv.NewScalar(float64(colorRange.LowerH), float64(colorRange.LowerS), float64(colorRange.LowerV), 0)
    upper := gocv.NewScalar(float64(colorRange.UpperH), float64(colorRange.UpperS), float64(colorRange.UpperV), 0)

    // 创建二值掩码：在范围内的像素 = 255(白)，范围外 = 0(黑)
    mask := gocv.NewMat()
    gocv.InRangeWithScalar(*hsvMat, lower, upper, &mask)

    return mask
}
```

**inRange 操作原理**：
```
对于每个像素 (h, s, v):
    if (LowerH <= h <= UpperH) AND
       (LowerS <= s <= UpperS) AND
       (LowerV <= v <= UpperV):
        mask[pixel] = 255  (白色)
    else:
        mask[pixel] = 0    (黑色)
```

**示例**：检测红色 HP 栏

```
原始 ROI (RGB)              转换为 HSV               创建掩码 (红色范围)
┌─────────────────┐       ┌─────────────────┐     ┌─────────────────┐
│ 红红红红░░灰灰灰 │       │ H:0 H:0 ... H:? │     │ 255 255 ... 0 0 │
│ 红红红红░░灰灰灰 │ RGB→  │ S:200 S:200 ... │ →   │ 255 255 ... 0 0 │
│ 背背背背背背背背 │  HSV  │ V:180 V:180 ... │掩码 │ 0 0 0 0 0 0 0 0 │
└─────────────────┘       └─────────────────┘     └─────────────────┘
                                                    白色 = 红色血条
                                                    黑色 = 其他颜色
```

---

#### 步骤 3: 形态学操作

**代码位置**：`applyMorphology()` Line 317-339

```go
func (si *StatInfo) applyMorphology(mask *gocv.Mat) gocv.Mat {
    kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
    defer kernel.Close()

    // 开运算 = 腐蚀 + 膨胀 → 去除小噪点
    temp := gocv.NewMat()
    gocv.Erode(*mask, &temp, kernel)     // 腐蚀
    opened := gocv.NewMat()
    gocv.Dilate(temp, &opened, kernel)   // 膨胀
    temp.Close()

    // 闭运算 = 膨胀 + 腐蚀 → 填充小孔
    temp2 := gocv.NewMat()
    gocv.Dilate(opened, &temp2, kernel)  // 膨胀
    result := gocv.NewMat()
    gocv.Erode(temp2, &result, kernel)   // 腐蚀
    temp2.Close()
    opened.Close()

    return result
}
```

**开运算（Opening）= 腐蚀 + 膨胀**

作用：**去除小的白色噪点**

```
原始掩码                腐蚀 (Erode)         膨胀 (Dilate)
┌──────────────┐       ┌──────────────┐     ┌──────────────┐
│ ██████ · ·   │       │ █████        │     │ ██████       │
│ ██████ ·     │  →    │ █████        │  →  │ ██████       │
│ ██████       │       │ █████        │     │ ██████       │
└──────────────┘       └──────────────┘     └──────────────┘
小噪点 (·) 被腐蚀掉了                        血条主体保留
```

**闭运算（Closing）= 膨胀 + 腐蚀**

作用：**填充血条内的小孔洞**

```
开运算后            膨胀 (Dilate)         腐蚀 (Erode)
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ ████ ████    │     │ █████████    │     │ ████████     │
│ ████·████    │  →  │ █████████    │  →  │ ████████     │
│ ████ ████    │     │ █████████    │     │ ████████     │
└──────────────┘     └──────────────┘     └──────────────┘
小孔洞 (·) 被填充                         平滑的血条
```

**为什么需要形态学操作**？

1. **去噪**：游戏截图可能有压缩噪点、抗锯齿边缘
2. **连接断裂**：血条可能因为光照不均导致中间有小缝隙
3. **平滑边界**：让轮廓更加清晰，便于宽度计算

---

#### 步骤 4: 查找轮廓

**代码位置**：`UpdateValueOpenCV()` Line 246

```go
contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
defer contours.Close()
```

**轮廓检测原理**：

在二值图像中找到所有白色（255）区域的边界。

```
形态学处理后的掩码           查找轮廓
┌──────────────────┐        ┌──────────────────┐
│                  │        │                  │
│  ████████        │   →    │  ┌──────┐        │
│  ████████        │        │  │轮廓1 │        │
│  ████████        │        │  └──────┘        │
│                  │        │                  │
│     ███          │        │     ┌┐           │
│     ███          │        │     └┘轮廓2      │
└──────────────────┘        └──────────────────┘
                            轮廓1 = 主血条（大）
                            轮廓2 = 噪点（小）
```

**参数说明**：
- `gocv.RetrievalExternal`：只检测外部轮廓（不检测内部孔洞）
- `gocv.ChainApproxSimple`：压缩轮廓，只保留关键点

---

#### 步骤 5: 找最大轮廓宽度

**代码位置**：`UpdateValueOpenCV()` Line 250-257

```go
maxWidth := 0
for i := 0; i < contours.Size(); i++ {
    contour := contours.At(i)
    rect := gocv.BoundingRect(contour)  // 获取轮廓的边界矩形
    if rect.Dx() > maxWidth {
        maxWidth = rect.Dx()  // 记录最大宽度
    }
}
```

**为什么取最大宽度**？

因为主血条是最大的轮廓，小的轮廓是噪点。

```
所有轮廓的边界矩形：

轮廓1（主血条）         轮廓2（噪点）
┌──────────────┐        ┌──┐
│ 宽度 = 100px │        │5px│
└──────────────┘        └──┘

maxWidth = max(100, 5) = 100  ← 选择主血条
```

---

#### 步骤 6: 计算百分比

**代码位置**：`UpdateValueOpenCV()` Line 272-284

```go
var newValue int
if roiWidth > 0 {
    valueFrac := float64(maxWidth) / float64(roiWidth)
    newValue = int(valueFrac * 100)
    if newValue < 0 {
        newValue = 0
    }
    if newValue > 100 {
        newValue = 100
    }
}
```

**计算公式**：

```
百分比 = (检测到的血条宽度 / ROI总宽度) × 100

示例：
- ROI 宽度 = 120 像素
- 检测到的血条宽度 = 96 像素
- 百分比 = (96 / 120) × 100 = 80%
```

**可视化**：

```
ROI 区域 (120px 宽)
┌─────────────────────────────┐
│ ████████████████████░░░░░░░ │  血条填充了 96px
│ ████████████████████░░░░░░░ │
│ ████████████████████░░░░░░░ │  96/120 = 0.8 = 80%
└─────────────────────────────┘
 ├──────96px──────┤├──24px──┤
  血条宽度          空白部分
```

---

## 4. 关键函数实现

### 4.1 UpdateValueOpenCV() - 核心检测函数

**函数签名**：
```go
func (si *StatInfo) UpdateValueOpenCV(hsvMat *gocv.Mat) bool
```

**输入**：
- `hsvMat`：完整游戏画面的 HSV 图像（gocv.Mat 格式）

**输出**：
- `bool`：状态值是否发生变化

**执行流程**：

```go
func (si *StatInfo) UpdateValueOpenCV(hsvMat *gocv.Mat) bool {
    // 1. 获取配置（ROI 区域 + HSV 颜色范围）
    config := GetStatusBarConfig(si.StatKind)

    // 2. 提取 ROI
    roiMat := hsvMat.Region(image.Rect(config.MinX, config.MinY, config.MaxX, config.MaxY))
    defer roiMat.Close()

    // 3. 创建 HSV 颜色掩码（匹配血条颜色）
    mask := si.createHSVMask(&roiMat, config.HSVRange)
    defer mask.Close()

    // 4. 形态学操作（去噪 + 填洞）
    morphed := si.applyMorphology(&mask)
    defer morphed.Close()

    // 5. 查找轮廓
    contours := gocv.FindContours(morphed, gocv.RetrievalExternal, gocv.ChainApproxSimple)
    defer contours.Close()

    // 6. 找最大轮廓宽度
    maxWidth := 0
    for i := 0; i < contours.Size(); i++ {
        contour := contours.At(i)
        rect := gocv.BoundingRect(contour)
        if rect.Dx() > maxWidth {
            maxWidth = rect.Dx()
        }
    }

    // 7. 计算百分比
    si.mu.Lock()
    defer si.mu.Unlock()

    roiWidth := config.MaxX - config.MinX
    valueFrac := float64(maxWidth) / float64(roiWidth)
    newValue := int(valueFrac * 100)

    // 8. 更新值并返回是否变化
    changed := (newValue != si.Value)
    si.Value = newValue
    return changed
}
```

**线程安全**：
- 使用 `sync.RWMutex` 保护 `Value` 字段
- 在修改前加锁 `si.mu.Lock()`
- 使用 `defer` 确保解锁

---

### 4.2 UpdateOpenCV() - 更新所有状态

**函数签名**：
```go
func (cs *ClientStats) UpdateOpenCV(hsvMat *gocv.Mat)
```

**作用**：一次性更新所有状态栏（HP/MP/FP/目标 HP/目标 MP）。

**执行流程**：

```go
func (cs *ClientStats) UpdateOpenCV(hsvMat *gocv.Mat) {
    cs.mu.Lock()
    defer cs.mu.Unlock()

    // 1. 更新所有状态栏
    cs.HP.UpdateValueOpenCV(hsvMat)
    cs.MP.UpdateValueOpenCV(hsvMat)
    cs.FP.UpdateValueOpenCV(hsvMat)
    cs.TargetHP.UpdateValueOpenCV(hsvMat)
    cs.TargetMP.UpdateValueOpenCV(hsvMat)

    // 2. 检测状态栏是否打开
    cs.HasTrayOpen = cs.detectStatTray()

    // 3. 更新玩家存活状态
    cs.IsAlive = cs.calculateAliveState()

    // 4. 推断目标类型
    hpVal := cs.TargetHP.GetValue()
    mpVal := cs.TargetMP.GetValue()
    cs.TargetIsNPC = hpVal == 100 && mpVal == 0    // NPC：满血无蓝
    cs.TargetIsMover = mpVal > 0                    // 怪物：有蓝条
    cs.TargetIsAlive = hpVal > 0                    // 存活：血量 > 0
}
```

**调用示例**：
```go
// 在主循环中：
analyzer.Capture()         // 截图
img := analyzer.GetImage()  // 获取图像
mat := imageToMat(img)      // 转换为 Mat
hsvMat := gocv.NewMat()
gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)  // 转换为 HSV

// 更新所有状态
stats.UpdateOpenCV(&hsvMat)

// 读取状态
hpPercent := stats.GetHPPercent()
isAlive := stats.IsAlive == AliveStateAlive
```

---

### 4.3 detectStatTray() - 检测状态栏是否打开

**代码位置**：Line 435-455

```go
func (cs *ClientStats) detectStatTray() bool {
    hpVal := cs.HP.GetValue()
    mpVal := cs.MP.GetValue()
    fpVal := cs.FP.GetValue()

    // 如果 HP/MP/FP 全部为 0，说明状态栏关闭了
    if hpVal == 0 && mpVal == 0 && fpVal == 0 {
        cs.StatTryNotDetectedCount++

        // 连续 5 次检测失败后，应该发送 T 键打开状态栏
        if cs.StatTryNotDetectedCount >= 5 {
            cs.StatTryNotDetectedCount = 0
            // 这里可以触发按键动作
        }
        return false
    }

    cs.StatTryNotDetectedCount = 0
    return true
}
```

**逻辑**：

游戏中玩家可以按 T 键关闭/打开状态栏，关闭时无法检测到血条。

```
状态栏打开                   状态栏关闭
┌─────────────┐            ┌─────────────┐
│ HP: ████░░  │            │             │
│ MP: ██████  │            │  (无状态栏) │
│ FP: ███████ │            │             │
└─────────────┘            └─────────────┘
HP/MP/FP 有值              HP/MP/FP 全为 0
HasTrayOpen = true         HasTrayOpen = false
```

---

### 4.4 calculateAliveState() - 判断玩家存活

**代码位置**：Line 458-469

```go
func (cs *ClientStats) calculateAliveState() AliveState {
    if !cs.HasTrayOpen {
        return AliveStateStatsTrayClosed  // 状态栏关闭，无法判断
    }

    hpVal := cs.HP.GetValue()
    if hpVal > 0 {
        return AliveStateAlive  // 有血 = 存活
    }

    return AliveStateDead  // 无血 = 死亡
}
```

**状态转换图**：

```
                 ┌────────────────┐
                 │ StatsTrayClosed│
                 │  (状态栏关闭)   │
                 └───────┬────────┘
                         │ 打开状态栏
                         ▼
    ┌──────────────────────────────────┐
    │                                  │
    │         Alive (存活)              │
    │         HP > 0                   │
    │                                  │
    └──────────┬───────────────────────┘
               │ HP 降至 0
               ▼
    ┌──────────────────────────────────┐
    │         Dead (死亡)               │
    │         HP = 0                   │
    └──────────────────────────────────┘
               │ 重生后
               ▼
           (回到 Alive)
```

---

## 5. HSV 颜色空间

### 5.1 为什么使用 HSV 而不是 RGB？

**RGB 的问题**：

RGB 颜色空间将颜色、亮度混合在一起，对光照敏感。

```
同样的红色，在不同光照下 RGB 值差异很大：

明亮红色：RGB(255, 0, 0)
暗红色：  RGB(128, 0, 0)
亮红色：  RGB(255, 50, 50)

需要定义多个 RGB 值才能匹配，且容易漏掉中间值。
```

**HSV 的优势**：

HSV 将颜色（H）和亮度（V）分离，更容易定义颜色范围。

```
所有红色都在 H=0-10 或 H=170-180 范围内，
无论亮度如何，只要调整 V 的范围即可。

明亮红色：HSV(0, 255, 255)
暗红色：  HSV(0, 255, 128)
亮红色：  HSV(0, 200, 255)
         ↑ 相同的 H 值！
```

### 5.2 HSV 颜色表

| 颜色 | H (色调) | 常见范围 | 用途 |
|------|---------|---------|------|
| 红色 | 0-10, 170-180 | H=0-10, S=100-255, V=100-255 | HP 栏、攻击性怪物 |
| 黄色 | 20-35 | H=20-35, S=100-255, V=150-255 | 被动怪物名字 |
| 绿色 | 40-80 | H=40-80, S=100-255, V=100-255 | FP 栏 |
| 蓝色 | 100-130 | H=100-130, S=100-255, V=150-255 | MP 栏、蓝色标记 |
| 紫色 | 130-160 | H=130-160, S=100-255, V=100-255 | 紫色怪物 |

**注意**：OpenCV 中 H 的范围是 0-180（而不是 0-360）

---

## 6. 形态学操作

### 6.1 核心概念

**结构元素（Kernel）**：

```go
kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))
```

这是一个 3x3 的矩形核：
```
┌───┬───┬───┐
│ 1 │ 1 │ 1 │
├───┼───┼───┤
│ 1 │ 1 │ 1 │
├───┼───┼───┤
│ 1 │ 1 │ 1 │
└───┴───┴───┘
```

### 6.2 腐蚀（Erosion）

**作用**：收缩白色区域，去除边缘像素。

```
原图:          腐蚀后:
█████████      ███████
███████        █████
█████          ███
```

**原理**：
- 核在图像上滑动
- 如果核覆盖的区域**全部为白色**，中心像素保留为白色
- 否则，中心像素变为黑色

**效果**：小的白色噪点会被完全腐蚀掉。

---

### 6.3 膨胀（Dilation）

**作用**：扩展白色区域，填充边缘。

```
原图:          膨胀后:
███████        █████████
█████          ███████
███            █████
```

**原理**：
- 核在图像上滑动
- 如果核覆盖的区域**有任何白色像素**，中心像素变为白色

**效果**：小的黑色孔洞会被填充。

---

### 6.4 开运算（Opening）= 腐蚀 + 膨胀

**效果**：去除小噪点，保留主体。

```
原始掩码:       腐蚀:          膨胀(开运算结果):
██████ ··      █████          ██████
██████ ·       █████          ██████
██████         █████          ██████

小噪点(··)     被腐蚀掉       主体恢复原样
```

---

### 6.5 闭运算（Closing）= 膨胀 + 腐蚀

**效果**：填充小孔洞，连接断裂。

```
原始掩码:       膨胀:          腐蚀(闭运算结果):
████ ████      █████████      ████████
████ ████      █████████      ████████

中间断裂       被连接         平滑的血条
```

---

## 7. 线程安全机制

### 7.1 为什么需要线程安全？

在游戏 bot 中，通常有多个线程：
- **检测线程**：不断截图并更新 `ClientStats`
- **决策线程**：读取 `ClientStats` 来决定行动
- **UI 线程**：显示当前状态

如果不加锁，可能出现**竞态条件**（Race Condition）：

```
时间线：
T1: 检测线程读取 HP.Value = 80
T2: 决策线程读取 HP.Value = 80  (正确)
T3: 检测线程写入 HP.Value = 60
T4: 决策线程再次读取，可能读到不一致的值
```

### 7.2 读写锁（sync.RWMutex）

**StatInfo 使用读写锁**：

```go
type StatInfo struct {
    Value int
    mu    sync.RWMutex  // 读写锁
}

// 读取（多个线程可以同时读）
func (si *StatInfo) GetValue() int {
    si.mu.RLock()         // 读锁
    defer si.mu.RUnlock()
    return si.Value
}

// 写入（独占，只能一个线程写）
func (si *StatInfo) UpdateValueOpenCV(...) bool {
    si.mu.Lock()          // 写锁
    defer si.mu.Unlock()
    si.Value = newValue
}
```

**读写锁的优势**：
- **多个读取可以并发**：决策线程和 UI 线程可以同时读取
- **写入时独占**：检测线程写入时，其他线程等待

### 7.3 ClientStats 的锁

**ClientStats 也使用读写锁保护聚合状态**：

```go
type ClientStats struct {
    HP       *StatInfo
    IsAlive  AliveState
    mu       sync.RWMutex
}

func (cs *ClientStats) UpdateOpenCV(...) {
    cs.mu.Lock()           // 更新时加写锁
    defer cs.mu.Unlock()

    // 更新所有字段
    cs.HP.UpdateValueOpenCV(...)
    cs.IsAlive = cs.calculateAliveState()
}
```

---

## 8. 使用示例

### 8.1 完整使用流程

```go
package main

import (
    "gocv.io/x/gocv"
)

func main() {
    // 1. 创建 ClientStats
    stats := NewClientStats()

    // 2. 游戏循环
    for {
        // 3. 截取游戏画面
        img := captureGameScreen()  // image.RGBA

        // 4. 转换为 gocv.Mat
        mat, _ := gocv.ImageToMatRGB(img)
        defer mat.Close()

        // 5. 转换为 HSV 色彩空间
        hsvMat := gocv.NewMat()
        gocv.CvtColor(mat, &hsvMat, gocv.ColorBGRToHSV)
        defer hsvMat.Close()

        // 6. 更新所有状态
        stats.UpdateOpenCV(&hsvMat)

        // 7. 读取状态并做决策
        hpPercent := stats.GetHPPercent()
        mpPercent := stats.GetMPPercent()
        isAlive := stats.IsAlive

        if hpPercent < 30 {
            // 血量低，喝血瓶
            drinkHPPotion()
        }

        if mpPercent < 20 {
            // 蓝量低，喝蓝瓶
            drinkMPPotion()
        }

        if isAlive == AliveStateDead {
            // 死亡，复活
            respawn()
        }

        // 8. 检查目标状态
        if stats.TargetIsAlive {
            targetHP := stats.TargetHP.GetValue()
            if targetHP < 50 {
                // 目标半血，放大招
                useUltimateSkill()
            }
        }

        time.Sleep(100 * time.Millisecond)
    }
}
```

### 8.2 单独检测某个状态栏

```go
// 只检测 HP
hpStat := NewStatInfo(0, 100, StatusBarHP)

// 在循环中
for {
    img := captureScreen()
    mat := imageToMat(img)
    hsvMat := rgbToHSV(mat)

    // 只更新 HP
    changed := hpStat.UpdateValueOpenCV(&hsvMat)

    if changed {
        fmt.Printf("HP changed to: %d%%\n", hpStat.GetValue())
    }
}
```

---

## 9. 调优指南

### 9.1 如何获取正确的 HSV 值

#### 方法 1：使用 Python + OpenCV 工具

创建 `hsv_picker.py`：

```python
import cv2
import numpy as np

def nothing(x):
    pass

# 读取游戏截图
img = cv2.imread('screenshot.png')
hsv = cv2.cvtColor(img, cv2.COLOR_BGR2HSV)

# 创建窗口和滑动条
cv2.namedWindow('HSV Picker')
cv2.createTrackbar('LowerH', 'HSV Picker', 0, 180, nothing)
cv2.createTrackbar('LowerS', 'HSV Picker', 100, 255, nothing)
cv2.createTrackbar('LowerV', 'HSV Picker', 100, 255, nothing)
cv2.createTrackbar('UpperH', 'HSV Picker', 10, 180, nothing)
cv2.createTrackbar('UpperS', 'HSV Picker', 255, 255, nothing)
cv2.createTrackbar('UpperV', 'HSV Picker', 255, 255, nothing)

while True:
    # 获取滑动条的值
    lh = cv2.getTrackbarPos('LowerH', 'HSV Picker')
    ls = cv2.getTrackbarPos('LowerS', 'HSV Picker')
    lv = cv2.getTrackbarPos('LowerV', 'HSV Picker')
    uh = cv2.getTrackbarPos('UpperH', 'HSV Picker')
    us = cv2.getTrackbarPos('UpperS', 'HSV Picker')
    uv = cv2.getTrackbarPos('UpperV', 'HSV Picker')

    # 创建掩码
    lower = np.array([lh, ls, lv])
    upper = np.array([uh, us, uv])
    mask = cv2.inRange(hsv, lower, upper)

    # 显示结果
    result = cv2.bitwise_and(img, img, mask=mask)
    cv2.imshow('Original', img)
    cv2.imshow('Mask', mask)
    cv2.imshow('Result', result)

    # 按 Q 退出
    if cv2.waitKey(1) & 0xFF == ord('q'):
        print(f"HSVRange{{LowerH: {lh}, LowerS: {ls}, LowerV: {lv}, UpperH: {uh}, UpperS: {us}, UpperV: {uv}}}")
        break

cv2.destroyAllWindows()
```

**使用步骤**：
1. 截取游戏画面保存为 `screenshot.png`
2. 运行 `python hsv_picker.py`
3. 调整滑动条，直到掩码只显示血条
4. 按 Q 键退出，终端会打印 HSV 范围

#### 方法 2：在线 HSV 拾色器

访问 [https://colorizer.org/](https://colorizer.org/) 等网站，上传截图并选择颜色。

---

### 9.2 调整 ROI 区域

如果检测不准确，可能是 ROI 区域不对：

1. **截图并测量**：
   - 截取游戏画面
   - 使用画图工具测量血条的位置
   - 记录左上角 (MinX, MinY) 和右下角 (MaxX, MaxY)

2. **更新配置**：
```go
case StatusBarHP:
    return StatusBarConfig{
        MinX: 105,  // 调整这些值
        MinY: 30,
        MaxX: 225,
        MaxY: 110,
        HSVRange: ...,
    }
```

---

### 9.3 调整形态学内核大小

如果血条检测有很多噪点或断裂：

```go
// 原来的 3x3 内核
kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3))

// 更大的内核（5x5）可以去除更多噪点，但可能丢失细节
kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(5, 5))

// 更小的内核（2x2）保留更多细节，但噪点可能更多
kernel := gocv.GetStructuringElement(gocv.MorphRect, image.Pt(2, 2))
```

**建议**：
- 从 3x3 开始
- 如果噪点多，增大到 5x5
- 如果血条边缘丢失，减小到 2x2

---

### 9.4 调试技巧

#### 技巧 1：保存中间结果

```go
func (si *StatInfo) UpdateValueOpenCV(hsvMat *gocv.Mat) bool {
    // ... 省略前面的代码

    // 保存掩码图像用于调试
    gocv.IMWrite("debug_mask.png", mask)

    // 保存形态学处理后的图像
    gocv.IMWrite("debug_morphed.png", morphed)

    // ... 继续执行
}
```

查看 `debug_mask.png` 和 `debug_morphed.png` 来诊断问题。

#### 技巧 2：输出调试日志

```go
LogDebug("HP Detection: maxWidth=%d, roiWidth=%d, percentage=%d%%",
         maxWidth, roiWidth, newValue)
```

#### 技巧 3：可视化轮廓

```go
// 在彩色图像上绘制检测到的轮廓
gocv.DrawContours(&mat, contours, -1, color.RGBA{0, 255, 0, 255}, 2)
gocv.IMWrite("debug_contours.png", mat)
```

---

## 总结

### 核心流程回顾

```
游戏截图 → RGB → HSV → 提取 ROI → 颜色掩码 → 形态学 → 轮廓检测 → 计算百分比
```

### 关键优势

1. **HSV 颜色空间**：对光照不敏感，易于调整
2. **形态学操作**：去除噪点，连接断裂
3. **轮廓检测**：精确提取血条形状
4. **线程安全**：支持多线程并发读写
5. **自适应**：自动学习最大宽度

### 需要调优的部分

1. **HSV 颜色范围**：根据实际游戏截图调整
2. **ROI 区域**：根据游戏分辨率调整
3. **形态学内核**：根据噪点情况调整

### 下一步

1. 安装 gocv：`go get -u gocv.io/x/gocv`
2. 截取游戏画面
3. 使用 Python 工具找到正确的 HSV 值
4. 更新代码中的占位符
5. 测试并微调
