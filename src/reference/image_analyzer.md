# Image Analyzer 图像分析器

## 概述

image_analyzer.rs 是机器人视觉系统的核心模块，负责从游戏窗口截图中识别和分析游戏元素。主要功能包括：
- 窗口截图捕获
- 像素级颜色检测
- 怪物识别和分类（被动/主动/紫色）
- 目标标记识别
- 距离计算
- 点云聚类

这个模块是机器人"眼睛"的实现，所有决策都基于它提供的视觉信息。

---

## 核心结构

### Color - 颜色表示

```rust
#[derive(Debug, Clone, Copy, Default)]
pub struct Color {
    pub refs: [u8; 3],  // RGB 颜色值
}
```

**方法**：
```rust
pub fn new(r: u8, g: u8, b: u8) -> Self
```

**用途**：
- 表示 RGB 颜色
- 用于像素匹配
- 怪物名称颜色识别

**示例**：
```rust
// 被动怪物颜色（黄色）
let passive_color = Color::new(234, 234, 149);

// 主动怪物颜色（红色）
let aggressive_color = Color::new(179, 23, 23);

// 紫色怪物颜色
let violet_color = Color::new(182, 144, 146);

// 目标标记颜色（蓝色）
let blue_marker = Color::new(131, 148, 205);

// 目标标记颜色（红色）
let red_marker = Color::new(246, 90, 106);
```

---

### ImageAnalyzer - 图像分析器

```rust
#[derive(Debug, Clone)]
pub struct ImageAnalyzer {
    image: Option<ImageBuffer>,  // 截图缓冲区
    pub window_id: u64,           // 窗口 ID
    pub client_stats: ClientStats, // 客户端状态
}
```

**字段说明**：

#### image
- 存储最新的窗口截图
- 类型：`Option<ImageBuffer>`（来自 libscreenshot）
- 每次循环更新

#### window_id
- 原生窗口标识符
- 用于截图 API
- 平台特定（Xlib/Win32/AppKit）

#### client_stats
- 游戏客户端状态信息
- 包含 HP/MP/FP、目标信息等
- 详见 stats_info.rs

---

## 核心方法

### 1. new(window) - 创建分析器

```rust
pub fn new(window: &Window) -> Self
```

**功能**：创建新的图像分析器实例。

**实现**：
```rust
Self {
    window_id: 0,              // 初始化为 0，稍后设置
    image: None,               // 没有截图
    client_stats: ClientStats::new(window.to_owned()),
}
```

**使用示例**：
```rust
let mut image_analyzer = ImageAnalyzer::new(&window);
image_analyzer.window_id = platform::get_window_id(&window).unwrap_or(0);
```

---

### 2. image_is_some() - 检查图像是否存在

```rust
pub fn image_is_some(&self) -> bool
```

**功能**：检查是否成功捕获了图像。

**使用场景**：
```rust
image_analyzer.capture_window(&logger);

if image_analyzer.image_is_some() {
    // 继续分析图像
    image_analyzer.client_stats.update(&image_analyzer, &logger);
} else {
    // 截图失败，跳过本次循环
}
```

---

### 3. capture_window(logger) - 捕获窗口截图

```rust
pub fn capture_window(&mut self, logger: &Logger)
```

**功能**：截取游戏窗口的当前画面。

**实现**：
```rust
let _timer = Timer::start_new("capture_window");

// 检查窗口 ID
if self.window_id == 0 {
    return;
}

// 获取截图提供者（平台特定）
if let Some(provider) = libscreenshot::get_window_capture_provider() {
    // 截取指定窗口
    if let Ok(image) = provider.capture_window(self.window_id) {
        self.image = Some(image);
    } else {
        slog::warn!(logger, "Failed to capture window"; "window_id" => self.window_id);
    }
}
```

**使用的库**：
- **libscreenshot**：跨平台截图库
- 支持 Linux (X11)、Windows、macOS
- 按窗口 ID 截图，不是全屏

