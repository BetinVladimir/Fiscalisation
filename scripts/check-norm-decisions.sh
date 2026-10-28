#!/usr/bin/env bash
# check-norm-decisions.sh — each NORM-xx card has a temporary rule or answer (NRM-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXES_DIR="$ROOT/docs/MVP1/fixes"
FIXED_DIR="$ROOT/docs/MVP1/fixed"

errors=0
checked=0

for dir in "$FIXES_DIR" "$FIXED_DIR"; do
  [ -d "$dir" ] || continue
  for card in "$dir"/NORM-*.md; do
    [ -f "$card" ] || continue
    id="$(basename "$card" .md)"
    checked=$((checked+1))
    # NRM-001: each NORM-xx card must have "Ответ:" or "Решение:" or "Временное правило:"
    if ! grep -qiE '(ответ|решение|временное правило|decision|answer|rule):' "$card"; then
      echo "NRM-001: $id has no decision/answer/rule" >&2
      errors=$((errors+1))
    fi
  done
done

if [ $checked -eq 0 ]; then
  echo "NORM-DECISIONS: no NORM-* cards found (0 checked)"
  exit 0
fi

if [ $errors -eq 0 ]; then
  echo "NORM-DECISIONS OK: $checked NORM-* cards checked"
  exit 0
else
  echo "NORM-DECISIONS FAIL: $errors/$checked cards missing decisions" >&2
  exit 1
fi
