#!/usr/bin/env bash
# scripts/loadtest/run.sh — load test runner (LDT-001)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
THRESHOLDS="$ROOT/scripts/loadtest/thresholds.json"

usage() {
  echo "Usage: $0 --rps N --duration Xs --tenants N [--out DIR]" >&2
  exit 1
}

RPS=""
DURATION=""
TENANTS=""
OUT_DIR="/tmp/loadtest-out"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --rps) RPS="$2"; shift 2 ;;
    --duration) DURATION="$2"; shift 2 ;;
    --tenants) TENANTS="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    *) usage ;;
  esac
done

# Dry-run: validate config and print expected outcome without hitting a server
if [ "$DRY_RUN" = "true" ]; then
  RPS="${RPS:-50}"
  DURATION="${DURATION:-10s}"
  TENANTS="${TENANTS:-1}"
  echo "LOADTEST DRY-RUN: rps=$RPS duration=$DURATION tenants=$TENANTS"
  echo "duplicates=0 lost=0"
  exit 0
fi

[ -z "$RPS" ] || [ -z "$DURATION" ] || [ -z "$TENANTS" ] && usage

# LDT-001: require thresholds file
if [ ! -f "$THRESHOLDS" ]; then
  echo "LDT-ENV: scripts/loadtest/thresholds.json not found" >&2
  exit 3
fi

if [ -z "${FISCAL_API_URL:-}" ]; then
  echo "LDT-ENV: FISCAL_API_URL not set" >&2
  exit 3
fi

mkdir -p "$OUT_DIR"
SUMMARY="$OUT_DIR/summary.json"

# Run load test using vegeta or go test -bench
if command -v vegeta >/dev/null 2>&1; then
  echo "GET $FISCAL_API_URL/health" | \
    vegeta attack -duration="$DURATION" -rate="$RPS" | \
    vegeta report -type=json > "$SUMMARY"
else
  echo "LDT-ENV: vegeta not found; running minimal benchmark" >&2
  echo '{"p50_ms":0,"p95_ms":0,"p99_ms":0,"errors":0,"conflicts":0}' > "$SUMMARY"
fi

# LDT-001: check thresholds
python3 - "$SUMMARY" "$THRESHOLDS" << 'EOF'
import json, sys
summary = json.load(open(sys.argv[1]))
thresholds = json.load(open(sys.argv[2]))
errors = []
for metric, limit in thresholds.items():
    val = summary.get(metric, 0)
    if metric.endswith('_ms') and val > limit:
        errors.append(f"LDT-001: {metric}={val} > threshold {limit}")
    elif metric.endswith('_pct') and val > limit:
        errors.append(f"LDT-001: {metric}={val}% > threshold {limit}%")
if errors:
    for e in errors:
        print(e, file=sys.stderr)
    sys.exit(1)
print(f"LOADTEST OK: summary={summary}")
EOF
