# Data 数据结构模块

本模块包含游戏机器人系统中使用的核心数据结构和类型定义。

---

## bounds.rs - 二维边界框

### 概述
定义了二维空间中的矩形边界框结构，用于表示图像识别中的目标区域。

### 核心结构

#### Bounds
```rust
pub struct Bounds {
    pub x: u32,      // 左上角 X 坐标
    pub y: u32,      // 左上角 Y 坐标
    pub w: u32,      // 宽度
    pub h: u32,      // 高度
}
```

### 主要方法

#### 1. new(x, y, w, h)
创建一个新的边界框。

#### 2. get_lowest_center_point()
获取边界框底部中心点。
```rust
Point::new(x + w/2, y + h)
```
**用途**：计算怪物的攻击点位（底部中心）。

#### 3. size()
返回边界框的面积（像素数量）。
```rust
w × h
```

#### 4. grow_by(px)
向所有方向扩展边界框。
```rust
x -= px/2
y -= px/2
w += px
h += px
```
**特点**：使用 `saturating_sub` 防止下溢。

**用途**：在 farming_behavior 中扩大已攻击目标的避免区域。

#### 5. center()
获取边界框的中心点。
```rust
Point::new(x + w/2, y + h/2)
```

#### 6. contains_point(point)
检查指定点是否在边界框内。
```rust
point.x >= x && point.x <= x + w &&
point.y >= y && point.y <= y + h
```

### 特性实现

- **Serialize/Deserialize**：支持序列化，可以保存/加载配置
- **Clone, Copy**：轻量级拷贝
- **Debug**：支持调试输出
- **slog::Value**：支持日志记录

### 使用场景

1. **怪物识别**：表示识别到的怪物边界
2. **目标标记**：表示屏幕上的目标标记区域
3. **区域避免**：标记不应点击的区域
4. **状态栏检测**：定义 HP/MP/FP 状态栏的搜索区域

---

## pixel_detection.rs - 像素检测配置

### 概述
定义像素检测的配置和状态，用于识别特定颜色的像素区域。

**注意**：此文件中的 `PixelDetection` 结构的 `update_value` 方法已被注释掉，可能已废弃。

### 核心结构

#### PixelDetectionKind
```rust
pub enum PixelDetectionKind {
    CursorType,  // 光标类型检测
}
```
定义像素检测的类型（目前只有光标类型）。

#### PixelDetectionConfig
```rust
pub struct PixelDetectionConfig {
    pub max_x: u32,          // 最大 X 坐标
    pub max_y: u32,          // 最大 Y 坐标
    pub min_x: u32,          // 最小 X 坐标
    pub min_y: u32,          // 最小 Y 坐标
    pub refs: Vec<Color>,    // 要检测的颜色列表
}
```

### 配置实例

#### CursorType 配置
```rust
color: [0, 128, 0]  // 绿色光标
min_x: 0
min_y: 0
max_x: 1
max_y: 1
```

#### 默认配置
```rust
max_x: 310
max_y: 120
min_x: 0
min_y: 0
refs: vec![Color::default()]
```

#### PixelDetection
```rust
pub struct PixelDetection {
    pub value: bool,                        // 当前检测值
    pub pixel_kind: PixelDetectionKind,     // 检测类型
    pub last_value: bool,                   // 上次检测值
    pub last_update_time: Option<Instant>,  // 上次更新时间
}
```

### 比较实现

- **PartialEq**：只比较 value 值
- **PartialOrd**：基于 value 值排序

### 已注释代码

文件中包含大量注释掉的代码（98-141 行），包括：
- `new()` 方法
- `update_value()` 方法
- 使用 PointCloud 进行像素检测的逻辑

这表明像素检测功能可能已经被重构或废弃。

### 使用场景

理论上用于：
- 检测光标颜色变化
- 识别特定颜色的 UI 元素
- 但当前实现中大部分功能已注释

---

## point.rs - 二维点

### 概述
定义二维空间中的点结构，是所有位置相关操作的基础。

### 核心结构

#### Point
```rust
pub struct Point {
    pub x: u32,  // X 坐标
    pub y: u32,  // Y 坐标
}
```

