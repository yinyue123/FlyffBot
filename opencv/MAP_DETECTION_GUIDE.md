# 地图怪物检测方案 - Map Monster Detection Guide

## 📋 项目概述

检测游戏地图中的怪物位置（黄色圆点），用于游戏角色自动导航和打怪。

## 🎯 检测目标

- **输入图片**: `map.jpeg` - 游戏地图截图
- **目标物体**: 黄色小圆点（怪物标记）
- **输出结果**: 怪物坐标列表 `[(x1, y1), (x2, y2), ...]`

## 🔧 检测方法对比

### Method 1: HSV 颜色检测 ⭐ 推荐

**优点:**
- ✓ 最适合检测特定颜色的对象（黄色圆点）
- ✓ 速度快，实时性好
- ✓ 对光照变化有一定容忍度
- ✓ 参数调整直观

**原理:**
1. 将图片从 BGR 转换到 HSV 色彩空间
2. 设置黄色的 HSV 范围进行颜色过滤
3. 形态学操作去除噪声
4. 轮廓检测找到所有圆点
5. 根据面积、圆形度过滤

**关键参数:**
- `H_min, H_max`: 色调范围（黄色约 20-35）
- `S_min, S_max`: 饱和度范围（高饱和度 100-255）
- `V_min, V_max`: 明度范围（亮度 100-255）
- `Min_Area, Max_Area`: 圆点面积范围
- `Circularity`: 圆形度阈值（0.7-1.0）

**适用场景:**
- 怪物颜色固定（黄色）
- 背景颜色差异明显
- 需要快速实时检测

---

### Method 2: 模板匹配

**优点:**
- ✓ 可以检测特定形状的怪物图标
- ✓ 对大小相似的目标效果好

**缺点:**
- ✗ 对缩放敏感（需要多尺度匹配）
- ✗ 速度较慢
- ✗ 对旋转敏感

**原理:**
1. 使用 `point.png` 作为模板
2. 多尺度滑动窗口匹配
3. 非极大值抑制去除重叠检测

**关键参数:**
- `Threshold`: 匹配相似度阈值（0.6-0.9）
- `Scale_Start, Scale_End, Scale_Step`: 多尺度参数
- `NMS_Threshold`: 非极大值抑制阈值

**适用场景:**
- 怪物图标形状固定
- 大小变化不大
- 背景复杂

---

### Method 3: 霍夫圆检测

**优点:**
- ✓ 专门检测圆形物体
- ✓ 对部分遮挡有容忍度

**缺点:**
- ✗ 参数调整复杂
- ✗ 可能检测到背景中的圆形

**原理:**
1. 边缘检测（Canny）
2. 霍夫变换检测圆形
3. 根据半径过滤

**关键参数:**
- `DP`: 累加器分辨率
- `Min_Dist`: 圆心最小距离
- `Param1`: Canny 高阈值
- `Param2`: 累加器阈值
- `Min_Radius, Max_Radius`: 半径范围

**适用场景:**
- 怪物是规则圆形
- 边缘清晰

---

## 🚀 推荐流程

### 第一步: HSV 颜色范围标定

1. 运行 `test_map.py`，选择方法 1
2. 调整 HSV 滑块，使黄色圆点被完全提取
3. 观察二值化结果（白色=检测到的区域）

**黄色 HSV 参考值:**
```
H: 20-35   (色调: 黄色)
S: 100-255 (饱和度: 高)
V: 100-255 (明度: 亮)
```

### 第二步: 形态学去噪

1. 调整 `Morph_Open` 去除小噪点
2. 调整 `Morph_Close` 填充圆点内部空洞

### 第三步: 面积和圆形度过滤

1. 调整 `Min_Area` 过滤太小的噪声
2. 调整 `Max_Area` 过滤太大的区域
3. 调整 `Circularity` 只保留圆形物体

### 第四步: 验证结果

1. 检查大窗口中的标注
2. 查看控制台输出的坐标列表
3. 按 `s` 保存结果图片
4. 按 `c` 复制坐标到剪贴板（如果实现）

---

## 📊 输出格式

### 控制台输出

