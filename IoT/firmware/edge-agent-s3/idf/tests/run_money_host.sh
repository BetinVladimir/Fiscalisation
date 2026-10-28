#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
source_root=${SOURCE_ROOT:-$root}
core=${PROTOCOL_SOURCE_ROOT:-$root/../../../protocol-abstraction}
cjson=${CJSON_SOURCE_DIR:-${HOME}/.platformio/packages/framework-espidf/components/json/cJSON}
openssl=${OPENSSL_PREFIX:-$(pkg-config --variable=prefix openssl)}
for tool in "${CC:-cc}" "${CXX:-c++}"; do command -v "$tool" >/dev/null 2>&1 || { echo "money host dependency missing: $tool" >&2; exit 2; }; done
[ -f "$core/src/CommandPayload.cpp" ] && [ -f "$cjson/cJSON.c" ] && [ -f "$openssl/include/openssl/sha.h" ] || { echo "money host dependency missing; set PROTOCOL_SOURCE_ROOT, CJSON_SOURCE_DIR, OPENSSL_PREFIX" >&2; exit 2; }
out=$(mktemp -d "${TMPDIR:-/tmp}/beefiscal-money.XXXXXX")
trap 'rm -rf "$out"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
"${CC:-cc}" -I"$cjson" -c "$cjson/cJSON.c" -o "$out/json.o"
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror=misleading-indentation -I"$root/tests/money_host" -I"$root/tests/sync_host" -I"$root/tests/storage_host" -I"$root/main" -I"$core/include" -I"$cjson" -I"$openssl/include" "$source_root/main/profile_executor.cpp" "$root/main/durable_storage.cpp" "$root/main/profile_orchestrator.cpp" "$root/main/runtime_config.cpp" "$root/main/provisioned_binding.cpp" "$core/src/CommandPayload.cpp" "$root/tests/money_host/money_native.cpp" "$out/json.o" -L"$openssl/lib" -lcrypto -lsqlite3 -o "$out/money"
"$out/money" "$root/tests/money_host/datecs.json" "$root/tests/money_host/daisy.json" "$out/state.sqlite"
