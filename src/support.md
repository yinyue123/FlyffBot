# Support Behavior 逻辑说明

## 概述

support.go 实现了一个基于状态机的辅助/治疗职业系统，负责跟随队长、治疗队友、施加增益效果和复活队友等功能。

## 状态机设计

### 状态定义

```
SupportStateNoTarget        // 无目标，尝试选择队长
SupportStateTargetSelected  // 已选择目标，验证状态
SupportStateFollowing       // 跟随目标
SupportStateTooFar          // 目标太远，移动靠近
SupportStateHealing         // 治疗目标
SupportStateBuffing         // 为目标施加增益
SupportStateSelfBuffing     // 为自己施加增益
SupportStateResurrecting    // 复活目标
```

### 状态转换流程

```
NoTarget
  ↓ (选择队长)
TargetSelected
  ↓ (目标验证成功)
Following
  ├─→ TooFar (目标超出范围)
  │     ↓ (移动靠近)
  │   Following
  ├─→ Healing (目标HP低)
  │     ↓ (治疗完成)
  │   Following
  ├─→ Buffing (增益计时器到期)
  │     ↓ (增益完成)
  │   Following
  ├─→ Resurrecting (目标死亡)
  │     ↓ (复活完成)
  │   Following
  └─→ SelfBuffing (自我增益计时器到期)
        ↓ (自我增益完成)
      NoTarget (重新选择队长)
```

## 核心功能模块

### 1. NoTarget（无目标）

**主要逻辑：**
- 检查是否在队伍中（`config.InParty`）
- 如果在队伍中，调用 `selectPartyLeader()` 选择队长
- 等待500ms让选择生效
- 转入 `TargetSelected` 状态

**选择队长过程：**
1. 按 P 键打开队伍菜单
2. 等待150ms
3. 点击队长位置坐标 (213, 440)
4. 按 Z 键确认跟随
5. 等待10ms
6. 按 P 键关闭队伍菜单
7. 等待500ms

**代码位置：** `onNoTarget()` (line 188-195), `selectPartyLeader()` (line 440-456)

### 2. TargetSelected（目标已选择）

**主要逻辑：**
- 验证目标是否在屏幕上（`hasTarget`）
- 如果没有目标，返回 `NoTarget` 状态
- 如果有目标，转入 `Following` 状态

**代码位置：** `onTargetSelected()` (line 198-203)

### 3. Following（跟随中）

**主要逻辑：**

**目标检测：**
- 检查目标是否存在，否则返回 `NoTarget`
- 检查目标是否死亡，如果死亡转入 `Resurrecting` 状态

**距离检测：**
- 调用 `isTargetInRange()` 检查目标距离
- 如果超出范围，转入 `TooFar` 状态

**治疗检测：**
- 获取目标HP（`analyzer.DetectTargetHP()`）
- 如果 HP > 0 且 HP < `HealThreshold`，转入 `Healing` 状态

**增益检测：**
- 目标增益：距离上次增益时间超过30秒，且有增益技能，转入 `Buffing` 状态
- 自我增益：在队伍模式下，距离上次自我增益超过60秒，且有增益技能，转入 `SelfBuffing` 状态

**持续跟随：**
- 调用 `followTarget()` 继续跟随（按 Z 键）

**代码位置：** `onFollowing()` (line 206-244), `followTarget()` (line 459-463)

### 4. TooFar（目标太远）

**主要逻辑：**
- 调用 `followTarget()` 向目标移动
- 检查是否回到范围内
  - 如果在范围内，返回 `Following` 状态
  - 否则继续 `TooFar` 状态

**代码位置：** `onTooFar()` (line 247-257)

### 5. Healing（治疗中）

**主要逻辑：**

**单体治疗（优先）：**
- 如果配置了 `HealSlots`
- 使用治疗技能
- 等待2000ms施法时间

**AOE治疗（备选）：**
- 如果配置了 `AOEHealSlots`
- 连续使用3次AOE治疗（间隔100ms）
- 最后等待100ms

**完成后返回 `Following` 状态**

