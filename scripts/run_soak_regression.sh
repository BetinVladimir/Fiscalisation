#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
report=${1:-/tmp/beeloy-soak-regression.jsonl}
(
  cd "$root/edge-agent"
  GOCACHE=/tmp/beeloy-edge-soak-cache go test -count=1 -json -run '^TestAccelerated' ./journal ./sync
) > "$report"
ruby "$root/scripts/verify_soak_regression.rb" "$report"
