# Shout Behavior 自动喊话逻辑

## 概述

shout_behavior.rs 实现了一个简单但实用的自动喊话系统，可以按照设定的时间间隔循环发送预设的消息列表。常用于游戏中的自动广告、招募队员、摆摊宣传等场景。

## 结构设计

### ShoutBehavior 结构体

```rust
pub struct ShoutBehavior<'a> {
    rng: rand::rngs::ThreadRng,
    logger: &'a Logger,
    movement: &'a MovementAccessor,
    window: &'a Window,
    last_shout_time: Instant,              // 上次喊话的时间戳
    shown_messages: Vec<String>,            // 要喊话的消息列表
    shout_interval: u64,                    // 喊话间隔（毫秒）
    message_iter: Option<Box<dyn Iterator<Item = String>>>,  // 消息循环迭代器
}
```

### 字段说明

- **last_shout_time**: 记录上次喊话的时间，用于计算是否到达下次喊话时间
- **shown_messages**: 存储所有需要喊话的消息文本列表
- **shout_interval**: 喊话间隔时间，单位为毫秒（默认 30000ms = 30 秒）
- **message_iter**: 循环迭代器，通过 `cycle()` 实现消息列表的无限循环

## 核心功能

### 1. 初始化 (new)

创建 ShoutBehavior 实例时：
- 设置默认喊话间隔为 30000ms（30 秒）
- 初始化空的消息列表
- message_iter 初始为 None

### 2. 启动和更新 (start/update)

从配置加载喊话设置：

```rust
fn update(&mut self, config: &BotConfig) {
    let config = config.shout_config();
    self.shown_messages = config.shout_messages();
    // 创建循环迭代器，消息会不断重复
    self.message_iter = Some(Box::new(self.shown_messages.clone().into_iter().cycle()));
    self.shout_interval = config.shout_interval();
}
```

- 加载喊话消息列表
- 创建循环迭代器（使用 `.cycle()` 实现消息列表的无限循环）
- 加载喊话间隔时间

### 3. 停止 (stop/interupt)

停止喊话时：
- 将 `message_iter` 设置为 None
- 停止消息循环

### 4. 主循环 (run_iteration)

每次迭代调用 `shout()` 方法执行喊话逻辑。

## 核心逻辑：shout 方法

### 执行流程

```
1. 检查时间间隔
   ↓
2. 获取下一条消息
   ↓
3. 验证消息有效性
   ↓
4. 模拟键盘操作
   ↓
5. 更新时间戳
```

### 详细步骤

#### 步骤 1: 时间间隔检查

```rust
if Instant::now()
    .duration_since(self.last_shout_time)
    .as_millis()
    < self.shout_interval as u128
{
    return;  // 未到喊话时间，直接返回
}
```

- 计算距离上次喊话的时间
- 如果小于设定的间隔时间，则跳过本次喊话
- 这样可以确保喊话频率不会过快

#### 步骤 2: 获取下一条消息

```rust
guard!(let Some(mut messages) = self.message_iter.as_mut() else { return });
guard!(let Some(message) = messages.next() else { return });
```

- 使用 `guard!` 宏进行安全检查
- 从循环迭代器中获取下一条消息
- 由于使用了 `.cycle()`，消息列表会无限循环
- 如果消息列表为空或迭代器不存在，则返回

#### 步骤 3: 验证消息有效性

```rust
if message.trim().is_empty() {
    return;  // 跳过空消息
}
```

- 检查消息是否为空（去除首尾空白后）
- 避免发送空白消息

#### 步骤 4: 模拟键盘操作

使用 movement 宏模拟游戏内的喊话操作：

```rust
play!(self.movement => [
    // 1. 打开聊天框
    PressKey("Enter"),
    Wait(dur::Random(100..250)),  // 随机等待 100-250ms

    // 2. 输入消息内容
    Type(message.to_string()),
    Wait(dur::Random(100..200)),  // 随机等待 100-200ms

    // 3. 发送消息
    PressKey("Enter"),
    Wait(dur::Random(100..250)),  // 随机等待 100-250ms

    // 4. 关闭聊天框
    PressKey("Escape"),
    Wait(dur::Fixed(100)),         // 固定等待 100ms
]);
```

