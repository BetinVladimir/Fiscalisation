#!/usr/bin/env bash
# check-decisions.sh — D-1..D-11 architectural decisions filled (DEC-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DECISIONS_FILE="$ROOT/docs/MVP1/DECISIONS.md"

errors=0

# DEC-001: DECISIONS.md exists
if [ ! -f "$DECISIONS_FILE" ]; then
  echo "DEC-001: docs/MVP1/DECISIONS.md not found" >&2
  exit 1
fi

# DEC-002: D-1 through D-11 all present and filled
for i in $(seq 1 11); do
  if ! grep -qE "D-${i}[[:space:]]|D-${i}:" "$DECISIONS_FILE"; then
    echo "DEC-002: D-$i not found in DECISIONS.md" >&2
    errors=$((errors+1))
  else
    # Check it has a non-empty answer (not just a placeholder)
    decision_line=$(grep -n "D-${i}[[:space:]]\|D-${i}:" "$DECISIONS_FILE" | head -1)
    lineno=$(echo "$decision_line" | cut -d: -f1)
    # Check the next 3 lines for content
    content=$(sed -n "$((lineno)),$((lineno+3))p" "$DECISIONS_FILE")
    if echo "$content" | grep -qiE '(TODO|TBD|PLACEHOLDER|pending|\?\?\?)'; then
      echo "DEC-002: D-$i has placeholder value in DECISIONS.md" >&2
      errors=$((errors+1))
    fi
  fi
done

if [ $errors -eq 0 ]; then
  echo "DECISIONS OK: D-1..D-11 all present"
  exit 0
else
  echo "DECISIONS FAIL: $errors checks failed" >&2
  exit 1
fi
