#!/usr/bin/env bash
# check-storno-coverage.sh — verify test count >= STORNO_RULES.md row count
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"

RULES_FILE="$REPO/docs/SUPTO/STORNO_RULES.md"
if [[ ! -f "$RULES_FILE" ]]; then
  echo "ERROR: $RULES_FILE not found" >&2; exit 1
fi

# Count only IMPLEMENTED rows in the STORNO_RULES table
rule_count=$(grep '| SR-' "$RULES_FILE" | grep -c 'IMPLEMENTED' || true)

# Count TestReversalPolicy tests in the Go test file
test_file="$REPO/fiscal-backend/internal/domain/reversal_policy_test.go"
if [[ ! -f "$test_file" ]]; then
  echo "ERROR: $test_file not found" >&2; exit 1
fi
test_count=$(grep -c 'func Test.*Reversal\|func TestReversalPolicy' "$test_file" || true)

echo "STORNO_RULES rows: $rule_count"
echo "ReversalPolicy tests: $test_count"

if [[ "$test_count" -lt "$rule_count" ]]; then
  echo "FAIL: need >= $rule_count tests, have $test_count" >&2
  exit 1
fi

echo "storno_coverage=pass"
