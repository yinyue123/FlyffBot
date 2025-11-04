# Farming Behavior 逻辑说明

## 概述

farming.go 实现了一个基于状态机的自动打怪系统，负责自主寻找怪物、攻击、拾取物品等功能。

## 状态机设计

### 状态定义

```
FarmingStateNoEnemyFound        // 未发现敌人，需要开始搜索
FarmingStateSearchingForEnemy   // 正在搜索敌人
FarmingStateEnemyFound          // 发现敌人，准备攻击
FarmingStateVerifyTarget        // 验证目标是否成功选中
FarmingStateAttacking           // 正在攻击
FarmingStateAfterEnemyKill      // 击杀后处理（拾取物品等）
```

### 状态转换流程

```
NoEnemyFound
  ↓ (旋转或移动后)
SearchingForEnemy
  ↓ (未找到怪物)          ↓ (发现怪物)
NoEnemyFound           EnemyFound
                         ↓ (点击怪物)
                      VerifyTarget
                         ↓ (目标确认失败)  ↓ (目标确认成功)
                    SearchingForEnemy   Attacking
                                          ↓ (怪物死亡)
                                       AfterEnemyKill
                                          ↓
                                    SearchingForEnemy
```

## 核心功能模块

### 1. NoEnemyFound（未发现敌人）

**主要逻辑：**
- 检查超时：如果配置了 `MobsTimeout`，超时后退出
- 旋转搜索：先尝试右旋转最多30次（每次50ms）
- 循环移动：如果配置了 `CircleMoveDuration`，执行循环移动模式
  - 同时按住 W + Space + D 键移动
  - 释放 D 键
  - 释放 Space 和 W 键
  - 按住 S 键后退50ms

**代码位置：** `onNoEnemyFound()` (line 187-217)

### 2. SearchingForEnemy（搜索敌人）

**主要逻辑：**
1. 检查是否应停止战斗（`config.StopFighting`）
2. 调用 `analyzer.IdentifyMobs()` 识别怪物
3. 如果没有怪物，返回 `NoEnemyFound` 状态
4. 怪物优先级处理：
   - 如果 `PrioritizeAggro` 开启，优先攻击主动怪
   - 特殊情况：如果只有一个主动怪且刚击杀过主动怪（5秒内），且HP足够，则攻击被动怪
5. 避让区域处理：
   - 如果有避让区域列表，过滤掉在避让区域内的怪物
   - 选择离屏幕中心最近的可攻击怪物
6. 设置 `currentTarget` 并转入 `EnemyFound` 状态

**代码位置：** `onSearchingForEnemy()` (line 230-290), `prioritizeMobs()` (line 293-327)

### 3. EnemyFound（发现敌人）

**主要逻辑：**
- 获取怪物的攻击坐标（通过 `AttackCoords()`）
- 保存点击位置到 `lastClickPos`
- 点击怪物位置
- 等待150ms
- 转入 `VerifyTarget` 状态

**代码位置：** `onEnemyFound()` (line 330-347)

### 4. VerifyTarget（验证目标）

**主要逻辑：**
- 检查目标标记是否在屏幕上
- 检查目标是否存活（`TargetIsAlive`）
- 如果验证成功，转入 `Attacking` 状态
- 如果验证失败：
  - 调用 `avoidLastClick()` 将点击位置加入避让列表（5秒）
  - 返回 `SearchingForEnemy` 状态

**代码位置：** `onVerifyTarget()` (line 350-360)

### 5. Attacking（攻击中）

**主要逻辑：**

**初始化（首次进入攻击状态）：**
- 重置旋转尝试和障碍物避让计数
- 记录攻击开始时间
- 设置 `isAttacking = true`

**目标检测：**
- 如果目标不在屏幕上或已死亡：
  - 如果玩家还活着，说明怪物已被击杀，转入 `AfterEnemyKill`
  - 否则返回 `SearchingForEnemy`

**障碍物避让：**
- 检测条件：目标HP更新时间超过 `ObstacleAvoidanceCooldown`
- 如果目标HP = 100%，最多尝试2次
- 否则最多尝试 `ObstacleAvoidanceMaxTry` 次
- 避让动作：
  - 第一次：按Z键 + W和Space前进800ms
  - 后续：W和Space + 左右交替旋转（A/D）200ms + 前进800ms
