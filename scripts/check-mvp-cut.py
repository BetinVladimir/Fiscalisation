#!/usr/bin/env python3
"""check-mvp-cut.py — validate docs/MVP1/execution/mvp-cut.v1.json.

Rules:
- Every plan ID in all_tasks must appear in exactly one of must_before_hil or after_hil.
- No plan ID may appear in both lists (duplicate).
- No plan ID in all_tasks may be absent from both lists (missing).
Exit 0: valid; Exit 1: any violation.
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
DEFAULT_FILE = ROOT / "docs" / "MVP1" / "execution" / "mvp-cut.v1.json"


def check(path: Path) -> int:
    if not path.exists():
        print(f"ERROR: {path} not found", file=sys.stderr)
        return 1

    try:
        data = json.loads(path.read_text())
    except json.JSONDecodeError as e:
        print(f"ERROR: JSON parse error: {e}", file=sys.stderr)
        return 1

    before = set(data.get("must_before_hil", []))
    after = set(data.get("after_hil", []))
    all_tasks = list(data.get("all_tasks", []))

    errors = []

    # Duplicates: ID in both lists
    duplicates = before & after
    for tid in sorted(duplicates):
        errors.append(f"DUPLICATE: '{tid}' appears in both must_before_hil and after_hil")

    # Missing: ID in all_tasks but not in either list
    for tid in all_tasks:
        if tid not in before and tid not in after:
            errors.append(f"MISSING: '{tid}' is in all_tasks but not in either list")

    if errors:
        for e in errors:
            print(e)
        print(f"FAIL: {len(errors)} violation(s)")
        return 1

    print(
        f"OK: mvp-cut.v1.json valid "
        f"({len(before)} before_hil, {len(after)} after_hil, {len(all_tasks)} all_tasks)"
    )
    return 0


if __name__ == "__main__":
    target = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_FILE
    sys.exit(check(target))
