# Farming Behavior 自动挂机逻辑

## 概述

farming_behavior.rs 实现了一个完整的自动挂机打怪系统，基于状态机设计，能够自动搜索敌人、攻击、拾取物品并管理角色状态。

## 核心状态机

系统使用 `State` 枚举管理 6 个主要状态：

### 1. NoEnemyFound (未找到敌人)
- **触发条件**：搜索后未发现任何敌人
- **处理逻辑**：
  - 旋转视角 30 次尝试发现附近敌人
  - 如果配置了超时时间且超时，则退出程序
  - 如果启用圆形移动模式(`circle_pattern_rotation_duration > 0`)，则进行圆形移动
  - 转换到 `SearchingForEnemy` 状态

### 2. SearchingForEnemy (搜索敌人中)
- **触发条件**：开始搜索或旋转后等待发现敌人
- **处理逻辑**：
  - 通过图像分析识别屏幕上的怪物
  - 根据配置决定最大搜索距离（是否圆形移动）
  - 优先处理主动攻击怪物（如果启用 `prioritize_aggro`）
  - 找到最近的可攻击目标（排除已标记避免的区域）
  - 转换到 `EnemyFound` 或 `NoEnemyFound` 状态

### 3. EnemyFound (发现敌人)
- **触发条件**：成功定位到目标敌人
- **处理逻辑**：
  - 计算攻击坐标
  - 模拟鼠标点击选中目标
  - 记录点击位置用于后续避免
  - 等待 150ms
  - 转换到 `VerifyTarget` 状态

### 4. VerifyTarget (验证目标)
- **触发条件**：点击目标后验证是否成功选中
- **处理逻辑**：
  - 检查目标是否在屏幕上且是可移动单位（非 NPC）
  - 如果验证成功，转换到 `Attacking` 状态
  - 如果验证失败，避免该点击位置，返回 `SearchingForEnemy` 状态

### 5. Attacking (攻击中)
- **触发条件**：成功选中有效目标
- **处理逻辑**：
  - **初次攻击**：
    - 检测目标是否已被其他玩家攻击（血量<100）
    - 重置障碍物规避计数
    - 标记为正在攻击状态

  - **攻击循环**：
    - 检查目标是否在屏幕上和存活状态
    - 如果目标不在视野内或长时间未更新血量，执行障碍物规避
    - 使用攻击技能 (`AttackSkill`)
    - 如果目标距离 < 75，使用 AOE 技能 (`AOEAttackSkill`)

  - **AOE 挂机模式**：
    - 如果配置 `max_aoe_farming > 1`，当目标血量降至 90% 以下时
    - 增加并发攻击计数，放弃当前目标寻找下一个
    - 最多同时攻击 `max_aoe_farming` 个怪物

  - **状态转换**：
    - 目标死亡：转换到 `AfterEnemyKill` 状态
    - 目标消失或其他情况：返回 `SearchingForEnemy` 状态

### 6. AfterEnemyKill (击杀敌人后)
- **触发条件**：成功击杀目标
- **处理逻辑**：
  - 增加击杀计数
  - 记录击杀的怪物类型（主动/被动/紫色）
  - 重置并发攻击计数
  - 调用拾取物品逻辑
  - 计算并记录统计数据（搜索时间、击杀时间、效率等）
  - 转换到 `SearchingForEnemy` 状态

## 核心功能模块

### 资源恢复 (check_restorations)
定期检查并恢复角色状态：

1. **HP 恢复**（优先级从高到低）：
   - Pill (药丸)
   - HealSkill (治疗技能)
   - AOEHealSkill (群体治疗技能，连续使用 3 次)
   - Food (食物)

2. **MP 恢复**：
   - MpRestorer (魔法恢复道具)

3. **FP 恢复**：
   - FpRestorer (疲劳恢复道具)

4. **队伍技能**：
   - 自动使用所有配置的 PartySkill

### 技能管理

#### 技能冷却系统
- 使用 `slots_usage_last_time: [[Option<Instant>; 10]; 9]` 记录每个技能栏的使用时间
- 自动更新冷却状态，冷却完成后重置为 None
- 支持 9 个技能栏，每个栏 10 个技能位

#### 技能类型 (SlotType)
- **BuffSkill**: Buff 技能（每次迭代优先检查）
- **AttackSkill**: 普通攻击技能
- **AOEAttackSkill**: AOE 攻击技能（距离 < 75 时使用）
- **HealSkill**: 单体治疗技能
- **AOEHealSkill**: 群体治疗技能
- **PartySkill**: 队伍技能（自动释放）
- **PickupPet**: 拾取宠物
- **PickupMotion**: 拾取动作

### 拾取系统 (pickup_items)

两种拾取方式：

1. **宠物拾取** (PickupPet)：
   - 召唤宠物后保持一定时间
   - 使用 `last_summon_pet_time` 记录召唤时间
   - 冷却时间到达后自动取消召唤

2. **动作拾取** (PickupMotion)：
   - 如果没有配置宠物，使用拾取动作
   - 连续执行 10 次，每次间隔 300ms

