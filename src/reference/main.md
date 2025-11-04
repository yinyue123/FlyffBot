# Main 主程序入口

## 概述

main.rs 是整个机器人程序的入口点和核心调度中心，负责：
- 初始化应用程序（日志、Sentry 错误追踪）
- 管理 Tauri 窗口和 WebView
- 处理配置文件（profiles）管理
- 启动和控制机器人主循环
- 协调前后端通信

---

## 模块声明

```rust
mod behavior;        // 行为模块（Farming/Support/Shout）
mod data;            // 数据结构
mod image_analyzer;  // 图像分析
mod ipc;             // 进程间通信
mod movement;        // 移动控制
mod platform;        // 平台抽象层
mod utils;           // 工具函数
```

---

## 应用状态

### AppState
```rust
struct AppState {
    logger: Logger,  // slog 日志器
}
```

**用途**：
- 存储全局状态
- 在 Tauri commands 间共享
- 通过 `tauri::State` 访问

---

## main 函数 - 程序入口

### 执行流程

```
1. 生成 Tauri 上下文
   ↓
2. 获取版本信息
   ↓
3. 初始化 Sentry 错误追踪
   ↓
4. 配置日志系统（slog）
   ↓
5. 构建 Tauri 应用
   ↓
6. 注册命令处理器
   ↓
7. 运行应用
```

### 详细步骤

#### 1. 生成 Tauri 上下文
```rust
let context = tauri::generate_context!();
```
- 编译时宏，生成应用配置
- 包含 package.json 信息
- 窗口配置等

#### 2. 获取版本信息
```rust
let neuz_version = context
    .config()
    .package.version.clone()
    .unwrap_or_else(|| "unknown".to_string());
```
- 从 package.json 读取版本号
- 用于 Sentry 和日志标识

#### 3. 初始化 Sentry
```rust
let sentry_options = sentry::ClientOptions {
    release: Some(std::borrow::Cow::from(format!("v{}", neuz_version))),
    server_name: Some(std::borrow::Cow::from(format!("neuz v{}", neuz_version))),
    ..Default::default()
};
let _sentry = sentry::init((
    "https://ce726b0d4b634de8a44ab5564bc924fe@o1322474.ingest.sentry.io/6579555",
    sentry_options,
));
```

**Sentry 配置**：
- DSN：错误报告地址
- release：版本标识
- server_name：服务器名称

**用途**：
- 自动捕获 panic 和错误
- 发送到 Sentry 服务器
- 帮助开发者追踪问题

#### 4. 配置日志系统
```rust
let drain = {
    let decorator = slog_term::TermDecorator::new().stdout().build();
    let drain = slog_term::CompactFormat
        ::new(decorator)
        .build()
        .filter_level(Level::Trace)
        .fuse();
    slog_async::Async::new(drain).build().fuse()
};
let drain = sentry_slog::SentryDrain::new(drain).fuse();
let logger = Logger::root(drain.fuse(), slog::o!());
```

**日志链**：
```
Logger
  ↓
SentryDrain (发送错误到 Sentry)
  ↓
AsyncDrain (异步日志)
  ↓
CompactFormat (格式化)
  ↓
TermDecorator (终端输出)
  ↓
stdout
```

**特点**：
- 异步日志，不阻塞主线程
- 自动发送错误到 Sentry
- Trace 级别（最详细）
- 输出到标准输出

#### 5. 构建和运行 Tauri 应用
```rust
tauri::Builder::default()
    .manage(AppState { logger })
    .invoke_handler(
        tauri::generate_handler![
            start_bot,
            create_window,
            get_profiles,
            create_profile,
            remove_profile,
            rename_profile,
            copy_profile,
            reset_profile,
            focus_client,
            toggle_main_size
        ]
    )
    .run(context)
    .expect("error while running tauri application");
```

**注册的命令**：
- start_bot：启动机器人
- create_window：创建游戏窗口
- 配置文件管理（get/create/remove/rename/copy/reset）
- 窗口控制（focus_client、toggle_main_size）

---

