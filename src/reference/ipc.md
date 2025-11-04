# IPC 进程间通信模块

本模块定义了前端（UI）和后端（机器人逻辑）之间的通信接口，包括配置管理和状态同步。

---

## bot_config.rs - 机器人配置

### 概述
定义了机器人的完整配置体系，包括技能栏配置、三种行为模式的配置（Farming、Support、Shout），以及配置的序列化/反序列化功能。

### 核心枚举

#### SlotType - 技能槽位类型
```rust
pub enum SlotType {
    Unused,           // 未使用
    Food,             // 食物
    Pill,             // 药丸
    HealSkill,        // 治疗技能
    AOEHealSkill,     // 群体治疗技能
    AOEAttackSkill,   // AOE 攻击技能
    MpRestorer,       // 魔法恢复
    FpRestorer,       // 疲劳恢复
    PickupPet,        // 拾取宠物
    PickupMotion,     // 拾取动作
    AttackSkill,      // 攻击技能
    BuffSkill,        // Buff 技能
    RezSkill,         // 复活技能
    Flying,           // 飞行
    PartySkill,       // 队伍技能
}
```

**特性**：
- 实现了 `Display` trait，支持友好的字符串输出
- 支持序列化/反序列化（Serde）
- 可用于配置不同类型的技能和道具

#### BotMode - 机器人模式
```rust
pub enum BotMode {
    Farming,     // 挂机打怪模式
    Support,     // 辅助模式
    AutoShout,   // 自动喊话模式
}
```

**ToString 实现**：
- Farming → "farming"
- Support → "support"
- AutoShout → "auto_shout"

### 核心结构

#### Slot - 技能槽位
```rust
pub struct Slot {
    slot_type: SlotType,           // 槽位类型
    slot_cooldown: Option<u32>,    // 冷却时间（毫秒）
    slot_threshold: Option<u32>,   // 触发阈值（HP/MP/FP 百分比）
    slot_enabled: bool,            // 是否启用
}
```

**默认值**：
- slot_type: Unused
- slot_cooldown: None
- slot_threshold: None
- slot_enabled: true

**方法**：

##### get_slot_cooldown()
获取槽位冷却时间。
```rust
// 如果有配置冷却时间，返回配置值
// 否则返回默认值 100ms
cooldown.unwrap_or(Some(100))
```

#### SlotBar - 技能栏
```rust
pub struct SlotBar {
    slots: Option<[Slot; 10]>,  // 10 个技能槽位
}
```

**默认值**：10 个空槽位（Unused）

**方法**：

##### 1. slots()
返回技能栏中的所有槽位。
```rust
Vec<Slot>
```

##### 2. get_slot_index(slot_type)
获取第一个匹配类型的槽位索引。
```rust
// 查找第一个匹配的槽位
slots.iter().position(|slot| slot.slot_type == slot_type)
```

**返回**：`Option<usize>`

##### 3. get_usable_slot_index(slot_type, threshold, last_slots_usage, slot_bar_index)
获取可用的匹配槽位（核心方法）。

**参数**：
- `slot_type`：要查找的槽位类型
- `threshold`：当前状态值（如 HP=50）
- `last_slots_usage`：冷却时间记录表
- `slot_bar_index`：技能栏索引

**过滤条件**：
```rust
slot.slot_type == slot_type                          // 类型匹配
&& slot.slot_enabled                                 // 已启用
&& slot.slot_threshold >= threshold                  // 阈值满足
&& last_slots_usage[slot_bar_index][index].is_none() // 冷却完成
```

**选择策略**：
```rust
// 选择阈值最小的槽位（优先使用低阈值技能）
.min_by(|x, y| x.1.slot_threshold.cmp(&y.1.slot_threshold))
```

**返回**：`Option<(usize, usize)>` - (技能栏索引, 槽位索引)

