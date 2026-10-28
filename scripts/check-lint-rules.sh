#!/usr/bin/env bash
# Enforces project-level code rules beyond what go vet/staticcheck cover.
# Exit 1 with details if violations found; exit 0 if clean.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAILED=0

# SCR-17: MemoryReceiptSagaStore must not appear outside receipt_saga.go
# (it is dead code kept only for compilation; re-introduction is forbidden)
SAGA_HITS=$(grep -rn 'MemoryReceiptSagaStore' "$ROOT/fiscal-backend/internal" \
  --include='*.go' | grep -v 'receipt_saga\.go' || true)
if [ -n "$SAGA_HITS" ]; then
  echo "SCR-17 VIOLATION: MemoryReceiptSagaStore used outside receipt_saga.go:"
  echo "$SAGA_HITS"
  FAILED=1
fi

# Money safety: no float64/float32/parseFloat in money calculation paths
# Exclusions: 'case float64:' in type switches (safe JSON number handling),
#             'UseNumber()' (prevents float64 parsing, not a violation)
FLOAT_HITS=$(grep -rn 'float64\|float32\|parseFloat\|strconv\.ParseFloat' \
  "$ROOT/fiscal-backend/internal/domain" --include='*.go' \
  | grep -v '_test\.go' \
  | grep -v 'case float64:' \
  | grep -v 'float64(int64(' \
  | grep -v 'UseNumber()' \
  | grep -v '// best-effort:' \
  || true)
if [ -n "$FLOAT_HITS" ]; then
  echo "MONEY_FLOAT VIOLATION: float/parseFloat in domain code:"
  echo "$FLOAT_HITS"
  FAILED=1
fi

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
echo "check-lint-rules: OK"
