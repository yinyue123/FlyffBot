# Support Behavior 辅助职业逻辑

## 概述

support_behavior.rs 实现了一个完整的辅助职业（如牧师、治疗师）自动化系统，主要功能包括：
- 自动跟随队长
- 为自己和队友加 Buff
- 自动治疗自己和目标
- 复活倒地的队友
- 智能距离检测和移动
- 防机器人检测的随机动作

## 结构设计

### SupportBehavior 结构体

```rust
pub struct SupportBehavior<'a> {
    logger: &'a Logger,
    movement: &'a MovementAccessor,
    window: &'a Window,
    self_buff_usage_last_time: [[Option<Instant>; 10]; 9],  // 自我 Buff 冷却记录
    slots_usage_last_time: [[Option<Instant>; 10]; 9],      // 目标技能冷却记录
    last_jump_time: Instant,                                 // 上次跳跃时间
    avoid_obstacle_direction: String,                        // 障碍物规避方向
    is_waiting_for_revive: bool,                            // 是否正在等待复活
    last_far_from_target: Option<Instant>,                  // 上次远离目标的时间
    last_target_distance: Option<i32>,                       // 上次目标距离
    wait_duration: Option<Duration>,                         // 等待持续时间
    wait_start: Instant,                                     // 等待开始时间
    has_target: bool,                                        // 是否有目标
    self_buffing: bool,                                      // 是否正在给自己加 Buff
    target_buffing: bool,                                    // 是否正在给目标加 Buff
    buff_counter: u32,                                       // Buff 计数器
}
```

### 重要常量

```rust
const HEAL_SKILL_CAST_TIME: u64 = 2000;   // 治疗技能施法时间（毫秒）
const BUFF_CAST_TIME: u64 = 2500;         // Buff 技能施法时间（毫秒）
const AOE_SKILL_CAST_TIME: u64 = 100;     // AOE 技能施法时间（毫秒）
```

## 主循环逻辑 (run_iteration)

### 执行流程

```
1. 更新技能冷却状态
   ↓
2. 检测目标状态
   ↓
3. 使用队伍技能
   ↓
4. 检查自我恢复
   ↓
5. 无目标时选择队长
   ↓
6. 跟随目标
   ↓
7. 复活目标（如果倒地）
   ↓
8. 检查目标恢复
   ↓
9. 距离检测和移动
   ↓
10. 随机摄像头移动
   ↓
11. 给自己加 Buff（队伍模式）
   ↓
12. 给目标加 Buff
```

### 详细步骤

#### 1. 更新技能冷却状态
```rust
self.update_slots_usage(config);
```
- 更新 `slots_usage_last_time`（目标技能）
- 更新 `self_buff_usage_last_time`（自我 Buff）
- 冷却完成后自动重置为 None

#### 2. 检测目标状态
```rust
self.has_target = image.client_stats.target_is_mover;
```
- 检查是否选中了可移动单位（玩家/怪物）
- 影响后续所有操作的执行

#### 3. 使用队伍技能
```rust
self.use_party_skills(config);
```
- 自动释放所有配置的 PartySkill
- 不需要目标，直接释放

#### 4. 检查自我恢复
```rust
self.check_self_restorations(config, image);
```
- 检查并恢复自己的 HP/MP/FP
- 优先级：Pill → HealSkill → AOEHealSkill → Food

#### 5. 无目标时选择队长
```rust
if self.has_target == false {
    if config.is_in_party() {
        self.select_party_leader(config);
    }
    return;
}
```
- 如果没有目标且在队伍中，自动选择队长
- 然后结束本次迭代

#### 6-12. 有目标时的处理
见下文详细说明。

## 核心功能模块

### 1. 目标选择：select_party_leader

自动选择队长作为跟随和辅助目标：

```rust
fn select_party_leader(&mut self, _config: &SupportConfig) {
    // 1. 打开队伍菜单
    PressKey("P")

    // 2. 点击队长位置
    eval_simple_click(window, Point::new(213, 440))

    // 3. 按 Z 键选中队长
    PressKey("Z")

    // 4. 关闭队伍菜单
    PressKey("P")
}
```

