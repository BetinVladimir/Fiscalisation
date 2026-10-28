#!/usr/bin/env bash
# check-ops-docs.sh — operational runbooks completeness (OPS-001..003)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OPS_DIR="$ROOT/docs/ops"

errors=0

# OPS-001: ops/ directory exists with at least a runbook
if [ ! -d "$OPS_DIR" ]; then
  echo "OPS-001: docs/ops/ directory not found" >&2
  errors=$((errors+1))
else
  # Required runbooks
  REQUIRED_RUNBOOKS=(
    "incident-response"
    "deployment"
    "rollback"
  )
  for rb in "${REQUIRED_RUNBOOKS[@]}"; do
    if ! find "$OPS_DIR" -name "*${rb}*" -name '*.md' | grep -q .; then
      echo "OPS-001: runbook '$rb' not found in docs/ops/" >&2
      errors=$((errors+1))
    fi
  done
fi

# OPS-002: each runbook has required sections
if [ -d "$OPS_DIR" ]; then
  while IFS= read -r -d '' f; do
    missing=""
    grep -qi 'prerequisites\|предусловия\|требования' "$f" || missing="$missing prerequisites"
    grep -qi 'steps\|шаги\|процедура' "$f" || missing="$missing steps"
    if [ -n "$missing" ]; then
      echo "OPS-002: $f missing sections:$missing" >&2
      errors=$((errors+1))
    fi
  done < <(find "$OPS_DIR" -name '*.md' -print0 2>/dev/null)
fi

# OPS-003: ARCH_21_DELIVERY_PIPELINE.md exists (delivery pipeline)
if [ ! -f "$ROOT/docs/MVP1/ARCH_21_DELIVERY_PIPELINE.md" ]; then
  echo "OPS-003: docs/MVP1/ARCH_21_DELIVERY_PIPELINE.md not found" >&2
  errors=$((errors+1))
fi

if [ $errors -eq 0 ]; then
  echo "OPS-DOCS OK"
  exit 0
else
  echo "OPS-DOCS FAIL: $errors checks failed" >&2
  exit 1
fi
