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
# Installer verifies the actual Go-issued package, signed schema3, actual X.509,
# encrypted NVS commit boundaries and a firmware-generated installation proof.
cmake --build "$work/mbedtls" --target mbedx509 --parallel 4 >> "$work/cmake.log" 2>&1
cjson_source="${CJSON_SOURCE:-${HOME}/.platformio/packages/framework-espidf/components/json/cJSON}"
[[ -f "$cjson_source/cJSON.c" ]] || { echo 'Actual cJSON source required (CJSON_SOURCE)' >&2; exit 1; }
mkdir "$work/installer"
(cd "$repo_root/fiscal-backend" && PROVISIONING_INSTALLER_VECTOR_DIR="$work/installer" GOCACHE="${GOCACHE:-$work/go-cache}" go test ./internal/domain -run '^TestExportProvisioningInstallerInterop$' -count=1)
"${CC:-cc}" -c "$cjson_source/cJSON.c" -I "$cjson_source" -o "$work/cjson.o"
"${CXX:-c++}" -std=c++17 -Wall -Wextra -Werror -I "$idf_root/tests/provisioning_host" -I "$idf_root/main" -I "$mbedtls_source/include" -I "$cjson_source" "$idf_root/main/provisioning_crypto.cpp" "$idf_root/main/device_identity.cpp" "$idf_root/main/provisioned_binding.cpp" "$idf_root/main/encrypted_provisioning.cpp" "$idf_root/tests/provisioning_host/installer_native.cpp" "$work/cjson.o" "$work/mbedtls/library/libmbedx509.a" "$work/mbedtls/library/libmbedcrypto.a" -o "$work/installer_native"
"$work/installer_native" "$work/installer"
(cd "$repo_root/fiscal-backend" && PROVISIONING_INSTALLER_VECTOR_DIR="$work/installer" GOCACHE="${GOCACHE:-$work/go-cache}" go test ./internal/domain -run '^TestVerifyActualFirmwareInstallProof$' -count=1)
