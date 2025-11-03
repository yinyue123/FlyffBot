# 玩家箭头检测 - 快速开始

## 🚀 快速使用

### 1. 运行程序

```bash
python test_arrow.py
```

### 2. 输入地图图片路径

```
Enter map image path: map.jpeg
```

### 3. 选择检测方法

```
1. HSV Color Detection (推荐) ⭐
2. Template Matching
```

**首次使用推荐选择 1**

### 4. 如果选择方法2（模板匹配）

需要提供箭头模板图片：
```
Enter arrow template path: arrow.png
```

---

## 🎮 界面说明

程序运行后会显示3个窗口：

### 窗口1: Controls（控制面板）
- ROI选择滑块
- 检测参数滑块

### 窗口2: Detection Result（检测结果）
- 2x2网格显示
- 左上：原图 + ROI框
- 右上：HSV图像/灰度图
- 左下：颜色遮罩
- 右下：最终检测结果 + 箭头标注

### 窗口3: Large View（大图视图）
- ROI区域放大显示
- 清晰显示箭头位置和方向

---

## 🎯 方法1（HSV检测）推荐参数

### 白色/灰色箭头的参数设置

```
H_min: 0       (色调最小值 - 白色无色调)
H_max: 180     (色调最大值 - 覆盖所有色调)
S_min: 0       (饱和度最小值)
S_max: 50      (饱和度最大值 - 白色低饱和度)
V_min: 150     (亮度最小值 - 箭头较亮)
V_max: 255     (亮度最大值)

Morph_Open: 2   (去除小噪点)
Morph_Close: 3  (填充箭头内部)

Min_Area: 200   (箭头最小面积)
Max_Area: 5000  (箭头最大面积)

Direction_Method: 1  (0=旋转矩形 1=凸包检测 2=PCA)
```

### 方向检测方法说明

- **0 = MinAreaRect** (旋转矩形)
  - 最快，但有180度歧义
  - 无法区分箭头头尾

- **1 = ConvexHull** (凸包检测) ⭐ 推荐
  - 找到箭头尖端
  - 准确判断方向
  - 推荐使用

- **2 = PCA** (主成分分析)
  - 找主轴方向
  - 有180度歧义
  - 适合狭长形状

---

## ⌨️ 快捷键

| 按键 | 功能 |
|------|------|
| `q` 或 `ESC` | 退出程序 |
| `s` | 保存结果（图片 + 坐标JSON） |
| `r` | 重置ROI到全图 |

---

## 💾 输出文件

按 `s` 保存后会生成3个文件：

### 1. arrow_detection_时间戳.png
2x2网格显示图，包含所有处理步骤

### 2. arrow_large_view_时间戳.png
ROI大图，清晰显示箭头位置和方向

### 3. arrow_result_时间戳.json
箭头数据，格式如下：

```json
{
  "timestamp": "20251102_153045",
  "arrow": {
    "found": true,
    "position": {
      "x": 262,
      "y": 315
    },
    "direction": {
      "angle": 225.3,
      "cardinal": "SW",
      "method": "ConvexHull"
    },
    "properties": {
      "area": 1245,
      "width": 45,
      "height": 38,
      "aspect_ratio": 1.18,
      "confidence": 0.89
    }
  }
}
```

---

## 🧭 方向说明

### 角度定义

```
        0° (N)
        ↑
        |
270° ←--+--→ 90° (E)
(W)     |
        ↓
      180° (S)
```

### 方位对照表

| 角度范围 | 方位 | 说明 |
|---------|------|------|
| 337.5° - 22.5° | N | 正上（北） |
| 22.5° - 67.5° | NE | 右上（东北） |
| 67.5° - 112.5° | E | 正右（东） |
| 112.5° - 157.5° | SE | 右下（东南） |
| 157.5° - 202.5° | S | 正下（南） |
| 202.5° - 247.5° | SW | 左下（西南） |
| 247.5° - 292.5° | W | 正左（西） |
| 292.5° - 337.5° | NW | 左上（西北） |

---

## 🔧 常见问题

### Q: 检测不到箭头？

**解决方法：**
1. 增加 S_max（改成100试试）- 如果箭头不是纯白色
2. 降低 V_min（改成100试试）- 如果箭头不够亮
3. 减小 Min_Area（改成100试试）- 如果箭头很小
4. 观察右下角窗口，箭头应该在遮罩中显示为白色

