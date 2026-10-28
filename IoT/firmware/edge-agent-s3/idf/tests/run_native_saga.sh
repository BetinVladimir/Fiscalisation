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
python3 "$root/tests/mbedtls_host.py" "$out" > "$out/mbedtls-paths-intent"
mbedtls_include_intent=$(sed -n '1p' "$out/mbedtls-paths-intent")
mbedtls_library_intent=$(sed -n '2p' "$out/mbedtls-paths-intent")
c++ -std=c++17 \
  -I"$mbedtls_include_intent" \
  -I"$root/tests/storage_host" \
  -I"$root/tests/intent_host" \
  -I"$root/main" \
  -I"$cjson" \
  -I"$openssl/include" \
  "$root/main/durable_storage.cpp" \
  "$root/main/intent_processor.cpp" \
  "$root/main/execution_authorization.cpp" \
  "$root/main/payment_reconciliation.cpp" \
  "$root/main/go_json.cpp" \
  "$root/main/go_wire.cpp" \
  "$root/main/canonical_cbor.cpp" \
  "$root/tests/intent_processor_native.cpp" \
  "$out/json.o" "$mbedtls_library_intent" \
  -L"$openssl/lib" -lcrypto -lsqlite3 \
  -o "$out/intent_processor_native"
"$out/intent_processor_native" "$out"
echo "receipt saga native fault matrix PASS"
