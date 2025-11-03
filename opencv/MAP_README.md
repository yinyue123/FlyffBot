# 地图怪物检测 - 快速开始

## 🚀 快速使用

### 1. 运行程序

```bash
python test_map.py
```

### 2. 输入地图图片路径

```
Enter map image path: map.jpeg
```

或完整路径：
```
Enter map image path: /Users/yinyue/flyff/FlyffBot/opencv/map.jpeg
```

### 3. 选择检测方法

```
1. HSV Color Detection (推荐) ⭐
2. Template Matching
3. Hough Circle Detection
```

**首次使用推荐选择 1**

### 4. 如果选择方法2（模板匹配）

```
Enter template path: point.png
```

或完整路径：
```
Enter template path: /Users/yinyue/flyff/FlyffBot/opencv/point.png
```

---

## 🎮 界面说明

程序运行后会显示3个窗口：

### 窗口1: Controls（控制面板）
- 包含所有调节滑块
- ROI选择（X, Y, Width, Height）
- 检测参数滑块

### 窗口2: Detection Result（检测结果）
- 2x2网格显示
- 左上：原图 + ROI框
- 右上：HSV遮罩/灰度图
- 左下：形态学处理结果
- 右下：最终检测结果

### 窗口3: Large View（大图视图）
- 只显示ROI区域
- 自动放大小区域
- 清晰显示所有检测到的怪物

---

## 🎯 方法1（HSV检测）调参指南

### 黄色怪物点的推荐参数

```
H_min: 20    (黄色色调下限)
H_max: 35    (黄色色调上限)
S_min: 100   (高饱和度)
S_max: 255   (饱和度上限)
V_min: 100   (亮度下限)
V_max: 255   (亮度上限)

Morph_Open: 2   (去除小噪点)
Morph_Close: 3  (填充空洞)

Min_Area: 20    (过滤太小的点)
Max_Area: 500   (过滤太大的区域)

Circularity: 50  (圆形度，50=0.5，越大越圆)
```

### 调参流程

1. **先调整H范围** - 观察右上角窗口，确保黄点显示为白色
2. **调整S和V** - 进一步过滤背景
3. **调整形态学** - 去除噪声，填充空洞
4. **调整面积** - 过滤掉太小或太大的区域
5. **调整圆形度** - 只保留圆形的点

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

### 1. map_detection_时间戳.png
2x2网格显示图，包含所有处理步骤

### 2. map_large_view_时间戳.png
ROI大图，清晰显示所有检测到的怪物

### 3. map_coordinates_时间戳.json
怪物坐标数据，格式如下：

```json
{
  "timestamp": "20251102_152030",
  "total_monsters": 45,
  "monsters": [
    {"id": 1, "x": 120, "y": 85, "area": 156},
    {"id": 2, "x": 145, "y": 92, "area": 148},
    ...
  ]
}
```

---

## 🔧 常见问题

### Q: 检测不到怪物？

**解决方法：**
1. 调整H范围（扩大到15-40试试）
2. 降低S_min（降到50试试）
3. 降低V_min（降到50试试）
4. 减小Min_Area（改成10试试）
5. 观察右上角窗口，怪物应该显示为白色

### Q: 检测到太多噪声？

**解决方法：**
1. 缩小H范围（20-35）
2. 提高S_min（提高到150）
3. 提高V_min（提高到150）
4. 增大Min_Area（改成50）
5. 增大Circularity（改成70）
6. 增大Morph_Open（改成3-5）

### Q: 检测结果不稳定？

**解决方法：**
1. 确保地图截图清晰
2. 使用固定的游戏窗口大小
3. 保持地图缩放级别一致
4. 避免截图时有UI遮挡

### Q: 坐标如何使用？

**示例代码：**

```python
import json

# 读取坐标
with open('map_coordinates_20251102_152030.json', 'r') as f:
    data = json.load(f)

# 获取所有怪物位置
monsters = data['monsters']

# 遍历怪物
for m in monsters:
    print(f"怪物 #{m['id']}: 位置({m['x']}, {m['y']}), 面积{m['area']}")

# 找最近的怪物
player_x, player_y = 300, 250  # 玩家当前位置

def distance(m):
    return ((m['x'] - player_x)**2 + (m['y'] - player_y)**2)**0.5

nearest = min(monsters, key=distance)
print(f"最近的怪物: #{nearest['id']} at ({nearest['x']}, {nearest['y']})")
```

---

## 📚 详细文档

查看 `MAP_DETECTION_GUIDE.md` 获取：
- 详细的原理说明
- 3种方法的对比
- 高级调参技巧
- 集成到游戏脚本的示例

---

## 🎯 推荐工作流程

1. **运行程序**
   ```bash
   python test_map.py
   ```

2. **选择方法1（HSV检测）**

3. **加载地图图片** (`map.jpeg`)

4. **调整参数直到满意**
   - 观察3个窗口的显示
   - 查看控制台输出的怪物数量

5. **按 `s` 保存结果**
   - 保存图片用于验证
   - 保存JSON用于编程

6. **在你的游戏脚本中读取JSON**
   - 使用坐标规划路线
   - 自动移动打怪

---

## 📦 文件清单

```
opencv/
├── map.jpeg                      # 游戏地图截图
├── point.png                     # 怪物模板图片
├── test_map.py                   # 检测程序 ⭐
├── MAP_README.md                 # 本文档 ⭐
├── MAP_DETECTION_GUIDE.md        # 详细文档
├── map_detection_*.png           # 保存的检测结果
├── map_large_view_*.png          # 保存的大图
└── map_coordinates_*.json        # 保存的坐标数据
```

---

## 🌟 小提示

- **首次使用先用默认参数试试**，大多数情况下已经很准确
- **ROI功能**可以只检测地图的一部分，提高速度
- **大图窗口**会自动放大小区域，看清所有细节
- **保存的JSON文件**可以直接用于游戏脚本
- **多调几次参数**，找到最适合你的地图的设置

---

**祝你打怪愉快！Happy Hunting! 🎮**
