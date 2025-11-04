# Utils 工具模块

本模块提供了通用的工具函数和辅助结构，包括时间格式化和性能计时功能。

---

## datetime.rs - 日期时间工具

### 概述
提供日期时间相关的格式化工具，用于统计数据的展示和时间信息的格式化输出。

### 核心结构

#### DateTime
```rust
pub struct DateTime {}
```

**设计特点**：
- 无字段的结构体
- 只包含静态方法
- 类似于命名空间的作用
- 不需要实例化

---

### 方法

#### 1. format_float(float, precision) - 格式化浮点数

```rust
pub fn format_float(float: f32, precision: usize) -> f32
```

**功能**：将浮点数格式化为指定精度，并返回新的浮点数。

**参数**：
- `float: f32` - 要格式化的浮点数
- `precision: usize` - 小数位数精度

**返回**：
- `f32` - 格式化后的浮点数

**实现原理**：
```rust
// 1. 格式化为字符串，指定精度
format!("{:.prec$}", float, prec = precision)

// 2. 解析回浮点数
.parse::<f32>()

// 3. 如果解析失败，返回 0.0
.unwrap_or_default()
```

**步骤详解**：

##### 步骤 1：格式化为字符串
```rust
format!("{:.prec$}", float, prec = precision)
```
- 使用格式化宏
- `{:.prec$}` 是动态精度语法
- `prec = precision` 指定精度参数

**示例**：
```rust
format!("{:.2}", 3.14159)  // "3.14"
format!("{:.0}", 3.14159)  // "3"
format!("{:.4}", 3.14159)  // "3.1416"
```

##### 步骤 2：解析回浮点数
```rust
.parse::<f32>()
```
- 将字符串解析为 f32
- 返回 `Result<f32, ParseFloatError>`

##### 步骤 3：错误处理
```rust
.unwrap_or_default()
```
- 如果解析失败，返回 `f32::default()` (0.0)
- 简单的错误恢复策略

**使用场景**：

##### 场景 1：格式化击杀效率
```rust
// 计算每分钟击杀数
let kill_per_minute = 60.0 / (search_time + kill_time);
// 4.567890 → 4.57（保留 2 位小数）
let formatted = DateTime::format_float(kill_per_minute, 2);
```

##### 场景 2：格式化百分比
```rust
// 计算命中率
let accuracy = hits as f32 / total as f32 * 100.0;
// 87.65432 → 87.65
let formatted = DateTime::format_float(accuracy, 2);
```

##### 场景 3：去除小数
```rust
// 四舍五入取整
let value = 123.789;
// 123.789 → 124.0
let rounded = DateTime::format_float(value, 0);
```

**示例**：
```rust
DateTime::format_float(3.14159, 2);   // 3.14
DateTime::format_float(123.456, 0);   // 123.0
DateTime::format_float(0.1234, 3);    // 0.123
DateTime::format_float(99.999, 1);    // 100.0 (四舍五入)
```

**注意事项**：
- 会进行四舍五入
- 精度为 0 时取整
- 解析失败返回 0.0（理论上不会失败）

---

#### 2. format_time(elapsed) - 格式化时间

```rust
pub fn format_time(elapsed: Duration) -> String
```

**功能**：将时间间隔格式化为 "HH:MM:SS" 格式的字符串。

**参数**：
- `elapsed: Duration` - 时间间隔

**返回**：
- `String` - 格式化的时间字符串，如 "01:23:45"

**实现步骤**：

##### 步骤 1：计算时分秒
```rust
let seconds = elapsed.as_secs() % 60;            // 秒数 (0-59)
let minutes = (elapsed.as_secs() / 60) % 60;    // 分钟数 (0-59)
let hours = (elapsed.as_secs() / 60) / 60;      // 小时数
```

**计算逻辑**：
- `as_secs()` 获取总秒数
- 秒数：总秒数 % 60
- 分钟数：(总秒数 / 60) % 60
- 小时数：总秒数 / 3600

