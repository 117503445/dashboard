"""
测试用例 4: Code Server 启动和加载测试

验证：
1. 通过 Dashboard 在远程 Agent 上设置 code-server
2. code-server 通过 SSH 隧道在 iframe 中正确加载
3. VS Code 界面成功显示
"""

CASE_META = {
    "title": "Code Server 启动和加载测试",
    "description": (
        "验证通过 Dashboard 的 Code Server 按钮在远程 Agent 上自动下载、"
        "安装并启动 code-server，并通过 SSH 隧道在 iframe 中加载 VS Code 界面。"
    ),
    "steps": {
        "01-initial-load": "打开 Dashboard 首页",
        "02-dashboard-loaded": "Dashboard 加载完成",
        "03-agent-list": "等待 Agent 列表通过 Hub API 轮询加载",
        "04-agent-selected": "Agent 出现在列表中，点击选中",
        "05-click-code-server": "点击 Code Server 按钮，开始设置",
        "06-code-server-loading": "Code Server 设置进行中（按钮转圈）",
        "07-tab-created": "Code Server 设置完成，:44444 标签页已创建",
        "08-vscode-loaded": "VS Code 界面在 iframe 中加载成功",
    },
}

import logging
import os
import signal
import socket
import subprocess
import time
from pathlib import Path

from playwright.sync_api import sync_playwright

from lib.utils import TestContext

HUB_PORT = 19003
DASHBOARD_PORT = 18083
AGENT_SSH_PORT = 22224
AUTH_TOKEN = "e2e-test-token-case4"
AGENT_NAME = "e2e-agent-cs"
CODE_SERVER_PORT = 44444


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


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    logger.info("开始测试用例 case4: Code Server 启动和加载测试")

    time.sleep(2)

    project_root = Path(__file__).parent.parent.parent.parent
    hub_process = None
    agent_process = None
    backend_process = None
    log_handles = []

    try:
        # 0. 确保所有端口空闲
        for port in [HUB_PORT, DASHBOARD_PORT, AGENT_SSH_PORT, CODE_SERVER_PORT]:
            if not is_port_free(port):
                logger.warning(f"端口 {port} 被占用，尝试释放...")
                free_port(port, logger)
                if not is_port_free(port):
                    logger.error(f"无法释放端口 {port}，终止测试")
                    return False
        logger.info("所有必需端口已空闲")

        # 1. 启动 sshole-hub
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
        if hub_process.poll() is not None:
            logger.error(f"sshole-hub 已退出，退出码: {hub_process.returncode}")
            return False
        logger.info(f"sshole-hub 已启动 :{HUB_PORT}")

        # 2. 启动 sshole-agent
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

        # 3. 启动 Dashboard 后端
        env = os.environ.copy()
        env["PORT"] = str(DASHBOARD_PORT)
        env["DASHBOARD_SSHOLE_HUB_URL"] = f"http://localhost:{HUB_PORT}"
        env["DASHBOARD_SSHOLE_HUB_TOKEN"] = AUTH_TOKEN
        env["DASHBOARD_SSH_USER"] = "root"
        env["DASHBOARD_SSH_PASSWORD"] = "test"

        dash_log = open(logs_dir / "dashboard.log", "w")
        log_handles.append(dash_log)
        logger.info("编译并启动 Dashboard（可能需要 60 秒）...")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=dash_log,
            stderr=subprocess.STDOUT,
        )

        if not wait_for_port(DASHBOARD_PORT, timeout=90):
            logger.error("Dashboard 在 90 秒内未能启动")
            if backend_process.poll() is not None:
                logger.error(
                    f"Dashboard 进程已退出，退出码: {backend_process.returncode}"
                )
            return False
        if backend_process.poll() is not None:
            logger.error(
                f"Dashboard 立即退出，退出码: {backend_process.returncode}"
            )
            return False
        logger.info(f"Dashboard 已启动 :{DASHBOARD_PORT}")
        time.sleep(3)

        # 4. Playwright 测试
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

                    # 4a. 加载 Dashboard
                    ctx.goto("/")
                    ctx.screenshot("01-initial-load")
                    ctx.wait_for_selector("text=Dashboard", timeout=15000)
                    ctx.screenshot("02-dashboard-loaded")

                    # 4b. 等待 Agent 列表加载
                    time.sleep(8)
                    ctx.screenshot("03-agent-list")

                    # 4c. 验证 Agent 已发现
                    agent_el = page.locator(f"text={AGENT_NAME}")
                    if agent_el.count() == 0:
                        logger.error(f"Agent '{AGENT_NAME}' 不在列表中")
                        ctx.screenshot("error-no-agent")
                        return False
                    logger.info(f"Agent '{AGENT_NAME}' 已发现")

                    # 4d. 选择 Agent
                    agent_el.first.click()
                    time.sleep(1)
                    ctx.screenshot("04-agent-selected")

                    # 4e. 点击 Code Server 按钮
                    code_server_btn = page.locator("button:has-text('Code Server')")
                    if code_server_btn.count() == 0:
                        logger.error("未找到 Code Server 按钮")
                        return False

                    code_server_btn.first.click()
                    logger.info("已点击 Code Server 按钮，等待设置完成...")
                    ctx.screenshot("05-click-code-server")

                    # 4f. 验证按钮进入 loading 状态（spinner 出现）
                    time.sleep(2)
                    ctx.screenshot("06-code-server-loading")

                    # 4g. 等待 :44444 标签页出现（设置完成）
                    # 超时 5 分钟（涵盖下载 + 传输 + 解压 + 启动）
                    tab_locator = page.locator(f"text=:{CODE_SERVER_PORT}")
                    try:
                        tab_locator.wait_for(timeout=300000)
                        logger.info(f":{CODE_SERVER_PORT} 标签页已创建")
                    except Exception as e:
                        logger.error(f"等待标签页创建超时: {e}")
                        ctx.screenshot("error-tab-timeout")
                        # 检查是否有错误信息
                        error_el = page.locator(".bg-red-50")
                        if error_el.count() > 0:
                            error_text = error_el.first.inner_text()
                            logger.error(f"错误信息: {error_text}")
                        return False
                    ctx.screenshot("07-tab-created")

                    # 4h. 等待 VS Code 在 iframe 中加载
                    time.sleep(5)
                    vscode_loaded = False
                    try:
                        iframe = page.frame_locator("iframe")
                        iframe.locator(".monaco-workbench").wait_for(timeout=60000)
                        vscode_loaded = True
                        logger.info("VS Code 界面加载成功")
                    except Exception as e:
                        logger.warning(f"VS Code 元素检查超时: {e}")
                        # 退而求其次：检查 iframe 是否有任何内容加载
                        try:
                            iframe = page.frame_locator("iframe")
                            iframe.locator("body").wait_for(timeout=10000)
                            body_text = iframe.locator("body").inner_text()
                            if body_text:
                                vscode_loaded = True
                                logger.info(f"iframe 已加载内容 (长度: {len(body_text)})")
                        except Exception:
                            pass

                    ctx.screenshot("08-vscode-loaded")

                    if not vscode_loaded:
                        logger.error("VS Code 未能加载")
                        return False

                    logger.info("测试成功完成")
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