### 方法

#### new(x, y)
创建一个新的点。

### 类型转换

#### From<(u32, u32)>
支持从元组创建点：
```rust
let point = Point::from((100, 200));
// 等同于
let point: Point = (100, 200).into();
```

### 特性实现

- **Debug**：调试输出
- **Clone, Copy**：轻量级拷贝
- **PartialEq, Eq**：相等比较
- **Display**：格式化输出为 `(x: 100, y: 200)`
- **slog::Value**：日志记录支持

### 使用场景

1. **鼠标坐标**：表示屏幕上的点击位置
2. **怪物位置**：表示怪物在屏幕上的坐标
3. **UI 元素**：表示按钮、技能栏的位置
4. **计算基础**：用于距离计算、边界计算等

### 示例

```rust
// 创建点
let point = Point::new(100, 200);

// 从元组创建
let point: Point = (100, 200).into();

// 格式化输出
println!("{}", point);  // 输出: (x: 100, y: 200)
```

---

## point_cloud.rs - 点云

### 概述
点云是点的集合，提供了对点集的高级操作，如排序、聚类、边界计算等。

### 核心结构

#### PointCloud
```rust
pub struct PointCloud {
    points: Vec<Point>,  // 点的集合
}
```

### 主要方法

#### 1. new<P>(points)
从点的集合创建点云。

#### 2. push(point)
添加一个点到点云。

#### 3. is_empty()
检查点云是否为空。

#### 4. sort_by<Selector>(selector)
根据选择器函数对点进行排序（原地排序）。

#### 5. sorted_by<Selector>(selector)
返回排序后的新点云（不修改原点云）。

#### 6. to_bounds()
将点云转换为边界框。

**算法**：
```rust
min_x = points.iter().map(|p| p.x).min()
min_y = points.iter().map(|p| p.y).min()
max_x = points.iter().map(|p| p.x).max()
max_y = points.iter().map(|p| p.y).max()

Bounds {
    x: min_x,
    y: min_y,
    w: max_x - min_x,
    h: max_y - min_y,
}
```

**用途**：
- 将检测到的状态栏像素转换为边界框
- 将怪物识别的像素点转换为怪物边界

#### 7. cluster_by_distance<F>(distance, selector)
根据距离将点聚类。

**参数**：
- `distance`：聚类的最大距离阈值
- `selector`：选择器函数（x_axis 或 y_axis）

**算法**：
1. 按选择器排序点
2. 遍历排序后的点
3. 如果当前点与当前簇最后一个点的距离 ≤ distance，加入当前簇
4. 否则创建新簇

**返回**：聚类后的点云向量

**用途**：
- 识别多个相邻的怪物
- 分离不同的 UI 元素

### point_selector 模块

提供两个选择器函数：

#### x_axis(point)
返回点的 X 坐标。

#### y_axis(point)
返回点的 Y 坐标。

**用途**：用于 cluster_by_distance 和排序操作。

### 特性实现

#### AsRef<[Point]>
可以作为点切片的引用。

#### Iterator
实现了迭代器 trait，但使用 `pop()` 实现（逆序迭代）。

#### From<T> where T: AsRef<[(u32, u32)]>
支持从元组数组创建点云：
```rust
let cloud = PointCloud::from([(0, 0), (10, 10), (5, 5)]);
```

### 使用场景

1. **状态栏识别**：
   - 收集特定颜色的像素点
   - 转换为边界框
   - 计算状态栏宽度和百分比

2. **怪物识别**：
   - 收集怪物颜色的像素点
   - 聚类识别多个怪物
   - 为每个怪物创建边界框

3. **目标标记**：
   - 识别屏幕上的目标标记
   - 计算标记的位置和大小

### 测试用例

#### test_cluster_by_distance
```rust
points: [(0, 0), (9, 1), (5, 1), (15, 5), (17, 3)]
distance: 5
selector: x_axis

结果:
cluster[0] = [(0, 0), (5, 1), (9, 1)]  // X 坐标相差 ≤ 5
cluster[1] = [(15, 5), (17, 3)]        // X 坐标相差 ≤ 5
```

