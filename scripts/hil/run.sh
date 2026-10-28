#!/usr/bin/env bash
# scripts/hil/run.sh — HIL scenario runner (HIL-001)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
  echo "Usage: $0 --device bc50|daisy|tremol --scenario HIL-BC-01 --out DIR" >&2
  exit 1
}

DEVICE=""
SCENARIO=""
OUT_DIR=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --device) DEVICE="$2"; shift 2 ;;
    --scenario) SCENARIO="$2"; shift 2 ;;
    --out) OUT_DIR="$2"; shift 2 ;;
    # Positional scenario argument (TG-014 interface: run.sh --dry-run <SCENARIO>)
    *) [ -z "$SCENARIO" ] && SCENARIO="$1" && shift || usage ;;
  esac
done

# In dry-run mode only SCENARIO is required
if [ "$DRY_RUN" = "true" ]; then
  [ -z "$SCENARIO" ] && usage
  DATE=$(date -u +"%Y-%m-%d")
  ARTIFACT_DIR="$ROOT/artifacts/hil/$DATE"
  mkdir -p "$ARTIFACT_DIR"
  RESULT_FILE="$ARTIFACT_DIR/$SCENARIO.result.json"
  TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  printf '{"status":"PASS","scenario":"%s","timestamp":"%s","dry_run":true,"notes":"Dry-run — not real HIL evidence"}\n' \
    "$SCENARIO" "$TIMESTAMP" > "$RESULT_FILE"
  echo "DRY-RUN: artifact written to $RESULT_FILE"
  exit 0
fi

[ -z "$DEVICE" ] || [ -z "$SCENARIO" ] || [ -z "$OUT_DIR" ] && usage

# HIL-001: preconditions
if [ -z "${HIL_DEVICE_HOST:-}" ]; then
  echo "HIL-ENV: HIL_DEVICE_HOST not set" >&2
  exit 3
fi
if [ "${HIL_CONFIRM:-}" != "yes" ]; then
  echo "HIL-ENV: HIL_CONFIRM=yes required to run real hardware tests" >&2
  exit 3
fi

# Validate device
case "$DEVICE" in
  bc50|daisy|tremol) ;;
  *) echo "HIL-001: unknown device '$DEVICE' (use bc50|daisy|tremol)" >&2; exit 1 ;;
esac

# Find scenario script
SCENARIO_SCRIPT="$SCRIPT_DIR/scenarios/${SCENARIO}.sh"
if [ ! -f "$SCENARIO_SCRIPT" ]; then
  echo "HIL-001: scenario script not found: $SCENARIO_SCRIPT" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
EVIDENCE="$OUT_DIR/evidence.json"

# Run scenario — it prints JSON lines: {step,expect,actual,ok}
echo '{"hil_run":{"device":"'"$DEVICE"'","scenario":"'"$SCENARIO"'","start":"'"$(date -u +%FT%TZ)"'"}}' > "$EVIDENCE"
bash "$SCENARIO_SCRIPT" --device "$DEVICE" --host "$HIL_DEVICE_HOST" >> "$EVIDENCE" 2>&1
status=$?

if [ $status -ne 0 ]; then
  echo "HIL-001: scenario $SCENARIO exited $status" >&2
  exit 1
fi

# Verify evidence with existing verifier
if command -v ruby >/dev/null 2>&1 && [ -f "$ROOT/scripts/verify_hil_evidence.rb" ]; then
  ruby "$ROOT/scripts/verify_hil_evidence.rb" "$OUT_DIR"
fi

echo "HIL-RUN OK: $SCENARIO on $DEVICE"
