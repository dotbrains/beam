#!/usr/bin/env python3
import json
import subprocess
import sys
from collections import Counter
from pathlib import Path

EXCLUDED_PARTS = {
    ".git",
    "dist",
    "build",
    "node_modules",
    "vendor",
    "fixtures",
}


def tracked_files():
    result = subprocess.run(
        ["git", "ls-files"],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return [Path(line) for line in result.stdout.splitlines() if line]


def main():
    budget_path = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("scripts/flat-directory-budgets.json")
    budget = json.loads(budget_path.read_text())
    default_files = budget["default_files"]
    overrides = budget.get("directories", {})
    counts = Counter()

    for path in tracked_files():
        if set(path.parts) & EXCLUDED_PARTS:
            continue
        directory = path.parent.as_posix()
        counts["." if directory == "." else directory] += 1

    failures = []
    for directory, count in sorted(counts.items()):
        override = overrides.get(directory)
        limit = override["limit"] if isinstance(override, dict) else default_files
        if count > limit:
            failures.append(f"{directory}: {count} files > budget {limit}")

    if failures:
        print("Flat directory budget failures:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