**示例**：
```rust
// HP = 60 时查找治疗技能
// Slot 1: HealSkill, threshold=50  ✓ (60 >= 50)
// Slot 2: HealSkill, threshold=70  ✗ (60 < 70)
// Slot 3: HealSkill, threshold=30  ✓ (60 >= 30)
//
// 选择 Slot 3（阈值最小）
```

##### 4. get_all_usable_slots_for_index(slot_type, slot_bar_index, last_slots_usage)
获取技能栏中所有可用的匹配槽位。

**用途**：用于 PartySkill（需要同时释放多个技能）

**返回**：`Vec<(usize, usize)>` - 所有匹配槽位的索引列表

---

### FarmingConfig - 挂机配置

#### 结构定义
```rust
pub struct FarmingConfig {
    // 技能配置
    slot_bars: Option<[SlotBar; 9]>,  // 9 个技能栏

    // 移动配置
    circle_pattern_rotation_duration: Option<u64>,  // 圆形移动旋转时间

    // 战斗配置
    farming_enabled: Option<bool>,              // 是否启用挂机
    prevent_already_attacked: Option<bool>,     // 防止攻击已被攻击的怪
    prioritize_aggro: Option<bool>,             // 优先攻击主动怪
    is_stop_fighting: Option<bool>,             // 停止战斗
    min_hp_attack: Option<u32>,                 // 攻击被动怪的最低 HP
    aoe_farming: Option<u32>,                   // AOE 挂机并发数

    // 怪物识别配置
    passive_mobs_colors: Option<[Option<u8>; 3]>,    // 被动怪颜色 RGB
    passive_tolerence: Option<u8>,                    // 被动怪容差
    aggressive_mobs_colors: Option<[Option<u8>; 3]>, // 主动怪颜色 RGB
    aggressive_tolerence: Option<u8>,                 // 主动怪容差
    violet_mobs_colors: Option<[Option<u8>; 3]>,     // 紫怪颜色 RGB
    violet_tolerence: Option<u8>,                     // 紫怪容差

    min_mobs_name_width: Option<u32>,           // 怪物名称最小宽度
    max_mobs_name_width: Option<u32>,           // 怪物名称最大宽度

    // 障碍物规避
    obstacle_avoidance_cooldown: Option<u64>,   // 障碍物规避冷却时间
    obstacle_avoidance_max_try: Option<u32>,    // 最大规避尝试次数

    // 超时和断线
    mobs_timeout: Option<u64>,                  // 找不到怪物的超时时间
    on_death_disconnect: Option<bool>,          // 死亡时断线
    on_afk_disconnect: Option<bool>,            // AFK 时断线
    afk_timeout: Option<u64>,                   // AFK 超时时间

    // 其他
    interval_between_buffs: Option<u64>,        // Buff 间隔时间
}
```

#### 方法（所有方法都有默认值）

##### 技能栏管理
```rust
slot_bars() -> Vec<SlotBar>
    // 返回 9 个技能栏，未配置则返回默认值

slots(slot_bar_index) -> Vec<Slot>
    // 返回指定技能栏的所有槽位

get_slot_cooldown(slot_bar_index, slot_index) -> Option<u32>
    // 获取指定槽位的冷却时间

slot_index(slot_type) -> Option<(usize, usize)>
    // 查找第一个匹配类型的槽位
    // 遍历所有 9 个技能栏

get_usable_slot_index(slot_type, threshold, last_slots_usage) -> Option<(usize, usize)>
    // 查找可用的槽位
    // 考虑冷却时间和阈值

get_all_usable_slot_for_type(slot_type, last_slots_usage) -> Vec<(usize, usize)>
    // 获取所有可用的指定类型槽位
    // 用于 PartySkill
```

##### 移动配置
```rust
circle_pattern_rotation_duration() -> u64
    // 默认: 30
    // 圆形移动的旋转时间
```

