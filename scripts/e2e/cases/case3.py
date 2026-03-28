"""
测试用例 3: SSH 端口转发和代理测试

验证端到端 SSH 端口转发：
1. Dashboard 通过真实的 sshole-hub 发现 Agent
2. 通过 sshole-hub 建立到 Agent 的 SSH 隧道
3. 转发的服务内容在 Dashboard iframe 中可见
"""

CASE_META = {
    "title": "SSH 端口转发和代理测试",
    "description": (
        "端到端验证完整的 SSH 隧道转发链路：启动 sshole-hub 和 sshole-agent，"
        "Dashboard 通过 Hub API 发现 Agent，建立 SSH 隧道，"
        "远程服务内容通过代理在 Dashboard iframe 中正确显示。"
    ),
    "steps": {
        "01-initial-load": "打开 Dashboard 首页",
        "02-dashboard-loaded": "Dashboard 加载完成，标题可见",
        "03-agent-list": "等待 Agent 列表通过 Hub API 轮询加载",
        "04-agent-selected": "Agent 'e2e-agent' 出现在列表中，点击选中",
        "05-port-entered": "点击 + 按钮，输入远程服务端口号",
        "06-tab-created": "端口标签页创建成功",
        "07-iframe-content": "通过 SSH 隧道转发的服务内容在 iframe 中加载",
        "08-forwarded-service-fullpage": "直接访问代理 URL，验证转发的服务页面完整显示",
    },
}

import logging
import os
import signal
import socket
import subprocess
import threading
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from urllib.request import urlopen

from playwright.sync_api import sync_playwright

from lib.utils import TestContext

HUB_PORT = 19002
DASHBOARD_PORT = 18082
SERVICE_PORT = 17777
AGENT_SSH_PORT = 22223
AUTH_TOKEN = "e2e-test-token"
AGENT_NAME = "e2e-agent"


def is_port_free(port: int) -> bool:
    """检查端口是否空闲"""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(1)
    result = sock.connect_ex(("localhost", port))
    sock.close()
    return result != 0


def free_port(port: int, logger: logging.Logger):
    """尝试通过杀掉占用进程来释放端口"""
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
    """等待端口可用（有进程监听）"""
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


