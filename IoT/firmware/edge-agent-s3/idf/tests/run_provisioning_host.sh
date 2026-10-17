#!/usr/bin/env bash
set -euo pipefail
idf_root="$(cd "$(dirname "$0")/.." && pwd)"
repo_root="$(cd "$idf_root/../../../.." && pwd)"
mbedtls_source="${MBEDTLS_SOURCE:-${HOME}/.platformio/packages/framework-espidf/components/mbedtls/mbedtls}"
[[ -f "$mbedtls_source/CMakeLists.txt" ]] || { echo 'Actual mbedTLS source required (MBEDTLS_SOURCE)' >&2; exit 1; }
work="$(mktemp -d "${TMPDIR:-/tmp}/beefiscal-provisioning.XXXXXX")"
trap 'rm -rf "$work"' EXIT
mkdir "$work/vectors"
cmake -S "$mbedtls_source" -B "$work/mbedtls" -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF > "$work/cmake.log"
cmake --build "$work/mbedtls" --target mbedcrypto --parallel 4 >> "$work/cmake.log" 2>&1
(cd "$repo_root/fiscal-backend" && PROVISIONING_VECTOR_DIR="$work/vectors" GOCACHE="${GOCACHE:-$work/go-cache}" go test ./internal/provisioningseal)
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror -I "$idf_root/tests/provisioning_host" -I "$idf_root/main" -I "$mbedtls_source/include" "$idf_root/main/provisioning_crypto.cpp" "$idf_root/main/device_identity.cpp" "$idf_root/tests/provisioning_host/provisioning_native.cpp" "$work/mbedtls/library/libmbedcrypto.a" -o "$work/provisioning_native"
"$work/provisioning_native" "$work/vectors"