**代码位置：** `onHealing()` (line 260-278)

### 6. Buffing（为目标增益）

**主要逻辑：**

**初始化（首次进入）：**
- 设置 `targetBuffing = true`
- 重置 `buffCounter = 0`
- 记录日志

**增益施加：**
- 遍历 `config.BuffSlots` 中的所有技能
- 每次使用一个技能
- `buffCounter++`
- 等待2500ms施法时间

**完成检测：**
- 当 `buffCounter >= len(config.BuffSlots)` 时
- 重置 `targetBuffing = false`
- 更新 `lastBuffTime` 为当前时间
- 记录日志
- 返回 `Following` 状态

**代码位置：** `onBuffing()` (line 281-307)

### 7. SelfBuffing（自我增益）

**主要逻辑：**

**初始化（首次进入）：**
- 设置 `selfBuffing = true`
- 重置 `buffCounter = 0`
- 记录日志

**取消目标：**
- 如果有目标，调用 `loseTarget()` 取消目标
  - 按 Escape 键
  - 随机等待200-250ms
- 等待250ms

**增益施加：**
- 遍历 `config.BuffSlots` 中的所有技能
- 每次使用一个技能（对自己施放）
- `buffCounter++`
- 等待2500ms施法时间

**完成检测：**
- 当 `buffCounter >= len(config.BuffSlots)` 时
- 重置 `selfBuffing = false`
- 更新 `lastSelfBuffTime` 为当前时间
- 记录日志
- 返回 `NoTarget` 状态（重新选择队长）

**代码位置：** `onSelfBuffing()` (line 310-344), `loseTarget()` (line 466-471)

### 8. Resurrecting（复活中）

**主要逻辑：**

**死亡检测：**
- 检查目标是否存在且死亡（`hasTarget && !TargetIsAlive`）

**等待复活状态：**
- 如果 `isWaitingForRevive = true`
  - 检查目标HP是否 > 0
  - 如果复活成功：
    - 记录日志
    - 重置 `isWaitingForRevive = false`
    - 返回 `Following` 状态
  - 否则继续等待在 `Resurrecting` 状态

**施放复活技能：**
- 如果 `isWaitingForRevive = false`
  - 如果配置了 `RezSlots`：
    - 记录日志
    - 使用复活技能
    - 等待3000ms（复活施法时间）
    - 设置 `isWaitingForRevive = true`
  - 如果未配置复活技能：
    - 记录警告
    - 设置 `isWaitingForRevive = true`（避免重复警告）

**目标存活检测：**
- 如果目标已存活或无目标
- 重置 `isWaitingForRevive = false`
- 返回 `Following` 状态

**代码位置：** `onResurrecting()` (line 347-379)

## 辅助功能

### 距离检测系统

**核心功能：** `isTargetInRange()` (line 382-416)

**距离检测逻辑：**

1. **获取距离：**
   - 调用 `analyzer.DetectTargetDistance()`
   - 如果返回9999（无法检测）：
     - 执行循环移动模式
     - 返回 false（不在范围内）

2. **范围判断：**
   - 比较距离与 `config.FollowDistance`
   - 如果 `distance > maxDistance`：
     - 距离超过2倍最大距离：立即执行循环移动
     - 距离在1-2倍之间：
       - 记录远离时间
       - 如果持续远离超过3秒且距离增加：执行循环移动
       - 否则等待观察
     - 更新 `lastTargetDistance`
     - 返回 false

3. **在范围内：**
   - 清除远离时间记录
   - 返回 true

### 循环移动模式

**功能：** `moveCirclePattern()` (line 419-437)

**移动逻辑：**
1. 同时按住 W + Space + 方向键（A或D）
2. 等待100ms
3. 释放方向键
4. 等待500ms继续前进
5. 释放 Space 和 W 键
6. 按 Z 键（重新跟随）
7. 交替方向（D ↔ A）

**使用场景：**
- 目标无法检测距离时
- 目标超出范围2倍时
- 目标持续远离超过3秒时

### 自我恢复系统

**功能：** `checkSelfRestorations()` (line 497-538)