**性能**：
- 使用 Timer 测量耗时
- 这是主循环中最耗时的操作
- 典型耗时：10-50ms

**错误处理**：
- window_id 为 0：直接返回
- 截图失败：记录警告日志
- 不会 panic，保持程序运行

---

### 4. pixel_detection(...) - 像素检测

```rust
pub fn pixel_detection(
    &self,
    colors: Vec<Color>,      // 要检测的颜色列表
    min_x: u32,              // 搜索区域最小 X
    min_y: u32,              // 搜索区域最小 Y
    mut max_x: u32,          // 搜索区域最大 X
    mut max_y: u32,          // 搜索区域最大 Y
    tolerence: Option<u8>,   // 颜色容差
) -> Receiver<Point>
```

**功能**：在指定区域检测匹配指定颜色的所有像素。

**这是图像分析的核心算法！**

#### 返回值
```rust
Receiver<Point>
```
- 使用 channel 异步返回匹配的像素点
- 避免一次性分配大量内存
- 支持并行处理

#### 参数处理
```rust
// 默认值处理
if max_x == 0 {
    max_x = image.width();
}

if max_y == 0 {
    max_y = image.height();
}
```
- max_x/max_y 为 0 表示使用图像的完整尺寸

#### 并行扫描
```rust
let (snd, recv) = sync_channel::<Point>(4096);

image
    .enumerate_rows()    // 遍历所有行
    .par_bridge()        // 并行桥接（Rayon）
    .for_each(move |(y, row)| {
        // 并行处理每一行
    });
```

**使用 Rayon 并行**：
- `par_bridge()`：将迭代器转换为并行迭代器
- 自动利用多核 CPU
- 显著提升性能

#### 区域过滤
```rust
// 跳过忽略区域
#[allow(clippy::absurd_extreme_comparisons)]
if y <= IGNORE_AREA_TOP
    || y > image_height.checked_sub(IGNORE_AREA_BOTTOM).unwrap_or(image_height)
    || y > IGNORE_AREA_TOP + max_y
    || y > max_y
    || y < min_y
{
    return;  // 跳过这一行
}
```

**忽略区域**：
- `IGNORE_AREA_TOP`：顶部忽略区域（macOS 标题栏）
- `IGNORE_AREA_BOTTOM`：底部忽略区域（游戏 UI）
- 自定义搜索区域（min/max）

#### 像素匹配
```rust
'outer: for (x, _, px) in row {
    // 检查 alpha 通道（透明度）
    if px.0[3] != 255 || x >= max_x {
        return;
    } else if x < min_x {
        continue;
    }

    // 检查是否匹配任何参考颜色
    for ref_color in colors.iter() {
        if Self::pixel_matches(&px.0, &ref_color.refs, tolerence.unwrap_or(5)) {
            // 发送匹配的点
            drop(snd.try_send(Point::new(x, y)).map_err(|err| {
                eprintln!("Error sending data: {}", err);
            }));

            // 继续下一列
            continue 'outer;
        }
    }
}
```

**优化**：
- 标签循环 `'outer`：匹配后立即跳到下一列
- `try_send`：非阻塞发送，避免等待
- alpha 检查：过滤透明像素

#### 使用示例
```rust
// 检测红色状态栏
let hp_colors = vec![
    Color::new(174, 18, 55),
    Color::new(188, 24, 62),
    Color::new(204, 30, 70),
    Color::new(220, 36, 78),
];

let receiver = image_analyzer.pixel_detection(
    hp_colors,
    105,    // min_x
    30,     // min_y
    225,    // max_x
    110,    // max_y
    Some(2) // 容差
);

// 接收匹配的像素
let mut cloud = PointCloud::default();
while let Ok(point) = receiver.recv() {
    cloud.push(point);
}
```

---

### 5. pixel_matches(...) - 像素颜色匹配

```rust
#[inline(always)]
fn pixel_matches(c: &[u8; 4], r: &[u8; 3], tolerance: u8) -> bool
```