- 超过最大尝试次数，调用 `abortAttack()` 中止攻击

**技能使用：**
- 使用攻击技能：`config.AttackSlots`
- AOE群攻逻辑：
  - 如果配置了 `MaxAOEFarming > 1`
  - 当当前并发攻击数 < MaxAOEFarming 且目标HP < 90%
  - 中止当前攻击，寻找下一个目标
- AOE技能：当目标距离 < 75 时使用 `config.AOEAttackSlots`

**代码位置：** `onAttacking()` (line 363-432), `avoidObstacle()` (line 435-464)

### 6. AfterEnemyKill（击杀后处理）

**主要逻辑：**
1. 记录统计数据：
   - 击杀时间 = 当前时间 - 攻击开始时间
   - 搜索时间 = 攻击开始时间 - 上次击杀时间
   - 调用 `stats.AddKill()` 记录
2. 更新计数器：
   - `killCount++`
   - 重置 `stealedTargetCount`
   - 更新 `lastKillTime`
3. 拾取物品：调用 `performPickup()`
4. 清除当前目标
5. 返回 `SearchingForEnemy` 状态

**代码位置：** `afterEnemyKill()` (line 514-533)

## 辅助功能

### 拾取系统

**拾取优先级：**
1. 宠物拾取（`PickupPetSlot`）
   - 使用宠物技能拾取
   - 等待1500ms
   - 检查宠物召唤冷却，到期后取消召唤
2. 动作拾取（`PickupMotionSlot`）
   - 使用拾取动作
   - 等待1000ms
3. 传统拾取槽（`PickupSlots`）
   - 使用技能栏拾取
   - 等待1000ms

**宠物管理：**
- 跟踪上次召唤时间
- 根据槽位冷却时间（默认3000ms）自动取消召唤

**代码位置：** `performPickup()` (line 558-583), `updatePickupPet()` (line 536-555)

### 恢复系统

**检查顺序：**
1. 组队技能：使用 `PartySkillSlots`（带冷却检测）
2. HP恢复：
   - HP < `HealThreshold` 时
   - 优先使用 `HealSlots`
   - 如果没有，使用 `AOEHealSlots`（连续使用3次，间隔100ms）
3. MP恢复：
   - MP < `MPThreshold` 时使用 `MPRestoreSlots`
4. FP恢复：
   - FP < `FPThreshold` 时使用 `FPRestoreSlots`

**代码位置：** `checkRestorations()` (line 607-643), `usePartySkills()` (line 646-658)

### 避让系统

**避让区域管理：**

1. **点击位置避让** (`avoidLastClick()`)
   - 当无法选中目标时触发
   - 创建2x2像素的避让区域
   - 持续时间：5秒

2. **中止攻击避让** (`abortAttack()`)
   - 当达到最大障碍物避让次数时触发
   - 如果有目标标记，创建40x40像素的避让区域
   - 根据已尝试次数扩大区域（每次+10像素）
   - 持续时间：2秒
   - 按Escape键取消目标

3. **自动清理**
   - 每次迭代检查避让区域是否过期
   - 过期的区域自动移除

**代码位置：** `avoidLastClick()` (line 497-511), `abortAttack()` (line 467-494), `updateTimestamps()` (line 661-673)

### 技能冷却管理

**`sendSlot()` 功能：**
- 跟踪每个槽位的上次使用时间
- 检查槽位冷却（从 `config.SlotCooldowns` 读取）
- 如果在冷却中，跳过使用
- 使用后记录时间戳

**应用场景：**
- 宠物召唤/取消
- 组队技能使用

**代码位置：** `sendSlot()` (line 586-604)

### 等待机制

**功能：**
- 设置等待时长（可累加）
- 等待期间可以使用buff技能
- 等待完成后自动清除

**使用场景：**
- 使用buff后等待1500ms
- 技能间隔等待

**代码位置：** `wait()` (line 676-684), `waitCooldown()` (line 687-695)

## 主循环执行流程