**操作序列**：
1. **按 Enter** - 打开游戏聊天框
2. **等待随机时间** - 模拟人类输入延迟（100-250ms）
3. **输入文本** - 模拟键盘输入消息内容
4. **等待随机时间** - 模拟输入后的思考时间（100-200ms）
5. **按 Enter** - 发送消息
6. **等待随机时间** - 等待消息发送（100-250ms）
7. **按 Escape** - 关闭聊天框
8. **等待固定时间** - 确保聊天框关闭（100ms）

**随机延迟的意义**：
- 模拟真实玩家的操作习惯
- 避免被游戏系统识别为机器人
- 使操作更加自然

#### 步骤 5: 更新时间戳

```rust
self.last_shout_time = Instant::now();
```

- 记录本次喊话的时间
- 用于下次间隔计算

### 日志记录

```rust
slog::debug!(self.logger, "Shouting"; "message" => &message);
```

- 在发送消息前记录调试日志
- 便于追踪和调试喊话行为

## 配置项 (ShoutConfig)

系统依赖以下配置：

- **shout_messages()**: 返回要喊话的消息列表 `Vec<String>`
- **shout_interval()**: 返回喊话间隔时间 `u64`（毫秒）

## 使用场景

1. **商店广告**：
   - 消息列表：["卖极品装备，便宜出售！", "各种材料，应有尽有！"]
   - 间隔：60000ms（1 分钟）

2. **队伍招募**：
   - 消息列表：["招募打本队友，欢迎加入！", "组队刷怪，速来！"]
   - 间隔：30000ms（30 秒）

3. **公会招人**：
   - 消息列表：["XX公会招新，福利多多！", "活跃公会，欢迎萌新！"]
   - 间隔：120000ms（2 分钟）

## 特点和优势

### 1. 循环机制
- 使用 `.cycle()` 实现消息的无限循环
- 消息列表用完后自动从头开始
- 不需要手动管理消息索引

### 2. 时间控制
- 精确的时间间隔控制
- 避免消息发送过快被屏蔽
- 默认 30 秒间隔符合大多数游戏的聊天限制

### 3. 安全检查
- 使用 `guard!` 宏进行空值检查
- 自动跳过空消息
- 避免发送无效内容

### 4. 自然模拟
- 随机延迟时间
- 模拟真实玩家的操作速度
- 降低被检测为机器人的风险

### 5. 简单可靠
- 代码逻辑清晰，易于维护
- 没有复杂的状态管理
- 功能单一，稳定性高

## 实现细节

### 迭代器的生命周期管理

```rust
message_iter: Option<Box<dyn Iterator<Item = String>>>
```

- 使用 `Box` 进行堆分配，因为迭代器类型在编译时大小未知
- 使用 `Option` 允许在停止时清空迭代器
- `dyn Iterator` 实现了动态分发，支持不同类型的迭代器

### 循环迭代的实现

```rust
self.shown_messages.clone().into_iter().cycle()
```

- `into_iter()` 将 Vec 转换为迭代器
- `cycle()` 创建无限循环迭代器
- `clone()` 避免消耗原始消息列表

## 潜在改进方向

虽然当前实现已经很完善，但可以考虑以下扩展：

1. **随机顺序**：
   - 使用 `shuffle()` 打乱消息顺序
   - 避免消息过于规律

2. **权重系统**：
   - 为不同消息设置出现频率
   - 重要消息可以更频繁地发送

3. **时间段控制**：
   - 根据游戏时间或现实时间调整喊话频率
   - 在人多的时候增加频率

4. **变量替换**：
   - 支持消息中的变量（如当前时间、玩家数量等）
   - 使消息更加动态

5. **频道选择**：
   - 支持不同的聊天频道（世界、公会、队伍等）
   - 根据内容自动选择合适的频道

6. **消息模板**：
   - 支持消息模板和参数填充
   - 提高消息的多样性

## 总结

ShoutBehavior 是一个设计简洁但功能完善的自动喊话模块。它通过循环迭代器实现消息的自动轮播，通过时间间隔控制避免刷屏，通过随机延迟模拟真实玩家行为。代码可读性强，易于配置和使用，是游戏自动化中一个典型的实用功能实现。
