# Movement 移动控制模块

本模块提供了游戏角色的移动控制、按键模拟和动作序列执行功能，是机器人与游戏交互的核心模块。

---

## movement_coordinator.rs - 移动协调器

### 概述
MovementCoordinator 是移动控制的核心实现，负责将高级移动指令转换为具体的按键操作，并通过 Tauri Window 发送到游戏窗口。

### 核心枚举

#### MovementDirection - 移动方向
```rust
pub enum MovementDirection {
    Forward,   // 前进（W 键）
    Backward,  // 后退（S 键）
    Random,    // 随机（前进或后退）
}
```

**用途**：
- 控制角色前进/后退
- Random 模式用于模拟真实玩家的不确定性

#### RotationDirection - 旋转方向
```rust
pub enum RotationDirection {
    Left,    // 左转（Left 键）
    Right,   // 右转（Right 键）
    Random,  // 随机（左转或右转）
}
```

**用途**：
- 控制摄像头/角色旋转
- 搜索敌人时旋转视角
- Random 模式增加行为随机性

#### ActionDuration - 动作持续时间
```rust
pub enum ActionDuration {
    Fixed(u64),           // 固定时长（毫秒）
    Random(Range<u64>),   // 随机时长（范围）
}
```

**方法**：
```rust
fn to_duration(&self, rng: &mut ThreadRng) -> Duration
```

**实现**：
- `Fixed(ms)`：直接转换为 Duration
- `Random(range)`：使用 RNG 在范围内随机生成

**示例**：
```rust
// 固定 500ms
ActionDuration::Fixed(500)

// 100-250ms 之间随机
ActionDuration::Random(100..250)
```

**用途**：
- Fixed：精确控制，如技能施放
- Random：模拟人类行为，防止被检测

#### Movement - 移动指令
```rust
pub enum Movement<'a> {
    Jump,                                          // 跳跃
    Move(MovementDirection, ActionDuration),       // 移动
    Rotate(RotationDirection, ActionDuration),     // 旋转
    PressKey(&'a str),                             // 按下并释放键
    HoldKeyFor(&'a str, ActionDuration),           // 按住键一段时间
    HoldKey(&'a str),                              // 按住键
    HoldKeys(Vec<&'a str>),                        // 按住多个键
    ReleaseKey(&'a str),                           // 释放键
    ReleaseKeys(Vec<&'a str>),                     // 释放多个键
    Repeat(u64, Vec<Movement<'a>>),                // 重复动作序列
    Type(String),                                  // 输入文本
    Wait(ActionDuration),                          // 等待
}
```

**生命周期参数 `'a`**：
- 键名使用 `&'a str` 避免字符串拷贝
- 提高性能，减少内存分配

---

### MovementCoordinator 结构

```rust
pub struct MovementCoordinator {
    rng: rand::rngs::ThreadRng,  // 随机数生成器
    window: Window,                // Tauri 窗口引用
}
```

**字段说明**：
- `rng`：用于生成随机时长和随机方向
- `window`：通过窗口向游戏发送按键事件

---

### 核心方法

#### 1. new(window)
创建新的移动协调器。

```rust
pub fn new(window: Window) -> Self {
    Self {
        rng: rand::thread_rng(),
        window,
    }
}
```

**初始化**：
- 创建线程本地随机数生成器
- 存储窗口引用

#### 2. play<M>(movements)
执行一系列移动指令（核心方法）。

```rust
pub fn play<M>(&mut self, movements: M)
where
    M: AsRef<[Movement<'a>]>
```

**参数**：
- `movements`：可以是 `Vec<Movement>`、`&[Movement]` 或数组

**实现**：
```rust
for movement in movements.as_ref() {
    self.play_single(movement.clone());
}
```

**特点**：
- 顺序执行所有指令
- 每个指令独立处理
- 支持多种容器类型

#### 3. play_single(movement)
执行单个移动指令（内部方法）。

```rust
fn play_single(&mut self, movement: Movement)
```

**实现逻辑**：根据 Movement 类型执行不同操作。

---

### Movement 指令详解

#### Jump - 跳跃
```rust
Movement::Jump
```

**实现**：
```rust
eval_send_key(&window, "Space", KeyMode::Hold);
thread::sleep(Duration::from_millis(500));
eval_send_key(&window, "Space", KeyMode::Release);
```

**流程**：
1. 按住空格键
2. 等待 500ms
3. 释放空格键

**用途**：
- 跨越障碍物
- 跳跃移动
- 规避攻击

#### Move - 移动
```rust
Movement::Move(direction, duration)
```

