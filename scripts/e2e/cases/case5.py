"""
测试用例 5: 全屏模式布局测试

验证：
1. Code Server 在 iframe 中加载完成
2. 点击全屏后，Agent 面板铺满整个视口
3. 全屏状态只保留工具栏和原始 iframe，不创建第二个 fullscreen iframe
"""

CASE_META = {
    "title": "全屏模式布局测试",
    "description": (
        "验证 code-server 加载完成后点击全屏，页面只保留工具栏和原始 iframe，"
        "Agent 面板应铺满整个视口，且不能额外创建第二个 fullscreen iframe。"
    ),
    "steps": {
        "01-initial-load": "打开 Dashboard 首页",
        "02-dashboard-loaded": "Dashboard 加载完成",
        "03-agent-list": "等待 Agent 列表加载",
        "04-agent-selected": "选中 Agent",
        "05-click-code-server": "点击 Code Server 按钮",
        "06-vscode-loaded": "Code Server 在 iframe 中加载完成",
        "07-enter-fullscreen": "点击全屏按钮",
        "08-fullscreen-layout": "验证全屏后只保留工具栏和原始 iframe",
    },
}

import logging
import os
import shutil
import socket
import subprocess
import time
from pathlib import Path

from playwright.sync_api import sync_playwright

from lib.utils import TestContext

HUB_PORT = 19004
DASHBOARD_PORT = 18084
AGENT_SSH_PORT = 22225
AUTH_TOKEN = "e2e-test-token-case5"
AGENT_NAME = "e2e-agent-fs"
CODE_SERVER_PORT = 44444
WORKBENCH_ERROR_PATTERNS = [
    "An unexpected error occurred that requires a reload of this page.",
    "The workbench failed to connect to the server",
    "WebSocket close with status code 1006",
]


def is_port_free(port: int) -> bool:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(1)
    result = sock.connect_ex(("localhost", port))
    sock.close()
    return result != 0


def free_port(port: int, logger: logging.Logger):
    try:
        subprocess.run(
            f"fuser -k {port}/tcp 2>/dev/null || true",
            shell=True,
            capture_output=True,
        )
    except Exception:
        pass
    time.sleep(0.5)
    if not is_port_free(port):
        logger.warning(f"端口 {port} 在 fuser -k 后仍被占用")


def wait_for_port(port: int, timeout: int = 30) -> bool:
    start = time.time()
    while time.time() - start < timeout:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(1)
            result = sock.connect_ex(("localhost", port))
            sock.close()
            if result == 0:
                return True
        except Exception:
            pass
        time.sleep(0.5)
    return False