**示例计算**：
```rust
// elapsed = 3665 秒 (1 小时 1 分 5 秒)
seconds = 3665 % 60 = 5
minutes = (3665 / 60) % 60 = 61 % 60 = 1
hours = 3665 / 3600 = 1
```

##### 步骤 2：格式化秒数
```rust
let seconds_formatted = {
    if seconds < 10 {
        format!("0{}", seconds)  // 补零：5 → "05"
    } else {
        seconds.to_string()      // 直接转换：15 → "15"
    }
};
```

**补零规则**：
- 0-9：补零为 "00"-"09"
- 10-59：直接转换为 "10"-"59"

##### 步骤 3：格式化分钟数
```rust
let minutes_formatted = {
    if minutes < 10 {
        format!("0{}", minutes)  // 补零：3 → "03"
    } else {
        minutes.to_string()      // 直接转换：45 → "45"
    }
};
```

##### 步骤 4：格式化小时数
```rust
let hours_formatted = {
    if hours < 10 {
        format!("0{}", hours)    // 补零：2 → "02"
    } else {
        hours.to_string()        // 直接转换：12 → "12"
    }
};
```

##### 步骤 5：组合结果
```rust
format!(
    "{}:{}:{}",
    hours_formatted,
    minutes_formatted,
    seconds_formatted
)
```

**输出格式**：`"HH:MM:SS"`

**使用场景**：

##### 场景 1：显示运行时间
```rust
let start_time = Instant::now();
// ... 运行一段时间
let elapsed = start_time.elapsed();
let formatted = DateTime::format_time(elapsed);
println!("运行时间: {}", formatted);  // "运行时间: 01:23:45"
```

##### 场景 2：显示击杀统计
```rust
fn after_enemy_kill_debug(&mut self, frontend_info: &mut FrontendInfo) {
    let started_elapsed = self.start_time.elapsed();
    let started_formatted = DateTime::format_time(started_elapsed);

    let elapsed = format!(
        "Elapsed time : since start {} to kill {} to find {} ",
        started_formatted,
        elapsed_time_to_kill_string,
        elapsed_search_time_string
    );
    slog::debug!(self.logger, "Monster was killed {}", elapsed);
}
```

##### 场景 3：计算剩余时间
```rust
let total_time = Duration::from_secs(7200);  // 2 小时
let elapsed_time = Instant::now().elapsed();
let remaining = total_time - elapsed_time;
let formatted = DateTime::format_time(remaining);
println!("剩余时间: {}", formatted);  // "剩余时间: 00:45:30"
```

**示例**：
```rust
// 5 秒
DateTime::format_time(Duration::from_secs(5));
// 输出: "00:00:05"

// 1 分 30 秒
DateTime::format_time(Duration::from_secs(90));
// 输出: "00:01:30"

// 1 小时 23 分 45 秒
DateTime::format_time(Duration::from_secs(5025));
// 输出: "01:23:45"

// 25 小时 5 分 10 秒
DateTime::format_time(Duration::from_secs(90310));
// 输出: "25:05:10"
```

**特点**：
- 自动补零，保持格式一致
- 小时数没有上限（可以超过 24）
- 始终显示三个部分（时:分:秒）
- 适合显示经过的时间（而非时钟时间）

---

### 设计特点

#### 1. 静态方法设计
```rust
pub struct DateTime {}
```
- 无需实例化
- 直接调用：`DateTime::format_float(...)`
- 类似于工具类或命名空间

**优势**：
- 简单直接
- 不占用内存（无状态）
- 语义清晰

#### 2. 精度控制
```rust
format_float(value, precision)
```
- 灵活的精度参数
- 支持任意精度
- 自动四舍五入

#### 3. 可读性优先
```rust
format_time() -> "HH:MM:SS"
```
- 人类友好的格式
- 自动补零对齐
- 易于阅读和比较

