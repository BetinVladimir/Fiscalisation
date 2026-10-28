#!/usr/bin/env python3
"""
gen-annex29-columns.py — extract Annex 29 table column names from N-18 canonical clean doc.

Reads docs/SUPTO/Naredba N-18 2006 canonical clean.md, extracts tables 18.1..18.5 and 18.9,
outputs fiscal-backend/internal/domain/testdata/annex29_columns.json.
"""
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).parent.parent
SOURCE = ROOT / "docs" / "SUPTO" / "Naredba N-18 2006 canonical clean.md"
OUT = ROOT / "fiscal-backend" / "internal" / "domain" / "testdata" / "annex29_columns.json"

TABLES_OF_INTEREST = {"18.1", "18.2", "18.3", "18.4", "18.5", "18.9"}
# Match section headers like "18.1. Таблица" or "18.9. Таблици"
SECTION_RE = re.compile(r"^(18\.\d+)\.\s")
# Match column bullet "– some text"
BULLET_RE = re.compile(r"^\s*[–\-]\s+(.+)")


def main() -> None:
    text = SOURCE.read_text(encoding="utf-8")
    lines = text.splitlines()

    tables: dict[str, list[str]] = {}
    current: str | None = None

    for line in lines:
        m = SECTION_RE.match(line)
        if m:
            tbl = m.group(1)
            if tbl in TABLES_OF_INTEREST:
                current = tbl
                if current not in tables:
                    tables[current] = []
            else:
                current = None
            continue

        if current is None:
            continue

        b = BULLET_RE.match(line)
        if b:
            # Take the first clause before " – " continuation or parenthetical
            col = b.group(1).strip()
            col = re.split(r"\s+[–\-]\s+|\s*\(", col)[0].strip().rstrip(";,.")
            if col:
                tables[current].append(col)

    if not tables:
        print("ERROR: no tables extracted — check source document path", file=sys.stderr)
        sys.exit(1)

    total = sum(len(v) for v in tables.values())
    result = {"tables": tables, "total_columns": total}

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    for tbl, cols in sorted(tables.items()):
        print(f"  table {tbl}: {len(cols)} columns")
    print(f"total: {total} columns → {OUT}")


if __name__ == "__main__":
    main()