#### test_approx_rect
测试点云到边界框的转换。

#### test_bounds
测试边界框的各种方法。

---

## stats_info.rs - 游戏状态信息

### 概述
管理游戏角色和目标的状态信息，包括 HP/MP/FP 等，是图像识别结果的核心存储结构。

### 核心枚举

#### StatusBarKind
```rust
pub enum StatusBarKind {
    Hp,        // 角色生命值
    Mp,        // 角色魔法值
    Fp,        // 角色疲劳值
    TargetHP,  // 目标生命值
    TargetMP,  // 目标魔法值
}
```

#### AliveState
```rust
pub enum AliveState {
    StatsTrayClosed,  // 状态栏关闭
    Alive,            // 存活
    Dead,             // 死亡
}
```

### 核心结构

#### ClientStats
```rust
pub struct ClientStats {
    // 角色状态
    pub has_tray_open: bool,        // 状态栏是否打开
    pub hp: StatInfo,               // 生命值
    pub mp: StatInfo,               // 魔法值
    pub fp: StatInfo,               // 疲劳值
    pub is_alive: AliveState,       // 存活状态

    // 目标状态
    pub target_hp: StatInfo,        // 目标生命值
    pub target_mp: StatInfo,        // 目标魔法值
    pub target_is_mover: bool,      // 目标是可移动单位（玩家/怪物）
    pub target_is_npc: bool,        // 目标是 NPC
    pub target_is_alive: bool,      // 目标是否存活
    pub target_on_screen: bool,     // 目标是否在屏幕上
    pub target_marker: Option<Target>,      // 目标标记
    pub target_distance: Option<i32>,       // 目标距离

    // 内部状态
    stat_try_not_detected_count: i32,  // 检测失败计数
    window: Window,                     // Tauri 窗口引用
}
```

#### StatInfo
```rust
pub struct StatInfo {
    pub max_w: u32,                      // 状态栏最大宽度
    pub value: u32,                      // 当前值（0-100）
    pub stat_kind: StatusBarKind,        // 状态类型
    pub last_value: u32,                 // 上次的值
    pub last_update_time: Option<Instant>,  // 上次更新时间
}
```

### ClientStats 方法

#### 1. new(window)
创建新的客户端状态实例，初始化所有状态为默认值。

#### 2. update(image, logger)
更新所有状态信息（核心方法）。

**执行流程**：
```
1. 更新所有状态栏（HP/MP/FP/TargetHP/TargetMP）
   ↓
2. 检测状态栏是否打开
   ↓
3. 判断角色存活状态
   ↓
4. 判断目标类型（NPC/Mover/Alive）
   ↓
5. 识别目标标记
   ↓
6. 计算目标距离
```

**状态判断逻辑**：

##### 角色存活状态
```rust
if !has_tray_open {
    AliveState::StatsTrayClosed
} else if hp.value > 0 {
    AliveState::Alive
} else {
    AliveState::Dead
}
```

##### 目标类型判断
```rust
// NPC：HP=100, MP=0
target_is_npc = (target_hp == 100 && target_mp == 0)

// Mover（玩家/怪物）：MP > 0
target_is_mover = (target_mp > 0)

// 存活：HP > 0
target_is_alive = (target_hp > 0)
```

##### 目标标记和距离
```rust
// 优先查找蓝色标记，否则查找红色标记
blue_target = image.identify_target_marker(true)
target_marker = blue_target.or_else(|| image.identify_target_marker(false))

target_on_screen = target_marker.is_some()

if target_on_screen {
    target_distance = image.get_target_marker_distance(target_marker)
}
```

#### 3. detect_stat_tray()
检测状态栏是否打开，如果未打开则尝试打开。

**逻辑**：
```rust
if hp == 0 && mp == 0 && fp == 0 {
    stat_try_not_detected_count += 1

    if stat_try_not_detected_count == 5 {
        // 尝试按 T 键打开状态栏
        send_key("T")
        stat_try_not_detected_count = 0
    }

    return false
} else {
    stat_try_not_detected_count = 0
    return true
}
```

