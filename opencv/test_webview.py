#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
PyWebView + OpenCV 截图程序
支持配置文件热重载、自动截图、Cookie管理
"""

import webview
import cv2
import numpy as np
from PIL import Image
import threading
import time
import sys
import os
import argparse
from test_config import config, load_config


class WebViewCapture:
    """
    WebView截图捕获器
    集成配置管理、截图、Cookie管理功能
    """

    def __init__(self, config_file, frame_callback=None):
        """
        初始化WebView截图捕获器

        参数:
            config_file: 配置文件路径
            frame_callback: 截图处理回调函数，接收cv2图像作为参数
        """
        # 加载配置
        if not load_config(config_file):
            raise Exception(f"无法加载配置文件: {config_file}")

        self.window = None
        self.is_running = False
        self.frame_callback = frame_callback

        # 线程控制
        self.capture_thread = None
        self.cookie_save_thread = None
        self.config_check_thread = None

        # 性能计数
        self.frame_count = 0
        self.last_fps_time = time.time()

    def create_window(self):
        """创建WebView窗口"""
        print(f"正在创建WebView窗口...")
        print(f"URL: {config.url}")

        self.window = webview.create_window(
            title='WebView Capture',
            url=config.url,
            width=1280,
            height=720,
            resizable=True,
            fullscreen=False
        )

        # 窗口事件
        self.window.events.loaded += self.on_loaded
        self.window.events.closing += self.on_closing

        return self.window

    def on_loaded(self):
        """窗口加载完成回调"""
        print("WebView加载完成")

        # 恢复Cookie
        if config.cookie:
            self.restore_cookies(config.cookie)

        # 开始各种线程
        self.start_threads()

    def on_closing(self):
        """窗口关闭回调"""
        print("正在关闭WebView...")
        self.stop_threads()

        # 保存Cookie
        print("正在保存Cookie...")
        self.save_cookies()

        # 保存配置
        config.save()

    def start_threads(self):
        """启动所有工作线程"""
        self.is_running = True

        # 1. 截图线程
        self.capture_thread = threading.Thread(
            target=self.capture_loop,
            daemon=True,
            name="CaptureThread"
        )
        self.capture_thread.start()
        print("截图线程已启动")

        # 2. Cookie保存线程（每5分钟）
        self.cookie_save_thread = threading.Thread(
            target=self.cookie_save_loop,
            daemon=True,
            name="CookieSaveThread"
        )
        self.cookie_save_thread.start()
        print("Cookie保存线程已启动")

        # 3. 配置检查线程（每5秒）
        self.config_check_thread = threading.Thread(
            target=self.config_check_loop,
            daemon=True,
            name="ConfigCheckThread"
        )
        self.config_check_thread.start()
        print("配置检查线程已启动")

    def stop_threads(self):
        """停止所有工作线程"""
        self.is_running = False
        print("正在停止所有工作线程...")

    def capture_loop(self):
        """截图循环"""
        print("截图循环开始运行")

        while self.is_running:
            try:
                # 检查配置重新加载
                config.reload_if_needed(check_interval=5.0)

                # 检查是否启用
                if not config.is_enabled():
                    time.sleep(0.5)
                    continue

                # 等待窗口准备就绪
                if not self.window:
                    time.sleep(0.1)
                    continue

                # 记录开始时间
                start_time = time.time()

                # 截图
                screenshot = self.capture_screenshot()

                # 计算截图时间
                screenshot_time = time.time() - start_time

                # 处理截图
                if screenshot is not None:
                    # 保存截图（如果配置了）
                    if config.should_save_screenshot():
                        cv2.imwrite(config.screenshot, screenshot)

                    # 调用回调函数
                    if self.frame_callback:
                        callback_start = time.time()
                        try:
                            self.frame_callback(screenshot)
                        except Exception as e:
                            print(f"回调函数执行错误: {e}")
                        callback_time = time.time() - callback_start
                    else:
                        callback_time = 0

                    # 更新FPS统计
                    self.update_fps()
                else:
                    callback_time = 0

                # 计算总处理时间
                total_time = screenshot_time + callback_time

                # 计算sleep时间（frequency - 处理时间）
                frequency_seconds = config.get_frequency_seconds()
                sleep_time = max(0, frequency_seconds - total_time)

                time.sleep(sleep_time)

            except Exception as e:
                print(f"截图循环错误: {e}")
                time.sleep(0.5)

    def capture_screenshot(self):
        """
        捕获WebView截图

        返回:
            numpy.ndarray: OpenCV格式的图像(BGR)，失败返回None
        """
        try:
            # 使用pywebview的JS接口获取canvas截图
            # 注意：这种方法可能在某些平台上不可用
            # 如果失败，可以使用PIL的屏幕截图作为备选

            # 尝试方法1：使用HTML Canvas
            js_code = """
                (async function() {
                    try {
                        const canvas = document.createElement('canvas');
                        canvas.width = window.innerWidth;
                        canvas.height = window.innerHeight;
                        const ctx = canvas.getContext('2d');

                        // 绘制整个页面
                        const html = document.documentElement;
                        const data = await html2canvas(html);

                        return canvas.toDataURL('image/png');
                    } catch(e) {
                        return null;
                    }
                })();
            """

            # 由于pywebview的限制，我们使用更简单的方法
            # 通过PIL截取整个屏幕，然后裁剪窗口区域
            # 这需要pyautogui或其他屏幕截图工具

            try:
                import pyautogui
                # 截取全屏
                screenshot = pyautogui.screenshot()

                # 转换为OpenCV格式
                screenshot_np = np.array(screenshot)
                screenshot_bgr = cv2.cvtColor(screenshot_np, cv2.COLOR_RGB2BGR)

                return screenshot_bgr

            except ImportError:
                print("警告: pyautogui未安装，无法截图")
                print("请运行: pip install pyautogui")
                return None

        except Exception as e:
            print(f"截图失败: {e}")
            return None

    def update_fps(self):
        """更新FPS统计"""
        self.frame_count += 1
        current_time = time.time()

        if current_time - self.last_fps_time >= 5.0:
            fps = self.frame_count / (current_time - self.last_fps_time)
            print(f"截图FPS: {fps:.2f}")
            self.frame_count = 0
            self.last_fps_time = current_time

    def cookie_save_loop(self):
        """Cookie保存循环（每5分钟）"""
        print("Cookie保存循环开始运行")

        while self.is_running:
            try:
                # 每5分钟保存一次
                time.sleep(300)

                if self.is_running:
                    self.save_cookies()

            except Exception as e:
                print(f"Cookie保存循环错误: {e}")

    def config_check_loop(self):
        """配置检查循环（每5秒）"""
        print("配置检查循环开始运行")

        while self.is_running:
            try:
                time.sleep(5)

                if self.is_running:
                    # 重新加载配置（如果需要）
                    if config.reload_if_needed(check_interval=5.0):
                        print("配置已重新加载")

            except Exception as e:
                print(f"配置检查循环错误: {e}")

    def execute_js(self, js_code):
        """
        执行JavaScript代码

        参数:
            js_code: JavaScript代码字符串

        返回:
            执行结果
        """
        if self.window:
            try:
                return self.window.evaluate_js(js_code)
            except Exception as e:
                print(f"执行JS失败: {e}")
                return None
        return None

    def save_cookies(self):
        """保存Cookie到配置文件"""
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

            # 保存到配置
            config.save_cookie(cookies)
            print(f"Cookie已保存（共{len(cookies)}个）")
        else:
            print("未获取到Cookie")

    def restore_cookies(self, cookies):
        """
        恢复Cookie到浏览器

        参数:
            cookies: Cookie字典
        """
        if not cookies:
            print("没有需要恢复的Cookie")
            return

        for key, value in cookies.items():
            js_code = f'document.cookie = "{key}={value}; path=/";'
            self.execute_js(js_code)

        print(f"Cookie已恢复（共{len(cookies)}个）")

    def start(self):
        """启动WebView（阻塞式）"""
        self.create_window()
        print("启动WebView...")
        webview.start(debug=True)


def opencv_process_frame(frame):
    """
    OpenCV处理帧的回调函数示例
    你可以在这里实现自己的图像识别和处理逻辑

    参数:
        frame: OpenCV图像(BGR格式)
    """
    # 示例：转换为灰度图
    gray = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY)

    # 示例：检测边缘
    edges = cv2.Canny(gray, 50, 150)

    # 示例：查找轮廓
    contours, _ = cv2.findContours(edges, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)

    # 在原图上绘制轮廓
    result = frame.copy()
    cv2.drawContours(result, contours, -1, (0, 255, 0), 2)

    # 显示结果（可选）
    # cv2.imshow('OpenCV Processing', result)
    # cv2.waitKey(1)

    # 这里可以添加更多的OpenCV处理逻辑
    # 例如：模板匹配、颜色检测、OCR识别等


def main():
    """主函数"""
    # 解析命令行参数
    parser = argparse.ArgumentParser(description='PyWebView截图程序')
    parser.add_argument(
        'config',
        nargs='?',
        default='test_setting.json',
        help='配置文件路径（默认: test_setting.json）'
    )
    args = parser.parse_args()

    config_file = args.config

    # 检查配置文件是否存在
    if not os.path.exists(config_file):
        print(f"错误: 配置文件不存在: {config_file}")
        sys.exit(1)

    print("=" * 60)
    print("PyWebView截图程序")
    print("=" * 60)
    print(f"配置文件: {config_file}")
    print()

    try:
        # 创建WebView捕获器
        # 如果需要OpenCV处理，传入回调函数：frame_callback=opencv_process_frame
        # 如果不需要处理，传入None或不传参数
        capture = WebViewCapture(
            config_file=config_file,
            frame_callback=None  # 设置为opencv_process_frame启用OpenCV处理
        )

        # 启动（阻塞式）
        capture.start()

    except KeyboardInterrupt:
        print("\n收到中断信号，正在退出...")
    except Exception as e:
        print(f"程序错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
