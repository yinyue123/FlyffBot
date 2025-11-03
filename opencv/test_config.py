#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
配置文件管理模块
用于加载、保存和管理配置文件
"""

import json
import os
import time
from threading import Lock


class Config:
    """
    配置管理类（全局单例）
    用于管理所有配置信息，支持热重载
    """

    def __init__(self):
        """初始化配置对象"""
        self.config_file = ""
        self.url = ""
        self.cookie = {}
        self.frequency = 100  # 毫秒
        self.stats = {}
        self.target = {}
        self.enable = True
        self.slots = {}
        self.screenshot = ""

        self.last_load_time = 0
        self.config_lock = Lock()

    def load(self, config_file):
        """
        加载配置文件

        参数:
            config_file: 配置文件路径

        返回:
            bool: 加载是否成功
        """
        with self.config_lock:
            try:
                self.config_file = config_file

                if not os.path.exists(config_file):
                    print(f"警告: 配置文件不存在: {config_file}")
                    return False

                with open(config_file, 'r', encoding='utf-8') as f:
                    data = json.load(f)

                # 更新配置
                self.url = data.get('url', '')
                self.cookie = data.get('cookie', {})
                self.frequency = data.get('frequency', 100)
                self.stats = data.get('stats', {})
                self.target = data.get('target', {})
                self.enable = data.get('enable', True)
                self.slots = data.get('slots', {})
                self.screenshot = data.get('screenshot', '')

                self.last_load_time = time.time()

                print(f"配置文件加载成功: {config_file}")
                print(f"  - URL: {self.url}")
                print(f"  - Frequency: {self.frequency}ms")
                print(f"  - Enable: {self.enable}")
                print(f"  - Screenshot: {self.screenshot if self.screenshot else '不保存'}")

                return True

            except json.JSONDecodeError as e:
                print(f"错误: 配置文件JSON格式错误: {e}")
                return False
            except Exception as e:
                print(f"错误: 加载配置文件失败: {e}")
                return False

    def reload_if_needed(self, check_interval=5.0):
        """
        检查是否需要重新加载配置文件
        如果距离上次加载超过指定时间，则重新加载

        参数:
            check_interval: 检查间隔（秒）

        返回:
            bool: 是否重新加载了配置
        """
        current_time = time.time()
        if current_time - self.last_load_time >= check_interval:
            print(f"检测到配置文件需要重新加载（已经过{current_time - self.last_load_time:.1f}秒）")
            return self.load(self.config_file)
        return False

    def save(self, config_file=None):
        """
        保存配置文件

        参数:
            config_file: 配置文件路径，如果为None则使用当前配置文件

        返回:
            bool: 保存是否成功
        """
        with self.config_lock:
            try:
                if config_file is None:
                    config_file = self.config_file

                data = {
                    'url': self.url,
                    'cookie': self.cookie,
                    'frequency': self.frequency,
                    'stats': self.stats,
                    'target': self.target,
                    'enable': self.enable,
                    'slots': self.slots,
                    'screenshot': self.screenshot
                }

                with open(config_file, 'w', encoding='utf-8') as f:
                    json.dump(data, f, indent=4, ensure_ascii=False)

                print(f"配置文件保存成功: {config_file}")
                return True

            except Exception as e:
                print(f"错误: 保存配置文件失败: {e}")
                return False

    def save_cookie(self, cookie):
        """
        保存cookie到配置

        参数:
            cookie: cookie字典
        """
        with self.config_lock:
            self.cookie = cookie
            self.save()

    def get_frequency_seconds(self):
        """
        获取频率（转换为秒）

        返回:
            float: 频率（秒）
        """
        return self.frequency / 1000.0

    def is_enabled(self):
        """
        检查是否启用

        返回:
            bool: 是否启用
        """
        return self.enable

    def should_save_screenshot(self):
        """
        检查是否应该保存截图

        返回:
            bool: 是否应该保存截图
        """
        return bool(self.screenshot)

    def __str__(self):
        """返回配置的字符串表示"""
        return (f"Config(url={self.url}, frequency={self.frequency}ms, "
                f"enable={self.enable}, screenshot={self.screenshot})")


# 全局配置实例
config = Config()


def load_config(config_file):
    """
    加载配置文件（全局函数）

    参数:
        config_file: 配置文件路径

    返回:
        bool: 加载是否成功
    """
    return config.load(config_file)


def get_config():
    """
    获取全局配置实例

    返回:
        Config: 全局配置实例
    """
    return config


def save_config(config_file=None):
    """
    保存配置文件（全局函数）

    参数:
        config_file: 配置文件路径，如果为None则使用当前配置文件

    返回:
        bool: 保存是否成功
    """
    return config.save(config_file)
