#!/usr/bin/env python3
import re
import subprocess
import sys
from pathlib import Path

GO_TEST_RE = re.compile(r"`(Test[A-Za-z0-9_]+)`")
SWIFT_TEST_RE = re.compile(r"`SwiftTest:([A-Za-z0-9_]+)`")
BACKTICKED_IDENTIFIER_RE = re.compile(r"`([a-z][A-Za-z0-9_]+)`")
BACKTICKED_PATH_RE = re.compile(r"`([A-Za-z0-9_./-]+/[A-Za-z0-9_./-]+)`")
GO_SYMBOL_RE = re.compile(r"`GoSymbol:([A-Za-z0-9_]+)`")
LOCAL_MARKDOWN_LINK_RE = re.compile(r"\[[^\]]+\]\(([^)#]+)(?:#[^)]+)?\)")
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


def go_symbol_names():
    names = set()
    decl_re = re.compile(r"^(?:func|type|const|var)\s+([A-Za-z_][A-Za-z0-9_]*)")
    const_var_block_re = re.compile(r"^(?:const|var)\s*\(")
    block_name_re = re.compile(r"^\s*([A-Za-z_][A-Za-z0-9_]*)\b")
    in_block = False
    for path in Path(".").glob("**/*.go"):
        if any(part in {".git", "vendor"} for part in path.parts):
            continue
        for line in path.read_text().splitlines():
            if in_block:
                if line.strip() == ")":
                    in_block = False
                    continue
                match = block_name_re.match(line)
                if match:
                    names.add(match.group(1))
                continue
            match = decl_re.match(line)
            if match:
                names.add(match.group(1))
            if const_var_block_re.match(line):
                in_block = True
    return names


def acceptance_rows(text):
    rows = []
    for line_number, line in enumerate(text.splitlines(), start=1):
        if not line.startswith("|") or line.startswith("|---") or "| Acceptance |" in line:
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if len(cells) >= 2:
            rows.append((line_number, cells[0], cells[1]))
    return rows


def has_checkable_evidence(evidence):
    if GO_TEST_RE.search(evidence) or SWIFT_TEST_RE.search(evidence) or GO_SYMBOL_RE.search(evidence):
        return True
    for item in BACKTICKED_PATH_RE.findall(evidence):
        if Path(item).exists():
            return True
    for target in LOCAL_MARKDOWN_LINK_RE.findall(evidence):
        if target.startswith(("http://", "https://", "mailto:")):
            continue
        if Path("docs", target).exists():
            return True
    return False


def main():
    path = Path("docs/spec-acceptance.md")
    text = path.read_text()
    referenced_go = set(GO_TEST_RE.findall(text))
    referenced_swift = set(SWIFT_TEST_RE.findall(text))
    referenced_paths = {item for item in BACKTICKED_PATH_RE.findall(text) if not item.startswith(("http://", "https://"))}
    referenced_symbols = set(GO_SYMBOL_RE.findall(text))
    available_swift = swift_test_names()
    untyped_swift = sorted(set(BACKTICKED_IDENTIFIER_RE.findall(text)) & available_swift)
    missing_go = sorted(referenced_go - go_test_names())
    missing_swift = sorted(referenced_swift - available_swift)
    missing_paths = sorted(item for item in referenced_paths if not Path(item).exists())
    missing_symbols = sorted(referenced_symbols - go_symbol_names())
    weak_rows = [
        (line_number, acceptance)
        for line_number, acceptance, evidence in acceptance_rows(text)
        if not has_checkable_evidence(evidence)
    ]
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
    if missing_paths:
        print("SPEC acceptance references missing paths:", file=sys.stderr)
        for name in missing_paths:
            print(f"  {name}", file=sys.stderr)
    if missing_symbols:
        print("SPEC acceptance references missing Go symbols:", file=sys.stderr)
        for name in missing_symbols:
            print(f"  {name}", file=sys.stderr)
    if weak_rows:
        print("SPEC acceptance rows without machine-checkable evidence:", file=sys.stderr)
        for line_number, acceptance in weak_rows:
            print(f"  line {line_number}: {acceptance}", file=sys.stderr)
    if missing_go or missing_swift or untyped_swift or missing_paths or missing_symbols or weak_rows:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
