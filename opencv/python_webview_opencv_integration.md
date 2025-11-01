# Python WebView + OpenCV 集成方案

本文档提供多种Python加载WebView、截图、JS注入和Cookie管理的完整方案。

## 方案对比

| 方案 | 性能 | 易用性 | 截图质量 | Cookie管理 | JS注入 | 跨平台 | 推荐场景 |
|-----|------|--------|---------|-----------|--------|--------|---------|
| pywebview | 好 | 简单 | 高 | 简单 | 容易 | 是 | 轻量级应用、游戏辅助 |
| Selenium | 中 | 简单 | 高 | 自动 | 容易 | 是 | Web自动化、爬虫 |
| Playwright | 优秀 | 中等 | 高 | 自动 | 容易 | 是 | 现代Web自动化 |
| PyQt5 WebEngine | 好 | 复杂 | 高 | 手动 | 容易 | 是 | 完整GUI应用 |
| CEF Python | 优秀 | 复杂 | 高 | 手动 | 容易 | 是 | 高性能嵌入式浏览器 |

---

## 方案1：pywebview（推荐用于游戏辅助）

**优点**：轻量级、原生WebView、性能好、API简单
**缺点**：功能相对简单

### 1.1 安装依赖

```bash
pip install pywebview opencv-python pillow numpy
# Windows需要: pip install pythonnet
# Linux需要: pip install PyGObject
# macOS自动使用WebKit
```

### 1.2 完整实现

```python
import webview
import cv2
import numpy as np
from PIL import Image
import threading
import time
import json
import os
from pathlib import Path

class WebViewCapture:
    def __init__(self, url, width=1280, height=720):
        """
        初始化WebView截图捕获器

        参数:
            url: 要加载的网页URL
            width: 窗口宽度
            height: 窗口高度
        """
        self.url = url
        self.width = width
        self.height = height
        self.window = None
        self.is_running = False
        self.screenshot_interval = 0.1  # 截图间隔（秒）
        self.cookie_file = "webview_cookies.json"

        # OpenCV处理回调
        self.frame_callback = None

    def create_window(self):
        """创建WebView窗口"""
        # 加载保存的Cookie
        cookies = self.load_cookies()

        self.window = webview.create_window(
            title='WebView Capture',
            url=self.url,
            width=self.width,
            height=self.height,
            resizable=True,
            fullscreen=False
        )

        # 窗口加载完成后的回调
        self.window.events.loaded += self.on_loaded
        self.window.events.closing += self.on_closing

        return self.window

    def on_loaded(self):
        """窗口加载完成回调"""
        print("WebView加载完成")

        # 恢复Cookie
        cookies = self.load_cookies()
        if cookies:
            self.restore_cookies(cookies)

        # 开始截图循环
        self.is_running = True
        threading.Thread(target=self.capture_loop, daemon=True).start()

    def on_closing(self):
        """窗口关闭回调"""
        print("正在保存Cookie...")
        self.save_cookies()
        self.is_running = False

    def capture_loop(self):
        """截图循环"""
        while self.is_running:
            try:
                # 等待窗口准备就绪
                if not self.window:
                    time.sleep(0.1)
                    continue

                # 截图
                screenshot = self.capture_screenshot()

                if screenshot is not None and self.frame_callback:
                    # 调用OpenCV处理回调
                    self.frame_callback(screenshot)

                time.sleep(self.screenshot_interval)

            except Exception as e:
                print(f"截图错误: {e}")
                time.sleep(0.5)

    def capture_screenshot(self):
        """
        捕获WebView截图

        返回:
            numpy.ndarray: OpenCV格式的图像(BGR)
        """
        try:
            # pywebview截图返回PNG字节
            png_bytes = self.window.evaluate_js("""
                (function() {
                    const canvas = document.createElement('canvas');
                    canvas.width = window.innerWidth;
                    canvas.height = window.innerHeight;
                    const ctx = canvas.getContext('2d');

                    // 这种方法在某些情况下可能不工作
                    // 改用window.pywebview的截图API
                    return null;
                })();
            """)

            # 使用pywebview的内置截图方法（如果支持）
            # 注意：某些平台可能不支持直接截图
            # 替代方案：使用PIL截图整个窗口

            from PIL import ImageGrab
            import pyautogui

            # 获取窗口位置（这是一个简化版本）
            # 实际应用中需要获取窗口的真实坐标
            screenshot = pyautogui.screenshot()

            # 转换为OpenCV格式
            screenshot_np = np.array(screenshot)
            screenshot_bgr = cv2.cvtColor(screenshot_np, cv2.COLOR_RGB2BGR)

            return screenshot_bgr

        except Exception as e:
            print(f"截图失败: {e}")
            return None

    def execute_js(self, js_code):
        """
        执行JavaScript代码

        参数:
            js_code: JavaScript代码字符串

        返回:
            执行结果
        """
        if self.window:
            return self.window.evaluate_js(js_code)
        return None

    def click_element(self, selector):
        """
        点击页面元素

        参数:
            selector: CSS选择器
        """
        js_code = f"""
            (function() {{
                const element = document.querySelector('{selector}');
                if (element) {{
                    element.click();
                    return true;
                }}
                return false;
            }})();
        """
        return self.execute_js(js_code)

    def input_text(self, selector, text):
        """
        向输入框输入文本

        参数:
            selector: CSS选择器
            text: 要输入的文本
        """
        js_code = f"""
            (function() {{
                const element = document.querySelector('{selector}');
                if (element) {{
                    element.value = '{text}';
                    element.dispatchEvent(new Event('input', {{ bubbles: true }}));
                    return true;
                }}
                return false;
            }})();
        """
        return self.execute_js(js_code)

    def get_element_position(self, selector):
        """
        获取元素在页面中的位置

        参数:
            selector: CSS选择器

        返回:
            dict: {x, y, width, height}
        """
        js_code = f"""
            (function() {{
                const element = document.querySelector('{selector}');
                if (element) {{
                    const rect = element.getBoundingClientRect();
                    return {{
                        x: rect.left,
                        y: rect.top,
                        width: rect.width,
                        height: rect.height
                    }};
                }}
                return null;
            }})();
        """
        return self.execute_js(js_code)

    def save_cookies(self):
        """保存Cookie到文件"""
        js_code = """
            (function() {
                return document.cookie;
            })();
        """
        cookies_str = self.execute_js(js_code)

        if cookies_str:
            # 解析Cookie字符串
            cookies = {}
            for cookie in cookies_str.split(';'):
                if '=' in cookie:
                    key, value = cookie.strip().split('=', 1)
                    cookies[key] = value

            # 保存到文件
            with open(self.cookie_file, 'w') as f:
                json.dump(cookies, f, indent=2)

            print(f"Cookie已保存到 {self.cookie_file}")

    def load_cookies(self):
        """从文件加载Cookie"""
        if os.path.exists(self.cookie_file):
            with open(self.cookie_file, 'r') as f:
                return json.load(f)
        return {}

    def restore_cookies(self, cookies):
        """恢复Cookie到浏览器"""
        for key, value in cookies.items():
            js_code = f"""
                document.cookie = "{key}={value}; path=/";
            """
            self.execute_js(js_code)

        print("Cookie已恢复")

    def set_frame_callback(self, callback):
        """
        设置帧处理回调函数

        参数:
            callback: 回调函数，接收cv2图像作为参数
        """
        self.frame_callback = callback

    def start(self):
        """启动WebView"""
        self.create_window()
        webview.start(debug=True)


# 使用示例
def opencv_process_frame(frame):
    """OpenCV处理帧的回调函数"""
    # 转换为灰度图
    gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)

    # 检测边缘
    edges = cv2.Canny(gray, 50, 150)

    # 查找轮廓
    contours, _ = cv2.findContours(edges, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

    # 在原图上绘制轮廓
    result = frame.copy()
    cv2.drawContours(result, contours, -1, (0, 255, 0), 2)

    # 显示结果
    cv2.imshow('OpenCV Processing', result)
    cv2.waitKey(1)


if __name__ == '__main__':
    # 创建WebView捕获器
    capture = WebViewCapture(
        url='https://example.com',
        width=1280,
        height=720
    )

    # 设置OpenCV处理回调
    capture.set_frame_callback(opencv_process_frame)

    # 启动（阻塞式）
    capture.start()
```

