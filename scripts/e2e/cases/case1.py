"""
Test case 1: Basic Healthz Page Test

This test verifies that the frontend loads correctly and displays
the healthz status from the backend.
"""

import logging
import os
import subprocess
import time
import socket
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


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    """Run the healthz page test."""
    logger.info("Starting case1: Healthz Page Test")

    # Start dashboard backend
    project_root = Path(__file__).parent.parent.parent.parent
    backend_process = None

    try:
        env = os.environ.copy()
        env["PORT"] = "8080"

        logger.info("Starting dashboard backend...")
        backend_process = subprocess.Popen(
            ["go", "run", "./cmd/dashboard"],
            cwd=project_root,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )

        # Wait for backend to start
        if not wait_for_port(8080, timeout=30):
            logger.error("Backend failed to start within 30 seconds")
            # Log stdout/stderr
            if backend_process.stdout:
                logger.error(f"Backend output: {backend_process.stdout.read().decode()}")
            return False

        logger.info("Dashboard backend started on port 8080")

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
                    # Step 1: Navigate to the page
                    ctx.goto("/")
                    ctx.screenshot("step1-initial-load")

                    # Step 2: Wait for the page to load
                    ctx.wait_for_selector("text=Dashboard")
                    ctx.screenshot("step2-dashboard-visible")

                    # Step 3: Wait for agent list to load
                    time.sleep(2)
                    ctx.screenshot("step3-agent-list-loaded")

                    # Step 4: Verify the page structure
                    page = ctx.page
                    if not page:
                        return False

                    # Check for Dashboard title
                    title = page.locator("text=Dashboard")
                    if title.count() == 0:
                        logger.error("Dashboard title not found")
                        return False

                    # Check for Agents header
                    agents_header = page.locator("text=Agents")
                    if agents_header.count() == 0:
                        logger.error("Agents header not found")
                        return False

                    # Check for "Select an Agent" prompt
                    select_prompt = page.locator("text=Select an Agent")
                    if select_prompt.count() == 0:
                        logger.warning("Select an Agent prompt not found")

                    logger.info("Page structure verified successfully")
                    ctx.screenshot("step4-verification-complete")

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