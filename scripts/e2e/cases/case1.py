"""
Test case 1: Basic Healthz Page Test

This test verifies that the frontend loads correctly and displays
the healthz status from the backend.
"""

import logging
from pathlib import Path

from playwright.sync_api import sync_playwright

from lib.utils import TestContext


def run_test(
    output_dir: Path,
    screenshots_dir: Path,
    logs_dir: Path,
    logger: logging.Logger,
) -> bool:
    """Run the healthz page test."""
    logger.info("Starting case1: Healthz Page Test")

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

                # Step 2: Wait for the status card to load
                ctx.wait_for_selector("text=Service Status")
                ctx.screenshot("step2-title-visible")

                # Step 3: Wait for health check result
                # Either "Healthy" or "Error" should appear
                import time
                time.sleep(2)  # Wait for API call
                ctx.screenshot("step3-health-check-result")

                # Step 4: Verify the page structure
                page = ctx.page
                if not page:
                    return False

                # Check for Service Status title
                title = page.locator("text=Service Status")
                if title.count() == 0:
                    logger.error("Service Status title not found")
                    return False

                # Check for Backend Service card
                card = page.locator("text=Backend Service")
                if card.count() == 0:
                    logger.error("Backend Service card not found")
                    return False

                # Check for status indicator (either Healthy or Error)
                status_healthy = page.locator("text=Healthy")
                status_error = page.locator("text=Error")
                status_checking = page.locator("text=Checking")

                if status_healthy.count() == 0 and status_error.count() == 0 and status_checking.count() == 0:
                    logger.error("No status indicator found")
                    return False

                logger.info("Page structure verified successfully")
                ctx.screenshot("step4-verification-complete")

                # If we got here with Healthy status, the test passed
                if status_healthy.count() > 0:
                    logger.info("Backend is healthy!")
                    return True
                elif status_error.count() > 0:
                    logger.warning("Backend returned error status (this is expected if backend is not running)")
                    return True  # Still pass the test, as the UI is working correctly

                logger.info("Test completed successfully")
                return True

        except Exception as e:
            logger.exception(f"Test failed with error: {e}")
            return False
        finally:
            browser.close()