### 1.3 高级功能模块

#### 自动化操作模块
```python
class WebViewAutomation:
    """WebView自动化操作模块"""

    def __init__(self, capture: WebViewCapture):
        self.capture = capture

    def auto_login(self, username, password):
        """自动登录"""
        # 等待页面加载
        time.sleep(2)

        # 输入用户名
        self.capture.input_text('#username', username)

        # 输入密码
        self.capture.input_text('#password', password)

        # 点击登录按钮
        self.capture.click_element('#login-button')

    def monitor_element(self, selector, callback, interval=1.0):
        """监控元素变化"""
        def monitor_loop():
            last_state = None
            while True:
                current_state = self.capture.execute_js(f"""
                    (function() {{
                        const el = document.querySelector('{selector}');
                        return el ? el.textContent : null;
                    }})();
                """)

                if current_state != last_state:
                    callback(current_state)
                    last_state = current_state

                time.sleep(interval)

        threading.Thread(target=monitor_loop, daemon=True).start()

    def inject_custom_js(self, js_file_path):
        """注入自定义JS文件"""
        with open(js_file_path, 'r', encoding='utf-8') as f:
            js_code = f.read()

        return self.capture.execute_js(js_code)
```

#### 游戏辅助模块
```python
class GameAssist:
    """游戏辅助模块"""

    def __init__(self, capture: WebViewCapture):
        self.capture = capture
        self.running = False

    def find_and_click_by_template(self, template_path, threshold=0.8):
        """
        通过模板匹配查找并点击

        参数:
            template_path: 模板图片路径
            threshold: 匹配阈值
        """
        # 获取当前截图
        screenshot = self.capture.capture_screenshot()
        if screenshot is None:
            return False

        # 读取模板
        template = cv2.imread(template_path, cv2.IMREAD_COLOR)
        if template is None:
            print(f"无法读取模板: {template_path}")
            return False

        # 转换为灰度图
        gray_screenshot = cv2.cvtColor(screenshot, cv2.COLOR_BGR2GRAY)
        gray_template = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY)

        # 模板匹配
        result = cv2.matchTemplate(gray_screenshot, gray_template, cv2.TM_CCOEFF_NORMED)
        min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)

        if max_val >= threshold:
            # 计算中心点
            h, w = gray_template.shape
            center_x = max_loc[0] + w // 2
            center_y = max_loc[1] + h // 2

            # 执行点击（通过JS）
            self.capture.execute_js(f"""
                (function() {{
                    const event = new MouseEvent('click', {{
                        view: window,
                        bubbles: true,
                        cancelable: true,
                        clientX: {center_x},
                        clientY: {center_y}
                    }});
                    document.elementFromPoint({center_x}, {center_y}).dispatchEvent(event);
                }})();
            """)

            print(f"找到并点击目标，置信度: {max_val:.2f}")
            return True

        return False

    def find_color_region(self, lower_hsv, upper_hsv, min_area=500):
        """
        查找指定颜色区域

        参数:
            lower_hsv: HSV下限 [H, S, V]
            upper_hsv: HSV上限 [H, S, V]
            min_area: 最小面积

        返回:
            list: 找到的区域列表 [{x, y, w, h}, ...]
        """
        screenshot = self.capture.capture_screenshot()
        if screenshot is None:
            return []

        # 转换到HSV
        hsv = cv2.cvtColor(screenshot, cv2.COLOR_BGR2HSV)

        # 创建掩码
        mask = cv2.inRange(hsv, np.array(lower_hsv), np.array(upper_hsv))

        # 形态学处理
        kernel = np.ones((5, 5), np.uint8)
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)

        # 查找轮廓
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        regions = []
        for contour in contours:
            area = cv2.contourArea(contour)
            if area > min_area:
                x, y, w, h = cv2.boundingRect(contour)
                regions.append({
                    'x': x, 'y': y, 'w': w, 'h': h,
                    'center': (x + w//2, y + h//2),
                    'area': area
                })

        return regions

    def auto_battle(self):
        """自动战斗示例"""
        self.running = True

        def battle_loop():
            while self.running:
                # 查找敌人（假设敌人是红色）
                enemies = self.find_color_region(
                    lower_hsv=[0, 100, 100],
                    upper_hsv=[10, 255, 255],
                    min_area=1000
                )

                if enemies:
                    # 点击第一个敌人
                    enemy = enemies[0]
                    center = enemy['center']

                    self.capture.execute_js(f"""
                        (function() {{
                            const event = new MouseEvent('click', {{
                                view: window,
                                bubbles: true,
                                cancelable: true,
                                clientX: {center[0]},
                                clientY: {center[1]}
                            }});
                            document.elementFromPoint({center[0]}, {center[1]}).dispatchEvent(event);
                        }})();
                    """)

                    print(f"攻击敌人: {center}")

                time.sleep(0.5)

        threading.Thread(target=battle_loop, daemon=True).start()

    def stop(self):
        """停止自动化"""
        self.running = False
```

