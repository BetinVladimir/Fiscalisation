#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_root=${SOURCE_ROOT:-$root}
cjson=${CJSON_SOURCE_DIR:-${HOME}/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-$(pkg-config --variable=prefix openssl)}
for tool in "${CC:-cc}" "${CXX:-c++}"; do command -v "$tool" >/dev/null 2>&1 || { echo "saga host dependency missing: $tool" >&2; exit 2; }; done
[ -f "$cjson/cJSON.c" ] && [ -f "$openssl/include/openssl/sha.h" ] || { echo 'saga host dependencies missing; set CJSON_SOURCE_DIR/OPENSSL_PREFIX' >&2; exit 2; }
out=$(mktemp -d "${TMPDIR:-/tmp}/beefiscal-saga.XXXXXX")
trap 'rm -rf "$out"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"${CC:-cc}" -I"$cjson" -c "$cjson/cJSON.c" -o "$out/json.o"
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror=misleading-indentation -I"$root/tests/storage_host" -I"$root/main" -I"$cjson" -I"$openssl/include" "$source_root/main/durable_storage.cpp" "$source_root/main/profile_orchestrator.cpp" "$root/tests/saga_host/recovery_native.cpp" "$out/json.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/saga"
for mode in ${RECOVERY_TEST_MODES:-compensated declined reserved second-unknown-resolved partial-reversed-resume query-race query-approved query-declined query-unknown foreign-query conflict-query query-fault operation-fault reversal-query reversal-unknown reversal-fault comp-resume comp-unknown comp-unverified comp-intent-fault comp-outcome-fault fiscal-proven fiscal-unproven fiscal-conflict legacy-card terminal-owner stale-cas snapshot-fault}; do "$out/saga" "$mode" "$out/$mode.sqlite"; done
