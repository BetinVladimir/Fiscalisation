#!/usr/bin/env bash
# Finds _ = or _, _ = patterns in domain code without a // best-effort: comment.
# Exit 0 if clean; exit 1 with details if unannotated swallowed errors are found.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAILED=0

# Check Go domain package for swallowed errors without justification
HITS=$(grep -rn '^\s*\(_, \)\?_ = ' "$ROOT/fiscal-backend/internal/domain" \
  --include='*.go' \
  | grep -v '_test\.go' \
  | grep -v '// best-effort:' \
  || true)

if [ -n "$HITS" ]; then
  echo "SWALLOWED_ERRORS: unannotated _ = in fiscal-backend/internal/domain:"
  echo "$HITS"
  echo ""
  echo "Add '// best-effort: <reason>' comment to each line, or propagate the error."
  FAILED=1
fi

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
echo "check-swallowed-errors: OK"
