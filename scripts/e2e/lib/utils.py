"""E2E 测试工具函数"""

import logging
from pathlib import Path

from playwright.sync_api import Page, BrowserContext, Browser


def take_screenshot(page: Page, screenshots_dir: Path, name: str) -> Path:
    """截图并保存到截图目录"""
    screenshot_path = screenshots_dir / f"{name}.png"
    page.screenshot(path=str(screenshot_path))
    return screenshot_path


def save_trace(context: BrowserContext, output_dir: Path) -> Path:
    """保存 Playwright 追踪到输出目录"""
    trace_path = output_dir / "trace.zip"
    context.tracing.stop(path=str(trace_path))
    return trace_path


def setup_playwright_tracing(context: BrowserContext) -> None:
    """为上下文启动 Playwright 追踪"""
    context.tracing.start(screenshots=True, snapshots=True, sources=True)


class TestContext:
    """E2E 测试上下文管理器（使用 Playwright）"""

    def __init__(
        self,
        browser: Browser,
        output_dir: Path,
        screenshots_dir: Path,
        logs_dir: Path,
        logger: logging.Logger,
        base_url: str = "http://localhost:8080",
    ):
        self.browser = browser
        self.output_dir = output_dir
        self.screenshots_dir = screenshots_dir
        self.logs_dir = logs_dir
        self.logger = logger
        self.base_url = base_url
        self.context: BrowserContext | None = None
        self.page: Page | None = None

    def __enter__(self):
        self.context = self.browser.new_context()
        setup_playwright_tracing(self.context)
        self.page = self.context.new_page()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.context:
            save_trace(self.context, self.output_dir)
            self.context.close()
        return False

    def screenshot(self, name: str) -> Path:
        """使用指定名称截图"""
        if not self.page:
            raise RuntimeError("页面未初始化")
        self.logger.info(f"截图: {name}")
        return take_screenshot(self.page, self.screenshots_dir, name)

    def goto(self, path: str = "/") -> None:
        """导航到 base_url 的相对路径"""
        if not self.page:
            raise RuntimeError("页面未初始化")
        url = self.base_url + path
        self.logger.info(f"导航到: {url}")
        self.page.goto(url)

    def wait_for_selector(self, selector: str, timeout: int = 10000) -> None:
        """等待选择器出现"""
        if not self.page:
            raise RuntimeError("页面未初始化")
        self.logger.info(f"等待选择器: {selector}")
        self.page.wait_for_selector(selector, timeout=timeout)