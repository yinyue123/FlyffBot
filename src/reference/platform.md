# Platform 平台抽象层

本模块提供了跨平台的窗口操作、键盘/鼠标事件模拟和平台特定配置，是机器人与操作系统和游戏窗口交互的底层接口。

---

## shared.rs - 共享平台代码

### 概述
包含所有平台通用的功能实现，提供键盘、鼠标事件模拟和窗口管理的核心 API。这是 platform 模块的主要实现文件。

### 依赖项
```rust
use raw_window_handle::{HasRawWindowHandle, RawWindowHandle};
use tauri::Window;
use crate::data::Point;
```

- **raw_window_handle**：跨平台窗口句柄抽象
- **tauri::Window**：Tauri 窗口对象
- **Point**：二维坐标点

---

### 核心枚举

#### KeyMode - 按键模式
```rust
pub enum KeyMode {
    Press,     // 按下并释放（完整按键）
    Hold,      // 按住（不释放）
    Release,   // 释放（之前按住的键）
}
```

**用途**：
- 控制按键的不同状态
- 支持组合键操作
- 模拟真实键盘行为

**使用场景**：

##### Press - 单次按键
```rust
eval_send_key(window, "Enter", KeyMode::Press);
```
- 等同于按下并立即释放
- 用于触发单次动作
- 如：打开菜单、确认操作

##### Hold - 按住不放
```rust
eval_send_key(window, "W", KeyMode::Hold);
// ... 角色开始前进
```
- 按住键不释放
- 用于持续动作
- 如：移动、旋转

##### Release - 释放按键
```rust
eval_send_key(window, "W", KeyMode::Release);
// ... 角色停止前进
```
- 释放之前按住的键
- 与 Hold 配对使用
- 停止持续动作

**完整示例**：
```rust
// 前进 500ms
eval_send_key(window, "W", KeyMode::Hold);
thread::sleep(Duration::from_millis(500));
eval_send_key(window, "W", KeyMode::Release);
```

---

### 常量

#### IGNORE_AREA_BOTTOM
```rust
pub const IGNORE_AREA_BOTTOM: u32 = 110;
```

**用途**：
- 定义屏幕底部的忽略区域（像素）
- 避免点击到游戏 UI 元素
- 防止误点击技能栏、聊天框等

**注释说明**：
- 用于视觉识别
- 避免鼠标点击窗口外
- 忽略距离底部太近的怪物名称
- 范围：>100 <230（会出现"已被攻击"的红色提示）

**应用场景**：
```rust
// 在图像识别中过滤掉底部区域的怪物
if mob_y > (screen_height - IGNORE_AREA_BOTTOM) {
    continue;  // 忽略这个怪物
}
```

---

### 核心函数

#### 1. get_window_id(window) - 获取原生窗口 ID

```rust
pub fn get_window_id(window: &Window) -> Option<u64>
```

**功能**：获取平台原生的窗口标识符。

**参数**：
- `window: &Window` - Tauri 窗口引用

**返回**：
- `Some(u64)` - 窗口 ID
- `None` - 获取失败

**实现**：根据不同平台获取窗口 ID。

##### Linux (Xlib)
```rust
RawWindowHandle::Xlib(handle) => {
    Some(handle.window as u64)
}
```
- 使用 X11 窗口句柄
- 直接转换为 u64

##### Windows
```rust
RawWindowHandle::Win32(handle) => {
    Some(handle.hwnd as u64)
}
```
- 使用 Win32 HWND（窗口句柄）
- 直接转换为 u64

##### macOS (AppKit)
```rust
RawWindowHandle::AppKit(handle) => {
    #[cfg(target_os = "macos")]
    unsafe {
        use std::ffi::c_void;
        let ns_window_ptr = handle.ns_window as *const c_void;
        libscreenshot::platform::macos::macos_helper::ns_window_to_window_id(ns_window_ptr)
            .map(|id| id as u64)
    }
}
```
- 使用 NSWindow 指针
- 调用 macOS 特定的辅助函数
- 需要 unsafe 代码处理 C 指针
- 依赖 libscreenshot 库

##### 其他平台
```rust
_ => Some(0_u64)
```
- 返回默认值 0
- 占位符实现

**用途**：
- 屏幕截图（指定窗口）
- 窗口管理
- 平台特定操作

**使用示例**：
```rust
if let Some(window_id) = get_window_id(&window) {
    // 使用窗口 ID 进行截图或其他操作
    screenshot::capture_window(window_id);
}
```