---

## 方案2：Selenium（最成熟的方案）

**优点**：功能强大、社区活跃、文档完善、Cookie自动管理
**缺点**：需要浏览器驱动、资源占用较大

### 2.1 安装依赖

```bash
pip install selenium opencv-python pillow numpy webdriver-manager
```

### 2.2 完整实现

```python
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager
import cv2
import numpy as np
from PIL import Image
import io
import pickle
import os
import time
import threading

class SeleniumCapture:
    def __init__(self, url, width=1280, height=720, headless=False):
        """
        初始化Selenium截图捕获器

        参数:
            url: 要加载的网页URL
            width: 窗口宽度
            height: 窗口高度
            headless: 是否无头模式
        """
        self.url = url
        self.width = width
        self.height = height
        self.headless = headless
        self.driver = None
        self.is_running = False
        self.screenshot_interval = 0.1
        self.cookie_file = "selenium_cookies.pkl"
        self.frame_callback = None

    def create_driver(self):
        """创建WebDriver"""
        options = webdriver.ChromeOptions()

        if self.headless:
            options.add_argument('--headless')

        options.add_argument(f'--window-size={self.width},{self.height}')
        options.add_argument('--disable-blink-features=AutomationControlled')
        options.add_argument('--disable-gpu')
        options.add_argument('--no-sandbox')
        options.add_argument('--disable-dev-shm-usage')

        # 自动下载并使用ChromeDriver
        service = Service(ChromeDriverManager().install())
        self.driver = webdriver.Chrome(service=service, options=options)

        # 设置窗口大小
        self.driver.set_window_size(self.width, self.height)

        return self.driver

    def start(self):
        """启动浏览器"""
        self.create_driver()

        # 加载URL
        self.driver.get(self.url)

        # 等待页面加载
        time.sleep(2)

        # 恢复Cookie
        self.load_cookies()

        # 刷新页面以应用Cookie
        if os.path.exists(self.cookie_file):
            self.driver.refresh()
            time.sleep(2)

        # 开始截图循环
        self.is_running = True
        self.capture_thread = threading.Thread(target=self.capture_loop, daemon=True)
        self.capture_thread.start()

    def capture_loop(self):
        """截图循环"""
        while self.is_running:
            try:
                screenshot = self.capture_screenshot()

                if screenshot is not None and self.frame_callback:
                    self.frame_callback(screenshot)

                time.sleep(self.screenshot_interval)

            except Exception as e:
                print(f"截图错误: {e}")
                time.sleep(0.5)

    def capture_screenshot(self):
        """
        捕获浏览器截图

        返回:
            numpy.ndarray: OpenCV格式的图像(BGR)
        """
        try:
            # Selenium截图返回PNG字节
            png_bytes = self.driver.get_screenshot_as_png()

            # 使用PIL打开
            image = Image.open(io.BytesIO(png_bytes))

            # 转换为numpy数组
            image_np = np.array(image)

            # 转换为BGR格式（OpenCV使用BGR）
            image_bgr = cv2.cvtColor(image_np, cv2.COLOR_RGB2BGR)

            return image_bgr

        except Exception as e:
            print(f"截图失败: {e}")
            return None

    def execute_js(self, js_code):
        """执行JavaScript代码"""
        return self.driver.execute_script(js_code)

    def click_element(self, selector, by=By.CSS_SELECTOR):
        """
        点击元素

        参数:
            selector: 选择器
            by: 选择器类型（By.CSS_SELECTOR, By.XPATH等）
        """
        try:
            element = WebDriverWait(self.driver, 10).until(
                EC.element_to_be_clickable((by, selector))
            )
            element.click()
            return True
        except Exception as e:
            print(f"点击元素失败: {e}")
            return False

    def input_text(self, selector, text, by=By.CSS_SELECTOR):
        """输入文本"""
        try:
            element = WebDriverWait(self.driver, 10).until(
                EC.presence_of_element_located((by, selector))
            )
            element.clear()
            element.send_keys(text)
            return True
        except Exception as e:
            print(f"输入文本失败: {e}")
            return False

    def get_element_position(self, selector, by=By.CSS_SELECTOR):
        """获取元素位置"""
        try:
            element = self.driver.find_element(by, selector)
            location = element.location
            size = element.size

            return {
                'x': location['x'],
                'y': location['y'],
                'width': size['width'],
                'height': size['height']
            }
        except Exception as e:
            print(f"获取元素位置失败: {e}")
            return None

    def save_cookies(self):
        """保存Cookie"""
        cookies = self.driver.get_cookies()
        with open(self.cookie_file, 'wb') as f:
            pickle.dump(cookies, f)
        print(f"Cookie已保存到 {self.cookie_file}")

    def load_cookies(self):
        """加载Cookie"""
        if os.path.exists(self.cookie_file):
            with open(self.cookie_file, 'rb') as f:
                cookies = pickle.load(f)

            for cookie in cookies:
                try:
                    self.driver.add_cookie(cookie)
                except Exception as e:
                    print(f"恢复Cookie失败: {e}")

            print("Cookie已恢复")

    def set_frame_callback(self, callback):
        """设置帧处理回调"""
        self.frame_callback = callback

    def stop(self):
        """停止并保存Cookie"""
        self.is_running = False
        if self.capture_thread:
            self.capture_thread.join(timeout=2)

        if self.driver:
            self.save_cookies()
            self.driver.quit()

    def __enter__(self):
        """上下文管理器入口"""
        self.start()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """上下文管理器退出"""
        self.stop()


# 使用示例
if __name__ == '__main__':
    def process_frame(frame):
        # OpenCV处理
        gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
        edges = cv2.Canny(gray, 50, 150)
        cv2.imshow('Selenium Capture', edges)
        cv2.waitKey(1)

    with SeleniumCapture('https://example.com', headless=False) as capture:
        capture.set_frame_callback(process_frame)

        # 等待用户操作
        input("按Enter键保存Cookie并退出...")
```

