#!/usr/bin/env bash
# check-test-matrix.sh — Validate docs/MVP1/test-matrix.tsv
# Exit 0: valid; Exit 1: structural errors found
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MATRIX="$ROOT/docs/MVP1/test-matrix.tsv"

if [[ ! -f "$MATRIX" ]]; then
    echo "FAIL: test-matrix.tsv not found at $MATRIX"
    exit 1
fi

ERRORS=0

# Required columns in header
REQUIRED_COLS="module test_command cwd wave status"
HEADER=$(head -1 "$MATRIX")
for col in $REQUIRED_COLS; do
    if ! echo "$HEADER" | grep -qw "$col"; then
        echo "FAIL: missing column '$col' in header"
        ERRORS=$((ERRORS+1))
    fi
done

# Required modules (canonical from ARCH_21 §6)
REQUIRED_MODULES="fiscal-backend s3 protocols smartdevices pos minipos e2e"
for mod in $REQUIRED_MODULES; do
    if ! grep -qw "^${mod}" "$MATRIX" 2>/dev/null && ! awk -F'\t' 'NR>1{print $1}' "$MATRIX" | grep -qw "^${mod}$"; then
        echo "FAIL: canonical module '$mod' not found in test-matrix.tsv"
        ERRORS=$((ERRORS+1))
    fi
done

# Each data row must have at least 5 tab-separated fields
ROW_NUM=1
while IFS=$'\t' read -r module test_command cwd wave status rest; do
    ROW_NUM=$((ROW_NUM+1))
    [[ "$module" == "module" ]] && continue  # skip header if re-read
    if [[ -z "$module" || -z "$test_command" || -z "$cwd" || -z "$wave" || -z "$status" ]]; then
        echo "FAIL: row $ROW_NUM has empty required field(s): module='$module' cwd='$cwd' wave='$wave' status='$status'"
        ERRORS=$((ERRORS+1))
    fi
done < <(tail -n +2 "$MATRIX")

if [[ $ERRORS -eq 0 ]]; then
    MODULE_COUNT=$(tail -n +2 "$MATRIX" | wc -l | tr -d ' ')
    echo "OK: test-matrix.tsv is valid ($MODULE_COUNT module rows)"
    exit 0
else
    echo "FAIL: $ERRORS error(s) found in test-matrix.tsv"
    exit 1
fi