**功能**：检查像素颜色是否在容差范围内匹配参考颜色。

**参数**：
- `c: &[u8; 4]`：待检查的像素 [R, G, B, A]
- `r: &[u8; 3]`：参考颜色 [R, G, B]
- `tolerance: u8`：容差值（通常 2-10）

**实现算法**：
```rust
let matches_inner = |a: u8, b: u8| match (a, b) {
    (a, b) if a == b => true,                              // 完全匹配
    (a, b) if a > b => a.saturating_sub(b) <= tolerance,  // a 较大
    (a, b) if a < b => b.saturating_sub(a) <= tolerance,  // b 较大
    _ => false,
};

// 检查 RGB 三个通道
let perm = [(c[0], r[0]), (c[1], r[1]), (c[2], r[2])];
perm.iter().all(|&(a, b)| matches_inner(a, b))
```

**逻辑**：
1. 定义内部匹配函数
2. 计算差值的绝对值
3. 检查差值是否在容差内
4. 所有三个通道都必须匹配

**容差作用**：
- 补偿光照变化
- 补偿渲染差异
- 补偿抗锯齿效果

**示例**：
```rust
// 参考颜色：(179, 23, 23)
// 容差：5

// 匹配的像素
pixel_matches(&[179, 23, 23, 255], &[179, 23, 23], 5);  // true (完全匹配)
pixel_matches(&[180, 24, 22, 255], &[179, 23, 23], 5);  // true (差值 ≤ 5)
pixel_matches(&[175, 20, 25, 255], &[179, 23, 23], 5);  // true (差值 ≤ 5)

// 不匹配的像素
pixel_matches(&[190, 30, 30, 255], &[179, 23, 23], 5);  // false (差值 > 5)
pixel_matches(&[100, 100, 100, 255], &[179, 23, 23], 5); // false
```

**性能优化**：
- `#[inline(always)]`：强制内联
- 避免函数调用开销
- 在热路径中调用数百万次

---

### 6. merge_cloud_into_mobs(...) - 点云聚类

```rust
fn merge_cloud_into_mobs(
    config: Option<&FarmingConfig>,
    cloud: &PointCloud,
    mob_type: TargetType,
) -> Vec<Target>
```

**功能**：将散乱的像素点聚类成目标对象（怪物）。

**这是从像素到目标的关键转换！**

#### 聚类算法

##### 步骤 1：X 轴聚类
```rust
let max_distance_x: u32 = 50;
let x_clusters = cloud.cluster_by_distance(max_distance_x, point_selector::x_axis);
```
- 沿 X 轴方向聚类
- 距离 ≤ 50 像素的点归为一组
- 分离不同的怪物名称

##### 步骤 2：Y 轴聚类
```rust
let max_distance_y: u32 = 3;

for x_cluster in x_clusters {
    let local_y_clusters = x_cluster.cluster_by_distance(
        max_distance_y,
        point_selector::y_axis
    );
    xy_clusters.extend(local_y_clusters);
}
```
- 在每个 X 簇内沿 Y 轴聚类
- 距离 ≤ 3 像素的点归为一组
- 处理上下重叠的名称

**为什么两次聚类？**
- 怪物名称通常是水平的文本
- X 方向宽度大（几十像素）
- Y 方向高度小（几像素）
- 两阶段聚类更准确

##### 步骤 3：创建目标
```rust
xy_clusters
    .into_iter()
    .map(|cluster| Target {
        target_type: mob_type,
        bounds: cluster.to_bounds(),
    })
    .filter(|mob| {
        if let Some(config) = config {
            mob.bounds.w > config.min_mobs_name_width() &&
            mob.bounds.w < config.max_mobs_name_width()
        } else {
            true
        }
    })
    .collect()
```

**过滤条件**：
- `min_mobs_name_width`（默认 11）：过滤噪点
- `max_mobs_name_width`（默认 180）：过滤特殊怪物