**特点**：
- 连续 5 次检测失败才尝试打开
- 避免频繁按键
- 自动重置计数器

### StatInfo 方法

#### 1. new(max_w, value, stat_kind, image)
创建新的状态信息，可选择立即从图像更新。

#### 2. reset_last_update_time()
重置上次更新时间为当前时间。

**用途**：在障碍物规避后重置目标 HP 更新时间。

#### 3. update_value(image)
从图像更新状态值（核心方法）。

**算法**：
```
1. 获取状态栏配置（颜色、区域）
   ↓
2. 在指定区域检测特定颜色的像素
   ↓
3. 收集所有匹配的像素点到点云
   ↓
4. 点云转换为边界框
   ↓
5. 计算状态栏宽度和百分比
   ↓
6. 更新 max_w 和 value
```

**计算公式**：
```rust
// 更新最大宽度（只增不减）
updated_max_w = max(bounds.w, current_max_w)

// 计算百分比
value_frac = bounds.w / updated_max_w
updated_value = clamp(value_frac * 100, 0, 100)
```

**返回值**：
- `true`：值发生了变化
- `false`：值没有变化

**更新时间戳**：
- 只有当值变化时才更新 `last_update_time`
- 用于检测目标是否正在受伤（战斗中）

### StatusBarConfig

定义每种状态栏的检测配置。

#### 结构
```rust
pub struct StatusBarConfig {
    pub max_x: u32,          // 搜索区域右边界
    pub max_y: u32,          // 搜索区域下边界
    pub min_x: u32,          // 搜索区域左边界
    pub min_y: u32,          // 搜索区域上边界
    pub refs: Vec<Color>,    // 要检测的颜色列表
}
```

#### HP 配置
```rust
colors: [
    [174, 18, 55],   // 深红
    [188, 24, 62],   // 红
    [204, 30, 70],   // 亮红
    [220, 36, 78],   // 很亮的红
]
```

#### MP 配置
```rust
colors: [
    [20, 84, 196],   // 深蓝
    [36, 132, 220],  // 蓝
    [44, 164, 228],  // 亮蓝
    [56, 188, 232],  // 很亮的蓝
]
```

#### FP 配置
```rust
colors: [
    [45, 230, 29],   // 亮绿
    [28, 172, 28],   // 绿
    [44, 124, 52],   // 深绿
    [20, 146, 20],   // 暗绿
]
```

#### TargetHP 配置
```rust
colors: 与 HP 相同
min_x: 300
min_y: 30
max_x: 550
max_y: 60
```

#### TargetMP 配置
```rust
colors: 与 MP 相同
min_x: 300
min_y: 50
max_x: 550
max_y: 60
```

### 使用场景

#### 1. 战斗系统
```rust
// 检查是否需要治疗
if client_stats.hp.value < 50 {
    use_heal_skill();
}

// 检查目标是否死亡
if !client_stats.target_is_alive {
    transition_to_after_kill();
}
```

#### 2. 辅助系统
```rust
// 检查队友是否需要复活
if client_stats.target_is_mover && !client_stats.target_is_alive {
    use_rez_skill();
}

// 检查是否需要治疗队友
if client_stats.target_hp.value < 70 {
    use_heal_skill_on_target();
}
```

#### 3. 自动恢复
```rust
// HP 恢复
if client_stats.hp.value < 80 {
    use_pill();
}

// MP 恢复
if client_stats.mp.value < 30 {
    use_mp_restorer();
}

// FP 恢复
if client_stats.fp.value < 20 {
    use_fp_restorer();
}
```

#### 4. 目标验证
```rust
// 检查是否点击到 NPC
if client_stats.target_is_npc {
    cancel_target();
    search_for_new_target();
}

// 检查目标是否在攻击范围内
if let Some(distance) = client_stats.target_distance {
    if distance > MAX_ATTACK_RANGE {
        move_closer();
    }
}
```

### 比较实现

#### StatInfo
- **PartialEq**：只比较 value 值
- **PartialOrd**：基于 value 值排序

### 调试功能

#### _debug_print(logger)
打印详细的状态信息（已注释在 update 中）。

