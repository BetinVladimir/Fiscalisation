#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
: "${MBEDTLS_SOURCE_DIR:?set to reviewed mbedTLS 3.6.7 source}"
: "${MBEDTLS_BUILD_DIR:?set to reviewed native build directory}"
command -v go >/dev/null
command -v cc >/dev/null
command -v python3 >/dev/null
test "$(git -C "$MBEDTLS_SOURCE_DIR" rev-parse HEAD)" = 068ff080b369adfac81509f9b57b2afabaf82dc5
test "$(git -C "$MBEDTLS_SOURCE_DIR/framework" rev-parse HEAD)" = dde0c4a0e448a0552f18817dcea633bb851fd288
git -C "$MBEDTLS_SOURCE_DIR" diff --quiet HEAD --
git -C "$MBEDTLS_SOURCE_DIR/framework" diff --quiet HEAD --
work=$(mktemp -d "${TMPDIR:-/tmp}/fiscal-ble-server-profile.XXXXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
cd "$repo/fiscal-backend"
BLE_PROFILE_SYNTHETIC_INTEROP_DIR="$work" go test ./internal/domain -run '^TestBLEServerProfileIsSeparatePurposeAndEnrolledKeyBound$' -count=1
cc -O2 -I "$MBEDTLS_SOURCE_DIR/include" "$repo/IoT/firmware/edge-agent-s3/idf/tests/ble_server_certificate/verify_profile.c" -o "$work/verify-profile" "$MBEDTLS_BUILD_DIR/library/libmbedx509.a" "$MBEDTLS_BUILD_DIR/library/libmbedcrypto.a"
python3 - "$work" <<'PY'
import pathlib,subprocess,sys
p=pathlib.Path(sys.argv[1]);name=(p/'expected-name').read_text();pin=(p/'expected-spki').read_text()
for cert,server,positive in [('server.pem',name,True),('client-only.pem',name,False),('server.pem','foreign.ble.beefiscal.invalid',False),('wrong-issuer.pem',name,False),('wrong-key.pem',name,False)]:
 r=subprocess.run([str(p/'verify-profile'),str(p/cert),str(p/'ca.pem'),server,pin],capture_output=True,text=True,timeout=10)
 if (r.returncode==0)!=positive:raise SystemExit(f'FAIL {cert} {server}: {r.stdout} {r.stderr}')
 print(f'PASS actual mbedTLS profile case {cert} expected={"accept" if positive else "reject"}')
PY
