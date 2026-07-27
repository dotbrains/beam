#!/usr/bin/env python3
import re
import sys
from pathlib import Path

LINK_RE = re.compile(r"\[[^\]]+\]\(([^)]+)\)")


def main():
    failures = []
    for path in [Path("README.md"), *Path("docs").glob("**/*.md"), Path("SPEC.md")]:
        if not path.exists():
            continue
        for target in LINK_RE.findall(path.read_text()):
            if target.startswith(("http://", "https://", "#", "mailto:")):
                continue
            clean = target.split("#", 1)[0]
            if not clean:
                continue
            resolved = (path.parent / clean).resolve()
            if not resolved.exists():
                failures.append(f"{path}: broken local link {target}")
    if failures:
        print("Documentation link failures:", file=sys.stderr)
        for failure in failures:
            print(f"  {failure}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
