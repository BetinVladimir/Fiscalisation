#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_root=${SOURCE_ROOT:-$root}
cjson=${CJSON_SOURCE_DIR:-$HOME/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-/opt/homebrew/opt/openssl@3}
out=${STORAGE_TEST_OUT:-${TMPDIR:-/tmp}/beefiscal-storage-host}
mkdir -p "$out"
cc -I"$cjson" -c "$cjson/cJSON.c" -o "$out/cJSON.o"
c++ -std=c++17 -I"$root/tests/storage_host" -I"$source_root/main" -I"$cjson" -I"$openssl/include" "$source_root/main/durable_storage.cpp" "$source_root/main/intent_processor.cpp" "$root/tests/storage_host/runtime_storage_test.cpp" "$out/cJSON.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/runtime_storage_test"
for mode in checkpoint outbox receipt payment collision normal cards; do "$out/runtime_storage_test" "$mode" "$out/$mode.sqlite"; done
