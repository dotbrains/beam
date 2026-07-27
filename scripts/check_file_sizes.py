#!/usr/bin/env python3
import json
import subprocess
import sys
from pathlib import Path

EXCLUDED_PARTS = {
    ".git",
    "dist",
    "build",
    "node_modules",
    "vendor",
    "fixtures",
}

CHECK_SUFFIXES = {
    ".go",
    ".md",
    ".py",
    ".sh",
    ".yml",
    ".yaml",
    ".json",
}


def tracked_files():
    result = subprocess.run(
        ["git", "ls-files"],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return [Path(line) for line in result.stdout.splitlines() if line]


def should_check(path):
    return path.suffix in CHECK_SUFFIXES and not (set(path.parts) & EXCLUDED_PARTS)


def main():
    budget_path = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("scripts/file-size-budgets.json")
    budget = json.loads(budget_path.read_text())
    default_lines = budget["default_lines"]
    overrides = budget.get("files", {})
    failures = []

    for path in tracked_files():
        if not should_check(path) or not path.exists():
            continue
        limit = overrides.get(path.as_posix(), default_lines)
        lines = len(path.read_text(errors="ignore").splitlines())
        if lines > limit:
            failures.append(f"{path}: {lines} lines > budget {limit}")

    if failures:
        print("File size budget failures:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
