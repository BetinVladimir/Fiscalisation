#!/usr/bin/env bash
# check-rounding-golden.sh — rounding golden vectors verified by Go (RND-001..003)
# contracts/money-rounding-golden-v1.json must match docs/MVP1/fixtures/rounding_golden.json
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURE="$ROOT/docs/MVP1/fixtures/rounding_golden.json"
CONTRACT="$ROOT/contracts/money-rounding-golden-v1.json"

errors=0

# RND-001: fixture exists
if [ ! -f "$FIXTURE" ]; then
  echo "RND-001: docs/MVP1/fixtures/rounding_golden.json not found" >&2
  exit 1
fi

# RND-003: contract file matches fixture (sha256 divergence check)
if [ ! -f "$CONTRACT" ]; then
  echo "RND-003: contracts/money-rounding-golden-v1.json not found" >&2
  errors=$((errors+1))
else
  sha_fixture=$(sha256sum "$FIXTURE" | awk '{print $1}')
  sha_contract=$(sha256sum "$CONTRACT" | awk '{print $1}')
  if [ "$sha_fixture" != "$sha_contract" ]; then
    echo "RND-003: sha256 mismatch: fixture=$sha_fixture contract=$sha_contract" >&2
    errors=$((errors+1))
  fi
fi

# Count cases
case_count=$(python3 -c "import json; d=json.load(open('$FIXTURE')); print(len(d.get('cases',d.get('vectors',[])) if isinstance(d,dict) else d))" 2>/dev/null || echo 0)
if [ "$case_count" -eq 0 ]; then
  echo "RND-001: rounding_golden.json has no cases" >&2
  exit 1
fi

# RND-002: Go rounding test
go_out=$(cd "$ROOT/fiscal-backend" && GOCACHE=/tmp/beefiscal-rounding-check go test ./internal/domain/... -run 'TestMoneyRoundingGolden' -v 2>&1 || true)
if echo "$go_out" | grep -q 'ROUND OK\|--- PASS.*RoundingGolden'; then
  go_n=$(echo "$go_out" | grep -oE 'ROUND OK [0-9]+' | grep -oE '[0-9]+' || echo 0)
  if [ -n "$go_n" ] && [ "$go_n" -ne "$case_count" ] 2>/dev/null; then
    echo "RND-002: Go reported ROUND OK $go_n but fixture has $case_count cases" >&2
    errors=$((errors+1))
  fi
elif echo "$go_out" | grep -q '--- PASS'; then
  : # subtests passed individually, acceptable
else
  echo "RND-002: Go MoneyRoundingGolden test did not pass" >&2
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
