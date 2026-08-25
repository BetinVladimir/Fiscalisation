#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

node --test "$root/tests/e2e/supto-n18/supto-requirements.spec.mjs"

if [ "${SUPTO_CONTRACT_ONLY:-0}" = 1 ]; then
  exit 0
fi

"$root/tests/e2e/full-fiscal/run.sh"

if [ "${STRICT_SUPTO_CERTIFICATION:-0}" = 1 ]; then
  node "$root/tests/e2e/supto-n18/certification-gate.mjs"
fi