**实现**：
```rust
let key = match direction {
    Forward => "W",
    Backward => "S",
    Random => if rng.gen() { "W" } else { "S" }
};

eval_send_key(&window, key, KeyMode::Hold);
thread::sleep(duration.to_duration(&mut rng));
eval_send_key(&window, key, KeyMode::Release);
```

**流程**：
1. 根据方向选择键（W/S）
2. 按住键
3. 等待指定时长
4. 释放键

**示例**：
```rust
// 前进 500ms
Move(MovementDirection::Forward, ActionDuration::Fixed(500))

// 随机方向移动 100-200ms
Move(MovementDirection::Random, ActionDuration::Random(100..200))
```

#### Rotate - 旋转
```rust
Movement::Rotate(direction, duration)
```

**实现**：
```rust
let key = match direction {
    Left => "Left",
    Right => "Right",
    Random => if rng.gen() { "Left" } else { "Right" }
};

eval_send_key(&window, key, KeyMode::Hold);
thread::sleep(duration.to_duration(&mut rng));
eval_send_key(&window, key, KeyMode::Release);
```

**流程**：
1. 根据方向选择键（Left/Right）
2. 按住键
3. 等待指定时长
4. 释放键

**用途**：
- 搜索敌人（旋转视角）
- 调整摄像头
- 防止机器人检测（随机旋转）

**示例**：
```rust
// 向右旋转 50ms
Rotate(RotationDirection::Right, ActionDuration::Fixed(50))
```

#### PressKey - 按键
```rust
Movement::PressKey(key)
```

**实现**：
```rust
eval_send_key(&window, key, KeyMode::Press);
```

**特点**：
- 按下并立即释放
- 等同于 Hold + Release
- 用于触发单次动作

**示例**：
```rust
PressKey("Enter")  // 打开聊天框
PressKey("Escape") // 取消目标
PressKey("Z")      // 跟随目标
```

#### HoldKeyFor - 按住键一段时间
```rust
Movement::HoldKeyFor(key, duration)
```

**实现**：
```rust
eval_send_key(&window, key, KeyMode::Hold);
thread::sleep(duration.to_duration(&mut rng));
eval_send_key(&window, key, KeyMode::Release);
```

**流程**：
1. 按住键
2. 等待指定时长
3. 释放键

**示例**：
```rust
// 按住 S 键后退 50ms
HoldKeyFor("S", ActionDuration::Fixed(50))

// 按住 A 键左移 100-200ms
HoldKeyFor("A", ActionDuration::Random(100..200))
```

#### HoldKey - 按住键（不释放）
```rust
Movement::HoldKey(key)
```

**实现**：
```rust
eval_send_key(&window, key, KeyMode::Hold);
```

**特点**：
- 只按住，不释放
- 需要后续手动释放
- 用于组合按键

**示例**：
```rust
HoldKey("W")      // 开始前进
// ... 其他操作
ReleaseKey("W")   // 停止前进
```

#### HoldKeys - 按住多个键
```rust
Movement::HoldKeys(keys)
```

**实现**：
```rust
for key in keys {
    eval_send_key(&window, key, KeyMode::Hold);
}
```

**用途**：
- 组合移动（W + Space = 跳跃前进）
- 复杂操作（W + D = 右前方移动）

**示例**：
```rust
// 跳跃 + 前进 + 右转
HoldKeys(vec!["W", "Space", "D"])

// 后续需要释放
ReleaseKeys(vec!["W", "Space", "D"])
```

#### ReleaseKey - 释放键
```rust
Movement::ReleaseKey(key)
```

**实现**：
```rust
eval_send_key(&window, key, KeyMode::Release);
```

**用途**：
- 停止按住的键
- 与 HoldKey 配对使用

#### ReleaseKeys - 释放多个键
```rust
Movement::ReleaseKeys(keys)
```

**实现**：
```rust
for key in keys {
    eval_send_key(&window, key, KeyMode::Release);
}
```

**用途**：
- 停止组合按键
- 与 HoldKeys 配对使用

#### Type - 输入文本
```rust
Movement::Type(text)
```

**实现**：
```rust
eval_send_message(&window, &text);
```

**用途**：
- 发送聊天消息
- 输入命令
- 自动喊话

**示例**：
```rust
// 打开聊天框
PressKey("Enter"),
Wait(Fixed(100)),

// 输入消息
Type("卖装备，便宜出售！".to_string()),
Wait(Fixed(100)),

// 发送
PressKey("Enter")
```

#### Wait - 等待
```rust
Movement::Wait(duration)
```

