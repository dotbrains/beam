#!/usr/bin/env python3
import re
import subprocess
import sys
from pathlib import Path

GO_TEST_RE = re.compile(r"`(Test[A-Za-z0-9_]+)`")


def go_test_names():
    result = subprocess.run(
        ["go", "test", "./internal/beam", "./cmd", "./internal/storage", "-list", "Test"],
        check=True,
        stdout=subprocess.PIPE,
        text=True,
    )
    names = set()
    for line in result.stdout.splitlines():
        if line.startswith("Test"):
            names.add(line.strip())
    return names


def main():
    path = Path("docs/spec-acceptance.md")
    referenced = set(GO_TEST_RE.findall(path.read_text()))
    available = go_test_names()
    missing = sorted(referenced - available)
    if missing:
        print("SPEC acceptance references missing Go tests:", file=sys.stderr)
        for name in missing:
            print(f"  {name}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
