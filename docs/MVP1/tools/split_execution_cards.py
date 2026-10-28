#!/usr/bin/env python3
"""split_execution_cards.py — Generate compact execution cards from plan.json.

One file per task unit in docs/MVP1/generated_cards/.
Also writes an index file: docs/MVP1/generated_cards/index.json.

Constraints (R-04):
- Each card file <= 150 lines and <= 20 KiB.
- If requirements exceed the limit, exit with FAIL and the unit ID — do not truncate.
- Deterministic regeneration (same output for same input).
- plan.json and status.json are read only (never written).

Usage:
  python3 docs/MVP1/tools/split_execution_cards.py [--plan PATH] [--output-dir DIR] [--status PATH]
"""
import argparse
import json
import os
import sys
import textwrap
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent.parent  # tools/ -> MVP1/ -> docs/ -> repo root
MAX_LINES = 150
MAX_BYTES = 20 * 1024  # 20 KiB


def load_json(path: str) -> dict:
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def build_card(task: dict, unit: dict, policy: str, source_docs: dict) -> str:
    """Build a compact card for one unit. Returns the card text."""
    tid = task["id"]
    uid = unit["id"]
    priority = task.get("priority", "?")
    source = task.get("source", "")
    depends = task.get("depends", [])
    alias_of = task.get("alias_of")
    external_requires = task.get("external_requires", [])
    notes = task.get("notes", "")

    lines = []
    lines.append(f"# {uid} · {priority} · {source}")
    lines.append("")
    if alias_of:
        lines.append(f"Alias: {alias_of}. No own implementation; close only via canonical task.")
        lines.append("")
    if depends:
        lines.append(f"Dependencies: {', '.join(depends)}")
    if external_requires:
        lines.append(f"External requires: {', '.join(str(e) for e in external_requires)}")
    if notes:
        lines.append(f"Notes: {notes}")
    if depends or external_requires or notes:
        lines.append("")

    # Policy excerpt (first 3 lines)
    if policy:
        policy_lines = [l for l in policy.splitlines() if l.strip()][:3]
        for pl in policy_lines:
            lines.append(pl)
        lines.append("")

    # Unit spec
    title = unit.get("title", "").strip()
    if title:
        lines.append(title)
        lines.append("")

    # Source reference
    lines.append(f"Source: {source}")
    lines.append(f"Task: {tid}  Unit: {uid}")

    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--plan",       default=str(ROOT / "docs/MVP1/fixes/plan.json"))
    ap.add_argument("--status",     default=str(ROOT / "docs/MVP1/execution/status.json"))
    ap.add_argument("--output-dir", default=str(ROOT / "docs/MVP1/generated_cards"))
    args = ap.parse_args()

    plan = load_json(args.plan)
    policy = plan.get("policy", "")
    source_docs = plan.get("source_documents", {})
    tasks = plan.get("tasks", [])

    out_dir = Path(args.output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    index = []
    errors = []

    for task in sorted(tasks, key=lambda t: t["id"]):
        tid = task["id"]
        units = task.get("units", [])

        # If no units (alias or simple task), create a stub unit
        if not units:
            units = [{"id": tid, "title": task.get("spec", "").strip()[:500]}]

        for unit in units:
            uid = unit["id"]
            card_text = build_card(task, unit, policy, source_docs)

            line_count = len(card_text.splitlines())
            byte_count = len(card_text.encode("utf-8"))

            if line_count > MAX_LINES or byte_count > MAX_BYTES:
                errors.append(
                    f"FAIL: unit {uid} exceeds limits: {line_count} lines "
                    f"(max {MAX_LINES}), {byte_count} bytes (max {MAX_BYTES})"
                )
                continue

            # Safe filename: replace / and spaces
            safe_uid = uid.replace("/", "_").replace(" ", "_")
            out_path = out_dir / f"{safe_uid}.md"
            out_path.write_text(card_text, encoding="utf-8")

            index.append({
                "unit_id": uid,
                "task_id": tid,
                "priority": task.get("priority", "?"),
                "source": task.get("source", ""),
                "file": f"{safe_uid}.md",
                "lines": line_count,
                "bytes": byte_count,
            })

    # Write index
    index_path = out_dir / "index.json"
    index_path.write_text(
        json.dumps({"schema": 1, "cards": index}, indent=2, ensure_ascii=False),
        encoding="utf-8"
    )

    if errors:
        for e in errors:
            print(e, file=sys.stderr)
        sys.exit(1)

    print(f"OK: generated {len(index)} cards in {out_dir}")
    sys.exit(0)


if __name__ == "__main__":
    main()