#### 4. 错误恢复
```rust
.unwrap_or_default()
```
- 简单的错误处理
- 失败时返回合理默认值
- 不会 panic

---

### 使用示例

#### 完整的统计显示
```rust
fn display_statistics(&self) {
    // 格式化运行时间
    let runtime = DateTime::format_time(self.start_time.elapsed());
    println!("运行时间: {}", runtime);

    // 格式化效率数据
    let kill_per_min = self.total_kills as f32 / runtime_minutes;
    let formatted_kpm = DateTime::format_float(kill_per_min, 2);
    println!("效率: {} 击杀/分钟", formatted_kpm);

    // 格式化百分比
    let success_rate = self.successful_attacks as f32 / self.total_attacks as f32 * 100.0;
    let formatted_rate = DateTime::format_float(success_rate, 1);
    println!("成功率: {}%", formatted_rate);
}
```

#### 性能统计
```rust
fn calculate_performance(&self) -> (f32, f32) {
    let total_time = self.search_time + self.kill_time;
    let kill_per_minute = 60.0 / total_time;
    let kill_per_hour = kill_per_minute * 60.0;

    (
        DateTime::format_float(kill_per_minute, 2),
        DateTime::format_float(kill_per_hour, 0)
    )
}
```

---

## timer.rs - 性能计时器

### 概述
提供性能计时功能，用于测量代码执行时间，帮助开发者进行性能分析和优化。这是一个开发调试工具。

### 核心结构

#### Timer
```rust
pub struct Timer {
    label: String,                  // 计时器标签
    start: Instant,                 // 开始时间
    is_silenced: RefCell<bool>,     // 是否静音
}
```

**字段说明**：

##### label
- 计时器的名称/标签
- 用于标识不同的计时器
- 输出时显示

##### start
- 计时器创建时的时间点
- 使用 `Instant::now()` 获取
- 用于计算经过时间

##### is_silenced
- 是否静音（不输出）
- 使用 `RefCell<bool>` 实现内部可变性
- 即使在 `&self` 方法中也能修改

**为什么使用 RefCell**：
```rust
pub fn silence(&self) {  // 注意这里是 &self，不是 &mut self
    *self.is_silenced.borrow_mut() = true;
}
```
- `silence()` 接收 `&self` 而非 `&mut self`
- 但需要修改 `is_silenced` 字段
- `RefCell` 提供运行时可变性检查

---

### 方法

#### 1. start_new<S>(label) - 创建计时器

```rust
pub fn start_new<S>(label: S) -> Timer
where
    S: ToString
```

**功能**：创建并启动一个新的计时器。

**参数**：
- `label: S` - 计时器标签，任何可以转换为 String 的类型

**返回**：
- `Timer` - 新的计时器实例

**实现**：
```rust
Timer {
    label: label.to_string(),           // 转换标签为 String
    start: Instant::now(),              // 记录当前时间
    is_silenced: RefCell::new(false),   // 默认不静音
}
```

**泛型约束 `S: ToString`**：
- 可以接受 `&str`、`String`、`i32` 等
- 灵活的标签类型
- 自动转换为 String

**使用示例**：
```rust
// 使用字符串字面量
let timer = Timer::start_new("image_analysis");

// 使用 String
let label = String::from("monster_detection");
let timer = Timer::start_new(label);

// 使用格式化字符串
let timer = Timer::start_new(format!("attack_{}", mob_id));
```

---

#### 2. lap(file, line) - 中间计时点

```rust
pub fn lap(&self, file: &'static str, line: u32)
```

**功能**：输出中间计时点的时间，用于测量代码段之间的时间。

**参数**：
- `file: &'static str` - 文件名（通常使用 `file!()` 宏）
- `line: u32` - 行号（通常使用 `line!()` 宏）

**实现**：
```rust
// 1. 检查是否静音
if *self.is_silenced.borrow() {
    return;
}

// 2. 检查环境变量
if std::env::var("NEUZ_TIMERS").is_err() {
    return;
}

// 3. 输出计时信息
println!(
    "[{} {}${}] took {:?}",
    self.label,
    file,
    line,
    self.elapsed()
);
```