### 2.3 自动化模块

```python
class SeleniumAutomation:
    """Selenium自动化模块"""

    def __init__(self, capture: SeleniumCapture):
        self.capture = capture
        self.driver = capture.driver

    def wait_for_element(self, selector, timeout=10, by=By.CSS_SELECTOR):
        """等待元素出现"""
        try:
            element = WebDriverWait(self.driver, timeout).until(
                EC.presence_of_element_located((by, selector))
            )
            return element
        except Exception as e:
            print(f"等待元素超时: {e}")
            return None

    def scroll_to_element(self, selector, by=By.CSS_SELECTOR):
        """滚动到元素"""
        try:
            element = self.driver.find_element(by, selector)
            self.driver.execute_script("arguments[0].scrollIntoView();", element)
            return True
        except Exception as e:
            print(f"滚动失败: {e}")
            return False

    def inject_script(self, script_path):
        """注入外部JS脚本"""
        with open(script_path, 'r', encoding='utf-8') as f:
            script = f.read()
        return self.capture.execute_js(script)

    def monitor_network(self):
        """监控网络请求（需要Chrome DevTools Protocol）"""
        # 启用网络监控
        self.driver.execute_cdp_cmd('Network.enable', {})

        def get_requests():
            return self.driver.execute_cdp_cmd('Network.getAllCookies', {})

        return get_requests()
```