**示例**：
```
原始像素点：
  .  .  .     . . . .     . . .
  .  .  .     . . . .     . . .
  (怪物 A)    (怪物 B)    (怪物 C)

X 轴聚类（距离 50）：
  [怪物 A]    [怪物 B]    [怪物 C]

Y 轴聚类（距离 3）：
  [怪物 A]    [怪物 B]    [怪物 C]

转换为 Target：
  Target { bounds: (x1, y1, w1, h1), type: Passive }
  Target { bounds: (x2, y2, w2, h2), type: Passive }
  Target { bounds: (x3, y3, w3, h3), type: Passive }
```

---

### 7. identify_mobs(config) - 识别怪物

```rust
pub fn identify_mobs(&self, config: &FarmingConfig) -> Vec<Target>
```

**功能**：识别屏幕上的所有怪物并分类。

**这是怪物识别的主入口！**

#### 执行流程

##### 步骤 1：准备颜色配置
```rust
// 被动怪物颜色（黄色）
let ref_color_pas: [u8; 3] = [
    config.passive_mobs_colors()[0].unwrap_or(234),
    config.passive_mobs_colors()[1].unwrap_or(234),
    config.passive_mobs_colors()[2].unwrap_or(149),
];

// 主动怪物颜色（红色）
let ref_color_agg: [u8; 3] = [
    config.aggressive_mobs_colors()[0].unwrap_or(179),
    config.aggressive_mobs_colors()[1].unwrap_or(23),
    config.aggressive_mobs_colors()[2].unwrap_or(23),
];

// 紫色怪物颜色
let ref_color_violet: [u8; 3] = [
    config.violet_mobs_colors()[0].unwrap_or(182),
    config.violet_mobs_colors()[1].unwrap_or(144),
    config.violet_mobs_colors()[2].unwrap_or(146),
];
```

**颜色来源**：
- 配置文件（可自定义）
- 默认值（Flyff Universe 标准）

##### 步骤 2：并行扫描像素
```rust
struct MobPixel(u32, u32, TargetType);
let (snd, recv) = sync_channel::<MobPixel>(4096);

image
    .enumerate_rows()
    .par_bridge()
    .for_each(move |(y, row)| {
        // 跳过忽略区域
        if y <= IGNORE_AREA_TOP || y > image.height() - IGNORE_AREA_BOTTOM {
            return;
        }

        for (x, _, px) in row {
            // 跳过透明像素
            if px.0[3] != 255 {
                return;
            }
            // 跳过状态栏区域（避免误识别）
            else if x <= 250 && y <= 110 {
                continue;
            }

            // 检查被动怪物
            if Self::pixel_matches(&px.0, &ref_color_pas, config.passive_tolerence()) {
                drop(snd.send(MobPixel(x, y, TargetType::Mob(MobType::Passive))));
            }
            // 检查主动怪物
            else if Self::pixel_matches(&px.0, &ref_color_agg, config.aggressive_tolerence()) {
                drop(snd.send(MobPixel(x, y, TargetType::Mob(MobType::Aggressive))));
            }
            // 检查紫色怪物
            else if Self::pixel_matches(&px.0, &ref_color_violet, config.violet_tolerence()) {
                drop(snd.send(MobPixel(x, y, TargetType::Mob(MobType::Violet))));
            }
        }
    });
```

**优化**：
- 并行处理（Rayon）
- 早期返回（跳过不相关区域）
- 专门过滤状态栏（x ≤ 250, y ≤ 110）

##### 步骤 3：收集像素点
```rust
let mut mob_coords_pas: Vec<Point> = Vec::default();
let mut mob_coords_agg: Vec<Point> = Vec::default();
let mut mob_coords_violet: Vec<Point> = Vec::default();

while let Ok(px) = recv.recv() {
    match px.2 {
        TargetType::Mob(MobType::Passive) => mob_coords_pas.push(Point::new(px.0, px.1)),
        TargetType::Mob(MobType::Aggressive) => mob_coords_agg.push(Point::new(px.0, px.1)),
        TargetType::Mob(MobType::Violet) => mob_coords_violet.push(Point::new(px.0, px.1)),
        _ => unreachable!(),
    }
}
```

