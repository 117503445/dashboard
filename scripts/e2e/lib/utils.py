"""Utility functions for E2E tests."""

import logging
from pathlib import Path

from playwright.sync_api import Page, BrowserContext, Browser


def take_screenshot(page: Page, screenshots_dir: Path, name: str) -> Path:
    """Take a screenshot and save it to the screenshots directory."""
    screenshot_path = screenshots_dir / f"{name}.png"
    page.screenshot(path=str(screenshot_path))
    return screenshot_path


def save_trace(context: BrowserContext, output_dir: Path) -> Path:
    """Save the Playwright trace to the output directory."""
    trace_path = output_dir / "trace.zip"
    context.tracing.stop(path=str(trace_path))
    return trace_path


def setup_playwright_tracing(context: BrowserContext) -> None:
    """Start Playwright tracing for the context."""
    context.tracing.start(screenshots=True, snapshots=True, sources=True)


class TestContext:
    """Context manager for E2E tests with Playwright."""

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
        """Take a screenshot with the given name."""
        if not self.page:
            raise RuntimeError("Page not initialized")
        self.logger.info(f"Taking screenshot: {name}")
        return take_screenshot(self.page, self.screenshots_dir, name)

    def goto(self, path: str = "/") -> None:
        """Navigate to a path relative to base_url."""
        if not self.page:
            raise RuntimeError("Page not initialized")
        url = self.base_url + path
        self.logger.info(f"Navigating to: {url}")
        self.page.goto(url)

    def wait_for_selector(self, selector: str, timeout: int = 10000) -> None:
        """Wait for a selector to appear."""
        if not self.page:
            raise RuntimeError("Page not initialized")
        self.logger.info(f"Waiting for selector: {selector}")
        self.page.wait_for_selector(selector, timeout=timeout)