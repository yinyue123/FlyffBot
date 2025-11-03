# 检测约束更新总结

## 更新时间
2025-11-03

## 更新内容

### 1. HSV 颜色范围更新

#### 状态栏 (stats.go)
- **HP**: H=170-175, S=120-200, V=150-230
- **MP**: H=99-117, S=114-200, V=190-240
- **FP**: H=52-60, S=150-173, V=150-230
- **Target HP**: H=170-175, S=120-200, V=150-230
- **Target MP**: H=99-117, S=114-200, V=190-240

#### 怪物 (analyzer.go)
- **被动怪**: H=29-31, S=50-90, V=180-255
- **主动怪**: H=0-5, S=200-255, V=200-255

### 2. 形态学参数更新

| 用途 | closesize | closeiter |
|------|-----------|-----------|
| 状态栏 | 25x25 | 3 |
| 怪物 | 10x10 | 5 |

### 3. 轮廓尺寸约束（新增）

#### 玩家状态栏 (HP/MP/FP)
```
宽度范围: 1-300 像素
高度范围: 12-30 像素
数量: HP、MP、FP 各只有一个（取最大轮廓）
```

#### 目标状态栏 (Target HP/MP)
```
宽度范围: 1-600 像素
高度范围: 12-30 像素
数量: HP、MP 各只有一个（取最大轮廓）
```

#### 怪物
```
没有数量限制，可以检测多个
宽度范围: 由 config.MinMobNameWidth 和 config.MaxMobNameWidth 控制
```

## 代码实现

### stats.go 轮廓过滤
```go
// 根据状态栏类型确定尺寸约束
if si.StatKind == StatusBarTargetHP || si.StatKind == StatusBarTargetMP {
    // Target: 宽度 1-600, 高度 12-30
    minWidthConstraint = 1
    maxWidthConstraint = 600
    minHeightConstraint = 12
    maxHeightConstraint = 30
} else {
    // Player: 宽度 1-300, 高度 12-30
    minWidthConstraint = 1
    maxWidthConstraint = 300
    minHeightConstraint = 12
    maxHeightConstraint = 30
}

// 只选择符合尺寸约束的最大轮廓
for i := 0; i < contours.Size(); i++ {
    rect := gocv.BoundingRect(contour)
    width := rect.Dx()
    height := rect.Dy()
    
    if width >= minWidthConstraint && width <= maxWidthConstraint &&
       height >= minHeightConstraint && height <= maxHeightConstraint {
        if width > maxWidth {
            maxWidth = width
        }
    }
}
```

## 为什么需要这些约束？

### 1. 宽度约束
- **下限 (1像素)**: 排除空轮廓
- **上限 (300/600像素)**: 排除误检的大区域（如背景）

### 2. 高度约束
- **下限 (12像素)**: 状态栏至少要有这个高度
- **上限 (30像素)**: 排除过高的误检区域

### 3. 选择最大轮廓
- 状态栏只有一个，选择最大的确保检测到主血条
- 避免检测到小的噪点或干扰

## 测试建议

1. **满血测试**: HP=100% 时检测是否准确
2. **半血测试**: HP=50% 时检测是否准确
3. **低血测试**: HP=10% 时检测是否准确
4. **多目标测试**: 有多个怪物时检测是否准确
5. **边界测试**: 血条非常短或非常长时的检测

## 调试方法

保存轮廓信息进行调试：
```go
LogDebug("Found %d contours for %s", contours.Size(), si.StatKind.String())
for i := 0; i < contours.Size(); i++ {
    rect := gocv.BoundingRect(contour)
    LogDebug("  Contour %d: width=%d, height=%d", i, rect.Dx(), rect.Dy())
}
```

## 相关文件

- `src/stats.go` - 状态栏检测（已更新）
- `src/analyzer.go` - 怪物检测（已更新）
