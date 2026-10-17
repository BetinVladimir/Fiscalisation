#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
core=${PROTOCOL_SOURCE_ROOT:-$root/../../../protocol-abstraction}
cjson=${CJSON_SOURCE_DIR:-${HOME}/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-$(pkg-config --variable=prefix openssl)}
out=$(mktemp -d "${TMPDIR:-/tmp}/beefiscal-native-saga.XXXXXX")
trap 'rm -rf "$out"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
c++ -std=c++17 -I"$root/tests/native_include" -I"$root/main" "$root/main/receipt_saga.cpp" "$root/tests/receipt_saga_native.cpp" -o "$out/receipt"
"$out/receipt"
cc -I"$cjson" -c "$cjson/cJSON.c" -o "$out/json.o"
c++ -std=c++17 -I"$root/tests/storage_host" -I"$root/main" -I"$core/include" -I"$cjson" -I"$openssl/include" "$root/main/durable_storage.cpp" "$root/main/profile_orchestrator.cpp" "$root/tests/profile_executor_native.cpp" "$core/src/CommandPayload.cpp" "$core/src/FrameCodec.cpp" "$out/json.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/profile"
"$out/profile"
echo "receipt saga native fault matrix PASS"
