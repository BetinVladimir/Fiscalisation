#!/bin/sh
# S3FIX-01: compile and run durable_storage_native.cpp host tests.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_root=${SOURCE_ROOT:-$root}
cjson=${CJSON_SOURCE_DIR:-${HOME:-}/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-}
if [ -z "$openssl" ]; then
  if command -v pkg-config >/dev/null 2>&1; then openssl=$(pkg-config --variable=prefix openssl 2>/dev/null || true); fi
  if [ -z "$openssl" ] && command -v brew >/dev/null 2>&1; then openssl=$(brew --prefix openssl@3 2>/dev/null || true); fi
fi
fail() { echo "durable_storage_native dependency error: $*" >&2; exit 2; }
command -v "${CC:-cc}"  >/dev/null 2>&1 || fail "C compiler ${CC:-cc} unavailable"
command -v "${CXX:-c++}" >/dev/null 2>&1 || fail "C++ compiler ${CXX:-c++} unavailable"
[ -f "$cjson/cJSON.c" ] && [ -f "$cjson/cJSON.h" ] || fail "cJSON unavailable; set CJSON_SOURCE_DIR"
[ -n "$openssl" ] && [ -f "$openssl/include/openssl/sha.h" ] || fail "OpenSSL headers unavailable; set OPENSSL_PREFIX"
[ -f "$source_root/main/durable_storage.cpp" ] || fail "SOURCE_ROOT must name the IDF runtime root"

parent=${STORAGE_TEST_OUT:-${TMPDIR:-/tmp}}
mkdir -p "$parent"
out=$(mktemp -d "$parent/beefiscal-ds-native.XXXXXX")
trap 'rm -rf "$out"' EXIT INT TERM

python3 "$root/tests/mbedtls_host.py" "$out" > "$out/mbedtls-paths"
mbedtls_include=$(sed -n '1p' "$out/mbedtls-paths")
mbedtls_library=$(sed -n '2p' "$out/mbedtls-paths")

"${CC:-cc}" -I"$cjson" -c "$cjson/cJSON.c" -o "$out/cJSON.o"
"${CXX:-c++}" -std=c++17 \
  -I"$mbedtls_include" \
  -I"$root/tests/storage_host" \
  -I"$source_root/main" \
  -I"$cjson" \
  -I"$openssl/include" \
  "$source_root/main/durable_storage.cpp" \
  "$source_root/main/intent_processor.cpp" \
  "$source_root/main/execution_authorization.cpp" \
  "$source_root/main/payment_reconciliation.cpp" \
  "$source_root/main/go_json.cpp" \
  "$source_root/main/go_wire.cpp" \
  "$source_root/main/canonical_cbor.cpp" \
  "$root/tests/durable_storage_native.cpp" \
  "$out/cJSON.o" "$mbedtls_library" \
  -L"$openssl/lib" -lcrypto -lsqlite3 \
  -o "$out/durable_storage_native"

for mode in storage-error-stops-io unp-seven-digits restart-no-unp-reuse; do
  "$out/durable_storage_native" "$mode" "$out/$mode.sqlite"
done
