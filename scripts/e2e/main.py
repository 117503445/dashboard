#!/usr/bin/env python3
"""
E2E Test Runner

Usage:
    uv run main.py              # Run all tests
    uv run main.py --case case1 # Run specific test case
"""

import argparse
import importlib.util
import logging
import os
import sys
from datetime import datetime
from pathlib import Path

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


def get_cases_dir() -> Path:
    """Get the cases directory path."""
    return Path(__file__).parent / "cases"


def get_output_base_dir() -> Path:
    """Get the base output directory for test results."""
    return Path(__file__).parent.parent.parent / "data" / "e2e" / "cases"


def discover_cases() -> list[str]:
    """Discover all available test cases."""
    cases_dir = get_cases_dir()
    cases = []
    for file in cases_dir.glob("case*.py"):
        case_name = file.stem
        cases.append(case_name)
    return sorted(cases)


def run_case(case_name: str) -> bool:
    """Run a specific test case."""
    cases_dir = get_cases_dir()
    case_file = cases_dir / f"{case_name}.py"

    if not case_file.exists():
        logger.error(f"Case file not found: {case_file}")
        return False

    # Create output directory with timestamp
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    output_dir = get_output_base_dir() / f"{timestamp}-{case_name}"
    output_dir.mkdir(parents=True, exist_ok=True)

    # Create subdirectories
    screenshots_dir = output_dir / "screenshots"
    screenshots_dir.mkdir(exist_ok=True)
    logs_dir = output_dir / "logs"
    logs_dir.mkdir(exist_ok=True)

    # Setup file logging
    log_file = logs_dir / "test.log"
    file_handler = logging.FileHandler(log_file)
    file_handler.setFormatter(logging.Formatter("%(asctime)s - %(levelname)s - %(message)s"))
    logger.addHandler(file_handler)

    logger.info(f"Starting test case: {case_name}")
    logger.info(f"Output directory: {output_dir}")

    # Load and run the case
    try:
        spec = importlib.util.spec_from_file_location(case_name, case_file)
        module = importlib.util.module_from_spec(spec)
        sys.modules[case_name] = module
        spec.loader.exec_module(module)

        # Run the test function
        if hasattr(module, "run_test"):
            success = module.run_test(
                output_dir=output_dir,
                screenshots_dir=screenshots_dir,
                logs_dir=logs_dir,
                logger=logger,
            )
        else:
            logger.error(f"Case {case_name} does not have run_test function")
            return False

        if success:
            logger.info(f"Test case {case_name} PASSED")
        else:
            logger.error(f"Test case {case_name} FAILED")

        return success

    except Exception as e:
        logger.exception(f"Error running test case {case_name}: {e}")
        return False
    finally:
        logger.removeHandler(file_handler)


def main():
    parser = argparse.ArgumentParser(description="E2E Test Runner")
    parser.add_argument(
        "--case",
        type=str,
        help="Run specific test case (e.g., case1)",
    )
    args = parser.parse_args()

    # Discover available cases
    all_cases = discover_cases()

    if not all_cases:
        logger.warning("No test cases found")
        return 0

    logger.info(f"Available cases: {all_cases}")

    # Determine which cases to run
    if args.case:
        if args.case not in all_cases:
            logger.error(f"Case '{args.case}' not found. Available: {all_cases}")
            return 1
        cases_to_run = [args.case]
    else:
        cases_to_run = all_cases

    logger.info(f"Running cases: {cases_to_run}")

    # Run tests
    results = {}
    for case_name in cases_to_run:
        success = run_case(case_name)
        results[case_name] = "PASSED" if success else "FAILED"

    # Print summary
    print("\n" + "=" * 50)
    print("Test Summary:")
    print("=" * 50)
    for case_name, result in results.items():
        print(f"  {case_name}: {result}")
    print("=" * 50)

    # Return non-zero if any test failed
    if "FAILED" in results.values():
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())