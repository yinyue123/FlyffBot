# Chrome DevTools Protocol (CDP) Screencast 流程详解

## 概述

Screencast 是 Chrome DevTools Protocol 提供的一种实时屏幕捕获机制，通过事件流的方式持续推送浏览器渲染的帧数据。相比传统的截图方式（CaptureScreenshot），screencast 提供了更高的性能和更低的延迟。

## 底层原理

### 1. CDP 事件驱动模型

CDP 基于 WebSocket 通信，使用事件驱动模型：
- **命令（Commands）**：客户端向浏览器发送请求（如 `Page.startScreencast`）
- **事件（Events）**：浏览器主动推送数据给客户端（如 `Page.screencastFrame`）

### 2. Screencast 工作机制

```
Client                          Chrome Browser
  |                                    |
  |---> Page.startScreencast()-------->|
  |                                    | 开始捕获屏幕
  |                                    | (每次渲染后生成帧)
  |                                    |
  |<--- Page.screencastFrame event ----|  事件1: 帧数据
  |                                    |  (base64 编码的 JPEG)
  |                                    |  等待 ACK...
  |                                    |
  |---> Page.screencastFrameAck()----->|
  |                                    |  继续捕获
  |<--- Page.screencastFrame event ----|  事件2: 下一帧
  |                                    |
  |---> Page.screencastFrameAck()----->|
  |                                    |
  ...                                 ...
  |                                    |
  |---> Page.stopScreencast()--------->|
  |                                    | 停止捕获
```

**关键特性：**
- **背压机制（Back Pressure）**：浏览器发送一帧后，必须等待客户端的 ACK 才会发送下一帧
- **异步推送**：帧数据通过事件异步推送，不需要客户端主动请求
- **格式压缩**：帧数据以 JPEG 格式编码，通过 base64 传输

### 3. 数据流转

```
Browser Rendering → Capture Frame → Encode to JPEG
    → Base64 Encode → WebSocket Event → Client Listener
    → Base64 Decode → JPEG Decode → RGBA Image → Channel
```

## debug_hsv.go 实现分析

### 完整流程时序图

```
时间线                    操作
────────────────────────────────────────────────────────
T0   NewDebugBrowser()
     └─ 创建 frameChan (buffer=1)

T1   Start()
     ├─ 创建 Context
     ├─ setupScreencastListener()  ← 必须在导航前设置
     │  └─ chromedp.ListenTarget() 注册监听器
     │     (此时监听器已激活，等待事件)
     │
     ├─ setCookies()
     │
     ├─ page.Navigate()  (不等待完全加载)
     │  └─ 页面开始加载...
     │
     ├─ Sleep(2s)  等待页面开始渲染
     │
     └─ page.StartScreencast()
        └─ 浏览器开始发送帧事件 ──────┐
                                      │
T2   (异步) EventScreencastFrame     │
     ├─ base64.Decode(ev.Data)       │
     ├─ image.Decode(jpeg data)      │
     ├─ 转换为 RGBA                   │
     ├─ frameChan <- rgba (非阻塞)   │ ← 监听器回调
     └─ page.ScreencastFrameAck()    │
        (异步执行，不阻塞)              │
                                      │
T3   (异步) EventScreencastFrame     │
     └─ ... 重复流程                  │
                                      │
T4   main() 等待第一帧               │
     └─ for i < 100                   │
        └─ GetFrame() (非阻塞)        │
           └─ select from frameChan  │
              └─ 成功获取! ───────────┘

T5   主循环
     └─ GetFrame() 持续获取最新帧
```

### 关键代码片段分析

#### 1. Channel 设计

```go
frameChan: make(chan *image.RGBA, 1)  // Buffer = 1
```

**为什么 buffer = 1？**
- **只保留最新帧**：新帧到来时，如果 channel 满了会丢弃（非阻塞发送）
- **避免积压**：不会累积大量过时的帧
- **低延迟**：始终读取最新的画面

#### 2. 监听器必须在导航前设置