**操作步骤**：
1. 按 P 打开队伍窗口
2. 等待 150ms
3. 点击坐标 (213, 440) - 队长的位置
4. 按 Z 键选中
5. 等待 10ms
6. 按 P 关闭队伍窗口
7. 等待 500ms

### 2. 跟随目标：follow_target

自动跟随当前选中的目标：

```rust
fn follow_target(&mut self) {
    if self.has_target {
        play!(self.movement => [
            PressKey("Z"),
        ]);
    }
}
```

- 简单地按 Z 键激活跟随
- 游戏会自动跟随选中的目标

### 3. 复活目标：rez_target

检测并复活倒地的队友：

```rust
fn rez_target(&mut self, config: &SupportConfig, image: &mut ImageAnalyzer) -> bool {
    if image.client_stats.target_is_mover && image.client_stats.target_is_alive == false {
        self.get_slot_for(config, None, SlotType::RezSkill, true, None);
        return true;
    }
    return false;
}
```

**复活逻辑**：
1. 检测目标是否为可移动单位且已死亡
2. 如果是，使用复活技能（RezSkill）
3. 设置 `is_waiting_for_revive = true`
4. 重置所有技能冷却时间
5. 等待目标复活（HP > 0）
6. 复活后设置 `is_waiting_for_revive = false`

### 4. 距离检测：is_target_in_range

智能检测目标距离并自动移动：

```rust
fn is_target_in_range(&mut self, config: &SupportConfig, image: &mut ImageAnalyzer) -> bool
```

**距离判断逻辑**：

#### 情况 1：无法获取距离（9999）
```rust
if distance == 9999 {
    self.move_circle_pattern();  // 圆形移动尝试靠近
    return false;
}
```

#### 情况 2：超出最大距离
```rust
if distance > get_max_main_distance() {
    // 如果距离超过最大距离的 2 倍
    if distance > get_max_main_distance() * 2 {
        self.move_circle_pattern();  // 圆形移动
    } else {
        // 如果持续 3 秒且距离还在增加
        if last_far_from_target.elapsed() > 3000 && last_distance < distance {
            self.move_circle_pattern();  // 圆形移动
        }
    }
    self.follow_target();  // 激活跟随
    return false;
}
```

#### 情况 3：在距离内
```rust
if distance <= get_max_main_distance() {
    self.last_far_from_target = None;  // 重置远离状态
    return true;
}
```

### 5. 圆形移动：move_circle_pattern

尝试绕过障碍物或靠近目标：

```rust
fn move_circle_pattern(&mut self) {
    // 按住 W + Space + (A/D) 前进跳跃
    HoldKeys(vec!["W", "Space", &self.avoid_obstacle_direction])
    Wait(100ms)

    // 释放方向键
    ReleaseKey(&self.avoid_obstacle_direction)
    Wait(500ms)

    // 释放前进和跳跃
    ReleaseKeys(vec!["Space", "W"])

    // 按 Z 重新激活跟随
    PressKey("Z")

    // 切换方向（A ↔ D）
    self.avoid_obstacle_direction = if direction == "D" { "A" } else { "D" }
}
```

**特点**：
- 交替使用 A 和 D 键，避免总是朝一个方向移动
- 跳跃 + 移动可以跨越小障碍物
- 自动重新激活跟随

### 6. 自我恢复：check_self_restorations

检查并恢复自己的状态：

```rust
fn check_self_restorations(&mut self, config: &SupportConfig, image: &mut ImageAnalyzer)
```

**HP 恢复优先级**：
1. **Pill**（药丸）- 最高优先级，立即使用
2. **HealSkill**（治疗技能）：
   - 如果在队伍中，需要先取消目标（lose_target）
   - 使用后标记为自我 Buff
3. **AOEHealSkill**（群体治疗）：
   - 连续使用 3 次
   - 每次间隔 100ms
4. **Food**（食物）- 最低优先级

**MP/FP 恢复**：
- **MpRestorer**：恢复魔法值
- **FpRestorer**：恢复疲劳值