---

#### 2. eval_send_key(window, key, mode) - 发送键盘事件

```rust
pub fn eval_send_key(window: &Window, key: &str, mode: KeyMode)
```

**功能**：向游戏窗口发送键盘事件。

**参数**：
- `window: &Window` - 目标窗口
- `key: &str` - 按键名称（如 "W", "Space", "Enter"）
- `mode: KeyMode` - 按键模式（Press/Hold/Release）

**实现**：通过 JavaScript 执行键盘事件。

##### Press 模式
```rust
KeyMode::Press => {
    window.eval("keyboardEvent('press', 'W');")
}
```
- 触发完整的按键（按下+释放）
- JavaScript 函数：`keyboardEvent('press', key)`

##### Hold 模式
```rust
KeyMode::Hold => {
    window.eval("keyboardEvent('hold', 'W');")
}
```
- 只按下键，不释放
- JavaScript 函数：`keyboardEvent('hold', key)`

##### Release 模式
```rust
KeyMode::Release => {
    window.eval("keyboardEvent('release', 'W');")
}
```
- 只释放键
- JavaScript 函数：`keyboardEvent('release', key)`

**返回值处理**：
```rust
drop(window.eval(...))
```
- `window.eval()` 返回 `Result`
- 使用 `drop()` 显式忽略结果
- 不关心 JavaScript 执行是否成功

**JavaScript 接口**：
前端需要实现 `keyboardEvent` 函数：
```javascript
function keyboardEvent(action, key) {
    if (action === 'press') {
        // 模拟按键按下并释放
        simulateKeyPress(key);
    } else if (action === 'hold') {
        // 模拟按键按住
        simulateKeyDown(key);
    } else if (action === 'release') {
        // 模拟按键释放
        simulateKeyUp(key);
    }
}
```

**使用示例**：
```rust
// 打开聊天框
eval_send_key(&window, "Enter", KeyMode::Press);

// 开始前进
eval_send_key(&window, "W", KeyMode::Hold);
thread::sleep(Duration::from_millis(500));

// 停止前进
eval_send_key(&window, "W", KeyMode::Release);
```

**常用按键名称**：
- 移动：`"W"`, `"A"`, `"S"`, `"D"`
- 功能：`"Space"`, `"Enter"`, `"Escape"`, `"Tab"`
- 方向：`"Left"`, `"Right"`, `"Up"`, `"Down"`
- 字母：`"Z"`, `"P"`, `"T"` 等

---

#### 3. send_slot_eval(window, slot_bar_index, slot_index) - 发送技能槽位

```rust
pub fn send_slot_eval(window: &Window, slot_bar_index: usize, k: usize)
```

**功能**：触发指定技能栏的指定槽位（使用技能/道具）。

**参数**：
- `window: &Window` - 目标窗口
- `slot_bar_index: usize` - 技能栏索引（0-8，共 9 个技能栏）
- `k: usize` - 槽位索引（0-9，每个技能栏 10 个槽位）

**实现**：
```rust
window.eval("sendSlot(0, 5)")
```
- JavaScript 函数：`sendSlot(slotBar, slotIndex)`
- 格式化字符串生成调用代码

**JavaScript 接口**：
前端需要实现 `sendSlot` 函数：
```javascript
function sendSlot(slotBar, slotIndex) {
    // 计算实际的按键
    const key = calculateSlotKey(slotBar, slotIndex);

    // 模拟按键
    simulateKeyPress(key);
}
```

**使用示例**：
```rust
// 使用技能栏 0 的第 3 个槽位
send_slot_eval(&window, 0, 2);

// 使用技能栏 2 的第 5 个槽位
send_slot_eval(&window, 2, 4);

// 在 Behavior 中使用
fn send_slot(&mut self, slot_index: (usize, usize)) {
    send_slot_eval(self.window, slot_index.0, slot_index.1);
    self.slots_usage_last_time[slot_index.0][slot_index.1] = Some(Instant::now());
}
```

**槽位编号**：
```
技能栏 0:  [0][1][2][3][4][5][6][7][8][9]
技能栏 1:  [0][1][2][3][4][5][6][7][8][9]
...
技能栏 8:  [0][1][2][3][4][5][6][7][8][9]
```

**应用场景**：
- 使用药丸（Pill）
- 释放攻击技能（AttackSkill）
- 使用治疗技能（HealSkill）
- 释放 Buff（BuffSkill）
- 召唤宠物（PickupPet）