**分类存储**：
- 三个独立的点集合
- 按怪物类型分组

##### 步骤 4：聚类和过滤
```rust
let mobs_pas = Self::merge_cloud_into_mobs(
    Some(config),
    &PointCloud::new(mob_coords_pas),
    TargetType::Mob(MobType::Passive),
);

let mobs_agg = Self::merge_cloud_into_mobs(
    Some(config),
    &PointCloud::new(mob_coords_agg),
    TargetType::Mob(MobType::Aggressive),
);

let _mobs_violet = Self::merge_cloud_into_mobs(
    Some(config),
    &PointCloud::new(mob_coords_violet),
    TargetType::Mob(MobType::Violet),
);
```

##### 步骤 5：合并结果
```rust
Vec::from_iter(mobs_agg.into_iter().chain(mobs_pas))
```

**注意**：
- 紫色怪物被识别但不返回
- 只返回被动和主动怪物
- 主动怪物在前（优先攻击）

#### 性能
- 使用 Timer 测量
- 并行处理提升性能
- 典型耗时：5-20ms

---

### 8. identify_target_marker(blue_target) - 识别目标标记

```rust
pub fn identify_target_marker(&self, blue_target: bool) -> Option<Target>
```

**功能**：识别屏幕上的目标标记（锁定图标）。

**参数**：
- `blue_target: bool`
  - `true`：查找蓝色标记（友方/队友）
  - `false`：查找红色标记（敌对）

#### 标记颜色

##### 蓝色标记（友方）
```rust
Color::new(131, 148, 205)  // #8394CD
```
- 队友目标
- NPC
- 友方单位

##### 红色标记（敌对）
```rust
Color::new(246, 90, 106)  // #F65A6A
```
- 敌对怪物
- 攻击目标
- 战斗状态

**注释说明**：
- 原始颜色在 Azria 地图不工作
- 使用更中心的箭头颜色
- 经过实际测试调整

#### 执行流程

##### 步骤 1：设置参考颜色
```rust
let ref_color: Color = {
    if blue_target {
        Color::new(131, 148, 205)  // 蓝色
    } else {
        Color::new(246, 90, 106)   // 红色
    }
};
```

##### 步骤 2：检测像素
```rust
let recv = self.pixel_detection(
    vec![ref_color],
    0, 0, 0, 0,  // 全屏搜索
    None         // 默认容差
);
```

##### 步骤 3：收集点
```rust
let mut coords = Vec::default();

while let Ok(point) = recv.recv() {
    coords.push(point);
}
```

##### 步骤 4：聚类
```rust
let target_markers = Self::merge_cloud_into_mobs(
    None,  // 不使用配置过滤
    &PointCloud::new(coords),
    TargetType::TargetMarker
);
```

##### 步骤 5：回退机制
```rust
if !blue_target && target_markers.is_empty() {
    return self.identify_target_marker(true);
}
```

**逻辑**：
- 如果找不到红色标记
- 尝试查找蓝色标记
- 处理目标类型变化

##### 步骤 6：选择最大标记
```rust
target_markers.into_iter().max_by_key(|x| x.bounds.size())
```

**原因**：
- 可能检测到多个区域
- 选择最大的（最可能是真实标记）
- 过滤噪点

#### 使用场景
```rust
// 在 ClientStats::update 中使用
let blue_target = image.identify_target_marker(true);
let target = if blue_target.is_some() {
    blue_target
} else {
    image.identify_target_marker(false)
};

self.target_marker = target;
self.target_on_screen = target.is_some();
```

---

### 9. get_target_marker_distance(target) - 计算目标距离

```rust
pub fn get_target_marker_distance(&self, target: Target) -> i32
```

**功能**：计算目标标记到屏幕中心的距离（2D 欧几里得距离）。