**特殊处理**：
- 使用 HealSkill 治疗自己时，如果在队伍中会先取消目标
- 这样可以将治疗施放在自己身上，然后重新选择队长

### 7. 目标恢复：check_target_restorations

检查并恢复目标（队友）的状态：

```rust
fn check_target_restorations(&mut self, config: &SupportConfig, image: &mut ImageAnalyzer)
```

**治疗目标优先级**：
1. **HealSkill**（单体治疗）- 优先使用
2. **AOEHealSkill**（群体治疗）：
   - 连续使用 3 次
   - 每次间隔 100ms

**与自我恢复的区别**：
- 不使用 Pill 和 Food（只对自己有效）
- 不需要取消目标
- 直接对当前目标施放

### 8. Buff 系统

#### 8.1 Buff 类型

系统维护两套独立的 Buff 冷却系统：

1. **自我 Buff**（self_buff_usage_last_time）：
   - 给自己加的 Buff
   - 在队伍模式下使用
   - 需要先取消目标（lose_target）

2. **目标 Buff**（slots_usage_last_time）：
   - 给当前目标加的 Buff
   - 直接对目标施放

#### 8.2 send_buff 方法

```rust
fn send_buff(
    &mut self,
    config: &SupportConfig,
    buff: Option<(usize, usize)>,
    is_self_buff: bool
)
```

**Buff 流程**：

##### 开始 Buff 阶段
```rust
if buff.is_some() {
    if is_self_buff {
        if !self.self_buffing {
            self.self_buffing = true;
            self.buff_counter = 0;
            self.lose_target();  // 取消目标
        }
    } else {
        if !self.target_buffing {
            self.target_buffing = true;
            self.buff_counter = 0;
        }
    }

    // 发送 Buff 技能
    self.send_slot(slot, is_self_buff);
    self.buff_counter += 1;
    self.wait(BUFF_CAST_TIME);  // 等待 2500ms
}
```

##### 结束 Buff 阶段
```rust
else {
    if is_self_buff && self.self_buffing {
        self.self_buffing = false;
        self.select_party_leader(config);  // 重新选择队长
    }
    if !is_self_buff && self.target_buffing {
        self.target_buffing = false;
    }
}
```

#### 8.3 Buff 执行顺序

在主循环中：

```rust
// 1. 如果在队伍中，先给自己加 Buff
if config.is_in_party() {
    let self_buff = self.get_slot_for(
        config,
        None,
        SlotType::BuffSkill,
        false,
        Some(self.self_buff_usage_last_time)  // 使用自我 Buff 冷却表
    );
    self.send_buff(config, self_buff, true);
}

// 2. 如果不在自我 Buff 状态且目标存活，给目标加 Buff
let target_buff = self.get_slot_for(
    config,
    None,
    SlotType::BuffSkill,
    false,
    None  // 使用目标 Buff 冷却表
);

if !self.self_buffing && image.client_stats.target_is_alive {
    self.send_buff(config, target_buff, false);
}
```

**关键点**：
- 自我 Buff 和目标 Buff 使用不同的冷却表
- 自我 Buff 优先级更高
- 自我 Buff 时会取消目标，完成后重新选择队长
- 目标 Buff 需要等待自我 Buff 完成

### 9. 技能管理

#### 9.1 冷却系统

```rust
self_buff_usage_last_time: [[Option<Instant>; 10]; 9]   // 自我 Buff 冷却
slots_usage_last_time: [[Option<Instant>; 10]; 9]       // 目标技能冷却
```

- 支持 9 个技能栏，每个栏 10 个技能位
- 每个技能记录上次使用时间
- 自动更新，冷却完成后设置为 None

#### 9.2 get_slot_for 方法

```rust
fn get_slot_for(
    &mut self,
    config: &SupportConfig,
    threshold: Option<u32>,           // 触发阈值（如 HP < 50）
    slot_type: SlotType,              // 技能类型
    send: bool,                       // 是否立即发送
    last_slots_usage: Option<...>     // 使用哪个冷却表
) -> Option<(usize, usize)>
```

