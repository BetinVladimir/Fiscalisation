#!/usr/bin/env bash
# check-test-matrix.sh — each TST-* card must have at least one test file that exists
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXES="$ROOT/docs/MVP1/fixes"

errors=0
checked=0

for card in "$FIXES"/TST-*.md; do
  [ -f "$card" ] || continue
  id="$(basename "$card" .md)"
  lower_id="$(echo "$id" | tr 'A-Z-' 'a-z_')"
  checked=$((checked+1))
  # Look for matching test file in tests/mvp1/
  found=0
  for tf in "$ROOT/tests/mvp1/test_${lower_id}"*.py; do
    [ -f "$tf" ] && found=1 && break
  done
  if [ $found -eq 0 ]; then
    echo "MATRIX-001: $id has no acceptance test in tests/mvp1/" >&2
    errors=$((errors+1))
  fi
done

if [ $errors -eq 0 ]; then
  echo "TEST-MATRIX OK: $checked TST-* cards checked"
  exit 0
else
  echo "TEST-MATRIX FAIL: $errors missing, $checked checked" >&2
  exit 1
fi