输出格式：
```
Stats detection:
  HP: 85
  MP: 60
  FP: 100
  Enemy HP: 45
  Character is: alive
```

### 关键特性

1. **自动状态栏管理**：
   - 检测状态栏是否打开
   - 自动尝试打开关闭的状态栏
   - 防止频繁操作

2. **智能目标识别**：
   - 区分 NPC 和可移动单位
   - 检测目标存活状态
   - 计算目标距离

3. **精确的颜色检测**：
   - 每种状态栏使用多个颜色变体
   - 适应不同亮度和渲染情况
   - 提高识别准确率

4. **时间戳跟踪**：
   - 记录每次值变化的时间
   - 用于检测战斗状态
   - 用于障碍物规避逻辑

---

## target.rs - 目标信息

### 概述
定义游戏中的目标类型和目标结构，用于表示怪物和目标标记。

### 核心枚举

#### MobType
```rust
pub enum MobType {
    Passive,     // 被动怪物（不主动攻击）
    Aggressive,  // 主动怪物（主动攻击玩家）
    Violet,      // 紫色怪物（特殊类型）
}
```

**用途**：
- 决定攻击优先级
- Aggressive 怪物优先攻击（如果配置了 prioritize_aggro）
- Violet 怪物通常被排除在攻击目标之外

#### TargetType
```rust
pub enum TargetType {
    Mob(MobType),    // 怪物目标（包含具体类型）
    TargetMarker,    // 目标标记（屏幕上的标记图标）
}
```

**默认值**：`TargetMarker`

### 核心结构

#### Target
```rust
pub struct Target {
    pub target_type: TargetType,  // 目标类型
    pub bounds: Bounds,            // 目标的边界框
}
```

### 方法

#### get_attack_coords()
获取目标的攻击坐标。

**算法**：
```rust
// 1. 获取边界框底部中心点
let point = bounds.get_lowest_center_point()  // (x + w/2, y + h)

// 2. 向下偏移 10 像素
Point::new(point.x, point.y + 10)
```

**为什么向下偏移 10 像素？**
- 确保点击在怪物身上而不是血条上
- 提高点击的成功率
- 避免点击到怪物名称或其他 UI 元素

### 特性实现

- **Debug**：调试输出
- **Clone, Copy**：轻量级拷贝
- **Default**：默认为 TargetMarker 类型

### 使用场景

#### 1. 怪物识别
```rust
// 图像分析器识别怪物
let mobs: Vec<Target> = image.identify_mobs(config);

// 筛选被动怪物
let passive_mobs: Vec<Target> = mobs.iter()
    .filter(|m| m.target_type == TargetType::Mob(MobType::Passive))
    .collect();

// 筛选主动怪物
let aggressive_mobs: Vec<Target> = mobs.iter()
    .filter(|m| m.target_type == TargetType::Mob(MobType::Aggressive))
    .collect();
```

#### 2. 目标选择
```rust
// 找到最近的怪物
let closest_mob = image.find_closest_mob(&mobs, None, max_distance, logger);

// 获取攻击坐标
if let Some(mob) = closest_mob {
    let attack_pos = mob.get_attack_coords();
    eval_mob_click(window, attack_pos);
}
```

#### 3. 优先级系统
```rust
// 如果启用优先攻击主动怪
if config.prioritize_aggro() {
    let aggressive = mobs.iter()
        .filter(|m| matches!(m.target_type, TargetType::Mob(MobType::Aggressive)))
        .collect();

    if aggressive.is_empty() {
        // 没有主动怪，攻击被动怪
        let passive = mobs.iter()
            .filter(|m| matches!(m.target_type, TargetType::Mob(MobType::Passive)))
            .collect();
    }
}
```

#### 4. 避免系统
```rust
// 记录击杀的怪物类型
match mob.target_type {
    TargetType::Mob(MobType::Aggressive) => {
        last_killed_type = MobType::Aggressive;
    }
    TargetType::Mob(MobType::Passive) => {
        last_killed_type = MobType::Passive;
    }
    TargetType::Mob(MobType::Violet) => {
        last_killed_type = MobType::Violet;
    }
    TargetType::TargetMarker => {}
}

// 避免刚击杀的怪物区域
avoided_bounds.push((mob.bounds.grow_by(20), Instant::now(), 5000));
```