##### 战斗配置
```rust
is_stop_fighting() -> bool
    // 默认: false
    // 是否停止战斗

prevent_already_attacked() -> bool
    // 默认: true
    // 防止攻击已被攻击的怪物

prioritize_aggro() -> bool
    // 默认: true
    // 优先攻击主动怪

min_hp_attack() -> u32
    // 默认: 0
    // 攻击被动怪的最低 HP 要求

max_aoe_farming() -> u32
    // 默认: 1
    // AOE 挂机并发目标数
```

##### 怪物识别配置
```rust
passive_mobs_colors() -> [Option<u8>; 3]
    // 默认: [None, None, None]
    // 被动怪物名称颜色

passive_tolerence() -> u8
    // 默认: 5
    // 被动怪颜色容差

aggressive_mobs_colors() -> [Option<u8>; 3]
    // 默认: [None, None, None]
    // 主动怪物名称颜色

aggressive_tolerence() -> u8
    // 默认: 10
    // 主动怪颜色容差

violet_mobs_colors() -> [Option<u8>; 3]
    // 默认: [None, None, None]
    // 紫色怪物名称颜色

violet_tolerence() -> u8
    // 默认: 10
    // 紫色怪颜色容差

min_mobs_name_width() -> u32
    // 默认: 11
    // 怪物名称最小宽度（过滤噪点）

max_mobs_name_width() -> u32
    // 默认: 180
    // 怪物名称最大宽度（过滤非怪物）
```

##### 障碍物规避
```rust
obstacle_avoidance_cooldown() -> u128
    // 默认: 5000
    // 触发障碍物规避的冷却时间（毫秒）

obstacle_avoidance_max_try() -> u32
    // 默认: 5
    // 最大障碍物规避尝试次数
```

##### 超时配置
```rust
mobs_timeout() -> u128
    // 默认: 0 (禁用)
    // 找不到怪物的超时时间（毫秒）
    // 超时后退出程序

interval_between_buffs() -> u128
    // 默认: 2000
    // Buff 之间的间隔时间（毫秒）
```

##### 断线配置
```rust
on_death_disconnect() -> bool
    // 默认: true
    // 角色死亡时是否断线

on_afk_disconnect() -> bool
    // 默认: false
    // AFK 时是否断线

afk_timeout() -> u128
    // 默认: 3000
    // AFK 超时时间（毫秒）
```

---

### SupportConfig - 辅助配置

#### 结构定义
```rust
pub struct SupportConfig {
    slot_bars: Option<[SlotBar; 9]>,            // 9 个技能栏
    obstacle_avoidance_cooldown: Option<u64>,   // 障碍物规避冷却
    on_death_disconnect: Option<bool>,          // 死亡断线
    on_afk_disconnect: Option<bool>,            // AFK 断线
    is_in_party: Option<bool>,                  // 是否在队伍中
    afk_timeout: Option<u64>,                   // AFK 超时
    interval_between_buffs: Option<u64>,        // Buff 间隔
    max_main_distance: Option<u32>,             // 最大跟随距离
}
```

#### 方法

##### 技能栏管理
```rust
slot_bars() -> Vec<SlotBar>
slots(slot_bar_index) -> Vec<Slot>
get_slot_cooldown(slot_bar_index, slot_index) -> Option<u32>
get_usable_slot_index(slot_type, threshold, last_slots_usage) -> Option<(usize, usize)>
get_all_usable_slot_for_type(slot_type, last_slots_usage) -> Vec<(usize, usize)>
```
（与 FarmingConfig 相同）

##### 辅助配置
```rust
is_in_party() -> bool
    // 默认: false
    // 是否在队伍中
    // 影响 Buff 和治疗行为

get_max_main_distance() -> u32
    // 默认: 100
    // 最大跟随距离
```

##### 障碍物规避
```rust
obstacle_avoidance_cooldown() -> u128
    // 默认: 0 (禁用)
    // Support 模式一般不需要障碍物规避
```

##### Buff 间隔
```rust
interval_between_buffs() -> u128
    // 默认: 2000
    // Buff 之间的间隔时间（毫秒）
```

