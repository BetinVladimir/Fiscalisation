#!/usr/bin/env bash
# check-migrations.sh — migration numbering and idempotency (MIG-001..003)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATIONS_DIR="$ROOT/fiscal-backend/database"

if [ ! -d "$MIGRATIONS_DIR" ]; then
  echo "MIG-ENV: migrations dir $MIGRATIONS_DIR not found" >&2
  exit 3
fi

errors=0

# MIG-001: migration files numbered sequentially with no gaps
mapfile -t files < <(find "$MIGRATIONS_DIR" -name '*.sql' | grep -E '/[0-9]+_' | sort)
prev=0
for f in "${files[@]}"; do
  num=$(basename "$f" | grep -oE '^[0-9]+')
  if [ -n "$num" ] && [ "$((10#$num))" -ne "$((prev + 1))" ] && [ "$((10#$num))" -ne 0 ]; then
    echo "MIG-001: gap in migration sequence: expected $((prev+1)), got $((10#$num)) ($f)" >&2
    errors=$((errors+1))
  fi
  prev=$((10#$num))
done

# MIG-002: each migration has a corresponding down migration or is marked no-down
for f in "${files[@]}"; do
  base=$(basename "$f" .sql)
  dir=$(dirname "$f")
  if echo "$base" | grep -qiE '\.up$|_up$'; then
    down="${f/\.up\.sql/.down.sql}"
    down="${down/_up\.sql/_down.sql}"
    if [ ! -f "$down" ] && ! grep -qi 'no-down\|irreversible' "$f"; then
      echo "MIG-002: no down migration for $f" >&2
      errors=$((errors+1))
    fi
  fi
done

# MIG-003: no raw DROP TABLE without IF EXISTS
if grep -rn 'DROP TABLE[^I]' "$MIGRATIONS_DIR" 2>/dev/null | grep -v 'IF EXISTS' | grep -q .; then
  echo "MIG-003: DROP TABLE without IF EXISTS in migration" >&2
  grep -rn 'DROP TABLE[^I]' "$MIGRATIONS_DIR" | grep -v 'IF EXISTS' >&2 || true
  errors=$((errors+1))
fi

if [ $errors -eq 0 ]; then
  echo "MIGRATIONS OK: $(echo "${files[@]}" | wc -w | tr -d ' ') migration files checked"
  exit 0
else
  echo "MIGRATIONS FAIL: $errors checks failed" >&2
  exit 1
fi
