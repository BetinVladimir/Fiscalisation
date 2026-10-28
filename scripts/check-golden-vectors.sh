#!/usr/bin/env bash
# check-golden-vectors.sh — golden vendor vectors exist and are reproduced (GLD-001..003)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURES="$ROOT/docs/MVP1/fixtures"
VENDOR="$ROOT/docs/MVP1/vendor"

errors=0

# GLD-001: datecs_golden_vectors.json exists (in vendor/ or fixtures/)
DATECS_VECTORS=""
if [ -f "$VENDOR/datecs_golden_vectors.json" ]; then
  DATECS_VECTORS="$VENDOR/datecs_golden_vectors.json"
elif [ -f "$FIXTURES/datecs_golden_vectors.json" ]; then
  DATECS_VECTORS="$FIXTURES/datecs_golden_vectors.json"
else
  echo "GLD-001: datecs_golden_vectors.json not found in docs/MVP1/vendor/ or docs/MVP1/fixtures/" >&2
  errors=$((errors+1))
fi

# rounding_golden.json in fixtures
if [ ! -f "$FIXTURES/rounding_golden.json" ]; then
  echo "GLD-001: golden vector file not found: docs/MVP1/fixtures/rounding_golden.json" >&2
  errors=$((errors+1))
fi

# GLD-002: datecs_golden_vectors.json has at least 11 request vectors
if [ -n "$DATECS_VECTORS" ]; then
  count=$(python3 -c "import json; d=json.load(open('$DATECS_VECTORS')); print(len([k for k,v in d.items() if 'request_hex' in v]))" 2>/dev/null || echo 0)
  if [ "$count" -lt 11 ] 2>/dev/null; then
    echo "GLD-002: datecs_golden_vectors.json has $count request vectors, need >= 11" >&2
    errors=$((errors+1))
  fi
fi

# GLD-003: Kotlin host test count matches JSON vector count
if [ -n "$DATECS_VECTORS" ]; then
  kt_host="$ROOT/SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt"
  if [ -f "$kt_host" ]; then
    json_count=$(python3 -c "import json; d=json.load(open('$DATECS_VECTORS')); print(len(d))" 2>/dev/null || echo 0)
    # Count "golden cmd" test entries in the Kotlin file
    kt_count=$(grep -c '"golden ' "$kt_host" 2>/dev/null || echo 0)
    if [ "$kt_count" -lt "$json_count" ] 2>/dev/null; then
      echo "GLD-003: Kotlin host has $kt_count golden tests but JSON has $json_count vectors" >&2
      errors=$((errors+1))
    fi
  fi
fi

if [ $errors -eq 0 ]; then
  echo "GOLDEN-VECTORS OK"
  exit 0
else
  echo "GOLDEN-VECTORS FAIL: $errors checks failed" >&2
  exit 1
fi