---

#### 4. eval_mob_click(window, pos) - 点击怪物

```rust
pub fn eval_mob_click(window: &Window, pos: Point)
```

**功能**：在指定位置点击鼠标，用于选中怪物。

**参数**：
- `window: &Window` - 目标窗口
- `pos: Point` - 点击坐标（相对于窗口）

**实现**：
```rust
window.eval("mouseEvent('moveClick', 100, 200, {checkMob: true});")
```
- JavaScript 函数：`mouseEvent(action, x, y, options)`
- action: `'moveClick'` - 移动并点击
- options: `{checkMob: true}` - 标记为怪物点击

**JavaScript 接口**：
```javascript
function mouseEvent(action, x, y, options) {
    if (action === 'moveClick') {
        // 移动鼠标到指定位置
        moveMouse(x, y);

        // 点击
        simulateClick();

        // 如果是怪物点击，可能需要特殊处理
        if (options && options.checkMob) {
            // 验证是否点击到怪物
            // 可能需要延迟或重试
        }
    }
}
```

**使用示例**：
```rust
// 获取怪物攻击坐标
let attack_pos = mob.get_attack_coords();

// 点击怪物
eval_mob_click(&window, attack_pos);

// 等待选中
thread::sleep(Duration::from_millis(150));

// 验证是否选中
if image.client_stats.target_is_mover {
    // 成功选中
}
```

**特点**：
- `checkMob: true` 参数用于前端识别这是怪物点击
- 可能触发特殊的验证或重试逻辑
- 与 `eval_simple_click` 区分

**坐标计算**：
```rust
// Target 提供攻击坐标计算
impl Target {
    pub fn get_attack_coords(&self) -> Point {
        let point = self.bounds.get_lowest_center_point();
        Point::new(point.x, point.y + 10)  // 向下偏移 10 像素
    }
}
```

---

#### 5. eval_simple_click(window, pos) - 简单点击

```rust
pub fn eval_simple_click(window: &Window, pos: Point)
```

**功能**：在指定位置简单点击，用于 UI 交互。

**参数**：
- `window: &Window` - 目标窗口
- `pos: Point` - 点击坐标

**实现**：
```rust
window.eval("mouseEvent('moveClick', 100, 200);")
```
- JavaScript 函数：`mouseEvent(action, x, y)`
- action: `'moveClick'` - 移动并点击
- 没有额外的 options 参数

**与 eval_mob_click 的区别**：
```rust
// eval_mob_click
mouseEvent('moveClick', x, y, {checkMob: true})

// eval_simple_click
mouseEvent('moveClick', x, y)
```

**使用示例**：
```rust
// 点击队伍窗口中的队长位置
fn select_party_leader(&mut self) {
    // 打开队伍菜单
    eval_send_key(self.window, "P", KeyMode::Press);
    thread::sleep(Duration::from_millis(150));

    // 点击队长位置
    let point = Point::new(213, 440);
    eval_simple_click(self.window, point);

    // 选中队长
    eval_send_key(self.window, "Z", KeyMode::Press);
}
```

**应用场景**：
- 点击 UI 按钮
- 选择菜单选项
- 点击队伍成员
- 打开/关闭窗口
- 任何非怪物的点击操作

**坐标系统**：
- 坐标相对于游戏窗口左上角
- (0, 0) 是窗口左上角
- X 轴向右增加
- Y 轴向下增加

---

#### 6. eval_send_message(window, text) - 发送消息

```rust
pub fn eval_send_message(window: &Window, text: &str)
```

**功能**：向聊天框输入文本消息。

**参数**：
- `window: &Window` - 目标窗口
- `text: &str` - 要输入的文本

**实现**：
```rust
window.eval("setInputChat('Hello World')")
```
- JavaScript 函数：`setInputChat(text)`
- 直接设置聊天框内容

**JavaScript 接口**：
```javascript
function setInputChat(text) {
    // 获取聊天输入框
    const chatInput = document.getElementById('chat-input');

    // 设置文本内容
    chatInput.value = text;

    // 可能需要触发 input 事件
    chatInput.dispatchEvent(new Event('input'));
}
```

**使用示例**：
```rust
// 发送喊话消息
fn shout(&mut self, message: String) {
    use crate::movement::prelude::*;

    play!(self.movement => [
        // 打开聊天框
        PressKey("Enter"),
        Wait(dur::Random(100..250)),

        // 输入消息
        Type(message.to_string()),
        Wait(dur::Random(100..200)),

        // 发送
        PressKey("Enter"),
        Wait(dur::Random(100..250)),

        // 关闭聊天框
        PressKey("Escape"),
        Wait(dur::Fixed(100))
    ]);
}
```