##### 断线配置
```rust
on_death_disconnect() -> bool
    // 默认: true

on_afk_disconnect() -> bool
    // 默认: false

afk_timeout() -> u128
    // 默认: 3000
```

---

### ShoutConfig - 喊话配置

#### 结构定义
```rust
pub struct ShoutConfig {
    shout_interval: Option<u64>,       // 喊话间隔
    shout_messages: Option<Vec<String>>, // 喊话消息列表
    on_afk_disconnect: Option<bool>,   // AFK 断线
    afk_timeout: Option<u64>,          // AFK 超时
}
```

#### 方法

```rust
shout_interval() -> u64
    // 默认: 30000 (30 秒)
    // 喊话间隔时间（毫秒）

shout_messages() -> Vec<String>
    // 默认: 空列表
    // 要喊话的消息列表

on_afk_disconnect() -> bool
    // 默认: false

afk_timeout() -> u128
    // 默认: 3000
```

---

### BotConfig - 机器人总配置

#### 结构定义
```rust
pub struct BotConfig {
    change_id: u64,                     // 配置变更 ID
    is_running: bool,                   // 是否运行中
    mode: Option<BotMode>,              // 机器人模式
    farming_config: FarmingConfig,      // 挂机配置
    support_config: SupportConfig,      // 辅助配置
    shout_config: ShoutConfig,          // 喊话配置
}
```

#### 核心方法

##### 状态管理
```rust
toggle_active(&mut self)
    // 切换运行状态
    is_running = !is_running

is_running() -> bool
    // 获取运行状态

mode() -> Option<BotMode>
    // 获取当前模式
```

##### 变更追踪
```rust
change_id() -> u64
    // 获取当前变更 ID
    // 用于前后端同步

changed(self) -> Self
    // 增加变更 ID
    change_id += 1
    // 返回新的配置实例
```

**用途**：前端检测配置是否变化。

##### 配置访问
```rust
farming_config() -> &FarmingConfig
    // 获取挂机配置引用

support_config() -> &SupportConfig
    // 获取辅助配置引用

shout_config() -> &ShoutConfig
    // 获取喊话配置引用
```

##### 持久化

###### serialize(path)
保存配置到文件。

**特点**：
```rust
// 保存时强制设置 is_running = false
// 避免启动时自动运行
let config = {
    let mut config = self.clone();
    config.is_running = false;
    config
};

// 序列化为 JSON
serde_json::to_writer(&mut file, &config)
```

###### deserialize_or_default(path)
从文件加载配置。

**特点**：
```rust
// 如果文件不存在或解析失败，返回默认配置
if let Ok(file) = File::open(path) {
    serde_json::from_reader(&file).unwrap_or_default()
} else {
    Self::default()
}
```

---

### 使用场景

#### 1. 技能槽位查找

```rust
// 查找治疗技能
if let Some((bar, slot)) = config.get_usable_slot_index(
    SlotType::HealSkill,
    Some(hp_value),
    last_slots_usage
) {
    send_slot(bar, slot);
}
```

#### 2. 批量技能释放

```rust
// 释放所有队伍技能
let party_skills = config.get_all_usable_slot_for_type(
    SlotType::PartySkill,
    last_slots_usage
);

for (bar, slot) in party_skills {
    send_slot(bar, slot);
}
```

#### 3. 怪物识别配置

```rust
// 获取被动怪物颜色配置
let colors = config.passive_mobs_colors();  // [R, G, B]
let tolerance = config.passive_tolerence(); // 容差值

// 在图像分析器中使用
image.identify_mobs_by_color(colors, tolerance)
```

#### 4. 配置持久化

```rust
// 保存配置
config.serialize("config.json");

// 加载配置
let config = BotConfig::deserialize_or_default("config.json");
```

#### 5. 模式切换

