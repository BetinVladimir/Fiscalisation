#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
out=${TMPDIR:-/tmp}/beefiscal-receipt-saga-test
c++ -std=c++17 -I"$root/tests/native_include" -I"$root/main" "$root/main/receipt_saga.cpp" "$root/tests/receipt_saga_native.cpp" -o "$out"
"$out"
profile_out=${TMPDIR:-/tmp}/beefiscal-profile-executor-test
c++ -std=c++17 -I"$root/tests/native_include" -I"$root/main" -I"$root/../../../protocol-abstraction/include" \
 "$root/main/profile_orchestrator.cpp" "$root/tests/profile_executor_native.cpp" \
 "$root/../../../protocol-abstraction/src/CommandPayload.cpp" \
 "$root/../../../protocol-abstraction/src/FrameCodec.cpp" -o "$profile_out"
"$profile_out"
echo "receipt saga native fault matrix PASS"