**实现**：
```rust
thread::sleep(duration.to_duration(&mut rng));
```

**用途**：
- 操作间隔
- 等待动画完成
- 模拟人类思考时间

**示例**：
```rust
// 固定等待 100ms
Wait(ActionDuration::Fixed(100))

// 随机等待 100-250ms
Wait(ActionDuration::Random(100..250))
```

#### Repeat - 重复执行
```rust
Movement::Repeat(times, movements)
```

**实现**：
```rust
for _ in 0..times {
    self.play(&movements);
}
```

**用途**：
- 重复拾取动作
- 循环攻击
- 连续使用技能

**示例**：
```rust
// 连续按 10 次拾取键
Repeat(10, vec![
    PressKey("F"),
    Wait(ActionDuration::Fixed(300))
])
```

---

### 已注释的代码

#### with_probability
```rust
// pub fn with_probability<F>(&mut self, probability: f64, func: F)
```

**功能**：基于概率执行函数。

**推测原因**：
- 功能可能已集成到其他地方
- 或者不再需要概率控制

---

### 使用示例

#### 1. 跳跃前进
```rust
play!(movement => [
    HoldKeys(vec!["W", "Space"]),
    Wait(dur::Fixed(800)),
    ReleaseKeys(vec!["Space", "W"])
]);
```

#### 2. 圆形移动
```rust
play!(movement => [
    HoldKeys(vec!["W", "Space", "D"]),
    Wait(dur::Fixed(100)),
    ReleaseKey("D"),
    Wait(dur::Fixed(500)),
    ReleaseKeys(vec!["Space", "W"])
]);
```

#### 3. 旋转搜索
```rust
play!(movement => [
    Rotate(rot::Right, dur::Fixed(50)),
    Wait(dur::Fixed(50))
]);
```

#### 4. 发送消息
```rust
play!(movement => [
    PressKey("Enter"),
    Wait(dur::Random(100..250)),
    Type(message.to_string()),
    Wait(dur::Random(100..200)),
    PressKey("Enter"),
    Wait(dur::Random(100..250)),
    PressKey("Escape"),
    Wait(dur::Fixed(100))
]);
```

#### 5. 障碍物规避
```rust
play!(movement => [
    HoldKeys(vec!["W", "Space"]),
    HoldKeyFor("A", dur::Fixed(200)),
    Wait(dur::Fixed(800)),
    ReleaseKeys(vec!["Space", "W"]),
    PressKey("Z")
]);
```

---

### 设计特点

#### 1. 声明式 API
- 使用枚举定义动作
- 清晰的意图表达
- 易于理解和维护

#### 2. 组合能力强
- 支持复杂动作序列
- 可以嵌套和重复
- 灵活的时长控制

#### 3. 随机性支持
- Random 方向
- Random 时长
- 模拟真实玩家行为

#### 4. 线程安全
- 内部使用 thread::sleep
- 顺序执行，避免冲突
- 适合单线程调用

#### 5. 零成本抽象
- 使用 `&str` 避免拷贝
- 枚举编译时优化
- 高效的执行性能

---

### 与平台层的交互

MovementCoordinator 通过以下平台函数与游戏交互：

#### eval_send_key(window, key, mode)
发送键盘事件。

**KeyMode**：
- `Press`：按下并释放
- `Hold`：按住
- `Release`：释放

**示例**：
```rust
eval_send_key(&window, "W", KeyMode::Hold);      // 开始前进
thread::sleep(Duration::from_millis(500));       // 前进 500ms
eval_send_key(&window, "W", KeyMode::Release);   // 停止前进
```

#### eval_send_message(window, text)
发送文本消息。

**用途**：
- 输入聊天消息
- 模拟键盘输入

**示例**：
```rust
eval_send_message(&window, "Hello World!");
```

---

## movement_accessor.rs - 移动访问器

### 概述
MovementAccessor 提供对 MovementCoordinator 的线程安全访问包装，使用互斥锁确保并发安全。

### 核心结构

```rust
pub struct MovementAccessor {
    coordinator: Mutex<MovementCoordinator>,
}
```

**设计目的**：
- 提供线程安全的访问
- 封装锁的管理
- 简化 API 调用

### 方法

#### 1. new(window)
创建新的移动访问器。

```rust
pub fn new(window: Window) -> Self {
    Self {
        coordinator: Mutex::new(MovementCoordinator::new(window))
    }
}
```

**实现**：
- 创建 MovementCoordinator
- 用 Mutex 包装
- 返回访问器实例