```go
// Start screencast BEFORE navigation
b.setupScreencastListener()

// 然后才导航
page.Navigate("https://universe.flyff.com/play")
```

**原因：**
- `chromedp.ListenTarget()` 会注册到当前 context
- 如果在导航后注册，可能错过早期的事件
- 监听器一旦注册，会持续监听所有 target 事件

#### 3. 非阻塞发送到 Channel

```go
select {
case b.frameChan <- rgba:
    // Successfully sent frame
default:
    // Drop frame if channel is full
}
```

**设计目的：**
- **监听器回调不能阻塞**：如果阻塞会影响整个事件循环
- **丢帧策略**：当消费者（GetFrame）读取慢时，自动丢弃过时的帧
- **保证实时性**：总是获取最新画面

#### 4. 异步 ACK 确认

```go
go chromedp.Run(b.ctx, page.ScreencastFrameAck(ev.SessionID))
```

**为什么使用 goroutine？**
- **避免阻塞监听器**：ACK 操作需要通过 WebSocket 发送命令
- **异步确认**：不影响下一帧的处理
- **性能优化**：监听器回调可以快速返回

#### 5. 等待第一帧的策略

```go
// 在 main() 中，Start() 完成后
for i := 0; i < 100; i++ {  // 最多等待 10 秒
    if frame, ok := browser.GetFrame(); ok {
        // 获取到第一帧，可以开始处理
        break
    }
    time.Sleep(100 * time.Millisecond)
}
```

**为什么需要等待？**
- screencast 启动后，第一帧需要时间：
  1. 页面开始渲染
  2. 浏览器捕获帧
  3. 编码为 JPEG
  4. 通过 WebSocket 发送
  5. 客户端接收并解码
- 这个过程通常需要 1-3 秒

## 常见问题

### 1. "no frame available" 错误

**原因：**
- Capture() 在 screencast 开始前被调用
- 页面还没有渲染出第一帧
- 监听器设置不正确

**解决方案：**
```go
// 在 Start() 中等待第一帧
for i := 0; i < 100; i++ {
    if _, ok := browser.GetFrame(); ok {
        break
    }
    time.Sleep(100 * time.Millisecond)
}
```

### 2. 帧率过低

**原因：**
- 没有及时调用 ScreencastFrameAck
- 监听器回调被阻塞

**解决方案：**
- 使用异步 ACK：`go chromedp.Run(...)`
- 监听器中避免耗时操作

### 3. 内存泄漏

**原因：**
- 大量帧累积在 channel 中
- 忘记关闭 channel

**解决方案：**
- 使用 buffer = 1 的 channel
- 使用非阻塞发送（select + default）
- 在 Stop() 中关闭 channel

## 性能对比

| 方式 | 延迟 | CPU 占用 | 适用场景 |
|------|------|----------|----------|
| CaptureScreenshot | 每次请求 100-300ms | 低 | 偶尔截图 |
| Screencast | 持续 16-33ms (30-60fps) | 中 | 实时监控 |

## 最佳实践

1. **初始化顺序**
   ```go
   CreateContext() → SetupListener() → Navigate() → StartScreencast()
   ```

2. **等待第一帧**
   ```go
   // 在开始使用 Capture() 前
   waitForFirstFrame(browser, 10*time.Second)
   ```

3. **清理资源**
   ```go
   defer func() {
       page.StopScreencast()
       close(frameChan)
   }()
   ```

4. **错误处理**
   ```go
   frame, err := browser.Capture()
   if err != nil {
       // 降级到截图模式
       fallbackToCaptureScreenshot()
   }
   ```

## 总结

Screencast 是一个**事件驱动 + 背压控制**的实时屏幕流系统：
- **监听器**：异步接收浏览器推送的帧事件
- **Channel**：缓冲区保存最新帧，自动丢弃过时数据
- **ACK 机制**：控制帧率，避免客户端过载
- **非阻塞设计**：保证高性能和实时性

相比传统截图，screencast 更适合需要持续监控浏览器画面的场景。