**条件检查**：

##### 检查 1：是否静音
```rust
if *self.is_silenced.borrow() {
    return;
}
```
- 如果计时器被静音，直接返回
- 不输出任何信息

##### 检查 2：环境变量
```rust
if std::env::var("NEUZ_TIMERS").is_err() {
    return;
}
```
- 检查 `NEUZ_TIMERS` 环境变量是否设置
- 只有设置了才输出
- 用于控制计时器的全局开关

**环境变量控制**：
```bash
# 启用计时器
export NEUZ_TIMERS=1

# 或者临时启用
NEUZ_TIMERS=1 cargo run

# 不设置则不输出
cargo run
```

**输出格式**：
```
[label file$line] took duration
```

**示例输出**：
```
[image_analysis src/image_analyzer.rs$123] took 45.2ms
[monster_detection src/behavior.rs$456] took 12.8ms
```

**使用示例**：
```rust
let timer = Timer::start_new("complex_calculation");

// 第一阶段
do_phase_1();
timer.lap(file!(), line!());  // 输出: [complex_calculation src/main.rs$10] took 10ms

// 第二阶段
do_phase_2();
timer.lap(file!(), line!());  // 输出: [complex_calculation src/main.rs$14] took 25ms

// 第三阶段
do_phase_3();
// drop(timer) 时自动输出总时间
```

**特点**：
- 带文件名和行号，便于定位
- 支持多个中间计时点
- 可选的输出（环境变量控制）

**标记为 dead_code**：
```rust
#[allow(dead_code)]
```
- 表明这个方法可能未被使用
- 但保留以供未来使用
- 不会触发编译警告

---

#### 3. silence() - 静音计时器

```rust
pub fn silence(&self)
```

**功能**：静音计时器，使其不再输出任何信息。

**实现**：
```rust
*self.is_silenced.borrow_mut() = true;
```

**使用 RefCell**：
- `borrow_mut()` 获取可变引用
- 修改内部的 bool 值
- 即使方法签名是 `&self`

**使用场景**：

##### 场景 1：条件性静音
```rust
let timer = Timer::start_new("optional_operation");

if !should_profile {
    timer.silence();  // 静音，不输出
}

// 执行操作
do_work();

// drop(timer) 时如果静音则不输出
```

##### 场景 2：选择性测量
```rust
fn process_items(items: &[Item]) {
    for item in items {
        let timer = Timer::start_new("process_item");

        if item.is_fast() {
            timer.silence();  // 快速项不需要计时
        }

        process(item);
    }
}
```

##### 场景 3：错误时静音
```rust
let timer = Timer::start_new("risky_operation");

match do_operation() {
    Ok(_) => {
        // 成功时输出计时
    }
    Err(_) => {
        timer.silence();  // 失败时不输出
    }
}
```

---

#### 4. elapsed() - 获取经过时间

```rust
pub fn elapsed(&self) -> Duration
```

**功能**：获取自计时器创建以来经过的时间。

**实现**：
```rust
self.start.elapsed()
```
- 调用 `Instant::elapsed()` 方法
- 返回 `Duration` 类型

**使用示例**：
```rust
let timer = Timer::start_new("operation");

// 执行一些操作
do_work();

// 获取经过时间
let duration = timer.elapsed();
println!("操作耗时: {:?}", duration);  // "操作耗时: 123.456ms"
```

---

#### 5. report() - 报告计时结果

```rust
pub fn report(&self)
```

**功能**：输出计时器的最终报告。

**实现**：
```rust
// 1. 检查是否静音
if *self.is_silenced.borrow() {
    return;
}

// 2. 检查环境变量
if std::env::var("NEUZ_TIMERS").is_err() {
    return;
}

// 3. 输出报告
println!("[{}] took {:?}", self.label, self.elapsed());
```