def contains_workbench_error(text: str) -> bool:
    return any(pattern in text for pattern in WORKBENCH_ERROR_PATTERNS)


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    logger.info("开始测试用例 case5: 全屏模式布局测试")

    time.sleep(2)

    project_root = Path(__file__).parent.parent.parent.parent
    hub_process = None
    agent_process = None
    backend_process = None
    log_handles = []

    try:
        for port in [HUB_PORT, DASHBOARD_PORT, AGENT_SSH_PORT, CODE_SERVER_PORT]:
            if not is_port_free(port):
                logger.warning(f"端口 {port} 被占用，尝试释放...")
                free_port(port, logger)
                if not is_port_free(port):
                    logger.error(f"无法释放端口 {port}，终止测试")
                    return False
        logger.info("所有必需端口已空闲")

        sshole_dir = Path.home() / ".sshole"
        authorized_keys = sshole_dir / "authorized_keys"
        sshole_dir.mkdir(parents=True, exist_ok=True)
        if authorized_keys.exists():
            authorized_keys.unlink()
        authorized_keys.touch(mode=0o600)
        logger.info(f"已清空 {authorized_keys}")

        env = os.environ.copy()
        env["DASHBOARD_PORT"] = str(DASHBOARD_PORT)
        env["DASHBOARD_SSHOLE_HUB_URL"] = f"http://localhost:{HUB_PORT}"
        env["DASHBOARD_SSHOLE_HUB_TOKEN"] = AUTH_TOKEN

        dash_log = open(logs_dir / "dashboard.log", "w")
        log_handles.append(dash_log)
        logger.info("编译并启动 Dashboard（生成 SSH 密钥对）...")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=dash_log,
            stderr=subprocess.STDOUT,
        )

        if not wait_for_port(DASHBOARD_PORT, timeout=90):
            logger.error("Dashboard 在 90 秒内未能启动")
            return False
        logger.info(f"Dashboard 已启动 :{DASHBOARD_PORT}")
        time.sleep(2)

        ssh_dir = project_root / "data" / ".ssh"
        private_key = ssh_dir / "id_ed25519"
        public_key = ssh_dir / "id_ed25519.pub"
        if not private_key.exists() or not public_key.exists():
            logger.error("SSH 密钥对未生成")
            return False

        pub_key_content = public_key.read_text().strip()
        with open(authorized_keys, "a") as f:
            f.write(pub_key_content + "\n")
        logger.info(f"已将 Dashboard 公钥添加到 {authorized_keys}")

        hub_bin = project_root / "data" / "bin" / "sshole_hub"
        if not hub_bin.exists():
            logger.error(f"未找到 sshole_hub: {hub_bin}")
            logger.error("请先运行 'task run:sshole-build'")
            return False

        hub_log = open(logs_dir / "sshole_hub.log", "w")
        log_handles.append(hub_log)
        hub_process = subprocess.Popen(
            [str(hub_bin), "--auth-token", AUTH_TOKEN, "--http-addr", f":{HUB_PORT}"],
            stdout=hub_log,
            stderr=subprocess.STDOUT,
        )
        if not wait_for_port(HUB_PORT, timeout=15):
            logger.error("sshole-hub 启动失败")
            return False
        logger.info(f"sshole-hub 已启动 :{HUB_PORT}")

        agent_log = open(logs_dir / "sshole_agent.log", "w")
        log_handles.append(agent_log)
        agent_bin = project_root / "data" / "bin" / "sshole_agent"
        agent_process = subprocess.Popen(
            [
                str(agent_bin),
                "--hub-server", f"http://localhost:{HUB_PORT}",
                "--auth", AUTH_TOKEN,
                "--name", AGENT_NAME,
                "--local-port", str(AGENT_SSH_PORT),
            ],
            stdout=agent_log,
            stderr=subprocess.STDOUT,
        )
        time.sleep(3)
        if agent_process.poll() is not None:
            logger.error(f"sshole-agent 已退出，退出码: {agent_process.returncode}")
            return False
        logger.info(f"sshole-agent '{AGENT_NAME}' 已启动 (ssh :{AGENT_SSH_PORT})")

        with sync_playwright() as p:
            browser = p.chromium.launch(headless=True)
            try:
                with TestContext(
                    browser=browser,
                    output_dir=output_dir,
                    screenshots_dir=screenshots_dir,
                    logs_dir=logs_dir,
                    logger=logger,
                    base_url=f"http://localhost:{DASHBOARD_PORT}",
                ) as ctx:
                    page = ctx.page
                    if not page:
                        return False

                    console_errors: list[str] = []
                    page_errors: list[str] = []

                    def record_console(msg):
                        text = msg.text
                        logger.info(f"浏览器控制台[{msg.type}] {text}")
                        if contains_workbench_error(text):
                            console_errors.append(text)

                    def record_page_error(exc):
                        text = str(exc)
                        logger.error(f"页面异常: {text}")
                        if contains_workbench_error(text):
                            page_errors.append(text)

                    def fail_if_workbench_error(stage: str) -> bool:
                        if console_errors or page_errors:
                            logger.error(f"{stage} 期间检测到 workbench 断连错误")
                            for item in console_errors:
                                logger.error(f"控制台错误: {item}")
                            for item in page_errors:
                                logger.error(f"页面错误: {item}")
                            ctx.screenshot(f"error-workbench-{stage}")
                            return True
                        return False

                    page.on("console", record_console)
                    page.on("pageerror", record_page_error)

                    ctx.goto("/")
                    ctx.screenshot("01-initial-load")
                    ctx.wait_for_selector("#app-root", timeout=15000)
                    ctx.screenshot("02-dashboard-loaded")

                    time.sleep(8)
                    ctx.screenshot("03-agent-list")

                    agent_el = page.locator(f"#agent-item-{AGENT_NAME}")
                    if agent_el.count() == 0:
                        logger.error(f"Agent '{AGENT_NAME}' 不在列表中")
                        return False

                    agent_el.first.click()
                    time.sleep(1)
                    ctx.screenshot("04-agent-selected")

                    code_server_btn = page.locator("#setup-code-server-button")
                    if code_server_btn.count() == 0:
                        logger.error("未找到 Code Server 按钮")
                        return False

                    code_server_btn.first.click()
                    ctx.screenshot("05-click-code-server")

                    tab_locator = page.locator(f"#iframe-tab-{CODE_SERVER_PORT}")
                    tab_locator.wait_for(timeout=300000)

                    iframe = page.frame_locator(f"#agent-iframe-{CODE_SERVER_PORT}")
                    iframe.locator(".monaco-workbench").wait_for(timeout=60000)
                    if fail_if_workbench_error("before-fullscreen"):
                        return False
                    ctx.screenshot("06-vscode-loaded")

                    page.locator("#open-fullscreen-button").click()
                    time.sleep(1)
                    ctx.screenshot("07-enter-fullscreen")

                    panel_box = page.locator(f"#agent-panel-{AGENT_NAME}").bounding_box()
                    if panel_box is None:
                        logger.error("未找到全屏后的 agent panel")
                        return False

                    viewport = page.viewport_size
                    if not viewport:
                        logger.error("无法获取视口大小")
                        return False

                    if abs(panel_box["x"]) > 1 or abs(panel_box["y"]) > 1:
                        logger.error(f"全屏后 agent panel 未贴齐左上角: {panel_box}")
                        return False

                    if abs(panel_box["width"] - viewport["width"]) > 2 or abs(panel_box["height"] - viewport["height"]) > 2:
                        logger.error(
                            f"全屏后 agent panel 未铺满视口: panel={panel_box}, viewport={viewport}"
                        )
                        return False

                    fullscreen_toolbar = page.locator("#fullscreen-agent-toolbar")
                    fullscreen_toolbar.wait_for(timeout=5000)

                    duplicate_iframe = page.locator(f"#fullscreen-iframe-{CODE_SERVER_PORT}")
                    if duplicate_iframe.count() != 0:
                        logger.error("全屏后出现了第二个 fullscreen iframe，这会导致页面重新加载白屏")
                        return False

                    original_iframe = page.locator(f"#agent-iframe-{CODE_SERVER_PORT}")
                    if original_iframe.count() != 1:
                        logger.error("全屏后原始 iframe 不存在")
                        return False

                    toolbar_count = page.locator("#agent-toolbar, #fullscreen-agent-toolbar").count()
                    if toolbar_count != 1:
                        logger.error(f"全屏后工具栏数量异常: {toolbar_count}")
                        return False

                    if fail_if_workbench_error("after-fullscreen"):
                        return False

                    ctx.screenshot("08-fullscreen-layout")
                    logger.info("全屏布局验证成功")
                    return True

            except Exception as e:
                logger.exception(f"测试失败: {e}")
                return False
            finally:
                browser.close()

    finally:
        for name, proc in [
            ("dashboard", backend_process),
            ("sshole-agent", agent_process),
            ("sshole-hub", hub_process),
        ]:
            if proc:
                logger.info(f"停止 {name}...")
                proc.terminate()
                try:
                    proc.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    proc.kill()
                logger.info(f"{name} 已停止")

        for fh in log_handles:
            try:
                fh.close()
            except Exception:
                pass