**参数说明**：
- **threshold**：触发条件，如 HP 值
- **slot_type**：技能类型（Buff、治疗、复活等）
- **send**：是否立即发送技能
- **last_slots_usage**：
  - `Some(self_buff_usage_last_time)` - 使用自我 Buff 冷却表
  - `None` - 使用目标技能冷却表

#### 9.3 send_slot 方法

```rust
fn send_slot(&mut self, slot_index: (usize, usize), is_self_buff: bool) {
    // 发送按键
    send_slot_eval(self.window, slot_index.0, slot_index.1);

    // 更新冷却时间
    if is_self_buff {
        self.self_buff_usage_last_time[slot_index.0][slot_index.1] = Some(Instant::now());
    } else {
        self.slots_usage_last_time[slot_index.0][slot_index.1] = Some(Instant::now());
    }
}
```

### 10. 取消目标：lose_target

```rust
fn lose_target(&mut self) {
    if self.has_target {
        play!(self.movement => [
            PressKey("Escape"),
            Wait(dur::Random(200..250)),
        ]);
    }
}
```

**使用场景**：
- 给自己加 Buff 之前
- 使用 HealSkill 治疗自己之前（队伍模式）

### 11. 防检测机制：random_camera_movement

```rust
fn random_camera_movement(&mut self) {
    // 每 10 秒执行一次
    if self.last_jump_time.elapsed().as_millis() > 10000 {
        play!(self.movement => [
            Rotate(rot::Right, dur::Fixed(50)),  // 向右旋转 50ms
            Wait(dur::Fixed(50)),                 // 等待 50ms
        ]);
        self.last_jump_time = Instant::now();
    }
}
```

**目的**：
- 模拟真实玩家的摄像头移动
- 每 10 秒轻微旋转一次
- 降低被检测为机器人的风险

### 12. 等待机制

#### wait 方法
```rust
fn wait(&mut self, duration: Duration) {
    self.wait_duration = {
        if self.wait_duration.is_some() {
            Some(self.wait_duration.unwrap() + duration)  // 累加等待时间
        } else {
            self.wait_start = Instant::now();
            Some(duration)
        }
    };
}
```

#### wait_cooldown 方法
```rust
fn wait_cooldown(&mut self) -> bool {
    if self.wait_duration.is_some() {
        if self.wait_start.elapsed() < self.wait_duration.unwrap() {
            return true;  // 仍在等待中
        } else {
            self.wait_duration = None;  // 等待完成
        }
    }
    return false;
}
```

**特点**：
- 支持等待时间累加
- 在等待期间跳过某些操作
- 等待完成后自动重置

## 技能类型 (SlotType)

Support Behavior 使用以下技能类型：

- **BuffSkill**：Buff 技能（给自己和目标）
- **HealSkill**：单体治疗技能
- **AOEHealSkill**：群体治疗技能
- **RezSkill**：复活技能
- **Pill**：药丸（自我恢复）
- **Food**：食物（自我恢复）
- **MpRestorer**：魔法恢复道具
- **FpRestorer**：疲劳恢复道具
- **PartySkill**：队伍技能（自动释放）

## 配置项 (SupportConfig)

系统依赖以下配置：

- **is_in_party()**: 是否在队伍中
- **get_max_main_distance()**: 最大跟随距离
- **get_slot_cooldown()**: 获取技能冷却时间
- **get_usable_slot_index()**: 获取可用技能索引
- **get_all_usable_slot_for_type()**: 获取某类型的所有可用技能

## 状态变量

### 目标状态
- **has_target**: 是否有目标
- **is_waiting_for_revive**: 是否等待复活中
- **last_target_distance**: 上次目标距离
- **last_far_from_target**: 上次远离目标的时间

### Buff 状态
- **self_buffing**: 是否正在给自己加 Buff
- **target_buffing**: 是否正在给目标加 Buff
- **buff_counter**: Buff 计数器（用于日志）

### 移动状态
- **avoid_obstacle_direction**: 当前障碍物规避方向（A 或 D）
- **last_jump_time**: 上次跳跃/旋转时间

### 等待状态
- **wait_duration**: 等待持续时间
- **wait_start**: 等待开始时间

## 完整工作流程

### 队伍辅助模式流程