#### 算法

##### 步骤 1：计算屏幕中心
```rust
let mid_x = (image.width() / 2) as i32;
let mid_y = (image.height() / 2) as i32;
```

**假设**：
- 玩家角色在屏幕中心
- 这是 3D 游戏的常见设计

##### 步骤 2：获取目标位置
```rust
let point = target.bounds.get_lowest_center_point();
```
- 使用边界框的底部中心点
- 更准确地表示目标位置

##### 步骤 3：计算欧几里得距离
```rust
(((mid_x - (point.x as i32)).pow(2) + (mid_y - (point.y as i32)).pow(2)) as f64)
    .sqrt() as i32
```

**公式**：
```
distance = √((x₁ - x₂)² + (y₁ - y₂)²)
```

**返回值**：
- 距离范围：`[0..=500]` 左右
- 单位：像素
- 用于判断攻击范围

#### 使用场景
```rust
if let Some(marker) = client_stats.target_marker {
    let distance = image.get_target_marker_distance(marker);

    // 判断是否在 AOE 范围内
    if distance < MAX_DISTANCE_FOR_AOE {
        use_aoe_skill();
    }
}
```

---

### 10. find_closest_mob(...) - 查找最近的怪物

```rust
pub fn find_closest_mob<'a>(
    &self,
    mobs: &'a [Target],
    avoid_list: Option<&Vec<(Bounds, Instant, u128)>>,
    max_distance: i32,
    _logger: &Logger,
) -> Option<&'a Target>
```

**功能**：从怪物列表中查找最近且可攻击的目标。

**这是目标选择的核心逻辑！**

#### 参数

##### mobs
- 待选择的怪物列表
- 通常来自 `identify_mobs()`

##### avoid_list
- 要避免的区域列表
- 格式：`(Bounds, Instant, u128)`
  - Bounds：避免区域
  - Instant：添加时间
  - u128：持续时间（毫秒）
- 用于避免重复攻击同一怪物

##### max_distance
- 最大攻击距离
- 超过此距离的怪物被忽略

#### 执行流程

##### 步骤 1：计算屏幕中心
```rust
let mid_x = (image.width() / 2) as i32;
let mid_y = (image.height() / 2) as i32;
```

##### 步骤 2：计算所有距离
```rust
let mut distances = Vec::default();

for mob in mobs {
    let point = mob.get_attack_coords();
    let distance = (((mid_x - (point.x as i32)).pow(2)
                   + (mid_y - (point.y as i32)).pow(2)) as f64)
        .sqrt() as i32;

    distances.push((mob, distance));
}
```

##### 步骤 3：按距离排序
```rust
distances.sort_by_key(|&(_, distance)| distance);
```
- 最近的在前
- 稳定排序

##### 步骤 4：过滤距离
```rust
distances = distances
    .into_iter()
    .filter(|&(_, distance)| distance <= max_distance)
    .collect();
```
- 移除超过最大距离的怪物

##### 步骤 5：应用避免列表
```rust
if let Some(avoided_bounds) = avoid_list {
    if let Some((mob, _distance)) = distances.iter().find(|(mob, _distance)| {
        let coords = mob.get_attack_coords();
        let mut result = true;

        // 检查是否在任何避免区域内
        for avoided_item in avoided_bounds {
            if avoided_item.0.contains_point(&coords) {
                result = false;
                break;
            }
        }

        result
    }) {
        Some(mob)
    } else {
        None
    }
}
```

**逻辑**：
- 遍历已排序的怪物列表
- 检查攻击坐标是否在避免区域内
- 返回第一个不在避免区域的怪物

##### 步骤 6：返回最近怪物
```rust
else {
    if let Some((mob, _distance)) = distances.first() {
        Some(*mob)
    } else {
        None
    }
}
```
- 没有避免列表时
- 返回最近的怪物

