#!/usr/bin/env bash
# check-nap-baseline.sh — NAP regulation freeze baseline (NAP-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE="$ROOT/docs/MVP1/fixtures/nap_baseline.json"
SUPTO_DIR="$ROOT/docs/SUPTO"

errors=0

# NAP-001: baseline freeze file exists
if [ ! -f "$BASELINE" ]; then
  echo "NAP-001: docs/MVP1/fixtures/nap_baseline.json not found" >&2
  errors=$((errors+1))
fi

# NAP-002: SUPTO normative docs directory exists with at least one document
if [ ! -d "$SUPTO_DIR" ]; then
  echo "NAP-002: docs/SUPTO/ directory not found" >&2
  errors=$((errors+1))
else
  count=$(find "$SUPTO_DIR" -name '*.md' -o -name '*.pdf' | wc -l | tr -d ' ')
  if [ "$count" -eq 0 ]; then
    echo "NAP-002: no normative documents in docs/SUPTO/" >&2
    errors=$((errors+1))
  fi
fi

# NAP-002b: baseline has freeze_date and version fields
if [ -f "$BASELINE" ]; then
  if ! python3 -c "import json,sys; d=json.load(open('$BASELINE')); assert 'freeze_date' in d or 'version' in d" 2>/dev/null; then
    echo "NAP-002: nap_baseline.json missing freeze_date or version" >&2
    errors=$((errors+1))
  fi
fi

if [ $errors -eq 0 ]; then
  echo "NAP-BASELINE OK"
  exit 0
else
  echo "NAP-BASELINE FAIL: $errors checks failed" >&2
  exit 1
fi