**Movement::Type 实现**：
```rust
Movement::Type(text) => {
    eval_send_message(&self.window, &text);
}
```

**特点**：
- 直接设置聊天框内容，不模拟逐个字符输入
- 效率高，速度快
- 适合发送完整消息

**编码问题**：
- Rust 字符串默认 UTF-8
- JavaScript 也使用 UTF-8
- 中文等多字节字符正常工作

**示例消息**：
```rust
eval_send_message(&window, "卖装备，便宜出售！");
eval_send_message(&window, "招募队友，组队打本！");
eval_send_message(&window, "Hello World!");
```

---

### 设计特点

#### 1. 跨平台抽象
- 统一的 API 接口
- 平台差异在内部处理
- 外部代码无需关心平台

#### 2. JavaScript 桥接
- 使用 Tauri 的 eval 机制
- 前端实现具体的输入模拟
- 后端只需调用 JavaScript 函数

#### 3. 简单易用
- 函数命名清晰
- 参数直观
- 不需要复杂配置

#### 4. 错误处理策略
```rust
drop(window.eval(...))
```
- 显式忽略错误
- 假设 JavaScript 正确实现
- 简化调用代码

**理由**：
- eval 失败通常是致命错误
- 无法恢复，无需处理
- 保持代码简洁

#### 5. 类型安全
- 使用枚举（KeyMode）
- 使用结构体（Point）
- 编译时检查

---

### 与前端的契约

#### 必需的 JavaScript 函数

##### 1. keyboardEvent(action, key)
```javascript
function keyboardEvent(action, key) {
    // action: 'press' | 'hold' | 'release'
    // key: 按键名称
}
```

##### 2. sendSlot(slotBar, slotIndex)
```javascript
function sendSlot(slotBar, slotIndex) {
    // slotBar: 0-8
    // slotIndex: 0-9
}
```

##### 3. mouseEvent(action, x, y, options)
```javascript
function mouseEvent(action, x, y, options) {
    // action: 'moveClick'
    // x, y: 坐标
    // options: { checkMob?: boolean }
}
```

##### 4. setInputChat(text)
```javascript
function setInputChat(text) {
    // text: 聊天消息
}
```

**前端实现要求**：
- 这些函数必须在全局作用域
- 必须正确模拟游戏输入
- 应该处理异步和延迟

---

## linux.rs - Linux 平台配置

### 概述
Linux 平台的特定配置文件，非常简单，只包含一个常量。

### 常量

```rust
pub const IGNORE_AREA_TOP: u32 = 0;
```

**用途**：
- 定义屏幕顶部的忽略区域（像素）
- Linux 上不需要忽略顶部区域
- 值为 0 表示不忽略

**应用**：
```rust
// 在图像识别中过滤顶部区域
if mob_y < IGNORE_AREA_TOP {
    continue;  // 忽略这个怪物
}
```

**为什么是 0**：
- Linux 窗口管理器通常没有标题栏（全屏游戏）
- 或者标题栏不在游戏窗口内
- 不需要排除顶部区域

---

## macos.rs - macOS 平台配置

### 概述
macOS 平台的特定配置文件，包含顶部忽略区域的配置。

### 常量

```rust
pub const IGNORE_AREA_TOP: u32 = 60;
```

**用途**：
- 定义屏幕顶部的忽略区域（60 像素）
- macOS 窗口有标题栏和交通灯按钮
- 避免识别到非游戏内容

**macOS 窗口特点**：
```
┌─────────────────────────────┐
│ ⚫ ⚫ ⚫  窗口标题            │ ← 60 像素（IGNORE_AREA_TOP）
├─────────────────────────────┤
│                             │
│     游戏内容区域            │
│                             │
│                             │
└─────────────────────────────┘
```

**为什么是 60**：
- macOS 标题栏高度约 22-28 像素
- 加上一些缓冲区
- 总共 60 像素足够

**应用场景**：
```rust
// 在图像识别中过滤顶部区域
if mob_y < IGNORE_AREA_TOP {
    continue;  // 忽略标题栏区域的检测结果
}
```

---

## windows.rs - Windows 平台配置

### 概述
Windows 平台的特定配置文件，配置与 Linux 相同。

