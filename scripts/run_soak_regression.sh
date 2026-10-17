#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
# No S3 accelerated 72h/7d harness exists yet. Do not relabel deleted-runtime
# report lines or short host tests as equivalent soak evidence.
exec ruby "$root/scripts/verify_soak_regression.rb" "${1:-/tmp/beeloy-soak-regression.jsonl}"