---

## 方案3：Playwright（现代化方案）

**优点**：速度快、API现代化、支持多浏览器、自动等待、更好的截图
**缺点**：相对较新、中文文档较少

### 3.1 安装依赖

```bash
pip install playwright opencv-python numpy
python -m playwright install chromium
```

### 3.2 完整实现

```python
from playwright.sync_api import sync_playwright, Page, Browser
import cv2
import numpy as np
from PIL import Image
import io
import json
import os
import time
import threading

class PlaywrightCapture:
    def __init__(self, url, width=1280, height=720, headless=False):
        """
        初始化Playwright截图捕获器

        参数:
            url: 要加载的网页URL
            width: 窗口宽度
            height: 窗口高度
            headless: 是否无头模式
        """
        self.url = url
        self.width = width
        self.height = height
        self.headless = headless
        self.playwright = None
        self.browser = None
        self.context = None
        self.page = None
        self.is_running = False
        self.screenshot_interval = 0.033  # 约30fps
        self.cookie_file = "playwright_cookies.json"
        self.frame_callback = None

    def start(self):
        """启动浏览器"""
        # 创建Playwright实例
        self.playwright = sync_playwright().start()

        # 启动浏览器
        self.browser = self.playwright.chromium.launch(
            headless=self.headless,
            args=['--disable-blink-features=AutomationControlled']
        )

        # 创建上下文（可以设置视口大小）
        self.context = self.browser.new_context(
            viewport={'width': self.width, 'height': self.height},
            user_agent='Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
        )

        # 恢复Cookie
        if os.path.exists(self.cookie_file):
            with open(self.cookie_file, 'r') as f:
                cookies = json.load(f)
                self.context.add_cookies(cookies)

        # 创建页面
        self.page = self.context.new_page()

        # 导航到URL
        self.page.goto(self.url)

        # 等待页面加载
        self.page.wait_for_load_state('networkidle')

        # 开始截图循环
        self.is_running = True
        self.capture_thread = threading.Thread(target=self.capture_loop, daemon=True)
        self.capture_thread.start()

    def capture_loop(self):
        """截图循环"""
        while self.is_running:
            try:
                screenshot = self.capture_screenshot()

                if screenshot is not None and self.frame_callback:
                    self.frame_callback(screenshot)

                time.sleep(self.screenshot_interval)

            except Exception as e:
                print(f"截图错误: {e}")
                time.sleep(0.5)

    def capture_screenshot(self):
        """
        捕获浏览器截图

        返回:
            numpy.ndarray: OpenCV格式的图像(BGR)
        """
        try:
            # Playwright截图（高性能）
            screenshot_bytes = self.page.screenshot(type='png', full_page=False)

            # 转换为PIL Image
            image = Image.open(io.BytesIO(screenshot_bytes))

            # 转换为numpy数组
            image_np = np.array(image)

            # 转换为BGR（OpenCV格式）
            if len(image_np.shape) == 3:
                if image_np.shape[2] == 4:  # RGBA
                    image_bgr = cv2.cvtColor(image_np, cv2.COLOR_RGBA2BGR)
                else:  # RGB
                    image_bgr = cv2.cvtColor(image_np, cv2.COLOR_RGB2BGR)
            else:
                image_bgr = image_np

            return image_bgr

        except Exception as e:
            print(f"截图失败: {e}")
            return None

    def execute_js(self, js_code):
        """执行JavaScript"""
        return self.page.evaluate(js_code)

    def click_element(self, selector):
        """点击元素"""
        try:
            self.page.click(selector, timeout=10000)
            return True
        except Exception as e:
            print(f"点击失败: {e}")
            return False

    def input_text(self, selector, text):
        """输入文本"""
        try:
            self.page.fill(selector, text)
            return True
        except Exception as e:
            print(f"输入失败: {e}")
            return False

    def get_element_position(self, selector):
        """获取元素位置"""
        try:
            box = self.page.locator(selector).bounding_box()
            if box:
                return {
                    'x': box['x'],
                    'y': box['y'],
                    'width': box['width'],
                    'height': box['height']
                }
        except Exception as e:
            print(f"获取位置失败: {e}")
        return None

    def save_cookies(self):
        """保存Cookie"""
        cookies = self.context.cookies()
        with open(self.cookie_file, 'w') as f:
            json.dump(cookies, f, indent=2)
        print(f"Cookie已保存到 {self.cookie_file}")

    def set_frame_callback(self, callback):
        """设置帧处理回调"""
        self.frame_callback = callback

    def stop(self):
        """停止浏览器"""
        self.is_running = False

        if self.capture_thread:
            self.capture_thread.join(timeout=2)

        if self.context:
            self.save_cookies()

        if self.browser:
            self.browser.close()

        if self.playwright:
            self.playwright.stop()

    def __enter__(self):
        self.start()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.stop()


# 使用示例
if __name__ == '__main__':
    def process_frame(frame):
        # 检测红色区域
        hsv = cv2.cvtColor(frame, cv2.COLOR_BGR2HSV)
        mask = cv2.inRange(hsv, np.array([0, 100, 100]), np.array([10, 255, 255]))

        cv2.imshow('Playwright Capture', mask)
        cv2.waitKey(1)

    with PlaywrightCapture('https://example.com') as capture:
        capture.set_frame_callback(process_frame)

        # 自动化操作示例
        time.sleep(3)
        capture.click_element('#some-button')

        input("按Enter退出...")
```