**输出格式**：
```
[label] took duration
```

**示例输出**：
```
[image_analysis] took 45.2ms
[monster_detection] took 12.8ms
[full_iteration] took 58.5ms
```

**与 lap() 的区别**：
```rust
// lap() 输出
[label file$line] took duration

// report() 输出
[label] took duration
```
- `report()` 没有文件名和行号
- 更简洁，用于最终报告

---

### Drop Trait 实现

```rust
impl Drop for Timer {
    fn drop(&mut self) {
        self.report();
    }
}
```

**功能**：自动报告计时结果。

**原理**：
- 当 `Timer` 离开作用域时自动调用 `drop()`
- `drop()` 调用 `report()`
- 自动输出计时结果

**RAII 模式**（Resource Acquisition Is Initialization）：
- 资源获取即初始化
- 资源释放即报告
- 不需要手动调用 `report()`

**使用示例**：

##### 示例 1：自动报告
```rust
{
    let timer = Timer::start_new("block_execution");
    // 执行操作
    do_work();
}  // timer 离开作用域，自动输出: [block_execution] took 123ms
```

##### 示例 2：函数计时
```rust
fn expensive_function() {
    let _timer = Timer::start_new("expensive_function");
    // 函数体
    // ...
}  // 函数返回时自动输出计时
```

##### 示例 3：条件作用域
```rust
if should_profile {
    let _timer = Timer::start_new("conditional_block");
    // 只有在条件满足时才计时
    do_special_work();
}  // 离开 if 块时自动输出
```

**命名约定**：
```rust
let _timer = Timer::start_new("...");
```
- 使用 `_timer` 前缀
- 表示变量未被直接使用
- 只依赖 Drop 行为
- 避免编译器警告

---

### 环境变量控制

#### NEUZ_TIMERS
- 全局开关，控制所有计时器输出
- 未设置：不输出任何计时信息
- 已设置（任何值）：启用计时器输出

**设置方式**：

##### Linux/macOS
```bash
# 临时启用（当前会话）
export NEUZ_TIMERS=1

# 临时启用（单次运行）
NEUZ_TIMERS=1 cargo run

# 永久启用（添加到 .bashrc 或 .zshrc）
echo "export NEUZ_TIMERS=1" >> ~/.bashrc
```

##### Windows (PowerShell)
```powershell
# 临时启用
$env:NEUZ_TIMERS="1"

# 运行程序
cargo run
```

##### Windows (CMD)
```cmd
# 临时启用
set NEUZ_TIMERS=1

# 运行程序
cargo run
```

**检查逻辑**：
```rust
if std::env::var("NEUZ_TIMERS").is_err() {
    return;  // 变量未设置，不输出
}
```

**优势**：
- 不需要重新编译
- 运行时控制
- 生产环境可以关闭
- 开发环境可以启用

---

### 使用示例

#### 示例 1：基本使用
```rust
fn process_image(image: &Image) {
    let _timer = Timer::start_new("process_image");

    // 图像处理代码
    let result = analyze(image);

    // 函数结束时自动输出: [process_image] took 45ms
}
```

#### 示例 2：多阶段计时
```rust
fn complex_operation() {
    let timer = Timer::start_new("complex_operation");

    // 阶段 1
    phase1();
    timer.lap(file!(), line!());  // [complex_operation src/main.rs$5] took 10ms

    // 阶段 2
    phase2();
    timer.lap(file!(), line!());  // [complex_operation src/main.rs$9] took 30ms

    // 阶段 3
    phase3();
    // 结束时自动输出: [complex_operation] took 50ms
}
```

#### 示例 3：条件计时
```rust
fn process_with_profiling(enable_profiling: bool) {
    let timer = Timer::start_new("process");

    if !enable_profiling {
        timer.silence();  // 禁用时静音
    }

    // 执行操作
    do_work();

    // 只有 enable_profiling=true 时才输出
}
```

