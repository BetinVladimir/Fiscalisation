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
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror=misleading-indentation -I"$root/tests/storage_host" -I"$root/main" -I"$cjson" -I"$openssl/include" "$source_root/main/durable_storage.cpp" "$source_root/main/profile_orchestrator.cpp" "$root/tests/saga_host/saga_native.cpp" "$out/json.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/saga"
for mode in ${SAGA_TEST_MODES:-all-approved decline-compensated second-not-sent second-unknown first-declined unverified reversal-unknown fiscal-unknown fiscal-rejected refund-no-original begin-table begin-operation intent-payment intent-operation outcome-payment outcome-operation fiscal-phase fiscal-operation comp-intent-payment comp-intent-operation comp-outcome-payment comp-outcome-operation comp-final-phase comp-final-operation begin-member cas-ownership comp-outcome-member}; do "$out/saga" "$mode" "$out/$mode.sqlite"; done