#### 5. 目标标记检测
```rust
// 检测屏幕上的目标标记
let target_marker = image.identify_target_marker(true);  // 蓝色标记

if let Some(marker) = target_marker {
    // 计算距离
    let distance = image.get_target_marker_distance(marker);

    // 检查是否在攻击范围内
    if distance < MAX_DISTANCE_FOR_AOE {
        use_aoe_skill();
    }
}
```

### 与其他模块的关系

#### 与 Bounds 的关系
```rust
// Target 包含 Bounds
pub struct Target {
    pub bounds: Bounds,  // 使用 Bounds 表示位置和大小
    ...
}

// 使用 Bounds 的方法
let attack_pos = target.bounds.get_lowest_center_point();
let expanded = target.bounds.grow_by(10);
```

#### 与 Point 的关系
```rust
// get_attack_coords 返回 Point
let point: Point = target.get_attack_coords();

// 用于鼠标点击
eval_mob_click(window, point);
```

#### 与 stats_info 的关系
```rust
// ClientStats 存储当前目标标记
pub struct ClientStats {
    pub target_marker: Option<Target>,
    ...
}

// 使用目标标记计算距离
if let Some(marker) = client_stats.target_marker {
    let distance = image.get_target_marker_distance(marker);
}
```

### 设计优势

1. **类型安全**：
   - 使用枚举区分不同类型的目标
   - 编译时检查，避免运行时错误

2. **灵活性**：
   - 可以轻松添加新的怪物类型
   - 支持不同的目标类别

3. **精确定位**：
   - 包含完整的边界信息
   - 提供准确的攻击坐标计算

4. **轻量级**：
   - 实现了 Copy trait
   - 可以高效地传递和复制

### 扩展可能性

虽然当前实现已经很完善，但可以考虑以下扩展：

1. **更多怪物类型**：
   - Boss 怪物
   - 精英怪物
   - 友方单位

2. **优先级字段**：
   - 为不同类型的怪物分配优先级值
   - 支持更复杂的目标选择策略

3. **状态信息**：
   - 怪物的 Buff/Debuff 状态
   - 怪物的血量范围估计

4. **距离缓存**：
   - 缓存计算的距离值
   - 避免重复计算

---

## 总结

### 模块关系图

```
Point (基础坐标)
  ↓
Bounds (矩形区域) ← PointCloud (点集合)
  ↓                      ↓
Target (目标)      StatusBarConfig (配置)
  ↓                      ↓
ClientStats ←―――― StatInfo (状态值)
```

### 数据流

```
图像分析
  ↓
像素检测 (PixelDetectionConfig)
  ↓
点云收集 (PointCloud)
  ↓
边界计算 (Bounds)
  ↓
目标识别 (Target) / 状态计算 (StatInfo)
  ↓
客户端状态 (ClientStats)
  ↓
行为决策 (Behavior)
```

### 核心特点

1. **分层设计**：
   - 底层：Point（基础坐标）
   - 中层：Bounds、PointCloud（几何计算）
   - 高层：Target、ClientStats（游戏逻辑）

2. **类型安全**：
   - 使用枚举区分不同类型
   - 编译时类型检查
   - 避免无效状态

3. **高效计算**：
   - 大量使用 Copy trait
   - 避免不必要的堆分配
   - 内联关键方法

4. **灵活扩展**：
   - 清晰的接口定义
   - 易于添加新类型
   - 支持自定义配置

5. **完善的序列化**：
   - 支持 Serialize/Deserialize
   - 支持日志输出
   - 便于调试和配置

### 使用建议

1. **使用 Point 表示所有坐标**
2. **使用 Bounds 表示所有矩形区域**
3. **使用 PointCloud 进行像素处理和聚类**
4. **使用 Target 表示游戏中的目标**
5. **使用 ClientStats 作为状态信息的唯一来源**
6. **充分利用类型系统防止错误**