#### 示例 4：嵌套计时
```rust
fn outer_function() {
    let _outer = Timer::start_new("outer_function");

    {
        let _inner1 = Timer::start_new("inner_task_1");
        task1();
    }  // 输出: [inner_task_1] took 10ms

    {
        let _inner2 = Timer::start_new("inner_task_2");
        task2();
    }  // 输出: [inner_task_2] took 20ms

}  // 输出: [outer_function] took 30ms
```

#### 示例 5：循环中的计时
```rust
fn process_items(items: &[Item]) {
    for (i, item) in items.iter().enumerate() {
        let timer = Timer::start_new(format!("item_{}", i));
        process(item);
        // 每个 item 都会输出独立的计时
    }
}
```

#### 示例 6：实际应用（图像分析）
```rust
fn analyze_game_state(&mut self, image: &ImageAnalyzer) {
    let _total = Timer::start_new("analyze_game_state");

    {
        let _stats = Timer::start_new("update_stats");
        self.update_stats(image);
    }

    {
        let _mobs = Timer::start_new("identify_mobs");
        let mobs = image.identify_mobs(config);
    }

    {
        let _target = Timer::start_new("find_target");
        let target = self.find_closest_target(&mobs);
    }
}

// 输出示例（假设 NEUZ_TIMERS 已设置）：
// [update_stats] took 5ms
// [identify_mobs] took 15ms
// [find_target] took 3ms
// [analyze_game_state] took 23ms
```

---

### 设计特点

#### 1. RAII 模式
- 自动资源管理
- 离开作用域自动报告
- 不会忘记调用 `report()`

#### 2. 内部可变性
```rust
is_silenced: RefCell<bool>
```
- 允许在 `&self` 方法中修改状态
- 运行时借用检查
- 灵活的 API 设计

#### 3. 环境变量控制
- 全局开关
- 运行时控制
- 不需要重新编译
- 生产/开发环境分离

#### 4. 灵活的标签
```rust
where S: ToString
```
- 支持多种标签类型
- 自动转换
- 方便使用

#### 5. 零成本抽象（关闭时）
- 环境变量未设置时，提前返回
- 不进行格式化
- 不输出
- 几乎零开销

---

### 性能分析工作流

#### 1. 启用计时器
```bash
export NEUZ_TIMERS=1
cargo run
```

#### 2. 查看输出
```
[image_analysis] took 45.2ms
[monster_detection] took 12.8ms
[state_update] took 5.3ms
[full_iteration] took 63.5ms
```

#### 3. 识别瓶颈
- 查找耗时最长的操作
- 使用 `lap()` 细化测量
- 定位性能问题

#### 4. 优化代码
- 针对性优化慢速部分
- 重新测量验证改进

#### 5. 生产环境
```bash
# 不设置 NEUZ_TIMERS
cargo build --release
./program
# 没有计时输出，零性能影响
```

---

### 与日志系统的关系

#### 已注释的日志依赖
```rust
//use slog::Logger;
```
- 早期版本可能使用 slog 日志
- 现在使用简单的 `println!`
- 更轻量，依赖更少

#### 可能的扩展
```rust
pub struct Timer {
    label: String,
    start: Instant,
    is_silenced: RefCell<bool>,
    logger: Option<&'a Logger>,  // 可选的日志器
}

impl Timer {
    pub fn report(&self) {
        if let Some(logger) = self.logger {
            slog::debug!(logger, "Timer"; "label" => &self.label, "duration" => ?self.elapsed());
        } else {
            println!("[{}] took {:?}", self.label, self.elapsed());
        }
    }
}
```

---

### 最佳实践

#### 1. 使用 RAII
```rust
// 好的做法
let _timer = Timer::start_new("operation");
do_work();
// 自动报告

// 不推荐
let timer = Timer::start_new("operation");
do_work();
timer.report();  // 手动调用，容易忘记
```