### 常量

```rust
pub const IGNORE_AREA_TOP: u32 = 0;
```

**用途**：
- 定义屏幕顶部的忽略区域（像素）
- Windows 上不需要忽略顶部区域
- 值为 0 表示不忽略

**为什么是 0**：
- Windows 游戏通常全屏运行
- 或者使用无边框窗口模式
- 标题栏不在游戏画面内
- 不需要排除顶部区域

**Windows 窗口模式**：
```
全屏模式：整个屏幕都是游戏内容
无边框窗口：没有标题栏和边框
```

---

## 平台差异总结

### 顶部忽略区域对比

| 平台    | IGNORE_AREA_TOP | 原因                     |
|---------|-----------------|--------------------------|
| Linux   | 0               | 全屏或无标题栏           |
| macOS   | 60              | 有标题栏和交通灯按钮     |
| Windows | 0               | 全屏或无边框窗口         |

### 底部忽略区域（所有平台）

```rust
pub const IGNORE_AREA_BOTTOM: u32 = 110;
```
- 在 shared.rs 中定义
- 所有平台共享
- 用于避免点击游戏 UI

### 窗口 ID 获取差异

#### Linux (Xlib)
```rust
handle.window as u64
```
- 直接使用 X11 window ID
- 类型转换简单

#### Windows (Win32)
```rust
handle.hwnd as u64
```
- 直接使用 HWND
- 类型转换简单

#### macOS (AppKit)
```rust
ns_window_to_window_id(ns_window_ptr)
```
- 需要调用辅助函数
- 涉及 Objective-C 桥接
- 需要 unsafe 代码
- 最复杂的实现

---

## 模块结构

```
platform/
├── mod.rs (未显示，导出公共接口)
├── shared.rs (跨平台共享代码)
│   ├── KeyMode 枚举
│   ├── IGNORE_AREA_BOTTOM 常量
│   └── 5 个核心函数
├── linux.rs (Linux 配置)
│   └── IGNORE_AREA_TOP = 0
├── macos.rs (macOS 配置)
│   └── IGNORE_AREA_TOP = 60
└── windows.rs (Windows 配置)
    └── IGNORE_AREA_TOP = 0
```

## 编译时平台选择

通过 `cfg` 属性选择平台：

```rust
#[cfg(target_os = "linux")]
mod linux;

#[cfg(target_os = "macos")]
mod macos;

#[cfg(target_os = "windows")]
mod windows;

// 导出平台特定常量
#[cfg(target_os = "linux")]
pub use linux::IGNORE_AREA_TOP;

#[cfg(target_os = "macos")]
pub use macos::IGNORE_AREA_TOP;

#[cfg(target_os = "windows")]
pub use windows::IGNORE_AREA_TOP;
```

---

## 使用示例

### 完整的怪物点击流程

```rust
// 1. 识别怪物
let mobs = image.identify_mobs(config);

// 2. 过滤边缘区域
let filtered_mobs: Vec<Target> = mobs.iter()
    .filter(|m| {
        let y = m.bounds.y;
        // 过滤顶部
        y >= IGNORE_AREA_TOP &&
        // 过滤底部
        y <= (screen_height - IGNORE_AREA_BOTTOM)
    })
    .cloned()
    .collect();

// 3. 选择最近的怪物
if let Some(mob) = image.find_closest_mob(&filtered_mobs, None, max_distance, logger) {
    // 4. 获取攻击坐标
    let attack_pos = mob.get_attack_coords();

    // 5. 点击怪物
    eval_mob_click(&window, attack_pos);

    // 6. 等待选中
    thread::sleep(Duration::from_millis(150));
}
```

### 完整的技能使用流程

```rust
// 1. 查找可用技能槽位
if let Some((bar, slot)) = config.get_usable_slot_index(
    SlotType::AttackSkill,
    None,
    last_slots_usage
) {
    // 2. 发送技能
    send_slot_eval(&window, bar, slot);

    // 3. 记录使用时间
    slots_usage_last_time[bar][slot] = Some(Instant::now());

    // 4. 等待技能施放
    thread::sleep(Duration::from_millis(500));
}
```

### 完整的移动流程

```rust
// 1. 开始前进
eval_send_key(&window, "W", KeyMode::Hold);

// 2. 开始跳跃
eval_send_key(&window, "Space", KeyMode::Hold);

// 3. 持续移动
thread::sleep(Duration::from_millis(800));

// 4. 停止跳跃
eval_send_key(&window, "Space", KeyMode::Release);

// 5. 停止前进
eval_send_key(&window, "W", KeyMode::Release);
```

