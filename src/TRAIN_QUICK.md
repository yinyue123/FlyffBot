# 🎯 Training Mode Quick Start

## 快速开始

### 1. 保存截图
将您提供的游戏截图保存为 `train.png`

### 2. 运行测试
```bash
./flyff-bot --train
```

### 3. 查看结果
- **result.png** - 可视化检测结果
- **Debug.log** - 详细检测日志

## 预期输出

### Console 输出:
```
[INFO] === Training Mode Started ===
[INFO] Loading train.png...
[INFO] Image loaded: 1000x600
[INFO] === Running Detection ===
[INFO] Detecting mobs...
[DEBUG] Found 234 passive points, 0 aggressive points, 0 violet points
[DEBUG] Passive clustering: 234 points -> 3 clusters
[DEBUG] Passive cluster REJECTED at (...)
[DEBUG] Passive mob ACCEPTED at (...) size 67x12
[INFO] Found 1 mobs
[INFO] Detecting player stats...
[INFO] HP: 100%, MP: 100%, FP: 100%
[INFO] === Creating Visualization ===
[INFO] Saving visualization to result.png...
[INFO] === Training Mode Completed ===
```

### result.png 包含:
- ✅ 黄色/红色边界框标记怪物
- ✅ HP/MP/FP 数值显示
- ✅ 青色框标记 HP 栏排除区域
- ✅ 检测参数信息

## 验证检测逻辑

打开 result.png，检查：
1. 是否正确识别到屏幕中的两个怪物？
2. 是否避免了误检测道具/气球？
3. 边界框是否准确框选怪物名字？

## 下一步

满意后，运行正常模式：
```bash
./flyff-bot
```

详细文档: [TRAIN_MODE.md](TRAIN_MODE.md)