```rust
// 检查当前模式
match config.mode() {
    Some(BotMode::Farming) => {
        let farming_cfg = config.farming_config();
        // 使用挂机配置
    }
    Some(BotMode::Support) => {
        let support_cfg = config.support_config();
        // 使用辅助配置
    }
    Some(BotMode::AutoShout) => {
        let shout_cfg = config.shout_config();
        // 使用喊话配置
    }
    None => {}
}
```

---

### 设计特点

#### 1. Option 模式
所有配置字段都是 `Option<T>` 类型，提供默认值：
- 灵活配置：只配置需要的项
- 向后兼容：新增字段不影响旧配置
- 默认值合理：开箱即用

#### 2. 9 个技能栏
```rust
slot_bars: Option<[SlotBar; 9]>
```
- 每个技能栏 10 个槽位
- 总共 90 个技能槽位
- 支持复杂的技能配置

#### 3. 阈值系统
```rust
slot_threshold: Option<u32>
```
- 控制技能触发时机
- 优先使用低阈值技能
- 避免资源浪费

**示例**：
```
Slot 1: HealSkill, threshold=30  // HP < 30 时使用
Slot 2: HealSkill, threshold=50  // HP < 50 时使用
Slot 3: Pill,      threshold=70  // HP < 70 时使用

当 HP = 45 时：
- 检查 Slot 1: 45 >= 30 ✓
- 检查 Slot 2: 45 >= 50 ✗
- 检查 Slot 3: 45 >= 70 ✗
- 使用 Slot 1 (阈值最小)
```

#### 4. 冷却管理集成
```rust
get_usable_slot_index(..., last_slots_usage)
```
- 配置提供冷却时间
- 行为记录使用时间
- 查询时自动过滤冷却中的技能

#### 5. 类型安全
使用枚举而非字符串：
- 编译时检查
- IDE 自动补全
- 避免拼写错误

---

## frontend_info.rs - 前端信息

### 概述
定义了后端向前端同步的状态信息，用于 UI 显示和用户反馈。这是一个单向数据流：后端 → 前端。

### 核心结构

#### FrontendInfo
```rust
pub struct FrontendInfo {
    // 击杀统计
    enemy_kill_count: u32,         // 击杀数
    kill_min_avg: f32,             // 每分钟击杀数
    kill_hour_avg: f32,            // 每小时击杀数

    // 时间统计
    last_fight_duration: u64,      // 上次战斗时长（毫秒）
    last_search_duration: u64,     // 上次搜索时长（毫秒）

    // 状态标志
    is_attacking: bool,            // 是否正在攻击
    is_running: bool,              // 是否运行中
    is_alive: bool,                // 是否存活
    afk_ready_to_disconnect: bool, // AFK 准备断线
}
```

**已注释字段**：
```rust
// enemy_bounds: Option<Vec<Bounds>>,        // 敌人边界列表
// active_enemy_bounds: Option<Bounds>,      // 当前攻击的敌人边界
```
这些字段可能在早期版本用于 UI 可视化，现已废弃。

### 方法

#### 击杀统计

##### set_kill_count(enemy_kill_count)
设置总击杀数。

```rust
self.enemy_kill_count = enemy_kill_count;
```

**调用时机**：每次击杀怪物后。

##### set_kill_stats(last_kill_avg, action_duration)
设置击杀效率统计。

**参数**：
- `last_kill_avg: (f32, f32)`：
  - `0`：每分钟击杀数
  - `1`：每小时击杀数
- `action_duration: (u128, u128)`：
  - `0`：搜索时长（毫秒）
  - `1`：战斗时长（毫秒）

**实现**：
```rust
self.kill_min_avg = last_kill_avg.0;
self.kill_hour_avg = last_kill_avg.1;

self.last_search_duration = action_duration.0 as u64;
self.last_fight_duration = action_duration.1 as u64;
```

**调用时机**：每次击杀怪物后，在 `after_enemy_kill` 方法中计算并更新。

#### 状态标志

##### set_is_attacking(is_attacking)
设置是否正在攻击。

```rust
self.is_attacking = is_attacking;
```

