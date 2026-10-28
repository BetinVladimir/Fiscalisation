#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
idf="$repo/IoT/firmware/edge-agent-s3/idf"
: "${MBEDTLS_SOURCE_DIR:?reviewed actual mbedTLS source required}"
: "${MBEDTLS_BUILD_DIR:?actual native mbedTLS build required}"
: "${CJSON_SOURCE_DIR:?actual cJSON source required for installed binding paths}"
command -v go >/dev/null
cxx="${CXX:-c++}"
cc="${CC:-cc}"
command -v "$cxx" >/dev/null
command -v "$cc" >/dev/null
test "$(git -C "$MBEDTLS_SOURCE_DIR" rev-parse HEAD)" = 068ff080b369adfac81509f9b57b2afabaf82dc5
test "$(git -C "$MBEDTLS_SOURCE_DIR/framework" rev-parse HEAD)" = dde0c4a0e448a0552f18817dcea633bb851fd288
git -C "$MBEDTLS_SOURCE_DIR" diff --quiet HEAD --
git -C "$MBEDTLS_SOURCE_DIR/framework" diff --quiet HEAD --
work=$(mktemp -d "${TMPDIR:-/tmp}/fiscal-ble-proof.XXXXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
"$cc" -c "$CJSON_SOURCE_DIR/cJSON.c" -I "$CJSON_SOURCE_DIR" -o "$work/cjson.o"
"$cxx" -std=c++17 -Wall -Wextra -Werror -pthread -DBEEFISCAL_BLE_PROOF_HOST_TEST \
 -I "$idf/tests/ble_proof_host" -I "$idf/tests/entropy_host" -I "$idf/tests/provisioning_host" \
 -I "$idf/main" -I "$MBEDTLS_SOURCE_DIR/include" -I "$CJSON_SOURCE_DIR" \
 "$idf/tests/ble_proof_host/proof_native.cpp" "$idf/main/device_identity.cpp" \
 "$idf/main/provisioned_binding.cpp" "$idf/main/encrypted_provisioning.cpp" \
 "$idf/main/provisioning_crypto.cpp" "$idf/main/go_json.cpp" "$work/cjson.o" \
 "$MBEDTLS_BUILD_DIR/library/libmbedx509.a" "$MBEDTLS_BUILD_DIR/library/libmbedcrypto.a" -o "$work/proof-native"
if [ -n "${BLE_CRYPTO_BUILD_MANIFEST:-}" ]; then
 python3 - "$work/proof-native" "$work/cjson.o" <<'PYMANIFEST'
import hashlib,json,os,pathlib,sys
build=pathlib.Path(os.environ['MBEDTLS_BUILD_DIR']);cjson=pathlib.Path(os.environ['CJSON_SOURCE_DIR'])
digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
data={'binary_sha256':digest(pathlib.Path(sys.argv[1])),'compiler':os.environ.get('CXX','c++'),'compile_flags':['-std=c++17','-Wall','-Wextra','-Werror','-pthread','-DBEEFISCAL_BLE_PROOF_HOST_TEST'],'host_test_root_injection':True,'linked_libraries':['libmbedx509.a','libmbedcrypto.a'],'libraries_sha256':{name:digest(build/'library'/name) for name in ['libmbedtls.a','libmbedx509.a','libmbedcrypto.a']},'cjson_object_sha256':digest(pathlib.Path(sys.argv[2])),'cjson_sources_sha256':{name:digest(cjson/name) for name in ['cJSON.c','cJSON.h']}}
pathlib.Path(os.environ['BLE_CRYPTO_BUILD_MANIFEST']).write_text(json.dumps(data,indent=2)+'\n')
PYMANIFEST
fi
cd "$repo/fiscal-backend"
BLE_CERTIFICATE_PROOF_HOST_BINARY="$work/proof-native" go test ./internal/domain -run '^TestBLECertificateActualFirmwareProofInterop$' -count=1 -v -timeout=90s
