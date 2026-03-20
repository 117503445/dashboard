"""
测试用例 1: 基础健康检查页面测试

验证前端正确加载并显示后端的健康检查状态。
"""

CASE_META = {
    "title": "基础健康检查页面测试",
    "description": "验证 Dashboard 前端能正确加载，并显示后端的健康检查状态（包括 Dashboard 标题、Agents 列表区域）。",
    "steps": {
        "step1-initial-load": "打开 Dashboard 首页，页面开始加载",
        "step2-dashboard-visible": "Dashboard 标题出现，页面主体框架渲染完成",
        "step3-agent-list-loaded": "等待 Agent 列表区域加载完毕",
        "step4-verification-complete": "验证页面结构：确认 Dashboard 标题、Agents 标题、Select an Agent 提示均存在",
    },
}

import logging
import os
import subprocess
import time
import socket
from pathlib import Path

from playwright.sync_api import sync_playwright

from lib.utils import TestContext


def wait_for_port(port: int, timeout: int = 30) -> bool:
    """等待端口可用（有进程监听）"""
    start = time.time()
    while time.time() - start < timeout:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(1)
            result = sock.connect_ex(('localhost', port))
            sock.close()
            if result == 0:
                return True
        except:
            pass
        time.sleep(0.5)
    return False


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    """运行健康检查页面测试"""
    logger.info("开始测试用例 case1: 健康检查页面测试")

    # 启动 Dashboard 后端
    project_root = Path(__file__).parent.parent.parent.parent
    backend_process = None

    try:
        env = os.environ.copy()
        env["PORT"] = "8080"

        logger.info("启动 Dashboard 后端...")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )

        # 等待后端启动
        if not wait_for_port(8080, timeout=30):
            logger.error("后端在 30 秒内未能启动")
            # 记录 stdout/stderr
            if backend_process.stdout:
                logger.error(f"后端输出: {backend_process.stdout.read().decode()}")
            return False

        logger.info("Dashboard 后端已在端口 8080 启动")

        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)

            try:
                with TestContext(
                    browser=browser,
                    output_dir=output_dir,
                    screenshots_dir=screenshots_dir,
                    logs_dir=logs_dir,
                    logger=logger,
                ) as ctx:
                    # 步骤 1: 导航到页面
                    ctx.goto("/")
                    ctx.screenshot("step1-initial-load")

                    # 步骤 2: 等待页面加载
                    ctx.wait_for_selector("text=Dashboard")
                    ctx.screenshot("step2-dashboard-visible")

                    # 步骤 3: 等待 Agent 列表加载
                    time.sleep(2)
                    ctx.screenshot("step3-agent-list-loaded")

                    # 步骤 4: 验证页面结构
                    page = ctx.page
                    if not page:
                        return False

                    # 检查 Dashboard 标题
                    title = page.locator("text=Dashboard")
                    if title.count() == 0:
                        logger.error("未找到 Dashboard 标题")
                        return False

                    # 检查 Agents 标题
                    agents_header = page.locator("text=Agents")
                    if agents_header.count() == 0:
                        logger.error("未找到 Agents 标题")
                        return False

                    # 检查 "Select an Agent" 提示
                    select_prompt = page.locator("text=Select an Agent")
                    if select_prompt.count() == 0:
                        logger.warning("未找到 Select an Agent 提示")

                    logger.info("页面结构验证成功")
                    ctx.screenshot("step4-verification-complete")

                    logger.info("测试成功完成")
                    return True

            except Exception as e:
                logger.exception(f"测试失败: {e}")
                return False
            finally:
                browser.close()

    finally:
        # 清理后端进程
        if backend_process:
            logger.info("停止 Dashboard 后端...")
            backend_process.terminate()
            try:
                backend_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                backend_process.kill()
            logger.info("Dashboard 后端已停止")