### 3.3 高级功能

```python
class PlaywrightAdvanced:
    """Playwright高级功能"""

    def __init__(self, capture: PlaywrightCapture):
        self.capture = capture
        self.page = capture.page

    def intercept_requests(self, pattern, handler):
        """拦截网络请求"""
        def route_handler(route, request):
            # 自定义处理逻辑
            if handler(request):
                route.continue_()
            else:
                route.abort()

        self.page.route(pattern, route_handler)

    def inject_css(self, css_code):
        """注入CSS"""
        self.page.add_style_tag(content=css_code)

    def inject_js_file(self, js_path):
        """注入JS文件"""
        self.page.add_script_tag(path=js_path)

    def record_video(self, output_path):
        """录制视频（需要在创建context时设置）"""
        # 需要重新创建context
        pass

    def emulate_mobile(self, device_name='iPhone 12'):
        """模拟移动设备"""
        device = self.capture.playwright.devices[device_name]
        # 需要在创建时设置
        pass
```

---

## 方案4：PyQt5 WebEngineView

**优点**：完整的GUI框架、高度可定制、性能好
**缺点**：代码较复杂、学习曲线陡峭

### 4.1 安装依赖

```bash
pip install PyQt5 PyQtWebEngine opencv-python numpy
```

### 4.2 完整实现

```python
from PyQt5.QtCore import QUrl, QTimer, Qt, pyqtSignal, QObject
from PyQt5.QtWidgets import QApplication, QMainWindow
from PyQt5.QtWebEngineWidgets import QWebEngineView, QWebEnginePage
from PyQt5.QtGui import QImage, QPixmap
import cv2
import numpy as np
import sys
import json
import os

class WebEnginePage(QWebEnginePage):
    """自定义WebEnginePage用于JS交互"""

    def javaScriptConsoleMessage(self, level, message, lineNumber, sourceID):
        """捕获JS控制台消息"""
        print(f"JS Console: {message}")

class PyQtCapture(QMainWindow):
    # 定义信号
    frame_ready = pyqtSignal(np.ndarray)

    def __init__(self, url, width=1280, height=720):
        super().__init__()

        self.url = url
        self.width = width
        self.height = height
        self.cookie_file = "pyqt_cookies.json"

        self.init_ui()
        self.init_capture()

    def init_ui(self):
        """初始化UI"""
        self.setWindowTitle('PyQt WebEngine Capture')
        self.setGeometry(100, 100, self.width, self.height)

        # 创建WebEngineView
        self.web_view = QWebEngineView()
        self.page = WebEnginePage(self.web_view)
        self.web_view.setPage(self.page)

        # 设置为中央部件
        self.setCentralWidget(self.web_view)

        # 加载URL
        self.web_view.setUrl(QUrl(self.url))

        # 页面加载完成信号
        self.web_view.loadFinished.connect(self.on_load_finished)

    def init_capture(self):
        """初始化截图定时器"""
        self.capture_timer = QTimer()
        self.capture_timer.timeout.connect(self.capture_frame)
        self.capture_timer.start(33)  # 约30fps

    def on_load_finished(self, success):
        """页面加载完成"""
        if success:
            print("页面加载成功")
            self.load_cookies()

    def capture_frame(self):
        """捕获帧"""
        # 获取widget的图像
        pixmap = self.web_view.grab()

        # 转换为QImage
        qimage = pixmap.toImage()

        # 转换为numpy数组
        width = qimage.width()
        height = qimage.height()

        # 获取图像数据
        ptr = qimage.bits()
        ptr.setsize(height * width * 4)
        arr = np.frombuffer(ptr, np.uint8).reshape((height, width, 4))

        # 转换为BGR
        frame = cv2.cvtColor(arr, cv2.COLOR_RGBA2BGR)

        # 发射信号
        self.frame_ready.emit(frame)

    def execute_js(self, js_code, callback=None):
        """执行JavaScript"""
        if callback:
            self.page.runJavaScript(js_code, callback)
        else:
            self.page.runJavaScript(js_code)

    def click_element(self, selector):
        """点击元素"""
        js_code = f"""
            (function() {{
                const el = document.querySelector('{selector}');
                if (el) {{
                    el.click();
                    return true;
                }}
                return false;
            }})();
        """
        self.execute_js(js_code)

    def input_text(self, selector, text):
        """输入文本"""
        js_code = f"""
            (function() {{
                const el = document.querySelector('{selector}');
                if (el) {{
                    el.value = '{text}';
                    el.dispatchEvent(new Event('input'));
                    return true;
                }}
                return false;
            }})();
        """
        self.execute_js(js_code)

    def save_cookies(self):
        """保存Cookie（PyQt5需要使用QWebEngineProfile）"""
        # 注意：PyQt5的Cookie管理比较复杂
        # 可以通过JS获取document.cookie
        def cookie_callback(cookies_str):
            if cookies_str:
                cookies = {}
                for cookie in cookies_str.split(';'):
                    if '=' in cookie:
                        k, v = cookie.strip().split('=', 1)
                        cookies[k] = v

                with open(self.cookie_file, 'w') as f:
                    json.dump(cookies, f, indent=2)
                print("Cookie已保存")

        self.execute_js("document.cookie", cookie_callback)

    def load_cookies(self):
        """加载Cookie"""
        if os.path.exists(self.cookie_file):
            with open(self.cookie_file, 'r') as f:
                cookies = json.load(f)

            for key, value in cookies.items():
                js = f'document.cookie = "{key}={value}; path=/";'
                self.execute_js(js)

            print("Cookie已加载")

    def closeEvent(self, event):
        """窗口关闭事件"""
        self.save_cookies()
        event.accept()


# 使用示例
def main():
    app = QApplication(sys.argv)

    capture = PyQtCapture('https://example.com')

    # 连接帧处理信号
    def process_frame(frame):
        # OpenCV处理
        gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
        cv2.imshow('PyQt Capture', gray)
        cv2.waitKey(1)

    capture.frame_ready.connect(process_frame)

    capture.show()

    sys.exit(app.exec_())

if __name__ == '__main__':
    main()
```