**恢复优先级：**

1. **HP恢复：**
   - HP < `HealThreshold` 时触发
   - 优先使用 `HealSlots`：
     - 如果在队伍中且有目标：先取消目标，使用技能，等待2000ms
     - 否则：直接使用技能，等待500ms
   - 如果没有单体治疗，使用 `AOEHealSlots`：
     - 连续使用3次（间隔100ms）

2. **MP恢复：**
   - MP < `MPThreshold` 时触发
   - 使用 `MPRestoreSlots`
   - 等待300ms

3. **FP恢复：**
   - FP < `FPThreshold` 时触发
   - 使用 `FPRestoreSlots`
   - 等待300ms

### 组队技能系统

**功能：** `usePartySkills()` (line 483-494)

**逻辑：**
- 检查 `config.PartySkillSlots` 是否配置
- 遍历所有组队技能槽位
- 使用每个槽位的技能
- 每次使用后等待100ms
- 冷却时间通常由外部管理（技能自带冷却）

**使用场景：**
- 每次主循环执行时调用
- 持续尝试使用组队技能

### 防AFK随机相机移动

**功能：** `randomCameraMovement()` (line 474-480)

**逻辑：**
- 检查距离上次移动时间是否超过10秒
- 如果超过：
  - 右旋转相机50ms
  - 等待50ms
  - 更新 `lastJumpTime`

**目的：**
- 模拟玩家活动，避免被系统检测为AFK（离开键盘）

### 等待机制

**设置等待：** `wait()` (line 541-549)
- 可累加等待时长
- 记录等待开始时间

**检查等待：** `waitCooldown()` (line 552-560)
- 检查是否还在等待期间
- 等待完成后自动清除

**使用场景：**
- 技能施法等待
- 状态转换延迟
- 组队技能间隔

## 主循环执行流程

```
Run() 被调用
  ↓
1. 更新玩家状态（analyzer.UpdateStats()）
  ↓
2. 检查玩家是否存活
  ↓ (已死亡则停止)
3. 使用组队技能
  ↓
4. 检查自我恢复（HP/MP/FP）
  ↓
5. 更新目标状态（hasTarget）
  ↓
6. 检查等待冷却
  ↓ (如果在等待，直接返回)
7. 随机相机移动（防AFK）
  ↓
8. 执行状态机逻辑
  ↓
返回
```

**代码位置：** `Run()` (line 129-161)

## 重要配置参数

| 参数 | 说明 |
|------|------|
| `InParty` | 是否在队伍中 |
| `FollowDistance` | 跟随最大距离 |
| `HealThreshold` | 治疗触发阈值（HP百分比） |
| `MPThreshold` | MP恢复阈值 |
| `FPThreshold` | FP恢复阈值 |
| `BuffSlots` | 增益技能槽位数组 |
| `HealSlots` | 单体治疗技能槽位数组 |
| `AOEHealSlots` | AOE治疗技能槽位数组 |
| `MPRestoreSlots` | MP恢复槽位数组 |
| `FPRestoreSlots` | FP恢复槽位数组 |
| `PartySkillSlots` | 组队技能槽位数组 |
| `RezSlots` | 复活技能槽位数组 |

## 关键数据结构

```go
type SupportBehavior struct {
    // 状态机
    state SupportState

    // 目标管理
    hasTarget          bool
    lastTargetDistance int

    // 时间管理
    lastJumpTime       time.Time
    lastFarFromTarget  *time.Time
    lastBuffTime       time.Time     // 目标增益冷却（30秒）
    lastSelfBuffTime   time.Time     // 自我增益冷却（60秒）

    // 等待管理
    waitDuration *time.Duration
    waitStart    time.Time

    // 增益状态
    selfBuffing    bool
    targetBuffing  bool
    buffCounter    int

    // 复活状态
    isWaitingForRevive bool

    // 障碍物避让
    avoidObstacleDirection string  // "D" 或 "A"，交替使用
}
```

## 时间控制

### 增益冷却时间