## Windows Subsystem 配置

```rust
#![cfg_attr(
    all(not(debug_assertions), target_os = "windows"),
    windows_subsystem = "windows"
)]
```

**作用**：
- Release 模式 + Windows 平台
- 使用 windows 子系统（不显示控制台窗口）
- Debug 模式保留控制台（便于调试）

---

## Tauri Commands - 窗口管理

### 1. toggle_main_size - 切换主窗口大小

```rust
#[tauri::command]
fn toggle_main_size(
    size: [u32; 2],
    should_not_toggle: Option<bool>,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
) -> bool
```

**功能**：在最小尺寸和默认尺寸之间切换主窗口。

**参数**：
- `size: [u32; 2]`：最小尺寸 [width, height]
- `should_not_toggle: Option<bool>`：是否禁止切换（只检查不切换）
- `app_handle`：应用句柄

**返回**：
- `true`：切换到最小尺寸
- `false`：切换到默认尺寸

**逻辑**：
```rust
let default_width = 550;
let default_height = 630;

let min_width = size[0];
let min_height = size[1];

if win_size.width == min_width && win_size.height == min_height {
    // 当前是最小尺寸，切换到默认尺寸
    resize_window(window, default_width, default_height, should_not_toggle);
    false
} else {
    // 当前不是最小尺寸，切换到最小尺寸
    resize_window(window, min_width, min_height, should_not_toggle);
    true
}
```

**使用场景**：
- 紧凑模式：只显示核心控件
- 完整模式：显示所有功能

### 2. focus_client - 聚焦客户端窗口

```rust
#[tauri::command]
fn focus_client(_state: tauri::State<AppState>, app_handle: tauri::AppHandle)
```

**功能**：将客户端窗口置前并聚焦。

**操作**：
1. 取消最小化（unminimize）
2. 设置焦点（set_focus）

**使用场景**：
- 用户想要查看游戏窗口
- 从托盘或其他窗口切换回来

---

## Tauri Commands - 配置文件管理

### 配置文件系统

#### 目录结构
```
AppData/
├── .botconfig_DEFAULT          # 配置文件
├── .botconfig_character1
├── profile_DEFAULT/            # 浏览器数据目录
│   ├── cookies
│   ├── localStorage
│   └── ...
├── profile_character1/
└── ...
```

**设计**：
- 每个 profile 有独立的配置文件
- 每个 profile 有独立的浏览器数据目录
- 支持多账号/多角色同时运行

#### 路径辅助函数

##### config_folder_path
```rust
fn config_folder_path(app_handle: &tauri::AppHandle, profile_id: &String) -> String {
    format!(
        r"{}\profile_{}",
        app_handle.path_resolver().app_data_dir().unwrap().to_string_lossy(),
        profile_id
    )
}
```
返回：`AppData/profile_<profile_id>`

##### config_file_path
```rust
fn config_file_path(app_handle: &tauri::AppHandle, profile_id: &String) -> String {
    format!(
        r"{}\.botconfig_{}",
        app_handle.path_resolver().app_data_dir().unwrap().to_string_lossy(),
        profile_id
    )
}
```
返回：`AppData/.botconfig_<profile_id>`

### 1. get_profiles - 获取配置文件列表

```rust
#[tauri::command]
fn get_profiles(_state: tauri::State<AppState>, app_handle: tauri::AppHandle) -> Vec<String>
```

**功能**：扫描并返回所有可用的配置文件。

**逻辑**：
```rust
// 1. 确保 AppData 目录存在
fs::create_dir(app_data_dir)

// 2. 读取目录内容
let paths = fs::read_dir(app_data_dir)

// 3. 过滤出 profile_* 开头的目录
for entry in paths.flatten() {
    if entry.file_name().to_str().unwrap().starts_with("profile_") {
        profiles.push(entry.file_name())
    }
}

// 4. 如果没有任何 profile，创建默认的
if profiles.is_empty() {
    fs::create_dir("profile_DEFAULT")
    profiles.push("profile_DEFAULT")
}
```