---

## 架构优势

### 1. 平台抽象
- 统一 API 隐藏平台差异
- 业务代码无需关心平台
- 易于添加新平台支持

### 2. 前后端分离
- Rust 后端控制逻辑
- JavaScript 前端模拟输入
- 清晰的职责划分

### 3. 类型安全
- KeyMode 枚举避免字符串错误
- Point 结构体表示坐标
- 编译时检查

### 4. 简洁高效
- 函数调用简单
- 无需复杂配置
- 性能开销小

### 5. 易于测试
- 可以 mock Window
- 可以验证 eval 调用
- 单元测试友好

---

## 设计模式

### 1. 外观模式（Facade）
- platform 模块作为外观
- 隐藏 Tauri eval 细节
- 提供简单 API

### 2. 桥接模式（Bridge）
- Rust 和 JavaScript 桥接
- 通过 eval 通信
- 解耦后端和前端

### 3. 策略模式（Strategy）
- KeyMode 定义不同策略
- Press/Hold/Release 是不同的策略
- 灵活组合

---

## 扩展性

### 添加新的输入类型

```rust
// 添加鼠标右键
pub fn eval_right_click(window: &Window, pos: Point) {
    drop(
        window.eval(
            format!("mouseEvent('rightClick', {}, {})", pos.x, pos.y).as_str()
        )
    );
}

// 添加鼠标拖拽
pub fn eval_mouse_drag(window: &Window, from: Point, to: Point) {
    drop(
        window.eval(
            format!(
                "mouseEvent('drag', {}, {}, {}, {})",
                from.x, from.y, to.x, to.y
            ).as_str()
        )
    );
}
```

### 添加新的平台

```rust
// freebsd.rs
pub const IGNORE_AREA_TOP: u32 = 0;

// shared.rs - get_window_id
RawWindowHandle::Wayland(handle) => {
    // Wayland 实现
    Some(handle.surface as u64)
}
```

---

## 最佳实践

### 1. 错误处理
虽然当前使用 `drop()`，但在生产环境可以考虑：
```rust
pub fn eval_send_key(window: &Window, key: &str, mode: KeyMode) -> Result<(), String> {
    let code = match mode {
        KeyMode::Press => format!("keyboardEvent('press', '{}')", key),
        // ...
    };

    window.eval(&code).map_err(|e| e.to_string())?;
    Ok(())
}
```

### 2. 日志记录
```rust
pub fn eval_send_key(window: &Window, key: &str, mode: KeyMode) {
    log::debug!("Sending key: {} in mode: {:?}", key, mode);
    drop(window.eval(...));
}
```

### 3. 延迟控制
```rust
pub fn eval_send_key_with_delay(
    window: &Window,
    key: &str,
    mode: KeyMode,
    delay: Duration
) {
    eval_send_key(window, key, mode);
    thread::sleep(delay);
}
```

### 4. 批量操作
```rust
pub fn eval_send_keys(window: &Window, keys: &[&str], mode: KeyMode) {
    for key in keys {
        eval_send_key(window, key, mode);
    }
}
```

---

## 总结

### Platform 模块的作用

1. **输入模拟**：
   - 键盘事件（Press/Hold/Release）
   - 鼠标事件（点击、移动）
   - 文本输入

2. **平台抽象**：
   - 统一的跨平台 API
   - 平台特定配置
   - 窗口句柄管理

3. **游戏交互**：
   - 技能/道具使用
   - 怪物选择
   - UI 操作

4. **区域管理**：
   - 顶部忽略区域（平台特定）
   - 底部忽略区域（通用）
   - 避免误操作

### 核心特点

1. **简洁明了**：
   - 清晰的函数命名
   - 直观的参数设计
   - 易于使用

2. **跨平台**：
   - Linux/macOS/Windows 支持
   - 平台差异透明
   - 统一接口

3. **JavaScript 桥接**：
   - 利用 Tauri eval
   - 前端实现输入
   - 灵活可扩展

4. **类型安全**：
   - 枚举和结构体
   - 编译时检查
   - 减少错误

### 使用建议

1. **始终检查返回值**（生产环境）
2. **添加适当的延迟**
3. **记录关键操作日志**
4. **测试不同平台行为**
5. **处理前端函数缺失**
6. **注意坐标系统差异**
