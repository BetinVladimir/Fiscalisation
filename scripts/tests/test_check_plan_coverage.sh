#!/usr/bin/env bash
# Self-test for scripts/check_plan_coverage.rb.
# Usage: bash scripts/tests/test_check_plan_coverage.sh
# Exit 0: all assertions pass; Exit 1: at least one failure.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="$ROOT/scripts/check_plan_coverage.rb"
FIXTURES="$ROOT/scripts/tests/fixtures/check_plan_coverage"

passed=0
failed=0

run_case() {
  local label="$1"
  local want_exit="$2"
  local plan_file="$3"
  local coverage_file="$4"

  actual_exit=0
  ruby "$SCRIPT" --plan "$plan_file" --coverage "$coverage_file" > /dev/null 2>&1 || actual_exit=$?

  if [ "$actual_exit" -eq "$want_exit" ]; then
    echo "PASS: $label (exit=$actual_exit)"
    passed=$((passed + 1))
  else
    echo "FAIL: $label (want=$want_exit, got=$actual_exit)"
    failed=$((failed + 1))
  fi
}

run_case "good fixture returns 0" \
  0 \
  "$FIXTURES/good/plan.json" \
  "$FIXTURES/good/plan_coverage.json"

run_case "bad fixture returns 1 (unknown task_id)" \
  1 \
  "$FIXTURES/bad/plan.json" \
  "$FIXTURES/bad/plan_coverage.json"

echo "---"
echo "Results: $passed passed, $failed failed"

if [ "$failed" -gt 0 ]; then
  exit 1
fi
exit 0
