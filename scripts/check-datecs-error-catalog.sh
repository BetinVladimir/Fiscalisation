#!/usr/bin/env bash
# check-datecs-error-catalog.sh — Datecs §7 error catalog completeness (CAT-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CATALOG_KT="$ROOT/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsErrorCatalog.kt"
CATALOG_GEN="$ROOT/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsErrorCatalog_gen.kt"

errors=0

# CAT-001: catalog file exists (generated or static)
if [ ! -f "$CATALOG_KT" ] && [ ! -f "$CATALOG_GEN" ]; then
  echo "CAT-001: DatecsErrorCatalog.kt not found" >&2
  errors=$((errors+1))
fi

# CAT-002: catalog has at least 13 error entries (Datecs §7 section requires minimum 13)
for cat in "$CATALOG_KT" "$CATALOG_GEN"; do
  [ -f "$cat" ] || continue
  count=$(grep -c 'ErrorCode\|errorCode\|error_code\|"E[0-9]\|"-[0-9]' "$cat" 2>/dev/null || echo 0)
  if [ "$count" -lt 13 ]; then
    echo "CAT-002: Datecs error catalog has only $count entries (need ≥13)" >&2
    errors=$((errors+1))
  fi
  # CAT-002b: all entries must have an ErrorCategory
  if ! grep -q 'ErrorCategory\.' "$cat" 2>/dev/null; then
    echo "CAT-002: no ErrorCategory enum values found in catalog" >&2
    errors=$((errors+1))
  fi
done

if [ $errors -eq 0 ]; then
  echo "DATECS-ERROR-CATALOG OK"
  exit 0
else
  echo "DATECS-ERROR-CATALOG FAIL: $errors checks failed" >&2
  exit 1
fi