```
Run() 被调用
  ↓
1. 更新玩家状态（analyzer.UpdateStats()）
  ↓
2. 检查玩家是否存活
  ↓ (已死亡则停止)
3. 更新时间戳（清理过期避让区域）
  ↓
4. 检查恢复（HP/MP/FP）
  ↓
5. 检查等待冷却
  ↓ (如果在等待且在AfterEnemyKill状态，直接返回)
6. 执行状态机逻辑
  ↓
返回
```

**代码位置：** `Run()` (line 128-164)

## 重要配置参数

| 参数 | 说明 |
|------|------|
| `MobsTimeout` | 未找到怪物的超时时间（毫秒） |
| `CircleMoveDuration` | 循环移动持续时间（毫秒） |
| `StopFighting` | 是否停止战斗 |
| `PrioritizeAggro` | 是否优先攻击主动怪 |
| `MinHPAttack` | 攻击被动怪的最低HP要求 |
| `ObstacleAvoidanceCooldown` | 障碍物检测冷却时间 |
| `ObstacleAvoidanceMaxTry` | 障碍物避让最大尝试次数 |
| `MaxAOEFarming` | AOE群攻最大并发目标数 |
| `HealThreshold` | HP恢复阈值 |
| `MPThreshold` | MP恢复阈值 |
| `FPThreshold` | FP恢复阈值 |
| `AttackSlots` | 攻击技能槽位 |
| `AOEAttackSlots` | AOE攻击技能槽位 |
| `BuffSlots` | Buff技能槽位 |
| `HealSlots` | 治疗技能槽位 |
| `AOEHealSlots` | AOE治疗技能槽位 |
| `MPRestoreSlots` | MP恢复槽位 |
| `FPRestoreSlots` | FP恢复槽位 |
| `PartySkillSlots` | 组队技能槽位 |
| `PickupPetSlot` | 拾取宠物槽位 |
| `PickupMotionSlot` | 拾取动作槽位 |
| `PickupSlots` | 传统拾取槽位 |
| `SlotCooldowns` | 槽位冷却时间映射 |

## 统计数据

**FarmingBehavior 跟踪以下数据：**
- `killCount`: 击杀总数
- `stealedTargetCount`: 被抢怪次数
- `lastKilledType`: 上次击杀的怪物类型
- `concurrentMobsAttack`: 当前并发攻击数（用于AOE）
- `lastKillTime`: 上次击杀时间
- `lastSearchTime`: 上次搜索时间
- `lastInitialAttackTime`: 攻击开始时间

**Statistics 对象记录：**
- 击杀时间统计
- 搜索时间统计

## 关键数据结构

```go
type FarmingBehavior struct {
    // 状态机
    state FarmingState

    // 时间管理
    lastKillTime          time.Time
    lastSearchTime        time.Time
    lastInitialAttackTime time.Time
    lastNoEnemyTime       *time.Time

    // 攻击管理
    currentTarget     *Target
    isAttacking       bool
    alreadyAttackCount int
    lastClickPos      *Point

    // 障碍物和避让
    rotationAttempts      int
    obstacleAvoidanceCount int
    avoidedBounds         []AvoidedArea

    // 统计
    killCount             int
    stealedTargetCount    int
    lastKilledType        MobType
    concurrentMobsAttack  int

    // 等待管理
    waitDuration *time.Duration
    waitStart    time.Time

    // 拾取宠物管理
    lastSummonPetTime time.Time
    slotUsageTimes    map[int]time.Time
}
```

## 特殊功能

### AOE 群攻模式

当 `MaxAOEFarming > 1` 时启用：
1. 攻击第一个目标直到HP < 90%
2. 中止当前攻击，并发攻击数+1
3. 寻找下一个目标
4. 重复直到达到 `MaxAOEFarming` 限制
5. 当所有目标被引到后，使用AOE技能清理

### 主动怪优先策略

当 `PrioritizeAggro` 开启时：
- 优先选择主动怪攻击
- 特殊情况：如果只有一个主动怪且刚击杀过主动怪（5秒内），且HP >= MinHPAttack，则攻击被动怪
- 如果没有主动怪，攻击被动怪

### 障碍物智能避让

- 检测：目标HP长时间未更新（超过 ObstacleAvoidanceCooldown）
- 第一次尝试：按Z + 向前跳跃
- 后续尝试：左右交替旋转 + 向前跳跃
- 达到最大次数后，将该位置加入避让列表