**返回示例**：
```json
["profile_DEFAULT", "profile_warrior", "profile_mage"]
```

### 2. create_profile - 创建配置文件

```rust
#[tauri::command]
fn create_profile(
    profile_id: String,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：创建新的配置文件目录。

**操作**：
```rust
fs::create_dir(config_folder_path(&app_handle, &profile_id))
```

**注意**：
- 只创建目录
- 不创建配置文件（首次运行时自动创建）

### 3. remove_profile - 删除配置文件

```rust
#[tauri::command]
fn remove_profile(
    profile_id: String,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：删除指定配置文件的所有数据。

**操作**：
```rust
// 删除配置目录（包含浏览器数据）
fs::remove_dir_all(config_folder_path(&app_handle, &profile_id))

// 删除配置文件
fs::remove_file(config_file_path(&app_handle, &profile_id))
```

**警告**：
- 操作不可逆
- 删除所有相关数据
- 包括浏览器 cookies、localStorage 等

### 4. rename_profile - 重命名配置文件

```rust
#[tauri::command]
fn rename_profile(
    profile_id: String,
    new_profile_id: String,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：重命名配置文件。

**操作**：
```rust
// 重命名配置目录
fs::rename(
    config_folder_path(&app_handle, &profile_id),
    config_folder_path(&app_handle, &new_profile_id)
)

// 重命名配置文件
fs::rename(
    config_file_path(&app_handle, &profile_id),
    config_file_path(&app_handle, &new_profile_id)
)
```

**注意**：
- 代码中有重复的重命名操作（可能是 bug）
- 第二次重命名使用了已经重命名过的路径

### 5. copy_profile - 复制配置文件

```rust
#[tauri::command]
fn copy_profile(
    profile_id: String,
    new_profile_id: String,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：复制配置文件到新的 profile。

**操作**：
```rust
// 复制配置文件
fs::copy(
    config_file_path(&app_handle, &profile_id),
    config_file_path(&app_handle, &new_profile_id)
)

// 递归复制配置目录
copy_dir_all(
    config_folder_path(&app_handle, &profile_id),
    config_folder_path(&app_handle, &new_profile_id)
)
```

**使用场景**：
- 创建配置模板
- 快速创建相似配置
- 备份配置

#### copy_dir_all 辅助函数
```rust
fn copy_dir_all(src: impl AsRef<Path>, dst: impl AsRef<Path>) -> io::Result<()> {
    fs::create_dir_all(&dst)?;
    for entry in fs::read_dir(src)? {
        let entry = entry?;
        let ty = entry.file_type()?;
        if ty.is_dir() {
            copy_dir_all(entry.path(), dst.as_ref().join(entry.file_name()))?;
        } else {
            fs::copy(entry.path(), dst.as_ref().join(entry.file_name()))?;
        }
    }
    Ok(())
}
```

**功能**：递归复制整个目录树。

**逻辑**：
- 创建目标目录
- 遍历源目录
- 子目录递归复制
- 文件直接复制

### 6. reset_profile - 重置配置文件

```rust
#[tauri::command]
fn reset_profile(
    profile_id: String,
    _state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：重置配置文件到初始状态。

**操作**：
```rust
// 删除配置目录
fs::remove_dir_all(config_folder_path(&app_handle, &profile_id))

// 删除配置文件
fs::remove_file(config_file_path(&app_handle, &profile_id))

// 重新创建空目录
fs::create_dir(config_folder_path(&app_handle, &profile_id))
```

**效果**：
- 清除所有配置
- 清除浏览器数据
- 重新开始

---

## Tauri Commands - 窗口创建

### create_window - 创建游戏窗口

```rust
#[tauri::command]
async fn create_window(profile_id: String, app_handle: tauri::AppHandle)
```

**功能**：创建游戏客户端窗口（WebView）。

**窗口配置**：
```rust
let window = tauri::WindowBuilder::new(
    &app_handle,
    "client",  // 窗口标识
    tauri::WindowUrl::External("https://universe.flyff.com/play".parse().unwrap())
)
.data_directory(PathBuf::from(config_folder_path))  // 独立的浏览器数据目录
.center()                                            // 居中显示
.inner_size(800.0, 600.0)                           // 初始尺寸
.title(format!("{} | Flyff Universe", profile_id))  // 窗口标题
.build()
.unwrap();
```

**关键特性**：

#### 1. 独立的浏览器数据目录
```rust
.data_directory(PathBuf::from(
    format!(r"{}\profile_{}", app_data_dir, profile_id)
))
```
- 每个 profile 有独立的 cookies、localStorage
- 支持多账号同时登录
- 数据隔离

#### 2. 游戏 URL
```rust
"https://universe.flyff.com/play"
```
- 直接加载 Flyff Universe 游戏
- WebView 环境
- 支持完整的 Web API

#### 3. 窗口标题
```rust
.title(format!("{} | Flyff Universe", profile_id))
```
- 显示当前 profile 名称
- 便于区分多个窗口

**后续操作**：
```rust
window.show();  // 显示窗口

// 更新主窗口标题
main_window.set_title(format!("{} Neuz | MadrigalStreetCartel", profile_id));
```

**注释掉的代码**：
```rust
// window.open_devtools();  // 开发者工具
// .resizable(false)         // 禁止调整大小
```

---

## Tauri Commands - 机器人控制

### start_bot - 启动机器人主循环

```rust
#[tauri::command]
fn start_bot(
    profile_id: String,
    state: tauri::State<AppState>,
    app_handle: tauri::AppHandle
)
```

**功能**：启动机器人的主循环线程。

**这是整个程序最核心的函数！**

### 执行流程概览

```
1. 加载配置
   ↓
2. 启动新线程
   ↓
3. 设置事件监听器
   ↓
4. 初始化组件
   ↓
5. 进入主循环
   ↓
6. 根据状态执行行为
```

### 详细步骤

#### 1. 加载配置
```rust
let config_path = format!(r"{}\.botconfig_{}", app_data_dir, profile_id);

let config: Arc<RwLock<BotConfig>> = Arc::new(
    RwLock::new(BotConfig::deserialize_or_default(config_path))
);
```

**使用 Arc<RwLock>**：
- `Arc`：多线程共享
- `RwLock`：读写锁，支持多读单写
- 配置可以被前端和主循环同时访问

#### 2. 启动新线程
```rust
std::thread::spawn(move || {
    // 主循环代码
});
```

**原因**：
- 避免阻塞 Tauri 主线程
- 允许前端继续响应
- 独立的机器人循环

#### 3. 设置事件监听器

##### 监听配置变更（前端 → 后端）
```rust
app_handle.listen_global("bot_config_c2s", move |e| {
    if let Some(payload) = e.payload() {
        match serde_json::from_str::<BotConfig>(payload) {
            Ok(new_config) => {
                *local_config.write() = new_config.changed();
            }
            Err(e) => {
                slog::error!(logger, "Failed to parse config change"; "error" => e.to_string());
            }
        }
    }
});
```

**流程**：
1. 前端发送 "bot_config_c2s" 事件
2. payload 包含 JSON 格式的配置
3. 解析配置
4. 更新配置（增加 change_id）
5. 主循环检测到变化后保存和应用

##### 监听启动/停止切换
```rust
app_handle.listen_global("toggle_bot", move |_| {
    local_config.write().toggle_active();
});
```

**功能**：
- 切换 `is_running` 状态
- 启动/暂停机器人

##### 发送配置到前端（后端 → 前端）
```rust
let send_config = |config: &BotConfig| {
    drop(app_handle.emit_all("bot_config_s2c", config))
};
```

##### 发送状态信息到前端
```rust
let send_info = |config: &FrontendInfo| {
    drop(app_handle.emit_all("bot_info_s2c", config))
};
```

#### 4. 初始化组件

##### 等待前端准备
```rust
std::thread::sleep(Duration::from_secs(1));
```

##### 发送初始配置
```rust
send_config(&config.read());
```

##### 注入 JavaScript 代码
```rust
let eval_js = include_str!("./platform/eval.js");

#[cfg(dev)]
let eval_js = eval_js.replace("$env.DEBUG", "true");
#[cfg(not(dev))]
let eval_js = eval_js.replace("$env.DEBUG", "false");

window.eval(&eval_js);
```

**eval.js 内容**：
- 实现 keyboardEvent 函数
- 实现 mouseEvent 函数
- 实现 sendSlot 函数
- 实现 setInputChat 函数
- 提供游戏输入模拟的 JavaScript 接口

##### 创建图像分析器
```rust
let mut image_analyzer: ImageAnalyzer = ImageAnalyzer::new(&window);
image_analyzer.window_id = platform::get_window_id(&window).unwrap_or(0);
```

##### 创建移动访问器
```rust
let movement = MovementAccessor::new(window.clone());
```

##### 实例化所有行为
```rust
let mut farming_behavior = FarmingBehavior::new(&logger, &movement, &window);
let mut shout_behavior = ShoutBehavior::new(&logger, &movement, &window);
let mut support_behavior = SupportBehavior::new(&logger, &movement, &window);
```

##### 初始化状态变量
```rust
let mut last_mode: Option<BotMode> = None;
let mut last_is_running: Option<bool> = None;
let mut frontend_info: Arc<RwLock<FrontendInfo>> = Arc::new(
    RwLock::new(FrontendInfo::deserialize_or_default())
);
```

#### 5. 进入主循环
```rust
loop {
    let timer = Timer::start_new("main_loop");
    // ... 主循环逻辑
}
```

---

## 主循环详解

### 循环结构

```
loop {
    ┌─ 读取配置
    ├─ 检查配置变更
    ├─ 保存和同步配置
    ├─ 检查模式是否设置
    ├─ 处理停止状态
    ├─ 处理模式切换
    ├─ 设置窗口属性
    ├─ 检查窗口是否关闭
    ├─ 检查是否运行中
    ├─ 捕获窗口图像
    ├─ 更新游戏状态
    ├─ 检查存活状态
    ├─ 执行行为逻辑
    └─ 发送状态到前端
}
```

### 详细流程

#### 1. 读取配置
```rust
let config = &*config.read();
let mut frontend_info_mut = *frontend_info.read();
```

#### 2. 检查配置变更
```rust
if last_config_change_id == 0 || config.change_id() > last_config_change_id {
    // 配置已变更
}
```

#### 3. 保存和同步配置
```rust
config.serialize(config_path);
send_config(config);
last_config_change_id = config.change_id();
```

#### 4. 更新行为
```rust
farming_behavior.update(config);
shout_behavior.update(config);
support_behavior.update(config);
```

#### 5. 检查模式是否设置
```rust
guard!(let Some(mode) = config.mode() else {
    std::thread::sleep(Duration::from_millis(100));
    timer.silence();
    continue;
});
```

**使用 guard! 宏**：
- 提前返回模式
- 如果 mode 为 None，等待 100ms 后继续

#### 6. 处理停止状态
```rust
if !config.is_running() {
    if let Some(last_is_running_value) = last_is_running.as_mut() {
        if *last_is_running_value {
            // 从运行切换到停止，调用 interupt
            match mode {
                BotMode::Farming => farming_behavior.interupt(config),
                BotMode::Support => support_behavior.interupt(config),
                BotMode::AutoShout => shout_behavior.interupt(config),
            }
        }
    }

    // 允许调整窗口大小
    if !window.is_resizable().unwrap() {
        drop(window.set_resizable(true));
    }

    std::thread::sleep(Duration::from_millis(100));
    continue;
}
```

#### 7. 处理模式切换
```rust
if let Some(last_mode_value) = last_mode.as_mut() {
    if &mode != last_mode_value {
        // 模式已切换

        // 停止旧模式
        match last_mode_value {
            BotMode::Farming => farming_behavior.stop(config),
            BotMode::Support => support_behavior.stop(config),
            BotMode::AutoShout => shout_behavior.stop(config),
        }

        // 启动新模式
        match mode {
            BotMode::Farming => farming_behavior.start(config),
            BotMode::Support => support_behavior.start(config),
            BotMode::AutoShout => shout_behavior.start(config),
        }
    }
}
```

#### 8. 设置窗口属性（Farming 模式）
```rust
if !config.farming_config().is_stop_fighting() {
    // 设置固定窗口大小
    window.set_size(Size::Logical(LogicalSize {
        width: 800.0,
        height: 600.0,
    }));

    // 禁止调整大小
    window.set_resizable(false);
}
```

**原因**：
- 图像识别依赖固定的窗口尺寸
- 避免用户调整大小导致识别失败

#### 9. 检查窗口是否关闭
```rust
if window.is_resizable().is_err() {
    #[cfg(dev)]
    std::process::exit(0);

    #[cfg(not(dev))]
    app_handle.restart();

    break;
}
```

**逻辑**：
- `is_resizable()` 返回 Err 说明窗口已关闭
- 开发模式：直接退出
- 生产模式：重启应用

#### 10. 再次检查运行状态
```rust
if !config.is_running() {
    std::thread::sleep(Duration::from_millis(100));
    timer.silence();
    continue;
}
```

#### 11. 设置运行状态
```rust
frontend_info_mut.set_is_running(true);
```

#### 12. 捕获窗口图像
```rust
image_analyzer.capture_window(&logger);
```

**使用 libscreenshot**：
- 截取游戏窗口
- 存储在 image_analyzer 中

#### 13. 检查图像是否捕获成功
```rust
if image_analyzer.image_is_some() {
    // 继续处理
}
```

#### 14. 更新游戏状态
```rust
image_analyzer.client_stats.update(&image_analyzer.clone(), &logger);
```

**更新内容**：
- HP/MP/FP 值
- 目标状态
- 存活状态
- 目标距离

#### 15. 检查 AFK 断线
```rust
if frontend_info_mut.is_afk_ready_to_disconnect() {
    app_handle.exit(0);
    return;
}
```

#### 16. 处理存活状态

##### should_disconnect_on_death 辅助函数
```rust
fn should_disconnect_on_death(config: &BotConfig) -> bool {
    match config.mode().unwrap() {
        BotMode::Farming => config.farming_config().on_death_disconnect(),
        BotMode::Support => config.support_config().on_death_disconnect(),
        BotMode::AutoShout => true,
    }
}
```

##### 存活状态处理
```rust
let is_alive = image_analyzer.client_stats.is_alive;

let return_earlier = match is_alive {
    AliveState::StatsTrayClosed => {
        // 状态栏关闭，无法识别
        true  // 跳过本次循环
    }

    AliveState::Alive => {
        // 角色存活
        if !frontend_info_mut.is_alive() {
            // 从死亡状态恢复（被复活）
            frontend_info_mut.set_is_alive(true);

            if !should_disconnect(config) {
                // 不断线，关闭可能的聊天框
                eval_send_key(&window, "Escape", KeyMode::Press);
                std::thread::sleep(Duration::from_millis(1000));
            }
        }
        false  // 继续执行
    }

    AliveState::Dead => {
        // 角色死亡
        if frontend_info_mut.is_alive() {
            // 刚死亡
            if should_disconnect(config) {
                app_handle.exit(0);  // 断线退出
            }

            frontend_info_mut.set_is_alive(false);
            frontend_info = Arc::new(RwLock::new(frontend_info_mut));
            send_info(&frontend_info.read());
        } else {
            // 持续死亡状态，按 Enter 尝试复活
            eval_send_key(&window, "Enter", KeyMode::Press);
            std::thread::sleep(Duration::from_millis(500));
        }
        true  // 跳过本次循环
    }
};

if return_earlier {
    std::thread::sleep(Duration::from_millis(10));
    timer.silence();
    continue;
}
```

**存活状态机**：
```
StatsTrayClosed → 无法识别 → 跳过
Alive → 正常运行 → 继续
Dead (首次) → 检查是否断线 → 断线或跳过
Dead (持续) → 按 Enter 尝试复活 → 跳过
Dead → Alive → 关闭聊天框 → 继续
```

#### 17. 执行行为逻辑
```rust
match mode {
    BotMode::Farming => {
        farming_behavior.run_iteration(
            &mut frontend_info_mut,
            config,
            &mut image_analyzer
        );
    }
    BotMode::AutoShout => {
        shout_behavior.run_iteration(
            &mut frontend_info_mut,
            config,
            &mut image_analyzer
        );
    }
    BotMode::Support => {
        support_behavior.run_iteration(
            &mut frontend_info_mut,
            config,
            &mut image_analyzer
        );
    }
}
```

#### 18. 更新前端信息
```rust
frontend_info = Arc::new(RwLock::new(frontend_info_mut));
send_info(&frontend_info.read());
```

#### 19. 更新状态跟踪
```rust
last_mode = config.mode();
last_is_running = Some(config.is_running());
```

---

## 数据流图

### 前后端通信

```
前端 (UI)
  │
  ├─ "bot_config_c2s" ──────────► 后端配置更新
  │                                  │
  │                                  ├─ 解析 JSON
  │                                  ├─ 更新配置
  │                                  └─ 增加 change_id
  │
  ├─ "toggle_bot" ──────────────► toggle_active()
  │
  ◄─ "bot_config_s2c" ───────────┐
  │                              │ 配置同步
  ◄─ "bot_info_s2c" ─────────────┘
                                 │ 状态信息
```

### 主循环数据流

```
配置文件 (.botconfig)
  ↓
Arc<RwLock<BotConfig>>
  ↓
主循环读取
  ↓
┌────────────────────┐
│ 配置变更检测       │
│ change_id 比较     │
└────────────────────┘
  ↓
保存配置 + 更新行为
  ↓
┌────────────────────┐
│ 窗口截图           │
│ libscreenshot      │
└────────────────────┘
  ↓
┌────────────────────┐
│ 图像分析           │
│ ImageAnalyzer      │
└────────────────────┘
  ↓
┌────────────────────┐
│ 状态更新           │
│ ClientStats        │
└────────────────────┘
  ↓
┌────────────────────┐
│ 行为执行           │
│ Behavior           │
└────────────────────┘
  ↓
┌────────────────────┐
│ 前端信息更新       │
│ FrontendInfo       │
└────────────────────┘
  ↓
发送到前端 (bot_info_s2c)
```

---

## 错误处理策略

### 1. drop() 显式忽略错误
```rust
drop(window.set_size(...));
drop(fs::create_dir(...));
```

**原因**：
- 这些操作失败通常不影响核心功能
- 避免过多的错误处理代码
- 保持代码简洁

### 2. unwrap_or / unwrap_or_default
```rust
let version = context.config().package.version.clone()
    .unwrap_or_else(|| "unknown".to_string());
```

**原因**：
- 提供合理的默认值
- 避免 panic
- 降级处理

### 3. guard! 宏提前返回
```rust
guard!(let Some(mode) = config.mode() else {
    continue;
});
```

**原因**：
- 清晰的流程控制
- 避免深层嵌套
- 提高可读性

### 4. is_err() 检测窗口关闭
```rust
if window.is_resizable().is_err() {
    break;
}
```

**原因**：
- 窗口关闭时 API 返回错误
- 通过错误检测关闭事件
- 优雅退出循环

---

## 并发和同步

### Arc<RwLock<T>> 模式

#### BotConfig
```rust
let config: Arc<RwLock<BotConfig>>
```

**使用场景**：
- 主循环读取（read）
- 事件监听器写入（write）
- 多线程共享

**读写模式**：
- 多个读者同时读取
- 单个写者独占
- 读写互斥

#### FrontendInfo
```rust
let frontend_info: Arc<RwLock<FrontendInfo>>
```

**使用场景**：
- 主循环更新
- 发送到前端

**注意**：
- 每次更新创建新的 Arc
- 确保数据一致性

### 线程安全

#### 事件监听器
```rust
app_handle.listen_global("bot_config_c2s", move |e| {
    // 闭包捕获 Arc<RwLock<BotConfig>>
});
```

**工作原理**：
- 事件在 Tauri 事件线程处理
- 通过 RwLock 安全访问共享数据
- 避免数据竞争

---

## 性能考虑

### 1. Timer 性能测量
```rust
let timer = Timer::start_new("main_loop");
// ... 循环逻辑
// 自动输出循环耗时
```

**用途**：
- 监控循环性能
- 识别瓶颈
- 优化目标

### 2. 条件性等待
```rust
if !config.is_running() {
    std::thread::sleep(Duration::from_millis(100));
    timer.silence();  // 不输出计时
    continue;
}
```

**优化**：
- 停止时降低 CPU 使用
- 不测量等待时间
- 减少日志输出

### 3. 图像捕获
```rust
image_analyzer.capture_window(&logger);

if image_analyzer.image_is_some() {
    // 处理图像
}
```

**性能影响**：
- 截图是最耗时的操作
- 每次循环都执行
- 影响循环频率

---

## 开发 vs 生产模式

### 编译时配置

#### Debug 模式
```rust
#[cfg(dev)]
let eval_js = eval_js.replace("$env.DEBUG", "true");
```

#### Release 模式
```rust
#[cfg(not(dev))]
let eval_js = eval_js.replace("$env.DEBUG", "false");
```

### 窗口关闭行为

#### Debug 模式
```rust
#[cfg(dev)]
std::process::exit(0);  // 直接退出
```

#### Release 模式
```rust
#[cfg(not(dev))]
app_handle.restart();  // 重启应用
```

### Windows 子系统

#### Debug 模式
- 显示控制台
- 查看日志输出

#### Release 模式
- 隐藏控制台（Windows）
- 更好的用户体验

---

## 最佳实践

### 1. 配置管理
- 使用 Arc<RwLock> 共享配置
- change_id 检测变更
- 自动保存和同步

### 2. 事件驱动
- 前端通过事件更新配置
- 后端通过事件发送状态
- 解耦前后端

### 3. 行为切换
- 优雅的启动/停止
- 支持模式切换
- 保持状态一致

### 4. 错误恢复
- 死亡检测和处理
- 窗口关闭重启
- AFK 断线

### 5. 性能监控
- Timer 测量循环时间
- 条件性静音
- 识别瓶颈

---

## 总结

### main.rs 的职责

1. **应用初始化**：
   - 日志系统
   - Sentry 错误追踪
   - Tauri 应用构建

2. **窗口管理**：
   - 主窗口（UI）
   - 客户端窗口（游戏）
   - 大小切换和聚焦

3. **配置管理**：
   - Profile 系统
   - 配置的 CRUD 操作
   - 浏览器数据隔离

4. **机器人控制**：
   - 主循环调度
   - 行为执行
   - 状态管理

5. **前后端通信**：
   - 事件监听
   - 配置同步
   - 状态推送

### 架构特点

1. **模块化**：
   - 清晰的模块划分
   - 职责明确
   - 易于维护

2. **并发安全**：
   - Arc<RwLock> 共享状态
   - 线程安全的事件系统
   - 避免数据竞争

3. **事件驱动**：
   - 松耦合的前后端
   - 灵活的通信机制
   - 易于扩展

4. **错误恢复**：
   - 优雅的错误处理
   - 自动重启
   - 降级策略

5. **性能监控**：
   - Timer 性能测量
   - 循环时间跟踪
   - 优化依据

### 核心流程

```
启动应用
  ↓
创建游戏窗口
  ↓
start_bot 启动主循环
  ↓
配置加载和事件监听
  ↓
初始化图像分析器和行为
  ↓
进入主循环:
  ├─ 读取配置
  ├─ 捕获图像
  ├─ 分析状态
  ├─ 执行行为
  └─ 更新前端
  ↓
循环直到窗口关闭
  ↓
退出或重启
```

main.rs 是整个机器人系统的指挥中心，协调各个模块的工作，管理应用的生命周期，是理解整个系统运作的关键入口。