```
============================================================
Detected 45 monsters on map (524x650), Time: 12.3ms
============================================================
Monster Positions:
  #1: (120, 85)   - Area: 156
  #2: (145, 92)   - Area: 148
  #3: (168, 105)  - Area: 152
  ...
  #45: (420, 380) - Area: 145
============================================================
✓ Total monsters found: 45
✓ Results saved to: map_detection_20251102_152030.png
✓ Coordinates saved to: map_coordinates_20251102_152030.json
============================================================
```

### JSON 坐标文件

```json
{
  "image": "map.jpeg",
  "timestamp": "2025-11-02 15:20:30",
  "total_monsters": 45,
  "monsters": [
    {"id": 1, "x": 120, "y": 85, "area": 156},
    {"id": 2, "x": 145, "y": 92, "area": 148},
    ...
  ]
}
```

---

## 🎮 集成到游戏脚本

### Python 示例

```python
import json
import cv2

# 读取检测结果
with open('map_coordinates.json', 'r') as f:
    data = json.load(f)

monsters = data['monsters']

# 获取当前角色位置
player_x, player_y = get_player_position()

# 找到最近的怪物
def find_nearest_monster(px, py, monsters):
    min_dist = float('inf')
    nearest = None
    for m in monsters:
        dist = ((m['x'] - px)**2 + (m['y'] - py)**2)**0.5
        if dist < min_dist:
            min_dist = dist
            nearest = m
    return nearest, min_dist

nearest_monster, distance = find_nearest_monster(player_x, player_y, monsters)

print(f"Nearest monster: ID={nearest_monster['id']}, Position=({nearest_monster['x']}, {nearest_monster['y']})")
print(f"Distance: {distance:.1f} pixels")

# 移动到该位置
move_to(nearest_monster['x'], nearest_monster['y'])
```

---

## 📝 调参技巧

### 检测不到怪物时

1. **降低 S_min** - 饱和度阈值太高
2. **降低 V_min** - 明度阈值太高
3. **扩大 H 范围** - 色调范围太窄
4. **减小 Min_Area** - 面积阈值太大
5. **减小 Circularity** - 圆形度要求太严格

### 检测到过多噪声时

1. **提高 S_min** - 提高饱和度要求
2. **提高 V_min** - 提高亮度要求
3. **缩小 H 范围** - 限制色调范围
4. **增大 Min_Area** - 过滤小噪点
5. **增大 Circularity** - 只保留圆形

### 圆点边缘不完整时

1. **增加 Morph_Close** - 填充内部空洞
2. **减小 Morph_Open** - 避免腐蚀太多

---

## 🔍 常见问题

### Q1: 为什么有些黄点检测不到？

**A:** 可能原因:
- HSV 范围太窄 → 扩大范围
- 圆点太小被过滤 → 减小 Min_Area
- 圆点不够圆 → 减小 Circularity

### Q2: 为什么背景也被检测为怪物？

**A:** 可能原因:
- HSV 范围太宽 → 缩小范围
- 面积过滤不够 → 调整 Min/Max_Area
- 圆形度过滤不够 → 增大 Circularity

### Q3: 检测速度太慢怎么办？

**A:** 优化方法:
- 使用方法 1（HSV 检测）而非模板匹配
- 缩小检测区域（ROI）
- 减少形态学操作次数
- 降低图片分辨率

### Q4: 如何处理地图缩放？

**A:**
- 记录当前地图缩放级别
- 坐标按比例转换到实际游戏世界坐标
- 或始终使用固定缩放级别截图

---

## 📦 文件说明

```
opencv/
├── map.jpeg                    # 游戏地图截图
├── point.png                   # 单个怪物模板
├── test_map.py                 # 检测程序（可调参）
├── MAP_DETECTION_GUIDE.md      # 本文档
├── map_detection_*.png         # 保存的检测结果
└── map_coordinates_*.json      # 保存的坐标数据
```

---

## 🎯 下一步计划

1. **实时检测**: 从游戏窗口实时截图并检测
2. **路径规划**: 基于怪物位置规划最优打怪路线
3. **自动战斗**: 移动到怪物位置并自动攻击
4. **区域优先级**: 标记高价值怪物区域优先打
5. **避障检测**: 检测地图障碍物，规划可行路径

---

**记住：先用 HSV 方法，90% 的情况下它是最好的选择！**
