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
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


@dataclass
class CaseResult:
    name: str
    success: bool
    output_dir: Path
    screenshots_dir: Path
    title: str = ""
    description: str = ""
    steps: dict[str, str] = field(default_factory=dict)


def get_cases_dir() -> Path:
    """获取测试用例目录路径"""
    return Path(__file__).parent / "cases"


def get_runs_dir() -> Path:
    """获取测试运行的基础目录"""
    return Path(__file__).parent.parent.parent / "data" / "e2e" / "runs"


def discover_cases() -> list[str]:
    """发现所有可用的测试用例"""
    cases_dir = get_cases_dir()
    cases = []
    for file in cases_dir.glob("case*.py"):
        case_name = file.stem
        cases.append(case_name)
    return sorted(cases)


def run_case(case_name: str, run_dir: Path) -> CaseResult:
    """运行指定的测试用例"""
    cases_dir = get_cases_dir()
    case_file = cases_dir / f"{case_name}.py"

    output_dir = run_dir / case_name
    output_dir.mkdir(parents=True, exist_ok=True)
    screenshots_dir = output_dir / "screenshots"
    screenshots_dir.mkdir(exist_ok=True)
    logs_dir = output_dir / "logs"
    logs_dir.mkdir(exist_ok=True)

    result = CaseResult(
        name=case_name,
        success=False,
        output_dir=output_dir,
        screenshots_dir=screenshots_dir,
    )

    if not case_file.exists():
        logger.error(f"测试用例文件未找到: {case_file}")
        return result

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

        # 提取 CASE_META
        meta = getattr(module, "CASE_META", {})
        result.title = meta.get("title", case_name)
        result.description = meta.get("description", "")
        result.steps = meta.get("steps", {})

        # 运行测试函数
        if hasattr(module, "run_test"):
            result.success = module.run_test(
                output_dir=output_dir,
                screenshots_dir=screenshots_dir,
                logs_dir=logs_dir,
                logger=logger,
            )
        else:
            logger.error(f"测试用例 {case_name} 没有 run_test 函数")
            return result

        if result.success:
            logger.info(f"测试用例 {case_name} 通过")
        else:
            logger.error(f"测试用例 {case_name} 失败")

        return result

    except Exception as e:
        logger.exception(f"运行测试用例 {case_name} 时出错: {e}")
        return result
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


def generate_readme(run_dir: Path, run_timestamp: str, results: list[CaseResult]) -> Path:
    """根据测试结果生成 README.md"""
    readme_path = run_dir / "README.md"

    passed = sum(1 for r in results if r.success)
    failed = len(results) - passed

    lines: list[str] = []
    lines.append(f"# E2E 测试报告\n")
    lines.append(f"**运行时间**: {run_timestamp}  \n")
    lines.append(f"**结果**: {passed} 通过 / {failed} 失败 / {len(results)} 总计\n")

    # 汇总表格
    lines.append("## 用例概览\n")
    lines.append("| 用例 | 名称 | 结果 |")
    lines.append("|------|------|------|")
    for r in results:
        status = "✅ 通过" if r.success else "❌ 失败"
        lines.append(f"| [{r.name}](#{r.name}) | {r.title} | {status} |")
    lines.append("")

    # 每个用例的详细信息
    for r in results:
        status = "✅ 通过" if r.success else "❌ 失败"
        lines.append(f"---\n")
        lines.append(f"## {r.name}\n")
        lines.append(f"**{r.title}** — {status}\n")

        if r.description:
            lines.append(f"{r.description}\n")

        # 收集截图文件并按文件名排序
        screenshots = sorted(r.screenshots_dir.glob("*.png"))
        if not screenshots:
            lines.append("*无截图*\n")
            continue

        lines.append("### 测试步骤\n")
        for img in screenshots:
            step_key = img.stem
            description = r.steps.get(step_key, step_key)
            rel_path = f"./{r.name}/screenshots/{img.name}"
            lines.append(f"#### {step_key}\n")
            lines.append(f"{description}\n")
            lines.append(f"![{step_key}]({rel_path})\n")

    readme_path.write_text("\n".join(lines), encoding="utf-8")
    return readme_path


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

    # 创建本次运行目录: data/e2e/runs/YYYYMMDD.HHMMSS/
    run_timestamp = datetime.now().strftime("%Y%m%d.%H%M%S")
    run_dir = get_runs_dir() / run_timestamp
    run_dir.mkdir(parents=True, exist_ok=True)

    # 运行测试
    case_results: list[CaseResult] = []
    for case_name in cases_to_run:
        result = run_case(case_name, run_dir)
        case_results.append(result)

    # 生成 README
    readme_path = generate_readme(run_dir, run_timestamp, case_results)
    logger.info(f"测试报告已生成: {readme_path}")

    # 打印摘要
    print("\n" + "=" * 50)
    print("测试摘要:")
    print("=" * 50)
    for r in case_results:
        status = "通过" if r.success else "失败"
        print(f"  {r.name}: {status}")
    print("=" * 50)
    print(f"\n📄 测试报告: {readme_path}")

    # 如果有测试失败，返回非零
    if any(not r.success for r in case_results):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())