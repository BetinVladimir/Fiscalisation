#!/usr/bin/env bash
# check-rabbit-topology.sh — RabbitMQ topology matches code (RBT-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
POS_GW="${ROOT}/../POS/pos-gateway"
DEPLOY_DIR="$ROOT/deploy"

errors=0

# RBT-001: collect exchange/queue names from code and deploy definitions
CODE_QUEUES=()
if [ -d "$POS_GW" ]; then
  while IFS= read -r name; do
    CODE_QUEUES+=("$name")
  done < <(rg -oh '"[a-z][a-z0-9._-]*\.(queue|exchange|events|results)"' "$POS_GW" 2>/dev/null \
           | tr -d '"' | sort -u)
fi

DEPLOY_QUEUES=()
if [ -d "$DEPLOY_DIR" ]; then
  while IFS= read -r name; do
    DEPLOY_QUEUES+=("$name")
  done < <(rg -oh '"[a-z][a-z0-9._-]*\.(queue|exchange|events|results)"' "$DEPLOY_DIR" 2>/dev/null \
           | tr -d '"' | sort -u)
fi

# Check for queues in code not in deploy
for q in "${CODE_QUEUES[@]}"; do
  found=0
  for dq in "${DEPLOY_QUEUES[@]}"; do
    [ "$q" = "$dq" ] && found=1 && break
  done
  if [ $found -eq 0 ] && [ ${#DEPLOY_QUEUES[@]} -gt 0 ]; then
    echo "RBT-001: queue '$q' in code but not in deploy definitions" >&2
    errors=$((errors+1))
  fi
done

# RBT-002: each result queue has a corresponding DLQ
for q in "${DEPLOY_QUEUES[@]}"; do
  if echo "$q" | grep -q '\.results'; then
    dlq="${q%.results}.dlq"
    found=0
    for dq in "${DEPLOY_QUEUES[@]}"; do
      [ "$dq" = "$dlq" ] && found=1 && break
    done
    if [ $found -eq 0 ]; then
      echo "RBT-002: result queue '$q' has no DLQ '$dlq'" >&2
      errors=$((errors+1))
    fi
  fi
done

if [ $errors -eq 0 ]; then
  echo "RABBIT-TOPOLOGY OK"
  exit 0
else
  echo "RABBIT-TOPOLOGY FAIL: $errors checks failed" >&2
  exit 1
fi
