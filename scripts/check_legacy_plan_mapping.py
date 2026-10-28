#!/usr/bin/env python3
"""check_legacy_plan_mapping.py — Validate docs/MVP1/fixtures/legacy_plan_mapping.json.

Exit 0: valid; Exit 1: errors found; Exit 2: plan.json not found.

Usage:
  python3 scripts/check_legacy_plan_mapping.py [--mapping PATH] [--plan PATH]
"""
import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent

REQUIRED_GROUP_IDS = {"P40", "P41", "P42", "P43", "P44"}
REQUIRED_GROUP_FIELDS = {"id", "name", "legacy_description", "task_ids", "evidence_gate", "owner", "manual_gate"}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--mapping", default=str(ROOT / "docs/MVP1/fixtures/legacy_plan_mapping.json"))
    ap.add_argument("--plan",    default=str(ROOT / "docs/MVP1/fixes/plan.json"))
    args = ap.parse_args()

    errors = []

    # Load files
    try:
        mapping = json.loads(Path(args.mapping).read_text())
    except Exception as e:
        print(f"FAIL: cannot parse mapping file: {e}", file=sys.stderr)
        sys.exit(1)

    try:
        plan = json.loads(Path(args.plan).read_text())
    except FileNotFoundError:
        print(f"FAIL: plan.json not found at {args.plan}", file=sys.stderr)
        sys.exit(2)
    except Exception as e:
        print(f"FAIL: cannot parse plan.json: {e}", file=sys.stderr)
        sys.exit(2)

    # Schema check
    if mapping.get("schema") != 1:
        errors.append(f"mapping.schema must be 1, got {mapping.get('schema')!r}")

    groups = mapping.get("groups", [])
    if not isinstance(groups, list) or len(groups) == 0:
        errors.append("mapping.groups must be a non-empty array")
        _report(errors)
        return

    # Collect plan task IDs
    plan_ids = {t["id"] for t in plan.get("tasks", [])}

    found_group_ids = set()
    for grp in groups:
        gid = grp.get("id", "?")
        found_group_ids.add(gid)

        # Required fields
        for field in REQUIRED_GROUP_FIELDS:
            if field not in grp:
                errors.append(f"Group {gid}: missing required field '{field}'")

        # task_ids must be non-empty list of strings in plan.json
        task_ids = grp.get("task_ids", [])
        if not isinstance(task_ids, list) or len(task_ids) == 0:
            errors.append(f"Group {gid}: task_ids must be a non-empty array")
        else:
            for tid in task_ids:
                if tid not in plan_ids:
                    errors.append(f"Group {gid}: task_id '{tid}' not found in plan.json")

        # evidence_gate must be non-empty
        if not grp.get("evidence_gate", "").strip():
            errors.append(f"Group {gid}: evidence_gate must not be empty")

        # owner must be a known value
        owner = grp.get("owner", "")
        if owner not in {"qwen", "codex-first", "human", "hil"}:
            errors.append(f"Group {gid}: owner '{owner}' is not in {{qwen, codex-first, human, hil}}")

    # All required group IDs must be present
    for gid in REQUIRED_GROUP_IDS:
        if gid not in found_group_ids:
            errors.append(f"Missing required group '{gid}'")

    _report(errors)


def _report(errors):
    if not errors:
        print("OK: legacy_plan_mapping.json is valid")
        sys.exit(0)
    else:
        for e in errors:
            print(f"FAIL: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
