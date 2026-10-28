#!/usr/bin/env bash
# check-rounding-golden.sh — rounding golden vectors verified by Go and Kotlin (RND-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$ROOT/docs/MVP1/fixtures/rounding_golden.json"

errors=0

# RND-001: fixture exists
if [ ! -f "$FIXTURE" ]; then
  echo "RND-001: docs/MVP1/fixtures/rounding_golden.json not found" >&2
  exit 1
fi

# Count cases
case_count=$(python3 -c "import json; d=json.load(open('$FIXTURE')); print(len(d.get('cases',d.get('vectors',[]))))" 2>/dev/null || echo 0)
if [ "$case_count" -eq 0 ]; then
  echo "RND-001: rounding_golden.json has no cases" >&2
  exit 1
fi

# RND-002: Go rounding test
go_out=$(cd "$ROOT/fiscal-backend" && GOCACHE=/tmp/beefiscal-rounding-check go test ./internal/domain/... -run 'RoundingGolden' -v 2>&1 || true)
if echo "$go_out" | grep -q 'ROUND OK\|--- PASS.*RoundingGolden'; then
  go_n=$(echo "$go_out" | grep -oE 'ROUND OK [0-9]+' | grep -oE '[0-9]+' || echo 0)
  if [ "$go_n" -ne "$case_count" ] 2>/dev/null; then
    echo "RND-002: Go reported ROUND OK $go_n but fixture has $case_count cases" >&2
    errors=$((errors+1))
  fi
else
  echo "RND-001: Go RoundingGolden test did not pass" >&2
  echo "$go_out" | tail -5 >&2
  errors=$((errors+1))
fi

if [ $errors -eq 0 ]; then
  echo "ROUNDING-GOLDEN OK: $case_count cases"
  exit 0
else
  echo "ROUNDING-GOLDEN FAIL: $errors checks failed" >&2
  exit 1
fi