#### 使用示例
```rust
// 识别怪物
let mobs = image.identify_mobs(config);

// 过滤类型
let passive_mobs: Vec<Target> = mobs.iter()
    .filter(|m| matches!(m.target_type, TargetType::Mob(MobType::Passive)))
    .cloned()
    .collect();

// 查找最近的
let closest = image.find_closest_mob(
    &passive_mobs,
    Some(&self.avoided_bounds),  // 避免列表
    325,                          // 最大距离
    &logger
);

if let Some(mob) = closest {
    // 攻击这个怪物
    let attack_pos = mob.get_attack_coords();
    eval_mob_click(&window, attack_pos);
}
```

---

## 算法详解

### 聚类算法

#### 两阶段聚类的原理

```
原始点云：
  1 2 3         45 46 47        90 91
  1 2 3         45 46 47        90 91

第一阶段（X 轴，距离 50）：
  [1,2,3]       [45,46,47]      [90,91]
   簇 A           簇 B             簇 C

第二阶段（Y 轴，距离 3）：
  簇 A 内：[点集1] [点集2]
  簇 B 内：[点集3]
  簇 C 内：[点集4]

最终结果：4 个独立的目标
```

**优势**：
- 适应怪物名称的矩形形状
- 处理垂直重叠
- 准确分离相邻目标

### 颜色匹配容差

#### 为什么需要容差？

1. **渲染差异**：
   - 不同显卡
   - 不同设置
   - 抗锯齿效果

2. **光照变化**：
   - 不同地图
   - 昼夜系统
   - 特效影响

3. **压缩损失**：
   - 截图压缩
   - 颜色空间转换

#### 容差选择

| 检测类型 | 典型容差 | 原因 |
|---------|---------|------|
| 状态栏  | 2       | 纯色，无干扰 |
| 被动怪物 | 5       | 黄色，较稳定 |
| 主动怪物 | 10      | 红色，变化大 |
| 紫色怪物 | 10      | 紫色，复杂 |

### 距离计算

#### 2D vs 3D

**当前实现**：2D 屏幕距离
```rust
√((x₁ - x₂)² + (y₁ - y₂)²)
```

**优点**：
- 简单快速
- 直观准确
- 适合固定摄像头视角

**局限**：
- 不考虑 Z 轴深度
- 假设平面距离 = 实际距离

**在实践中足够准确**：
- Flyff Universe 是固定视角
- 深度差异不大
- 2D 距离与 3D 距离高度相关

---

## 性能优化

### 1. 并行处理

```rust
image
    .enumerate_rows()
    .par_bridge()      // Rayon 并行
    .for_each(...)
```

**效果**：
- 利用多核 CPU
- 线性加速比（接近核心数）
- 显著降低帧时间

### 2. 早期返回

```rust
if y <= IGNORE_AREA_TOP || y > image_height - IGNORE_AREA_BOTTOM {
    return;  // 跳过整行
}

if px.0[3] != 255 {
    return;  // 跳过透明像素
}
```

**效果**：
- 减少不必要的计算
- 避免检查明显无关的区域

### 3. 内联优化

```rust
#[inline(always)]
fn pixel_matches(...)
```

**效果**：
- 消除函数调用开销
- 在热路径中关键
- 每帧调用数百万次

### 4. Channel 通信

```rust
let (snd, recv) = sync_channel::<Point>(4096);
```

**效果**：
- 异步处理结果
- 避免大内存分配
- 流式处理

### 5. Timer 性能监控

```rust
let _timer = Timer::start_new("identify_mobs");
```

**用途**：
- 识别瓶颈
- 指导优化
- 性能回归测试

---

## 常见问题和调试

### 1. 怪物识别失败

**原因**：
- 颜色配置不正确
- 容差太小
- 忽略区域覆盖了怪物

**调试**：
```rust
// 增加容差
config.passive_tolerence = 10;

// 调整颜色
config.passive_mobs_colors = [230, 230, 145];

// 检查忽略区域
println!("IGNORE_AREA_TOP: {}", IGNORE_AREA_TOP);
println!("IGNORE_AREA_BOTTOM: {}", IGNORE_AREA_BOTTOM);
```

