#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_root=${SOURCE_ROOT:-$root}
cjson=${CJSON_SOURCE_DIR:-${HOME:-}/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-}
if [ -z "$openssl" ]; then
  if command -v pkg-config >/dev/null 2>&1; then openssl=$(pkg-config --variable=prefix openssl 2>/dev/null || true); fi
  if [ -z "$openssl" ] && command -v brew >/dev/null 2>&1; then openssl=$(brew --prefix openssl@3 2>/dev/null || true); fi
fi
fail() { echo "storage host dependency error: $*" >&2; exit 2; }
command -v "${CC:-cc}" >/dev/null 2>&1 || fail "C compiler ${CC:-cc} unavailable; set CC"
command -v "${CXX:-c++}" >/dev/null 2>&1 || fail "C++ compiler ${CXX:-c++} unavailable; set CXX"
[ -f "$cjson/cJSON.c" ] && [ -f "$cjson/cJSON.h" ] || fail "cJSON source/header unavailable; set CJSON_SOURCE_DIR"
[ -n "$openssl" ] && [ -f "$openssl/include/openssl/sha.h" ] || fail "OpenSSL headers unavailable; set OPENSSL_PREFIX"
[ -f "$source_root/main/durable_storage.cpp" ] && [ -f "$source_root/main/intent_processor.cpp" ] || fail "SOURCE_ROOT must name the IDF runtime root"
# STORAGE_TEST_OUT is a parent directory; every invocation owns a unique child.
parent=${STORAGE_TEST_OUT:-${TMPDIR:-/tmp}}
mkdir -p "$parent"
out=$(mktemp -d "$parent/beefiscal-storage-host.XXXXXX")
trap 'rm -rf "$out"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"${CC:-cc}" -I"$cjson" -c "$cjson/cJSON.c" -o "$out/cJSON.o"
# A small executable probe distinguishes unavailable SQLite/OpenSSL from tests.
printf '%s\n' '#include <sqlite3.h>' '#include <openssl/sha.h>' 'int main(){return sqlite3_libversion_number()<1 || SHA256(0,0,0)==0;}' > "$out/dependency_probe.cpp"
"${CXX:-c++}" -I"$openssl/include" "$out/dependency_probe.cpp" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/dependency_probe" || fail "SQLite/OpenSSL development libraries unavailable; install them or set OPENSSL_PREFIX"
"${CXX:-c++}" -std=c++17 -I"$root/tests/storage_host" -I"$source_root/main" -I"$cjson" -I"$openssl/include" "$source_root/main/durable_storage.cpp" "$source_root/main/intent_processor.cpp" "$source_root/main/go_json.cpp" "$source_root/main/go_wire.cpp" "$root/tests/storage_host/runtime_storage_test.cpp" "$out/cJSON.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/runtime_storage_test"
for mode in currency-validation recovery-json json-result checkpoint outbox receipt payment collision normal cards recovery-outbox recovery-receipt recovery-normal recovery-progress recovery-preserve-cards recovery-storage-error recovery-missing-receipt; do "$out/runtime_storage_test" "$mode" "$out/$mode.sqlite"; done