```
启动
 ↓
检查是否有目标
 ↓ 无目标
选择队长
 ↓
跟随队长
 ↓
检查距离
 ↓ 距离合适
检查队长是否倒地
 ↓ 倒地
复活队长
 ↓ 存活
检查队长 HP
 ↓ HP 低
治疗队长
 ↓ HP 正常
给自己加 Buff
 ↓ Buff 完成
重新选择队长
 ↓
给队长加 Buff
 ↓
继续循环
```

### 距离管理流程

```
检测目标距离
 ↓
距离 == 9999？
 ↓ 是
圆形移动
 ↓
距离 > 最大距离 × 2？
 ↓ 是
圆形移动
 ↓
距离 > 最大距离？
 ↓ 是
持续远离超过 3 秒？
 ↓ 是
圆形移动
 ↓
激活跟随
 ↓
距离 ≤ 最大距离
 ↓
继续正常操作
```

## 特点和优势

### 1. 双冷却系统
- 自我 Buff 和目标技能独立管理
- 避免冷却时间冲突
- 可以给自己和队友加不同的 Buff

### 2. 智能距离管理
- 三级距离判断（正常、远、非常远）
- 自动尝试绕过障碍物
- 持续跟随失败时自动圆形移动

### 3. 自动复活
- 实时检测目标生命状态
- 自动使用复活技能
- 等待复活完成后继续辅助

### 4. 优先级系统
- 复活 > 治疗 > Buff
- 自我恢复优先（确保生存）
- 队伍技能自动释放

### 5. 防检测机制
- 随机摄像头移动
- 随机等待时间
- 模拟真实玩家行为

### 6. 稳定的跟随系统
- 多重保险机制
- 失去目标自动重新选择队长
- 智能障碍物规避

## 使用场景

### 1. 队伍辅助
- 跟随主力输出职业
- 自动加 Buff 和治疗
- 紧急复活倒地队友

### 2. 双开挂机
- 主号打怪，辅助号自动跟随加 Buff
- 主号 HP 低时自动治疗
- 主号倒地自动复活

### 3. 副本辅助
- 跟随队长进入副本
- 全程自动治疗和 Buff
- 保证队伍持续作战能力

## 潜在改进方向

1. **多目标监控**：
   - 监控整个队伍的 HP
   - 优先治疗 HP 最低的队员
   - 群体 Buff 优化

2. **预判治疗**：
   - 根据队友职业和怪物类型预判伤害
   - 提前施放治疗
   - 避免队友暴毙

3. **智能 Buff 管理**：
   - 检测 Buff 剩余时间
   - 只在 Buff 快过期时重新施放
   - 节省魔法值

4. **位置优化**：
   - 保持在团队后方安全位置
   - 避免被怪物攻击
   - 最大化治疗覆盖范围

5. **紧急模式**：
   - 检测到危险时自动使用强力恢复道具
   - 连续治疗或群体治疗
   - 保命优先

6. **魔法管理**：
   - 监控 MP 使用效率
   - MP 不足时减少 Buff 频率
   - 优先保证治疗魔法

## 调试和日志

系统在关键点记录日志：

```rust
slog::debug!(self.logger, "Starting self buffing");
slog::debug!(self.logger, "Ending self buffing {}", self.buff_counter);
slog::debug!(self.logger, "Starting target buffing");
slog::debug!(self.logger, "Ending target buffing {}", self.buff_counter);
slog::debug!(self.logger, "SupportBehavior stopped");
```

**日志信息**：
- Buff 开始/结束
- Buff 计数
- 行为停止

## 总结

SupportBehavior 是一个功能完善的辅助职业自动化模块。它通过智能的目标管理、双冷却系统、距离检测和自动复活等功能，实现了完全自动化的辅助职业操作。代码结构清晰，逻辑完整，特别适合牧师、治疗师等辅助职业的自动化需求。

核心亮点：
- ✅ 独立的自我/目标 Buff 系统
- ✅ 智能距离管理和障碍物规避
- ✅ 自动复活和治疗优先级
- ✅ 防机器人检测机制
- ✅ 稳定的队长跟随系统