- **目标增益：** 30秒冷却
  - 每30秒为目标施加所有增益技能
  - 通过 `lastBuffTime` 跟踪

- **自我增益：** 60秒冷却
  - 仅在队伍模式下启用
  - 每60秒为自己施加所有增益技能
  - 通过 `lastSelfBuffTime` 跟踪

### 技能施法等待

- **治疗技能：** 2000ms
- **AOE治疗技能：** 每次100ms，共3次
- **增益技能：** 每个2500ms
- **复活技能：** 3000ms
- **组队技能：** 每个100ms
- **自我恢复：**
  - HP治疗：500-2000ms（取决于是否在队伍中）
  - MP/FP恢复：300ms

### 其他时间控制

- **选择队长等待：** 500ms
- **取消目标等待：** 200-250ms（随机）
- **防AFK相机移动：** 每10秒触发一次
- **远离检测：** 持续远离3秒后触发循环移动

## 特殊功能

### 队长选择机制

**固定坐标点击：**
- 坐标位置：(213, 440)
- 假设这是队长在队伍菜单中的位置
- 通过 P 键打开/关闭队伍菜单

**局限性：**
- 依赖固定UI位置
- 如果UI布局改变可能失效

### 治疗目标切换

**目标治疗：**
- 直接对当前目标使用治疗技能
- 等待2000ms

**自我治疗：**
- 如果在队伍中且有目标：需要先取消目标
- 治疗后重新选择队长

**AOE治疗：**
- 不需要切换目标（范围治疗）
- 连续使用3次确保覆盖

### 复活流程

**两阶段复活：**
1. **施放阶段：**
   - 检测到目标死亡
   - 使用复活技能
   - 等待3000ms施法时间
   - 设置等待标志

2. **等待阶段：**
   - 持续检测目标HP
   - HP > 0 表示复活成功
   - 重置标志，返回跟随状态

**容错处理：**
- 如果未配置复活技能，只记录警告
- 通过等待标志避免重复尝试

### 障碍物避让策略

**循环移动：**
- 向前跳跃 + 旋转
- 左右交替（防止卡在同一位置）
- 使用 Z 键重新跟随目标

**触发条件：**
- 无法检测目标距离
- 目标距离超过2倍最大跟随距离
- 目标持续远离超过3秒

## 与 Farming 的区别

| 特性 | Farming | Support |
|------|---------|---------|
| 主要目标 | 主动寻找并攻击怪物 | 被动跟随并支援队友 |
| 状态数量 | 6个状态 | 8个状态 |
| 移动模式 | 搜索式移动 | 跟随式移动 |
| 技能使用 | 攻击技能为主 | 治疗/增益技能为主 |
| 目标选择 | 自动选择最近怪物 | 固定选择队长 |
| 复活功能 | 无 | 有复活队友功能 |
| 自我增益 | 战斗前buff | 定期60秒自我buff |
| 拾取功能 | 击杀后拾取 | 无拾取功能 |
| HP管理 | 低于阈值恢复 | 低于阈值恢复 + 治疗队友 |

## 使用建议

1. **配置增益技能：** 确保 `BuffSlots` 包含所有职业buff技能
2. **设置治疗阈值：** `HealThreshold` 建议设置为50-70%
3. **配置复活技能：** 如果职业有复活技能，添加到 `RezSlots`
4. **组队技能：** 将长冷却组队buff放入 `PartySkillSlots`
5. **跟随距离：** `FollowDistance` 根据职业技能范围调整（建议100-200）
6. **队长位置：** 如果队长不在固定位置(213,440)，需要修改代码

## 限制和注意事项

1. **固定坐标：** 队长选择使用固定坐标，UI改变可能导致失败
2. **单目标支援：** 只能支援队长，不支持多目标治疗
3. **距离依赖：** 需要 `analyzer.DetectTargetDistance()` 正常工作
4. **无拾取功能：** 辅助职业通常不负责拾取
5. **增益覆盖：** 所有增益技能会施加给同一目标，无法针对不同队友
6. **冷却管理：** 技能冷却主要依赖游戏内置冷却，无额外冷却检测