**Mutex 选择**：
- 使用 `parking_lot::Mutex`
- 比标准库 Mutex 性能更好
- 支持更好的调试信息

#### 2. schedule<F>(func)
调度执行函数（核心方法）。

```rust
pub fn schedule<F>(&self, func: F)
where
    F: Fn(&mut MovementCoordinator)
```

**参数**：
- `func`：接收 `&mut MovementCoordinator` 的闭包

**实现**：
```rust
let mut coordinator = self.coordinator.lock();
func(&mut coordinator);
```

**流程**：
1. 获取互斥锁（阻塞等待）
2. 执行闭包
3. 自动释放锁（离开作用域）

**使用示例**：
```rust
accessor.schedule(|coordinator| {
    coordinator.play([
        Movement::PressKey("Z"),
    ]);
});
```

---

### 宏支持：play!

在实际使用中，通常配合 `play!` 宏使用：

```rust
play!(self.movement => [
    PressKey("Enter"),
    Wait(dur::Fixed(100))
]);
```

**宏展开**（推测）：
```rust
self.movement.schedule(|coordinator| {
    coordinator.play([
        Movement::PressKey("Enter"),
        Movement::Wait(ActionDuration::Fixed(100))
    ]);
});
```

**优势**：
- 简洁的语法
- 自动处理锁
- 类型安全

---

### 线程安全性

#### Mutex 的作用
```rust
coordinator: Mutex<MovementCoordinator>
```

**保证**：
- 同一时间只有一个线程可以访问 coordinator
- 防止按键序列冲突
- 确保动作顺序执行

**场景示例**：
```rust
// 线程 1
accessor.schedule(|c| {
    c.play([Move(Forward, Fixed(500))]);  // 前进 500ms
});

// 线程 2（同时调用）
accessor.schedule(|c| {
    c.play([Rotate(Right, Fixed(100))]);  // 旋转 100ms
});

// Mutex 确保一个执行完才执行另一个
// 避免按键冲突
```

#### 死锁预防
- `schedule` 方法不嵌套调用
- 锁的作用域明确
- 自动释放（RAII）

---

### 设计模式

#### 1. 外观模式（Facade）
- MovementAccessor 作为外观
- 隐藏 Mutex 细节
- 提供简单 API

#### 2. 策略模式（Strategy）
- schedule 接收闭包
- 灵活的执行策略
- 可以传递不同的动作序列

#### 3. 命令模式（Command）
- Movement 枚举作为命令
- 封装操作请求
- 支持撤销和重做（潜在扩展）

---

### 已注释的代码

```rust
// use crate::platform::PlatformAccessor;
// platform: &'a PlatformAccessor<'a>
```

**推测**：
- 早期版本可能有 PlatformAccessor
- 现在直接使用 Window
- 简化了架构

---

### 使用场景

#### 1. Behavior 中使用
```rust
pub struct FarmingBehavior<'a> {
    movement: &'a MovementAccessor,
    // ...
}

impl FarmingBehavior {
    fn follow_target(&self) {
        play!(self.movement => [
            PressKey("Z")
        ]);
    }

    fn avoid_obstacle(&self) {
        play!(self.movement => [
            HoldKeys(vec!["W", "Space"]),
            Wait(dur::Fixed(800)),
            ReleaseKeys(vec!["Space", "W"])
        ]);
    }
}
```

#### 2. 多线程环境
```rust
let accessor = Arc::new(MovementAccessor::new(window));

// 线程 1：主循环
let accessor_clone = accessor.clone();
thread::spawn(move || {
    loop {
        accessor_clone.schedule(|c| {
            c.play([Move(Forward, Fixed(100))]);
        });
    }
});

// 线程 2：紧急操作
accessor.schedule(|c| {
    c.play([Jump]);
});
```

---

## 模块关系图

```
MovementAccessor
  └── Mutex<MovementCoordinator>
        ├── ThreadRng (随机数)
        ├── Window (Tauri 窗口)
        └── Movement 指令
              ├── MovementDirection
              ├── RotationDirection
              └── ActionDuration
                    └── Platform 层
                          ├── eval_send_key
                          └── eval_send_message
```

## 数据流

```
Behavior 行为
  ↓
play! 宏
  ↓
MovementAccessor::schedule
  ↓
Mutex 加锁
  ↓
MovementCoordinator::play
  ↓
play_single (逐个执行)
  ↓
eval_send_key / eval_send_message
  ↓
Tauri Window
  ↓
游戏窗口
```

## prelude 模块（推测）

在实际代码中常见：

```rust
use crate::movement::prelude::*;
```