**调用时机**：
- 进入攻击状态：`State::Attacking`
- 退出攻击状态：目标死亡或失去目标

##### set_is_running(is_running)
设置机器人是否运行中。

```rust
self.is_running = is_running;
```

**调用时机**：
- 启动机器人：`bot.start()`
- 停止机器人：`bot.stop()`

##### set_is_alive(is_alive)
设置角色是否存活。

```rust
self.is_alive = is_alive;
```

**调用时机**：每次更新游戏状态后。

##### is_alive()
获取角色存活状态。

```rust
self.is_alive
```

#### AFK 管理

##### set_afk_ready_to_disconnect(afk_ready_to_disconnect)
设置 AFK 准备断线标志。

```rust
self.afk_ready_to_disconnect = afk_ready_to_disconnect;
```

**用途**：通知前端即将因 AFK 断线。

##### is_afk_ready_to_disconnect()
获取 AFK 准备断线状态。

```rust
self.afk_ready_to_disconnect
```

#### 持久化

##### deserialize_or_default()
创建默认实例。

```rust
Self::default()
```

**注意**：没有 `serialize` 方法，因为 FrontendInfo 是运行时状态，不需要持久化。

### 已注释的方法

#### set_enemy_bounds(enemy_bounds)
设置敌人边界列表（已注释）。

```rust
// self.enemy_bounds = Some(enemy_bounds);
```

#### set_active_enemy_bounds(active_enemy_bounds)
设置当前攻击的敌人边界（已注释）。

```rust
// self.active_enemy_bounds = Some(active_enemy_bounds);
```

**推测原因**：
- UI 可视化功能已移除
- 性能优化（减少数据传输）
- 简化前端逻辑

---

### 使用场景

#### 1. 更新击杀统计

```rust
// 在 farming_behavior 的 after_enemy_kill 方法中
self.kill_count += 1;
frontend_info.set_kill_count(self.kill_count);

// 计算效率
let kill_per_minute = 60.0 / (search_time + kill_time);
let kill_per_hour = kill_per_minute * 60.0;

frontend_info.set_kill_stats(
    (kill_per_minute, kill_per_hour),
    (search_duration, kill_duration)
);
```

#### 2. 更新攻击状态

```rust
// 进入攻击状态
self.is_attacking = true;
frontend_info.set_is_attacking(true);

// 退出攻击状态
self.is_attacking = false;
frontend_info.set_is_attacking(false);
```

#### 3. 检查存活状态

```rust
// 检测角色死亡
if !frontend_info.is_alive() {
    if config.on_death_disconnect() {
        disconnect();
    }
}
```

#### 4. AFK 检测

```rust
// 检测 AFK 超时
if last_action_time.elapsed() > config.afk_timeout() {
    frontend_info.set_afk_ready_to_disconnect(true);

    if config.on_afk_disconnect() {
        disconnect();
    }
}
```

#### 5. 前端显示

```rust
// 前端可以定期获取 FrontendInfo 显示：
UI {
    击杀数: {frontend_info.enemy_kill_count}
    效率: {frontend_info.kill_min_avg}/分钟
    上次战斗: {frontend_info.last_fight_duration}ms
    状态: {frontend_info.is_attacking ? "攻击中" : "搜索中"}
}
```

---

### 数据流

```
后端行为逻辑
    ↓
更新 FrontendInfo
    ↓
IPC 通道
    ↓
前端 UI
    ↓
显示给用户
```

### 设计特点

#### 1. 只读接口
FrontendInfo 只提供 setter 方法（除了 `is_alive`），前端只能读取，不能修改。

#### 2. 单向数据流
- 后端 → 前端：状态同步
- 前端 → 后端：通过 BotConfig

#### 3. 实时统计
- 每次击杀后立即更新
- 实时计算效率指标
- 提供准确的时间统计

#### 4. 轻量级
- 只包含必要的显示信息
- 不包含复杂的嵌套结构
- 易于序列化和传输