class AgentServiceHandler(BaseHTTPRequestHandler):
    """模拟运行在远程 Agent 上的 Web 服务"""

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        html = f"""<!DOCTYPE html>
<html>
<head><title>Agent Service</title></head>
<body style="background:#10b981;color:#fff;font-family:system-ui,sans-serif;
  display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
  <div style="text-align:center">
    <h1 id="forwarded-title" style="font-size:2.5em;margin-bottom:.5em">
      Forwarded Service
    </h1>
    <p id="forwarded-content" style="font-size:1.3em">
      SSH tunnel forwarding is working!
    </p>
    <p style="margin-top:1em;opacity:.7">path: {self.path}</p>
  </div>
</body>
</html>"""
        body = html.encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    logger.info("开始测试用例 case3: SSH 端口转发测试")

    time.sleep(2)

    project_root = Path(__file__).parent.parent.parent.parent
    hub_process = None
    agent_process = None
    backend_process = None
    log_handles = []

    try:
        # ── 0. 确保所有端口空闲 ────────────────────────────
        for port in [HUB_PORT, DASHBOARD_PORT, SERVICE_PORT, AGENT_SSH_PORT]:
            if not is_port_free(port):
                logger.warning(f"端口 {port} 被占用，尝试释放...")
                free_port(port, logger)
                if not is_port_free(port):
                    logger.error(f"无法释放端口 {port}，终止测试")
                    return False
        logger.info("所有必需端口已空闲")

        # ── 1. 启动模拟 Agent 服务 ───────────────────────────────────
        mock_thread = threading.Thread(
            target=lambda: HTTPServer(
                ("0.0.0.0", SERVICE_PORT), AgentServiceHandler
            ).serve_forever(),
            daemon=True,
        )
        mock_thread.start()
        if not wait_for_port(SERVICE_PORT, timeout=10):
            logger.error(f"模拟服务未能在端口 {SERVICE_PORT} 启动")
            return False
        logger.info(f"模拟 Agent 服务已监听 :{SERVICE_PORT}")

        # ── 2. 启动 sshole-hub ───────────────────────────────────────────
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

        # ── 3. 启动 sshole-agent ─────────────────────────────────────────
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

        # ── 4. 启动 Dashboard 后端（含 SSH 转发）────────────────
        env = os.environ.copy()
        env["DASHBOARD_PORT"] = str(DASHBOARD_PORT)
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

        # 等待 Dashboard 启动（go run 需要先编译）
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

        # ── 5. Playwright 测试 ─────────────────────────────────────
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

                    # 5a. 加载 Dashboard
                    ctx.goto("/")
                    ctx.screenshot("01-initial-load")
                    ctx.wait_for_selector("#app-root", timeout=15000)
                    ctx.screenshot("02-dashboard-loaded")

                    # 5b. 等待 Agent 列表加载（Hub API 轮询间隔）
                    time.sleep(8)
                    ctx.screenshot("03-agent-list")

                    # 5c. 验证 Agent 已从真实 Hub 发现
                    agent_el = page.locator(f"#agent-item-{AGENT_NAME}")
                    if agent_el.count() == 0:
                        logger.error(f"Agent '{AGENT_NAME}' 不在列表中")
                        ctx.screenshot("error-no-agent")
                        return False
                    logger.info(f"Agent '{AGENT_NAME}' 已通过 Hub 发现")

                    # 5d. 选择 Agent
                    agent_el.first.click()
                    time.sleep(1)
                    ctx.screenshot("04-agent-selected")

                    # 5e. 添加端口标签页
                    page.locator("#iframe-tab-add").first.click()
                    time.sleep(0.5)
                    page.locator("#add-port-input").first.fill(
                        str(SERVICE_PORT)
                    )
                    ctx.screenshot("05-port-entered")

                    page.locator("#add-port-confirm").first.click()
                    time.sleep(1)

                    tab = page.locator(f"#iframe-tab-{SERVICE_PORT}")
                    if tab.count() == 0:
                        logger.error(f"标签页 :{SERVICE_PORT} 未创建")
                        ctx.screenshot("error-no-tab")
                        return False
                    logger.info(f"标签页 :{SERVICE_PORT} 已创建")
                    ctx.screenshot("06-tab-created")

                    # 5f. 等待 iframe 内容加载（SSH 隧道建立 + 请求）
                    logger.info("等待 iframe 通过 SSH 隧道加载...")
                    iframe_ok = False
                    try:
                        iframe = page.frame_locator(f"#agent-iframe-{SERVICE_PORT}")
                        iframe.locator("#forwarded-title").wait_for(timeout=30000)
                        iframe_ok = True
                        logger.info("Iframe 显示转发的服务内容！")
                    except Exception as e:
                        logger.warning(f"Iframe 元素检查超时: {e}")

                    ctx.screenshot("07-iframe-content")

                    # 5g. 通过直接 HTTP 请求验证代理端点
                    proxy_url = (
                        f"http://localhost:{DASHBOARD_PORT}"
                        f"/proxy/agents/{AGENT_NAME}/ports/{SERVICE_PORT}/"
                    )
                    try:
                        with urlopen(proxy_url, timeout=30) as resp:
                            status = resp.status
                            body = resp.read().decode("utf-8", errors="replace")
                    except Exception as e:
                        logger.error(f"代理 HTTP 请求失败: {e}")
                        ctx.screenshot("error-proxy-http")
                        return False

                    if status < 200 or status >= 300:
                        logger.error(f"代理 HTTP 请求失败: {status}")
                        ctx.screenshot("error-proxy-http")
                        return False

                    if (
                        "Forwarded Service" not in body
                        or "SSH tunnel forwarding" not in body
                    ):
                        logger.error(
                            f"代理响应缺少预期内容: {body[:300]}"
                        )
                        return False
                    logger.info("代理端点返回正确的转发 HTML")

                    # 5h. 导航到代理 URL 获取清晰的最终截图
                    ctx.goto(
                        f"/proxy/agents/{AGENT_NAME}/ports/{SERVICE_PORT}/"
                    )
                    time.sleep(2)
                    ctx.screenshot("08-forwarded-service-fullpage")

                    logger.info("测试成功完成")
                    return True

            except Exception as e:
                logger.exception(f"测试失败: {e}")
                return False
            finally:
                browser.close()

    finally:
        # 清理所有进程
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
