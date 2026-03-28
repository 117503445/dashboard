"""
测试用例 2: Agent 列表和代理测试

验证：
1. Agent 列表正确显示
2. 用户可以选择 Agent
3. 用户可以添加端口标签页
"""

CASE_META = {
    "title": "Agent 列表和代理测试",
    "description": "使用模拟 Agent 验证 Dashboard 的核心交互流程：Agent 列表展示、选择 Agent、添加端口标签页。",
    "steps": {
        "step1-initial-load": "打开 Dashboard 首页",
        "step2-dashboard-visible": "Dashboard 标题出现，页面加载完成",
        "step3-agent-list": "Agent 列表已加载（通过 API 获取）",
        "step4-agents-found": "验证 agent-1、agent-2、agent-3 三个 Agent 均在列表中",
        "step5-agent-1-selected": "点击选择 agent-1（在线状态）",
        "step6-add-port-input": "点击 + 按钮，弹出端口输入框",
        "step7-port-entered": "在输入框中填入端口号 3000",
        "step8-tab-created": "点击 Add 按钮，端口标签页创建成功",
        "step9-test-complete": "验证 :3000 标签页已显示，测试完成",
    },
}

import logging
import os
import socket
import subprocess
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
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


class MockProxyHandler(BaseHTTPRequestHandler):
    """模拟代理请求处理器（模拟 Agent 服务）"""

    def log_message(self, format, *args):
        """禁用默认日志"""
        pass

    def do_GET(self):
        """处理 GET 请求"""
        html = f"""
        <!DOCTYPE html>
        <html>
        <head><title>Mock Agent Page</title></head>
        <body>
            <h1>Mock Agent Service</h1>
            <p>Path: {self.path}</p>
            <p id="agent-content">This is a mock page from the agent</p>
        </body>
        </html>
        """
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(html.encode())


def run_mock_server(port: int, handler_class):
    """在线程中运行模拟 HTTP 服务器"""
    server = HTTPServer(("localhost", port), handler_class)
    server.serve_forever()


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    """运行 Agent 列表和代理测试"""
    logger.info("开始测试用例 case2: Agent 列表和代理测试")

    # 等待确保之前测试的清理已完成
    time.sleep(2)

    # 短暂延迟确保之前测试清理完成
    time.sleep(1)

    # 启动模拟 Agent 代理服务器
    mock_proxy_thread = threading.Thread(
        target=run_mock_server,
        args=(2222, MockProxyHandler),
        daemon=True,
    )
    mock_proxy_thread.start()
    logger.info("模拟 Agent 代理服务器已在端口 2222 启动")

    # 等待模拟服务器启动
    if not wait_for_port(2222, timeout=10):
        logger.error("模拟代理服务器未能启动")
        return False

    # 启动带有模拟 Agent 的 Dashboard 后端
    project_root = Path(__file__).parent.parent.parent.parent
    backend_process = None

    try:
        env = os.environ.copy()
        # 使用模拟 Agent 进行测试
        # 格式: "agent-name:hub-port:online"
        env["DASHBOARD_MOCK_AGENTS"] = "agent-1:2222:true,agent-2:2223:false,agent-3:2224:true"
        env["PORT"] = "8081"  # 使用不同端口避免冲突

        logger.info("启动带有模拟 Agent 的 Dashboard 后端...")
        logger.info(f"DASHBOARD_MOCK_AGENTS={env['DASHBOARD_MOCK_AGENTS']}")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )

        # 等待后端启动
        if not wait_for_port(8081, timeout=30):
            logger.error("后端在 30 秒内未能启动")
            if backend_process.stdout:
                output = backend_process.stdout.read().decode()
                logger.error(f"后端输出: {output}")
            return False

        logger.info("Dashboard 后端已在端口 8081 启动")

        # 给后端一点时间完全初始化
        time.sleep(3)

        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)

            try:
                with TestContext(
                    browser=browser,
                    output_dir=output_dir,
                    screenshots_dir=screenshots_dir,
                    logs_dir=logs_dir,
                    logger=logger,
                    base_url="http://localhost:8081",  # 使用不同端口
                ) as ctx:
                    # 步骤 1: 导航到页面
                    ctx.goto("/")
                    ctx.screenshot("step1-initial-load")

                    # 步骤 2: 等待页面加载
                    ctx.wait_for_selector("#app-root", timeout=15000)
                    ctx.screenshot("step2-dashboard-visible")

                    # 步骤 3: 验证 Agent 列表显示
                    page = ctx.page
                    if not page:
                        return False

                    # 检查 Agents 标题
                    agents_header = page.locator("#agents-sidebar-title")
                    if agents_header.count() == 0:
                        logger.error("未找到 Agents 标题")
                        return False

                    # 等待 Agent 列表加载（API 调用）
                    time.sleep(2)
                    ctx.screenshot("step3-agent-list")

                    # 步骤 4: 检查 Agent 项目
                    agent_1 = page.locator("#agent-item-agent-1")
                    agent_2 = page.locator("#agent-item-agent-2")
                    agent_3 = page.locator("#agent-item-agent-3")

                    if agent_1.count() == 0:
                        logger.error("列表中未找到 agent-1")
                        return False
                    if agent_2.count() == 0:
                        logger.error("列表中未找到 agent-2")
                        return False
                    if agent_3.count() == 0:
                        logger.error("列表中未找到 agent-3")
                        return False

                    logger.info("所有 3 个 Agent 已在列表中找到")
                    ctx.screenshot("step4-agents-found")

                    # 步骤 5: 点击 agent-1（在线）
                    agent_1.first.click()
                    time.sleep(1)
                    ctx.screenshot("step5-agent-1-selected")

                    # 步骤 6: 点击 + 按钮添加端口
                    plus_button = page.locator("#iframe-tab-add")
                    if plus_button.count() == 0:
                        logger.error("未找到 + 按钮")
                        return False

                    plus_button.first.click()
                    time.sleep(0.5)
                    ctx.screenshot("step6-add-port-input")

                    # 步骤 7: 输入端口号
                    port_input = page.locator("#add-port-input")
                    if port_input.count() == 0:
                        logger.error("未找到端口输入框")
                        return False

                    port_input.first.fill("3000")
                    ctx.screenshot("step7-port-entered")

                    # 步骤 8: 点击 Add 按钮
                    add_button = page.locator("#add-port-confirm")
                    if add_button.count() == 0:
                        logger.error("未找到 Add 按钮")
                        return False

                    add_button.first.click()
                    time.sleep(1)
                    ctx.screenshot("step8-tab-created")

                    # 步骤 9: 验证端口标签页已创建
                    port_tab = page.locator("#iframe-tab-3000")
                    if port_tab.count() == 0:
                        logger.error("未找到端口标签页 :3000")
                        return False

                    logger.info("端口标签页 :3000 创建成功")
                    ctx.screenshot("step9-test-complete")

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