---

## 完整游戏辅助示例

结合所有方案的游戏辅助完整示例：

```python
import cv2
import numpy as np
import time
from dataclasses import dataclass
from typing import Callable, List, Optional

@dataclass
class GameConfig:
    """游戏配置"""
    url: str
    width: int = 1280
    height: int = 720
    screenshot_fps: int = 30

    # 颜色检测配置
    enemy_color_lower: tuple = (0, 100, 100)
    enemy_color_upper: tuple = (10, 255, 255)
    item_color_lower: tuple = (40, 100, 100)
    item_color_upper: tuple = (80, 255, 255)

class GameBot:
    """游戏机器人"""

    def __init__(self, capture, config: GameConfig):
        self.capture = capture
        self.config = config
        self.running = False

        # 模板缓存
        self.templates = {}

    def load_template(self, name, path):
        """加载模板图片"""
        template = cv2.imread(path)
        if template is not None:
            self.templates[name] = template
            print(f"模板加载成功: {name}")
        else:
            print(f"模板加载失败: {path}")

    def find_template(self, frame, template_name, threshold=0.8):
        """查找模板"""
        if template_name not in self.templates:
            return None

        template = self.templates[template_name]

        # 转灰度
        gray_frame = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)
        gray_template = cv2.cvtColor(template, cv2.COLOR_BGR2GRAY)

        # 匹配
        result = cv2.matchTemplate(gray_frame, gray_template, cv2.TM_CCOEFF_NORMED)
        min_val, max_val, min_loc, max_loc = cv2.minMaxLoc(result)

        if max_val >= threshold:
            h, w = gray_template.shape
            return {
                'found': True,
                'confidence': max_val,
                'position': max_loc,
                'center': (max_loc[0] + w//2, max_loc[1] + h//2),
                'size': (w, h)
            }

        return None

    def find_color(self, frame, lower, upper, min_area=500):
        """查找颜色区域"""
        hsv = cv2.cvtColor(frame, cv2.COLOR_BGR2HSV)
        mask = cv2.inRange(hsv, np.array(lower), np.array(upper))

        # 形态学处理
        kernel = np.ones((5, 5), np.uint8)
        mask = cv2.morphologyEx(mask, cv2.MORPH_CLOSE, kernel)
        mask = cv2.morphologyEx(mask, cv2.MORPH_OPEN, kernel)

        # 查找轮廓
        contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

        results = []
        for contour in contours:
            area = cv2.contourArea(contour)
            if area > min_area:
                x, y, w, h = cv2.boundingRect(contour)
                results.append({
                    'position': (x, y),
                    'size': (w, h),
                    'center': (x + w//2, y + h//2),
                    'area': area
                })

        return results

    def click_at(self, x, y):
        """在指定位置点击"""
        self.capture.execute_js(f"""
            (function() {{
                const event = new MouseEvent('click', {{
                    view: window,
                    bubbles: true,
                    cancelable: true,
                    clientX: {x},
                    clientY: {y}
                }});
                document.elementFromPoint({x}, {y}).dispatchEvent(event);
            }})();
        """)

    def auto_collect_items(self, frame):
        """自动拾取物品"""
        items = self.find_color(
            frame,
            self.config.item_color_lower,
            self.config.item_color_upper,
            min_area=300
        )

        if items:
            # 点击最近的物品
            item = min(items, key=lambda x: x['center'][1])
            self.click_at(*item['center'])
            print(f"拾取物品: {item['center']}")
            return True

        return False

    def auto_attack_enemy(self, frame):
        """自动攻击敌人"""
        enemies = self.find_color(
            frame,
            self.config.enemy_color_lower,
            self.config.enemy_color_upper,
            min_area=1000
        )

        if enemies:
            # 攻击最近的敌人
            enemy = min(enemies, key=lambda x: x['center'][1])
            self.click_at(*enemy['center'])
            print(f"攻击敌人: {enemy['center']}")
            return True

        return False

    def run(self):
        """运行游戏机器人"""
        self.running = True

        def game_loop(frame):
            if not self.running:
                return

            # 绘制调试信息
            debug_frame = frame.copy()

            # 1. 拾取物品
            items = self.find_color(
                frame,
                self.config.item_color_lower,
                self.config.item_color_upper,
                min_area=300
            )

            for item in items:
                x, y = item['position']
                w, h = item['size']
                cv2.rectangle(debug_frame, (x, y), (x+w, y+h), (0, 255, 0), 2)
                cv2.putText(debug_frame, 'ITEM', (x, y-10),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 255, 0), 2)

            if items:
                self.auto_collect_items(frame)

            # 2. 攻击敌人
            enemies = self.find_color(
                frame,
                self.config.enemy_color_lower,
                self.config.enemy_color_upper,
                min_area=1000
            )

            for enemy in enemies:
                x, y = enemy['position']
                w, h = enemy['size']
                cv2.rectangle(debug_frame, (x, y), (x+w, y+h), (0, 0, 255), 2)
                cv2.putText(debug_frame, 'ENEMY', (x, y-10),
                           cv2.FONT_HERSHEY_SIMPLEX, 0.5, (0, 0, 255), 2)

            if enemies and not items:  # 优先拾取物品
                self.auto_attack_enemy(frame)

            # 3. 检测特殊UI元素（使用模板匹配）
            skill_button = self.find_template(frame, 'skill_button', threshold=0.85)
            if skill_button:
                x, y = skill_button['center']
                cv2.circle(debug_frame, (x, y), 30, (255, 0, 0), 2)

            # 显示调试画面
            cv2.imshow('Game Bot Debug', debug_frame)
            cv2.waitKey(1)

        self.capture.set_frame_callback(game_loop)

    def stop(self):
        """停止机器人"""
        self.running = False


# 完整使用示例
if __name__ == '__main__':
    # 选择方案（这里使用Selenium作为示例）
    from selenium import webdriver
    from selenium.webdriver.chrome.service import Service
    from webdriver_manager.chrome import ChromeDriverManager

    # 配置
    config = GameConfig(
        url='https://your-game-url.com',
        width=1280,
        height=720,
        screenshot_fps=30
    )

    # 创建捕获器
    capture = SeleniumCapture(
        url=config.url,
        width=config.width,
        height=config.height,
        headless=False
    )

    # 启动
    capture.start()

    # 等待页面加载
    time.sleep(5)

    # 创建游戏机器人
    bot = GameBot(capture, config)

    # 加载模板
    bot.load_template('skill_button', 'templates/skill_button.png')
    bot.load_template('hp_potion', 'templates/hp_potion.png')

    # 运行机器人
    bot.run()

    try:
        # 保持运行
        input("按Enter停止机器人...\n")
    finally:
        bot.stop()
        capture.stop()
        cv2.destroyAllWindows()
```

---

## 最佳实践建议

### 方案选择建议

1. **游戏辅助/简单自动化** → **Selenium** 或 **Playwright**
   - 优点：成熟稳定、Cookie自动管理、功能强大
   - 推荐：Selenium（文档多）或 Playwright（性能好）

2. **需要GUI界面** → **PyQt5 WebEngine**
   - 适合需要完整应用程序界面的场景

3. **轻量级嵌入** → **pywebview**
   - 适合简单的Web内容展示和交互

4. **高性能要求** → **Playwright** 或 **CEF Python**
   - Playwright更现代化，CEF Python更底层

### 性能优化建议

1. **降低截图频率**：根据需求调整fps（10-30fps）
2. **使用ROI**：只处理感兴趣的区域
3. **异步处理**：截图和OpenCV处理放在不同线程
4. **缓存模板**：避免重复加载模板图片
5. **GPU加速**：使用OpenCV的CUDA模块

### 安全建议

1. **不要在代码中硬编码密码**
2. **Cookie文件加密存储**
3. **使用环境变量存储敏感信息**
4. **遵守网站的robots.txt和使用条款**
5. **添加合理的延迟，避免被检测**
