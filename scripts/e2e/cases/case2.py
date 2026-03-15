"""
Test case 2: Agent List and Proxy Test

This test verifies that:
1. The agent list is displayed correctly
2. Users can select an agent
3. Users can add port tabs
"""

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
    """Wait for a port to be available."""
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
    """Mock handler for proxy requests (simulates agent service)."""

    def log_message(self, format, *args):
        """Suppress default logging."""
        pass

    def do_GET(self):
        """Handle GET requests."""
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
    """Run a mock HTTP server in a thread."""
    server = HTTPServer(("localhost", port), handler_class)
    server.serve_forever()


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    """Run the agent list and proxy test."""
    logger.info("Starting case2: Agent List and Proxy Test")

    # Delay to ensure previous test cleanup is complete
    time.sleep(2)

    # Small delay to ensure previous test cleanup
    time.sleep(1)

    # Start mock agent proxy server
    mock_proxy_thread = threading.Thread(
        target=run_mock_server,
        args=(2222, MockProxyHandler),
        daemon=True,
    )
    mock_proxy_thread.start()
    logger.info("Mock agent proxy server started on port 2222")

    # Wait for mock server to start
    if not wait_for_port(2222, timeout=10):
        logger.error("Mock proxy server failed to start")
        return False

    # Start dashboard backend with mock agents
    project_root = Path(__file__).parent.parent.parent.parent
    backend_process = None

    try:
        env = os.environ.copy()
        # Use mock agents for testing
        # Format: "agent-name:hub-port:online"
        env["DASHBOARD_MOCK_AGENTS"] = "agent-1:2222:true,agent-2:2223:false,agent-3:2224:true"
        env["PORT"] = "8081"  # Use different port to avoid conflicts

        logger.info("Starting dashboard backend with mock agents...")
        logger.info(f"DASHBOARD_MOCK_AGENTS={env['DASHBOARD_MOCK_AGENTS']}")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )

        # Wait for backend to start
        if not wait_for_port(8081, timeout=30):
            logger.error("Backend failed to start within 30 seconds")
            if backend_process.stdout:
                output = backend_process.stdout.read().decode()
                logger.error(f"Backend output: {output}")
            return False

        logger.info("Dashboard backend started on port 8081")

        # Give the backend a moment to fully initialize
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
                    base_url="http://localhost:8081",  # Use different port
                ) as ctx:
                    # Step 1: Navigate to the page
                    ctx.goto("/")
                    ctx.screenshot("step1-initial-load")

                    # Step 2: Wait for the page to load
                    ctx.wait_for_selector("text=Dashboard", timeout=15000)
                    ctx.screenshot("step2-dashboard-visible")

                    # Step 3: Verify agent list is displayed
                    page = ctx.page
                    if not page:
                        return False

                    # Check for Agents header
                    agents_header = page.locator("text=Agents")
                    if agents_header.count() == 0:
                        logger.error("Agents header not found")
                        return False

                    # Wait for agent list to load (API call)
                    time.sleep(2)
                    ctx.screenshot("step3-agent-list")

                    # Step 4: Check for agent items
                    agent_1 = page.locator("text=agent-1")
                    agent_2 = page.locator("text=agent-2")
                    agent_3 = page.locator("text=agent-3")

                    if agent_1.count() == 0:
                        logger.error("agent-1 not found in list")
                        return False
                    if agent_2.count() == 0:
                        logger.error("agent-2 not found in list")
                        return False
                    if agent_3.count() == 0:
                        logger.error("agent-3 not found in list")
                        return False

                    logger.info("All 3 agents found in list")
                    ctx.screenshot("step4-agents-found")

                    # Step 5: Click on agent-1 (online)
                    agent_1.first.click()
                    time.sleep(1)
                    ctx.screenshot("step5-agent-1-selected")

                    # Step 6: Click the + button to add a port
                    plus_button = page.locator("button:has-text('+')")
                    if plus_button.count() == 0:
                        logger.error("Plus button not found")
                        return False

                    plus_button.first.click()
                    time.sleep(0.5)
                    ctx.screenshot("step6-add-port-input")

                    # Step 7: Enter port number
                    port_input = page.locator("input[placeholder*='Port']")
                    if port_input.count() == 0:
                        logger.error("Port input not found")
                        return False

                    port_input.first.fill("3000")
                    ctx.screenshot("step7-port-entered")

                    # Step 8: Click Add button
                    add_button = page.locator("button:has-text('Add')")
                    if add_button.count() == 0:
                        logger.error("Add button not found")
                        return False

                    add_button.first.click()
                    time.sleep(1)
                    ctx.screenshot("step8-tab-created")

                    # Step 9: Verify tab was created with port
                    port_tab = page.locator("text=:3000")
                    if port_tab.count() == 0:
                        logger.error("Port tab :3000 not found")
                        return False

                    logger.info("Port tab :3000 created successfully")
                    ctx.screenshot("step9-test-complete")

                    logger.info("Test completed successfully")
                    return True

            except Exception as e:
                logger.exception(f"Test failed with error: {e}")
                return False
            finally:
                browser.close()

    finally:
        # Cleanup backend process
        if backend_process:
            logger.info("Stopping dashboard backend...")
            backend_process.terminate()
            try:
                backend_process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                backend_process.kill()
            logger.info("Dashboard backend stopped")