### Q: 检测到错误的物体？

**解决方法：**
1. 减小 S_max（改成30试试）- 只检测纯白色
2. 提高 V_min（改成180试试）- 只检测很亮的物体
3. 调整 Min_Area 和 Max_Area 限制大小范围
4. 使用ROI只检测地图中心区域

### Q: 方向不准确？

**解决方法：**
1. 尝试 Direction_Method = 1（凸包方法最准确）
2. 确保箭头形状完整（调整形态学参数）
3. 增大 Morph_Close 填充箭头内部空洞
4. 如果箭头被部分遮挡，可能影响方向检测

### Q: 方向有180度误差？

**A:** 使用 Direction_Method=0（旋转矩形）时常见

**解决方法：**
- 改用 Direction_Method=1（凸包检测）
- 凸包方法能正确识别箭头尖端方向

---

## 🎮 在游戏脚本中使用

### 读取箭头位置和方向

```python
import json
import numpy as np

# 读取检测结果
with open('arrow_result_20251102_153045.json', 'r') as f:
    data = json.load(f)

arrow = data['arrow']

if arrow['found']:
    # 获取玩家位置
    player_x = arrow['position']['x']
    player_y = arrow['position']['y']

    # 获取玩家朝向
    player_angle = arrow['direction']['angle']
    player_dir = arrow['direction']['cardinal']

    print(f"玩家位置: ({player_x}, {player_y})")
    print(f"玩家朝向: {player_angle:.1f}° ({player_dir})")
else:
    print("未检测到玩家箭头")
```

### 计算前方目标点

```python
# 计算玩家前方N像素的点
def get_front_position(player_x, player_y, angle, distance):
    """
    Args:
        player_x, player_y: 玩家当前位置
        angle: 玩家朝向角度（0=北）
        distance: 向前距离（像素）

    Returns:
        (target_x, target_y): 目标位置
    """
    rad = np.radians(angle)

    # 游戏坐标系：0度=正上，顺时针
    target_x = player_x + distance * np.sin(rad)
    target_y = player_y - distance * np.cos(rad)  # Y轴向下

    return int(target_x), int(target_y)

# 示例：向前移动50像素
front_x, front_y = get_front_position(player_x, player_y, player_angle, 50)
print(f"前方50像素: ({front_x}, {front_y})")

# 移动到该位置
move_to(front_x, front_y)
```

### 计算到目标的相对方向

```python
def calculate_relative_angle(from_x, from_y, to_x, to_y):
    """
    计算从起点到终点的角度

    Args:
        from_x, from_y: 起点坐标
        to_x, to_y: 终点坐标

    Returns:
        angle: 角度（0-360，0=北）
    """
    dx = to_x - from_x
    dy = to_y - from_y

    # 计算数学角度（0=东，逆时针）
    math_angle = np.degrees(np.arctan2(dy, dx))

    # 转换为游戏角度（0=北，顺时针）
    game_angle = (90 - math_angle) % 360

    return game_angle

# 示例：计算到怪物的方向
monster_x, monster_y = 350, 200

target_angle = calculate_relative_angle(player_x, player_y, monster_x, monster_y)
print(f"怪物方向: {target_angle:.1f}°")

# 计算需要旋转的角度
turn_angle = (target_angle - player_angle + 180) % 360 - 180

if abs(turn_angle) > 5:  # 如果偏差大于5度
    if turn_angle > 0:
        print(f"需要向右转 {turn_angle:.1f}°")
        turn_right(turn_angle)
    else:
        print(f"需要向左转 {-turn_angle:.1f}°")
        turn_left(-turn_angle)
else:
    print("方向正确，直接前进")
    move_forward()
```

### 实战示例：自动寻找并走向最近怪物

