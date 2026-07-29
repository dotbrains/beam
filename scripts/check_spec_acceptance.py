#!/usr/bin/env python3
import re
import subprocess
import sys
from pathlib import Path

GO_TEST_RE = re.compile(r"`(Test[A-Za-z0-9_]+)`")
SWIFT_TEST_RE = re.compile(r"`SwiftTest:([A-Za-z0-9_]+)`")
BACKTICKED_IDENTIFIER_RE = re.compile(r"`([a-z][A-Za-z0-9_]+)`")
SWIFT_DECL_RE = re.compile(r"@Test\s+func\s+([A-Za-z0-9_]+)\s*\(")


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


def swift_test_names():
    names = set()
    for path in Path("ios/Tests").glob("**/*.swift"):
        names.update(SWIFT_DECL_RE.findall(path.read_text()))
    return names


def main():
    path = Path("docs/spec-acceptance.md")
    text = path.read_text()
    referenced_go = set(GO_TEST_RE.findall(text))
    referenced_swift = set(SWIFT_TEST_RE.findall(text))
    available_swift = swift_test_names()
    untyped_swift = sorted(set(BACKTICKED_IDENTIFIER_RE.findall(text)) & available_swift)
    missing_go = sorted(referenced_go - go_test_names())
    missing_swift = sorted(referenced_swift - available_swift)
    if missing_go:
        print("SPEC acceptance references missing Go tests:", file=sys.stderr)
        for name in missing_go:
            print(f"  {name}", file=sys.stderr)
    if missing_swift:
        print("SPEC acceptance references missing Swift tests:", file=sys.stderr)
        for name in missing_swift:
            print(f"  {name}", file=sys.stderr)
    if untyped_swift:
        print("SPEC acceptance has untyped Swift test references:", file=sys.stderr)
        for name in untyped_swift:
            print(f"  `{name}` should be `SwiftTest:{name}`", file=sys.stderr)
    if missing_go or missing_swift or untyped_swift:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