### 2. 误识别

**原因**：
- UI 元素与怪物颜色相似
- 容差太大
- 聚类参数不当

**解决**：
```rust
// 降低容差
config.passive_tolerence = 3;

// 添加区域过滤
if x <= 250 && y <= 110 {
    continue;  // 跳过状态栏区域
}

// 调整尺寸过滤
config.min_mobs_name_width = 15;
config.max_mobs_name_width = 150;
```

### 3. 性能问题

**原因**：
- 截图耗时
- 全图扫描
- 聚类算法复杂

**优化**：
```rust
// 限制搜索区域
let recv = self.pixel_detection(
    colors,
    100, 100,  // 只搜索中心区域
    700, 500,
    Some(5)
);

// 降低截图频率
if frame_count % 2 == 0 {
    image_analyzer.capture_window(&logger);
}
```

### 4. 窗口尺寸问题

**原因**：
- 窗口大小改变
- 坐标系统混乱

**解决**：
```rust
// 强制固定窗口大小
window.set_size(Size::Logical(LogicalSize {
    width: 800.0,
    height: 600.0,
}));
window.set_resizable(false);
```

---

## 使用示例

### 完整的怪物识别和攻击流程

```rust
// 1. 捕获窗口
image_analyzer.capture_window(&logger);

if !image_analyzer.image_is_some() {
    return;  // 截图失败
}

// 2. 更新游戏状态
image_analyzer.client_stats.update(&image_analyzer, &logger);

// 3. 识别怪物
let mobs = image_analyzer.identify_mobs(config);

// 4. 优先级过滤
let priority_mobs = if config.prioritize_aggro() {
    // 优先主动怪物
    let aggressive: Vec<Target> = mobs.iter()
        .filter(|m| matches!(m.target_type, TargetType::Mob(MobType::Aggressive)))
        .cloned()
        .collect();

    if aggressive.is_empty() {
        // 没有主动怪物，攻击被动怪物
        mobs.iter()
            .filter(|m| matches!(m.target_type, TargetType::Mob(MobType::Passive)))
            .cloned()
            .collect()
    } else {
        aggressive
    }
} else {
    mobs
};

// 5. 查找最近目标
let closest = image_analyzer.find_closest_mob(
    &priority_mobs,
    Some(&avoided_bounds),
    max_distance,
    &logger
);

// 6. 攻击目标
if let Some(mob) = closest {
    let attack_pos = mob.get_attack_coords();
    eval_mob_click(&window, attack_pos);
}
```

---

## 总结

### ImageAnalyzer 的核心作用

1. **视觉感知**：
   - 捕获游戏画面
   - 识别游戏元素
   - 提供视觉信息

2. **目标识别**：
   - 怪物检测和分类
   - 目标标记识别
   - 距离计算

3. **智能决策**：
   - 目标选择
   - 避免重复攻击
   - 优先级管理

4. **高性能**：
   - 并行处理
   - 优化算法
   - 实时分析

### 架构特点

1. **模块化设计**：
   - 清晰的职责划分
   - 可复用的算法
   - 易于测试

2. **跨平台支持**：
   - libscreenshot 抽象
   - 平台特定配置
   - 统一接口

3. **可配置性**：
   - 颜色自定义
   - 容差调整
   - 区域过滤

4. **性能优先**：
   - 并行计算
   - 内联优化
   - 智能缓存

### 关键算法

1. **像素检测**：
   - 并行扫描
   - 容差匹配
   - Channel 通信

2. **聚类算法**：
   - 两阶段聚类
   - 距离阈值
   - 尺寸过滤

3. **距离计算**：
   - 欧几里得距离
   - 屏幕坐标系
   - 排序和选择

ImageAnalyzer 是机器人的"眼睛"，它将原始像素转换为有意义的游戏信息，是整个自动化系统的基础。