```python
import json
import numpy as np

# 1. 读取箭头位置
with open('arrow_result.json', 'r') as f:
    arrow_data = json.load(f)

player_x = arrow_data['arrow']['position']['x']
player_y = arrow_data['arrow']['position']['y']
player_angle = arrow_data['arrow']['direction']['angle']

# 2. 读取怪物位置
with open('map_coordinates.json', 'r') as f:
    monster_data = json.load(f)

monsters = monster_data['monsters']

# 3. 找到最近的怪物
def distance(m):
    return np.sqrt((m['x'] - player_x)**2 + (m['y'] - player_y)**2)

nearest_monster = min(monsters, key=distance)
dist = distance(nearest_monster)

print(f"最近的怪物: #{nearest_monster['id']}")
print(f"位置: ({nearest_monster['x']}, {nearest_monster['y']})")
print(f"距离: {dist:.1f} 像素")

# 4. 计算到怪物的方向
target_angle = calculate_relative_angle(
    player_x, player_y,
    nearest_monster['x'], nearest_monster['y']
)

# 5. 旋转角色面向怪物
turn_angle = (target_angle - player_angle + 180) % 360 - 180

if abs(turn_angle) > 5:
    print(f"旋转 {turn_angle:.1f}°")
    rotate_character(turn_angle)
    time.sleep(0.5)  # 等待旋转完成

# 6. 向前移动
print(f"向前移动 {dist:.1f} 像素")
move_forward(dist)

# 7. 到达后开始攻击
print("到达目标，开始攻击")
attack()
```

---

## 📝 调参技巧

### 第一步：确保能检测到箭头

1. 运行程序，观察右下角的"Color Mask"窗口
2. 箭头应该显示为白色区域
3. 如果看不到，调整HSV范围：
   - 增加 S_max（从50→100）
   - 降低 V_min（从150→100）

### 第二步：去除噪声

1. 如果检测到多个白色区域
2. 调整形态学参数：
   - 增加 Morph_Open（去除小噪点）
   - 调整 Min_Area 和 Max_Area

### 第三步：优化方向检测

1. 观察右下角的箭头方向指示
2. 如果方向不对：
   - 尝试 Direction_Method = 1
   - 增大 Morph_Close 确保箭头完整

### 第四步：保存参数

1. 找到最佳参数后按 `s` 保存
2. 记录当前滑块值
3. 下次运行时手动设置相同参数
4. 或修改程序中的默认值

---

## 🎯 最佳实践

### 1. 优先检测中心区域

玩家箭头通常在地图中心：

```python
# 设置ROI只检测中心区域
image_h, image_w = 650, 524
roi_x = image_w // 2 - 100  # 中心左侧100像素
roi_y = image_h // 2 - 100  # 中心上方100像素
roi_w = 200
roi_h = 200
```

### 2. 使用滑动平均平滑方向

避免方向抖动：

```python
from collections import deque

angle_history = deque(maxlen=5)

# 每帧检测
current_angle = detect_arrow_angle()
angle_history.append(current_angle)

# 使用平均值
smoothed_angle = np.mean(angle_history)
```

### 3. 处理检测失败

```python
retry_count = 0
max_retries = 3

while retry_count < max_retries:
    arrow = detect_arrow()
    if arrow['found']:
        break
    retry_count += 1
    time.sleep(0.1)  # 短暂等待后重试

if not arrow['found']:
    print("检测失败，使用上次已知位置")
    arrow = last_known_position
```

---

## 📚 详细文档

查看 `ARROW_DETECTION_GUIDE.md` 获取：
- 详细的原理说明
- 方向计算算法
- 高级调参技巧
- 坐标系转换公式

---

## 📦 文件清单

```
opencv/
├── map.jpeg                      # 游戏地图截图
├── arrow.png                     # 箭头模板（可选）
├── test_arrow.py                 # 箭头检测程序 ⭐
├── ARROW_README.md               # 本文档 ⭐
├── ARROW_DETECTION_GUIDE.md      # 详细文档
├── arrow_detection_*.png         # 保存的检测结果
├── arrow_large_view_*.png        # 保存的大图
└── arrow_result_*.json           # 保存的箭头数据
```

---

## 🌟 小提示

- **推荐使用方法1（HSV检测）+ Direction_Method=1（凸包）**
- **先用ROI限制检测区域**，提高速度和准确度
- **大图窗口会放大ROI区域**，方便查看细节
- **保存的JSON可以直接用于游戏脚本**
- **方向角度0度=正上方（北），顺时针增加**

---

## 🎯 完整工作流程

1. **运行 test_arrow.py**
2. **选择方法1（HSV）**
3. **调整参数直到检测成功**
4. **按 's' 保存结果JSON**
5. **在游戏脚本中读取JSON**
6. **使用位置和方向信息控制角色**

---

**祝你游戏愉快！Have fun! 🎮**
