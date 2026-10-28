#!/usr/bin/env bash
# check-golden-vectors.sh — golden vendor vectors exist and are reproduced (GLD-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURES="$ROOT/docs/MVP1/fixtures"

errors=0

# GLD-001: required golden vector files exist
REQUIRED_VECTORS=(
  "datecs_golden_vectors.json"
  "daisy_golden_vectors.json"
  "rounding_golden.json"
)
for v in "${REQUIRED_VECTORS[@]}"; do
  if [ ! -f "$FIXTURES/$v" ]; then
    echo "GLD-001: golden vector file not found: docs/MVP1/fixtures/$v" >&2
    errors=$((errors+1))
  fi
done

# GLD-002: each vector file has at least 1 case
for v in "${REQUIRED_VECTORS[@]}"; do
  f="$FIXTURES/$v"
  [ -f "$f" ] || continue
  count=$(python3 -c "import json,sys; d=json.load(open('$f')); k=[k for k in ('cases','vectors','tests') if k in d]; print(len(d[k[0]]) if k else 0)" 2>/dev/null || echo 0)
  if [ "$count" -eq 0 ] 2>/dev/null; then
    echo "GLD-002: golden vector file $v has no cases" >&2
    errors=$((errors+1))
  fi
done

if [ $errors -eq 0 ]; then
  echo "GOLDEN-VECTORS OK"
  exit 0
else
  echo "GOLDEN-VECTORS FAIL: $errors checks failed" >&2
  exit 1
fi