#### 2. 有意义的标签
```rust
// 好的标签
let _timer = Timer::start_new("image_analysis");
let _timer = Timer::start_new("monster_detection_phase1");

// 不好的标签
let _timer = Timer::start_new("t1");
let _timer = Timer::start_new("test");
```

#### 3. 使用 lap() 细化
```rust
let timer = Timer::start_new("full_process");

phase1();
timer.lap(file!(), line!());  // 查看每个阶段的时间

phase2();
timer.lap(file!(), line!());

phase3();
// 自动输出总时间
```

#### 4. 条件性静音
```rust
let timer = Timer::start_new("optional_profiling");

if !config.enable_profiling {
    timer.silence();
}

// 代码照常执行，但可能不输出
```

#### 5. 嵌套计时层次化
```rust
fn outer() {
    let _outer = Timer::start_new("outer");

    inner1();  // 内部有自己的计时器
    inner2();  // 内部有自己的计时器

}  // 查看总时间和子任务时间的关系
```

---

## 模块总结

### utils 模块的作用

#### DateTime
1. **时间格式化**：
   - 将 Duration 转换为可读格式
   - HH:MM:SS 格式
   - 人类友好

2. **数值格式化**：
   - 浮点数精度控制
   - 四舍五入
   - 统计数据展示

#### Timer
1. **性能测量**：
   - 代码执行时间
   - 瓶颈识别
   - 性能优化

2. **开发工具**：
   - 环境变量控制
   - 灵活的输出
   - 生产环境友好

### 设计理念

#### 1. 实用主义
- 简单直接的 API
- 无需复杂配置
- 开箱即用

#### 2. 开发友好
- 清晰的输出格式
- RAII 自动管理
- 易于添加和移除

#### 3. 生产就绪
- 环境变量控制
- 零开销（关闭时）
- 不影响性能

#### 4. 类型安全
- 使用枚举和结构体
- 泛型约束
- 编译时检查

### 使用场景对比

| 工具     | 用途                 | 使用时机         |
|----------|----------------------|------------------|
| DateTime | 格式化显示           | 用户界面、日志   |
| Timer    | 性能分析             | 开发、优化阶段   |

### 组合使用

```rust
fn display_performance_report() {
    let _timer = Timer::start_new("performance_report");

    // 获取统计数据
    let runtime = self.start_time.elapsed();
    let kill_per_min = self.calculate_efficiency();

    // 格式化显示
    let runtime_str = DateTime::format_time(runtime);
    let efficiency_str = DateTime::format_float(kill_per_min, 2);

    println!("运行时间: {}", runtime_str);
    println!("效率: {} 击杀/分钟", efficiency_str);

}  // 自动输出: [performance_report] took 1ms
```

### 扩展建议

#### 1. DateTime 扩展
```rust
// 格式化为分:秒
pub fn format_time_short(elapsed: Duration) -> String {
    let seconds = elapsed.as_secs() % 60;
    let minutes = elapsed.as_secs() / 60;
    format!("{:02}:{:02}", minutes, seconds)
}

// 格式化为可读的持续时间
pub fn format_duration_human(elapsed: Duration) -> String {
    if elapsed.as_secs() < 60 {
        format!("{} 秒", elapsed.as_secs())
    } else if elapsed.as_secs() < 3600 {
        format!("{} 分钟", elapsed.as_secs() / 60)
    } else {
        format!("{} 小时", elapsed.as_secs() / 3600)
    }
}
```

#### 2. Timer 扩展
```rust
// 暂停/恢复功能
pub fn pause(&mut self);
pub fn resume(&mut self);

// 统计功能
pub fn average(timers: &[Timer]) -> Duration;

// 比较功能
pub fn compare(label1: &str, label2: &str) -> Ordering;
```

---

## 总结

utils 模块提供了两个简单但强大的工具：

1. **DateTime**：用于格式化时间和数值，提供人类可读的输出
2. **Timer**：用于性能测量，帮助识别和优化性能瓶颈

两者都遵循简单、实用的设计理念，易于使用且功能完善。
