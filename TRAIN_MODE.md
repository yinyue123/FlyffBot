# Training Mode - 离线检测测试工具

## 📖 功能说明

Training Mode 是一个离线测试工具，可以加载截图进行怪物检测，并将检测结果可视化保存。这样您可以在不运行浏览器的情况下调试和验证检测逻辑。

## 🚀 使用方法

### 1. 准备测试图片

将游戏截图保存为 `train.png`，放在程序所在目录：

```bash
# 确保 train.png 存在
ls train.png
```

### 2. 运行训练模式

```bash
./flyff-bot --train
```

### 3. 查看结果

程序会生成两个输出：

- **result.png** - 可视化结果图片，包含：
  - 检测到的怪物边界框（黄色=被动怪，红色=主动怪）
  - HP/MP/FP 血量显示
  - 检测参数信息
  - HP 栏排除区域标记

- **Debug.log** - 详细的检测日志，包含：
  - 检测到的像素点数量
  - 聚类结果（多少个点聚合成多少个簇）
  - 每个聚类的接受/拒绝原因
  - 最终识别到的怪物数量

## 📊 可视化内容

### 怪物边界框
- **黄色框** = 被动怪物 (Passive)
- **红色框** = 主动怪物 (Aggressive)
- **紫色框** = Violet 怪物（会被过滤）

每个框上标注：`#序号 类型 (宽x高)`

### HP 栏区域
- **青色框** = HP 栏排除区域 (0,0)-(250,110)
- 此区域内的检测会被忽略，避免误识别

### 玩家状态
左侧显示：
- HP: XX%
- MP: XX%
- FP: XX%
- Target HP: XX%

### 检测参数
底部显示：
- MinWidth / MaxWidth
- 容差值
- 检测到的怪物总数

## 🔍 调试流程

### 步骤 1: 检查 Debug.log

查看检测过程：
```bash
tail -50 Debug.log
```

关键日志示例：
```
[INFO] Loading train.png...
[INFO] Image loaded: 1000x600
[DEBUG] Found 234 passive points, 0 aggressive points, 0 violet points
[DEBUG] Passive clustering: 234 points -> 3 clusters
[DEBUG] Passive cluster REJECTED at (123,45) size 3x8 (width must be >11 and <180, y: 45)
[DEBUG] Passive mob ACCEPTED at (456,234) size 67x12
[INFO] Found 1 mobs
```

### 步骤 2: 查看 result.png

打开可视化结果：
```bash
open result.png  # macOS
xdg-open result.png  # Linux
```

检查：
1. 怪物是否被正确框选？
2. 是否有误检测（气球、喇叭等）？
3. 边界框大小是否合理？

### 步骤 3: 调整参数

如果检测不准确，编辑 `data.json`：

```json
{
  "config": {
    "MinMobNameWidth": 11,      // 最小宽度（太小会误检测道具）
    "MaxMobNameWidth": 180,     // 最大宽度（太大会误检测）
    "PassiveTolerance": 5,      // 黄色容差（越大越宽松）
    "AggressiveTolerance": 10,  // 红色容差（越大越宽松）
    "PassiveColor": {
      "R": 234, "G": 234, "B": 149  // 被动怪颜色
    },
    "AggressiveColor": {
      "R": 179, "G": 23, "B": 23    // 主动怪颜色
    }
  }
}
```

### 步骤 4: 重新测试

修改参数后重新运行：
```bash
./flyff-bot --train
```

## 🐛 常见问题

### Q: 检测不到怪物？
**A:** 检查 Debug.log：
- 如果 "Found 0 passive points" → 颜色不匹配，调整 PassiveColor 或 PassiveTolerance
- 如果 "clustering: X points -> 0 clusters" → 聚类失败，检查 MinWidth/MaxWidth
- 如果 "cluster REJECTED" → 过滤太严格，调整宽度范围

### Q: 误检测到道具/UI？
**A:**
- 增加 MinMobNameWidth (11 → 15)
- 减少 Tolerance 值让匹配更严格
- 检查是否在 HP 栏区域内（应该被自动排除）

### Q: 聚类太碎？
**A:**
- 确认修复了排序问题（analyzer.go:402-414 应该有排序代码）
- 检查是否有多个相同的怪物重叠

### Q: result.png 显示不完整？
**A:**
- 检查 train.png 分辨率是否正常
- 确保图片格式为 PNG
- 查看 Debug.log 是否有错误信息

## 📝 示例工作流

```bash
# 1. 截图游戏并保存
# 将截图保存为 train.png

# 2. 运行训练模式
./flyff-bot --train

# 3. 查看日志
tail -50 Debug.log

# 4. 查看可视化结果
open result.png

# 5. 如果需要调整参数
vim data.json

# 6. 重新测试
./flyff-bot --train

# 7. 满意后运行正常模式
./flyff-bot
```

## 🎯 最佳实践

1. **使用多种场景的截图**
   - 远处的怪物（名字较小）
   - 近处的怪物（名字较大）
   - 有道具/气球的场景
   - 有其他玩家的场景

2. **逐步调整参数**
   - 先确保能检测到目标怪物
   - 再调整范围过滤掉误检测

3. **保留测试截图**
   - 将不同场景的 train.png 保存为 train_1.png, train_2.png 等
   - 可以批量测试确保稳定性

4. **对比 Rust 版本**
   - 如果有 Rust 版本，可以对比检测结果
   - 确保两个版本的参数一致

## 🔗 相关文件

- `train.go` - 训练模式实现
- `analyzer.go` - 检测逻辑
- `data.go` - 配置定义
- `data.json` - 用户配置

---

**提示**: 训练模式不会连接浏览器，也不会进行任何游戏操作，完全离线运行，非常安全！
