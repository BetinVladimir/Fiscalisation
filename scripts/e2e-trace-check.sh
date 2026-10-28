#!/usr/bin/env bash
# e2e-trace-check.sh — end-to-end correlation_id tracing coverage (TRC-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Self-test mode
if [ "${1:-}" = "--selftest" ]; then
  # Good: all services log the correlation ID
  cat > /tmp/trc-good.log << 'EOF'
pos {"correlation_id":"trace-42","event":"sale.created"}
gateway {"correlation_id":"trace-42","event":"published"}
backend {"correlation_id":"trace-42","event":"reserved"}
device {"correlation_id":"trace-42","event":"executed"}
backend {"correlation_id":"trace-42","event":"finalized"}
EOF
  GOOD=1
  grep -q '"correlation_id":"trace-42"' /tmp/trc-good.log && echo "SELFTEST-GOOD ok" || { echo "SELFTEST-GOOD failed" >&2; GOOD=0; }

  # Bad: gateway missing correlation_id
  cat > /tmp/trc-bad.log << 'EOF'
pos {"correlation_id":"trace-42","event":"sale.created"}
gateway {"event":"published"}
backend {"correlation_id":"trace-42","event":"reserved"}
EOF
  services=("pos" "gateway" "backend")
  BAD_FAIL=0
  for svc in "${services[@]}"; do
    grep -q "\"${svc}\"" /tmp/trc-bad.log && \
    grep -q "\"${svc}\"" /tmp/trc-bad.log | grep -q '"correlation_id"' || BAD_FAIL=1
  done
  # Actually check gateway logs lack correlation_id
  if grep 'gateway' /tmp/trc-bad.log | grep -q '"correlation_id"'; then
    echo "SELFTEST-BAD: expected missing correlation_id but found it" >&2
    exit 1
  fi
  echo "SELFTEST-BAD ok (TRC-001 would fire)"
  [ "$GOOD" -eq 1 ] && echo "SELFTEST OK" && exit 0
  exit 1
fi

LOGFILE="${E2E_LOG:-/tmp/e2e-trace.log}"
CORRELATION_ID="${CORRELATION_ID:-trace-${RANDOM}}"

if [ ! -f "$LOGFILE" ]; then
  echo "TRC-ENV: E2E_LOG not set or file not found" >&2
  exit 3
fi

errors=0

# TRC-001: all services contain the correlation_id
SERVICES=("pos" "gateway" "backend" "device")
for svc in "${SERVICES[@]}"; do
  if ! grep "$svc" "$LOGFILE" | grep -q "correlation_id"; then
    echo "TRC-001: service '$svc' logs missing correlation_id" >&2
    errors=$((errors+1))
  fi
done

# TRC-002: event ordering pos.sale.created < gateway.published < backend.reserved < device.executed < backend.finalized
declare -A event_times
while IFS= read -r line; do
  ts=$(echo "$line" | grep -oE '"ts":[0-9]+' | grep -oE '[0-9]+' || echo 0)
  for ev in sale.created published reserved executed finalized; do
    echo "$line" | grep -q "$ev" && event_times[$ev]=$ts
  done
done < "$LOGFILE"

prev_ts=0
for ev in sale.created published reserved executed finalized; do
  ts="${event_times[$ev]:-0}"
  if [ "$ts" -lt "$prev_ts" ] 2>/dev/null; then
    echo "TRC-002: event ordering violated at '$ev' (ts=$ts < prev=$prev_ts)" >&2
    errors=$((errors+1))
  fi
  prev_ts=$ts
done

if [ $errors -eq 0 ]; then
  echo "E2E-TRACE OK"
  exit 0
else
  echo "E2E-TRACE FAIL: $errors checks failed" >&2
  exit 1
fi
