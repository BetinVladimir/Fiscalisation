#!/usr/bin/env bash
set -euo pipefail
idf_root="$(cd "$(dirname "$0")/.." && pwd)"
mbedtls_source="${MBEDTLS_SOURCE:-${HOME}/.platformio/packages/framework-espidf/components/mbedtls/mbedtls}"
[[ -f "$mbedtls_source/CMakeLists.txt" ]] || { echo 'Actual mbedTLS source required (MBEDTLS_SOURCE)' >&2; exit 1; }
python3 "$idf_root/tests/test_startup_entropy_source.py"
work="$(mktemp -d "${TMPDIR:-/tmp}/beefiscal-identity-entropy.XXXXXX")"
trap 'rm -rf "$work"' EXIT
cmake -S "$mbedtls_source" -B "$work/mbedtls" -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF > "$work/cmake.log"
cmake --build "$work/mbedtls" --target mbedcrypto --parallel 4 >> "$work/cmake.log" 2>&1
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror -pthread -I "$idf_root/tests/entropy_host" -I "$idf_root/tests/provisioning_host" -I "$idf_root/main" -I "$mbedtls_source/include" "$idf_root/main/device_identity.cpp" "$idf_root/tests/entropy_host/identity_entropy_native.cpp" "$work/mbedtls/library/libmbedcrypto.a" -o "$work/identity_entropy_native"
"$work/identity_entropy_native"
# White-box fixture sets the existing private counter near its real boundary;
# no production test hook or million-signature delay is introduced.
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror -pthread -I "$idf_root/tests/entropy_host" -I "$idf_root/tests/provisioning_host" -I "$idf_root/main" -I "$mbedtls_source/include" "$idf_root/tests/entropy_host/identity_rng_budget_native.cpp" "$work/mbedtls/library/libmbedcrypto.a" -o "$work/identity_rng_budget_native"
"$work/identity_rng_budget_native"
