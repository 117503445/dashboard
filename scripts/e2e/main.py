#!/usr/bin/env python3
"""
E2E 测试运行器

用法:
    uv run main.py              # 运行所有测试
    uv run main.py --case case1 # 运行指定测试用例
"""

import argparse
import importlib.util
import logging
import os
import signal
import subprocess
import sys
from datetime import datetime
from pathlib import Path

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


def get_cases_dir() -> Path:
    """获取测试用例目录路径"""
    return Path(__file__).parent / "cases"


def get_output_base_dir() -> Path:
    """获取测试结果的基础输出目录"""
    return Path(__file__).parent.parent.parent / "data" / "e2e" / "cases"


def discover_cases() -> list[str]:
    """发现所有可用的测试用例"""
    cases_dir = get_cases_dir()
    cases = []
    for file in cases_dir.glob("case*.py"):
        case_name = file.stem
        cases.append(case_name)
    return sorted(cases)


def run_case(case_name: str) -> bool:
    """运行指定的测试用例"""
    cases_dir = get_cases_dir()
    case_file = cases_dir / f"{case_name}.py"

    if not case_file.exists():
        logger.error(f"测试用例文件未找到: {case_file}")
        return False

    # 创建带时间戳的输出目录
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    output_dir = get_output_base_dir() / f"{timestamp}-{case_name}"
    output_dir.mkdir(parents=True, exist_ok=True)

    # 创建子目录
    screenshots_dir = output_dir / "screenshots"
    screenshots_dir.mkdir(exist_ok=True)
    logs_dir = output_dir / "logs"
    logs_dir.mkdir(exist_ok=True)

    # 设置文件日志
    log_file = logs_dir / "test.log"
    file_handler = logging.FileHandler(log_file)
    file_handler.setFormatter(logging.Formatter("%(asctime)s - %(levelname)s - %(message)s"))
    logger.addHandler(file_handler)

    logger.info(f"开始测试用例: {case_name}")
    logger.info(f"输出目录: {output_dir}")

    # 加载并运行用例
    try:
        spec = importlib.util.spec_from_file_location(case_name, case_file)
        module = importlib.util.module_from_spec(spec)
        sys.modules[case_name] = module
        spec.loader.exec_module(module)

        # 运行测试函数
        if hasattr(module, "run_test"):
            success = module.run_test(
                output_dir=output_dir,
                screenshots_dir=screenshots_dir,
                logs_dir=logs_dir,
                logger=logger,
            )
        else:
            logger.error(f"测试用例 {case_name} 没有 run_test 函数")
            return False

        if success:
            logger.info(f"测试用例 {case_name} 通过")
        else:
            logger.error(f"测试用例 {case_name} 失败")

        return success

    except Exception as e:
        logger.exception(f"运行测试用例 {case_name} 时出错: {e}")
        return False
    finally:
        logger.removeHandler(file_handler)


# 测试前需要清理的进程
PROCESSES_TO_KILL = ["sshole_hub", "sshole_agent", "dashboard"]


def cleanup_processes():
    """清理之前测试运行遗留的进程"""
    logger.info("清理遗留进程...")
    for proc_name in PROCESSES_TO_KILL:
        try:
            result = subprocess.run(
                ["pkill", "-f", proc_name],
                capture_output=True,
                text=True,
            )
            if result.returncode == 0:
                logger.info(f"已终止 {proc_name} 进程")
        except Exception as e:
            logger.warning(f"终止 {proc_name} 失败: {e}")


def main():
    parser = argparse.ArgumentParser(description="E2E 测试运行器")
    parser.add_argument(
        "--case",
        type=str,
        help="运行指定测试用例（如 case1）",
    )
    args = parser.parse_args()

    # 运行测试前清理遗留进程
    cleanup_processes()

    # 发现可用用例
    all_cases = discover_cases()

    if not all_cases:
        logger.warning("未找到测试用例")
        return 0

    logger.info(f"可用用例: {all_cases}")

    # 确定要运行的用例
    if args.case:
        if args.case not in all_cases:
            logger.error(f"用例 '{args.case}' 未找到。可用: {all_cases}")
            return 1
        cases_to_run = [args.case]
    else:
        cases_to_run = all_cases

    logger.info(f"运行用例: {cases_to_run}")

    # 运行测试
    results = {}
    for case_name in cases_to_run:
        success = run_case(case_name)
        results[case_name] = "通过" if success else "失败"

    # 打印摘要
    print("\n" + "=" * 50)
    print("测试摘要:")
    print("=" * 50)
    for case_name, result in results.items():
        print(f"  {case_name}: {result}")
    print("=" * 50)

    # 如果有测试失败，返回非零
    if "失败" in results.values():
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())