### 障碍物规避 (avoid_obstacle)

当目标不在视野内或长时间无法攻击时触发：

1. **第一次尝试** (obstacle_avoidance_count == 0)：
   - 按 Z 键
   - 按住 W + Space 前进跳跃 800ms

2. **后续尝试**：
   - 随机选择 A 或 D 键
   - 按住 W + Space + (A/D) 斜向跳跃 800ms
   - 释放按键后按 Z

3. **失败处理**：
   - 超过 `obstacle_avoidance_max_try` 次后放弃攻击
   - 将目标区域标记为避免区域
   - 返回搜索状态

### 目标优先级 (prioritize_aggro)

如果启用优先级系统：

1. **优先攻击主动怪**：
   - 筛选所有 Aggressive 类型的怪物

2. **切换到被动怪的条件**：
   - 没有主动怪，或
   - 只有 1 个主动怪且刚击杀过主动怪（5 秒内），且
   - 当前 HP ≥ `min_hp_attack`

3. **如果不启用优先级**：
   - 攻击所有非紫色怪物

### 避免区域系统 (avoided_bounds)

防止重复攻击同一位置：

- 存储格式：`(Bounds, Instant, u128)` - 区域、开始时间、持续时间
- 自动过期清理机制
- 应用场景：
  - 刚击杀的怪物区域（5 秒）
  - 已被攻击的怪物区域（2 秒）
  - 点击失败的位置（5 秒）

### 圆形移动模式 (move_circle_pattern)

当配置了 `circle_pattern_rotation_duration > 0` 时：

1. 按住 W + Space + D 键移动指定时间
2. 释放 D 键
3. 等待 20ms
4. 释放 W + Space
5. 按住 S 键后退 50ms

rotation_duration 越小，圆圈越大；越大，圆圈越小。

## 主循环逻辑 (run_iteration)

每次迭代执行以下步骤：

1. **更新时间戳**：
   - 更新宠物召唤状态
   - 更新技能冷却状态
   - 清理过期的避免区域

2. **检查资源恢复**：
   - HP/MP/FP 检查和恢复
   - 使用队伍技能

3. **处理 Buff**：
   - 如果不在冷却中，检查并使用 BuffSkill
   - 使用后等待 1500ms

4. **状态机处理**：
   - 根据当前状态调用相应的处理函数
   - 状态转换

5. **更新前端信息**：
   - 更新攻击状态
   - 更新击杀数
   - 更新统计数据

## 统计数据

系统自动跟踪和计算：

- **击杀数** (`kill_count`)
- **搜索时间** - 从上次击杀到发现目标的时间
- **击杀时间** - 从开始攻击到击杀的时间
- **每分钟击杀数** = 60 / (搜索时间 + 击杀时间)
- **每小时击杀数** = 每分钟击杀数 × 60
- **运行总时间** - 从开始到当前的时间

## 重要常量

- `MAX_DISTANCE_FOR_AOE`: 75 - AOE 技能的最大使用距离
- 默认最大搜索距离：325（无圆形移动）或 1000（圆形移动）
- 旋转尝试次数：30 次
- 拾取动作重复次数：10 次
- 拾取动作间隔：300ms

## 状态变量

### 计数器
- `kill_count`: 总击杀数
- `already_attack_count`: 已被攻击的目标计数
- `obstacle_avoidance_count`: 当前目标的障碍物规避次数
- `rotation_movement_tries`: 旋转搜索尝试次数
- `stealed_target_count`: 目标被抢计数
- `concurrent_mobs_under_attack`: 当前同时攻击的怪物数

### 时间戳
- `last_initial_attack_time`: 最后一次初始攻击时间
- `last_kill_time`: 最后一次击杀时间
- `last_summon_pet_time`: 最后一次召唤宠物时间
- `last_no_ennemy_time`: 最后一次未找到敌人的时间
- `start_time`: 开始时间

### 状态标志
- `is_attacking`: 是否正在攻击
- `last_killed_type`: 最后击杀的怪物类型
- `last_click_pos`: 最后点击的位置

## 配置项 (FarmingConfig)

系统支持以下配置：

- `prioritize_aggro()`: 是否优先攻击主动怪
- `min_hp_attack()`: 攻击被动怪的最低 HP 要求
- `circle_pattern_rotation_duration()`: 圆形移动的旋转时间
- `obstacle_avoidance_cooldown()`: 障碍物规避触发的冷却时间
- `obstacle_avoidance_max_try()`: 最大障碍物规避尝试次数
- `max_aoe_farming()`: AOE 挂机的最大并发目标数
- `mobs_timeout()`: 未找到怪物的超时时间（毫秒）
- `is_stop_fighting()`: 是否停止战斗
- 各种技能槽位配置和冷却时间

## 待优化点

代码中注释的 TODO 和待优化项：

1. 拾取动作的重复次数应该可配置（当前固定 10 次）
2. 已注释掉的防止攻击已被攻击怪物的逻辑（584-596 行）
3. 搜索敌人超时机制已注释（38、69 行）