#### 5. 可扩展性
虽然目前字段不多，但可以轻松添加：
- 更多统计数据
- 更多状态标志
- 错误信息
- 日志信息

---

## 模块关系图

```
BotConfig (总配置)
  ├── FarmingConfig (挂机配置)
  │     ├── SlotBar[9] (9 个技能栏)
  │     │     └── Slot[10] (每栏 10 个槽位)
  │     │           ├── SlotType (槽位类型)
  │     │           ├── slot_cooldown (冷却)
  │     │           ├── slot_threshold (阈值)
  │     │           └── slot_enabled (启用)
  │     └── 怪物识别配置
  │
  ├── SupportConfig (辅助配置)
  │     ├── SlotBar[9]
  │     └── 跟随配置
  │
  └── ShoutConfig (喊话配置)
        └── 消息列表

FrontendInfo (前端信息)
  ├── 击杀统计
  ├── 效率统计
  └── 状态标志
```

## 数据流向

```
用户配置 (UI)
    ↓
BotConfig
    ↓
行为逻辑 (Behavior)
    ↓
FrontendInfo
    ↓
UI 显示
```

## 配置示例

### 完整的挂机配置示例

```json
{
  "mode": "Farming",
  "farming_config": {
    "slot_bars": [
      {
        "slots": [
          {
            "slot_type": "Pill",
            "slot_cooldown": 1000,
            "slot_threshold": 70,
            "slot_enabled": true
          },
          {
            "slot_type": "HealSkill",
            "slot_cooldown": 2000,
            "slot_threshold": 50,
            "slot_enabled": true
          },
          {
            "slot_type": "AttackSkill",
            "slot_cooldown": 500,
            "slot_threshold": null,
            "slot_enabled": true
          },
          {
            "slot_type": "BuffSkill",
            "slot_cooldown": 30000,
            "slot_threshold": null,
            "slot_enabled": true
          }
        ]
      }
    ],
    "circle_pattern_rotation_duration": 30,
    "prioritize_aggro": true,
    "min_hp_attack": 70,
    "passive_mobs_colors": [255, 255, 0],
    "passive_tolerence": 5,
    "aggressive_mobs_colors": [255, 0, 0],
    "aggressive_tolerence": 10,
    "obstacle_avoidance_cooldown": 5000,
    "obstacle_avoidance_max_try": 5,
    "on_death_disconnect": true
  }
}
```

## 总结

### IPC 模块的作用

1. **配置管理**：
   - 提供完整的机器人配置体系
   - 支持三种行为模式
   - 灵活的技能槽位配置

2. **状态同步**：
   - 后端状态实时同步到前端
   - 用户可见的运行状态
   - 详细的统计信息

3. **持久化**：
   - 配置保存/加载
   - JSON 格式，易于编辑
   - 默认值保证兼容性

4. **类型安全**：
   - 强类型定义
   - 枚举避免错误
   - 编译时检查

### 核心优势

1. **解耦设计**：
   - 配置与逻辑分离
   - 前端与后端分离
   - 易于维护和扩展

2. **灵活配置**：
   - Option 模式提供默认值
   - 90 个技能槽位支持复杂配置
   - 阈值系统精细控制

3. **实时反馈**：
   - 击杀统计
   - 效率计算
   - 状态显示

4. **容错性好**：
   - 所有字段都有默认值
   - 加载失败不崩溃
   - 向后兼容

### 使用建议

1. **配置管理**：
   - 使用 BotConfig 作为唯一配置源
   - 通过 mode() 区分不同模式
   - 定期保存配置

2. **技能配置**：
   - 合理设置 threshold 避免资源浪费
   - 配置 cooldown 防止频繁使用
   - 启用/禁用控制技能开关

3. **状态更新**：
   - 及时更新 FrontendInfo
   - 提供清晰的用户反馈
   - 记录关键统计数据

4. **扩展性**：
   - 新增配置字段使用 Option
   - 提供合理的默认值
   - 保持向后兼容