**推测内容**：
```rust
pub mod prelude {
    pub use super::Movement::*;
    pub use super::MovementDirection as mov;
    pub use super::RotationDirection as rot;
    pub use super::ActionDuration as dur;
}
```

**使用效果**：
```rust
// 不使用 prelude
Movement::Rotate(
    RotationDirection::Right,
    ActionDuration::Fixed(50)
)

// 使用 prelude
Rotate(rot::Right, dur::Fixed(50))
```

---

## 完整示例：圆形移动模式

```rust
fn move_circle_pattern(&self, rotation_duration: u64) {
    use crate::movement::prelude::*;

    play!(self.movement => [
        // 按住前进 + 跳跃 + 右转
        HoldKeys(vec!["W", "Space", "D"]),
        Wait(dur::Fixed(rotation_duration)),

        // 释放右转，保持前进和跳跃
        ReleaseKey("D"),
        Wait(dur::Fixed(20)),

        // 释放所有键
        ReleaseKeys(vec!["Space", "W"]),

        // 后退一小段
        HoldKeyFor("S", dur::Fixed(50))
    ]);
}
```

**效果**：
1. 跳跃并向右前方移动
2. 逐渐转为前进
3. 停止移动
4. 轻微后退调整位置

**用途**：
- 找不到怪物时搜索
- 绕过障碍物
- 保持在特定区域

---

## 完整示例：自动喊话

```rust
fn shout(&mut self, message: String) {
    use crate::movement::prelude::*;

    play!(self.movement => [
        // 打开聊天框
        PressKey("Enter"),
        Wait(dur::Random(100..250)),

        // 输入消息
        Type(message),
        Wait(dur::Random(100..200)),

        // 发送消息
        PressKey("Enter"),
        Wait(dur::Random(100..250)),

        // 关闭聊天框
        PressKey("Escape"),
        Wait(dur::Fixed(100))
    ]);
}
```

**特点**：
- 随机等待时间模拟人类
- 完整的聊天流程
- 自动清理状态

---

## 设计优势

### 1. 分层清晰
- **访问层**（MovementAccessor）：线程安全
- **协调层**（MovementCoordinator）：指令执行
- **指令层**（Movement）：动作定义
- **平台层**（eval_send_key）：系统调用

### 2. 类型安全
- 使用枚举避免魔法字符串
- 编译时检查
- IDE 自动补全

### 3. 易于测试
- 纯数据结构（Movement）
- 可以模拟 Window
- 单元测试友好

### 4. 性能优良
- 零成本抽象
- 最小化锁竞争
- 高效的指令执行

### 5. 可扩展性
- 易于添加新指令
- 支持自定义动作
- 灵活的组合方式

---

## 最佳实践

### 1. 使用 prelude
```rust
use crate::movement::prelude::*;
```
简化代码，提高可读性。

### 2. 随机时长
```rust
Wait(dur::Random(100..250))
```
模拟人类行为，防止检测。

### 3. 组合键释放
```rust
HoldKeys(vec!["W", "Space", "D"]);
// ... 操作
ReleaseKeys(vec!["D", "Space", "W"]);
```
确保所有按键都释放。

### 4. 等待动画
```rust
PressKey("Z");
Wait(dur::Fixed(100));  // 等待跟随动画
```
给游戏响应时间。

### 5. 错误恢复
```rust
// 确保键释放
ReleaseKeys(vec!["W", "A", "S", "D", "Space"]);
```
在异常情况下清理状态。

---

## 总结

### Movement 模块的作用

1. **按键模拟**：
   - 提供完整的键盘控制
   - 支持组合按键
   - 精确的时间控制

2. **动作抽象**：
   - 高级移动指令
   - 声明式 API
   - 易于理解和维护

3. **线程安全**：
   - Mutex 保护
   - 避免按键冲突
   - 支持并发调用

4. **随机化**：
   - 模拟人类行为
   - 防止机器人检测
   - 增加行为多样性

### 核心优势

1. **简洁的 API**：
   - 使用 Movement 枚举
   - play! 宏语法糖
   - 直观的表达方式

2. **灵活的组合**：
   - 支持复杂序列
   - 可重复执行
   - 嵌套能力强

3. **高性能**：
   - 零成本抽象
   - 编译时优化
   - 最小化开销

4. **可维护性**：
   - 清晰的分层
   - 单一职责
   - 易于扩展

### 使用建议

1. **始终使用 prelude**
2. **为人类行为添加随机性**
3. **确保按键最终释放**
4. **给游戏足够的响应时间**
5. **避免过于频繁的操作**
6. **在异常处理中清理键状态**
