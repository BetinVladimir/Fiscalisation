#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
evidence=${MVP1_EVIDENCE_DIR:-/tmp/beefiscal-mvp1-software-evidence}
mkdir -p "$evidence"
if [ "${MVP1_GATE_INNER:-0}" != 1 ]; then
  rm -f "$evidence/status.txt"
  for run in 1 2; do
    run_evidence="$evidence/run-$run"
    mkdir -p "$run_evidence"
    if ! MVP1_GATE_INNER=1 MVP1_EVIDENCE_DIR="$run_evidence" "$0" >"$run_evidence/gate.log" 2>&1; then
      tail -n 200 "$run_evidence/gate.log" >&2
      echo "MVP1 SOFTWARE GATE FAILED on clean run $run" >&2
      exit 1
    fi
  done
  find "$evidence" -type f ! -name SHA256SUMS -exec shasum -a 256 {} \; | sort >"$evidence/SHA256SUMS"
  printf '%s\n' PASS >"$evidence/status.txt"
  echo "MVP1 software gate PASS twice; evidence: $evidence"
  exit 0
fi
{
  date -u '+generated_at=%Y-%m-%dT%H:%M:%SZ'
  git rev-parse HEAD 2>/dev/null || true
  go version
  node --version
  ruby --version
  docker --version
  "${PLATFORMIO_BIN:-$HOME/.platformio/penv/bin/platformio}" --version
  java -version 2>&1 || true
  (cd minipos/BeeMiniPOS/android && ./gradlew --version) || true
} >"$evidence/toolchain.txt"
git status --porcelain=v1 >"$evidence/source-status.txt"
{ git ls-files -co --exclude-standard -z | sort -z | while IFS= read -r -d '' source; do test -f "$source" && shasum -a 256 "$source"; done; } >"$evidence/source-SHA256SUMS"
cp contracts/CONTRACT_LOCK.md contracts/openapi-runtime-v1.yaml contracts/mvp1-simulated-profile-evidence.json "$evidence/"
cp ../BeeloyBackend/docs/Fiscal/events/asyncapi-device-v1.yaml "$evidence/asyncapi-device-v1.yaml"
cp minipos/BeeMiniPOS/package-lock.json BeeFiscalApp/package-lock.json fiscal-backend/go.mod fiscal-backend/go.sum "$evidence/" 2>/dev/null || true

python3 -m unittest discover -s IoT/firmware/edge-agent-s3/idf/tests -p 'test_*.py'
ruby -rjson -e 'v=JSON.parse(File.read("contracts/mvp1-simulated-profile-evidence.json")); expected=%w[DATECS_BLUECASH50_EMBEDDED DATECS_DP150_BLUEPAD50 DAISY_COMPACT_S01]; abort "profile evidence manifest incomplete" unless v["evidence_state"]=="SIMULATED" && v["profiles"].map{|p|p["profile"]}.sort==expected.sort'
make iot-test
make test
make contract-test
make typecheck
make ui-acceptance-test
make fault-regression-test soak-regression-test security-test
make ui-interaction-test
make smart-device-test
make postgres-integration
make compose-check compose-e2e

(cd fiscal-backend && GOCACHE=${GOCACHE:-/tmp/beefiscal-gate-go-cache} go test -json ./... >"$evidence/fiscal-backend-tests.json")
(cd minipos/beeminipos-backend && GOCACHE=${GOCACHE:-/tmp/beefiscal-gate-go-cache} go test -json ./... >"$evidence/minipos-backend-tests.json")
(cd minipos/BeeMiniPOS && npm test >"$evidence/minipos-tests.tap")
(cd BeeFiscalApp && npm test >"$evidence/beefiscalapp-tests.tap")
(cd fiscal-backend && GOCACHE=${GOCACHE:-/tmp/beefiscal-gate-go-cache} go test -coverprofile="$evidence/fiscal-backend.coverage" ./... >/dev/null)
(cd minipos/beeminipos-backend && GOCACHE=${GOCACHE:-/tmp/beefiscal-gate-go-cache} go test -coverprofile="$evidence/minipos-backend.coverage" ./... >/dev/null)
(cd minipos/BeeMiniPOS/android && ./gradlew -p ../../../SmartDevices :bluecash-app:testDebugUnitTest --no-daemon --rerun-tasks --tests '*BlueCashEngineTest*' --tests '*BlueCashComplianceIntentTest*')
cp -R SmartDevices/bluecash-app/build/test-results/testDebugUnitTest "$evidence/bluecash-junit"

if [ -x "${PLATFORMIO_BIN:-$HOME/.platformio/penv/bin/platformio}" ]; then
  pio="${PLATFORMIO_BIN:-$HOME/.platformio/penv/bin/platformio}"
  "$pio" run -d IoT/firmware/edge-agent-s3/idf -t clean
  "$pio" run -d IoT/firmware/edge-agent-s3/idf
  test -s IoT/firmware/edge-agent-s3/idf/.pio/build/edge-agent-s3-idf/firmware.elf
  cp IoT/firmware/edge-agent-s3/idf/.pio/build/edge-agent-s3-idf/firmware.elf "$evidence/firmware.elf"
  cp IoT/firmware/edge-agent-s3/idf/.pio/build/edge-agent-s3-idf/firmware.bin "$evidence/firmware.bin"
else
  echo "MVP1 SOFTWARE GATE FAILED: pinned PlatformIO ESP-IDF toolchain is not installed" >&2
  exit 78
fi

if rg -n 'ESP_ERR_NOT_FINISHED|PendingPhysicalExecutor|BLUEPAD_RESPONSE_PENDING' \
  IoT/firmware/edge-agent-s3/idf/main; then
  echo "MVP1 SOFTWARE GATE FAILED: executable firmware contains a P0 placeholder" >&2
  exit 1
fi

test -s SmartDevices/bluecash-app/build/outputs/apk/release/bluecash-app-release-unsigned.apk
test -s SmartDevices/daisy-smart-app/build/outputs/apk/release/daisy-smart-app-release-unsigned.apk
cp SmartDevices/bluecash-app/build/outputs/apk/release/bluecash-app-release-unsigned.apk "$evidence/bluecash-app-release-unsigned.apk"
cp SmartDevices/daisy-smart-app/build/outputs/apk/release/daisy-smart-app-release-unsigned.apk "$evidence/daisy-smart-app-release-unsigned.apk"
for required in toolchain.txt source-status.txt source-SHA256SUMS CONTRACT_LOCK.md openapi-runtime-v1.yaml asyncapi-device-v1.yaml mvp1-simulated-profile-evidence.json fiscal-backend-tests.json minipos-backend-tests.json minipos-tests.tap beefiscalapp-tests.tap fiscal-backend.coverage minipos-backend.coverage firmware.elf firmware.bin bluecash-app-release-unsigned.apk daisy-smart-app-release-unsigned.apk; do
  test -s "$evidence/$required" || { echo "missing required evidence: $required" >&2; exit 1; }
done
test -d "$evidence/bluecash-junit" && find "$evidence/bluecash-junit" -name '*.xml' -type f | grep -q . || { echo "missing BlueCash JUnit evidence" >&2; exit 1; }
printf '%s\n' PASS >"$evidence/status.txt"

echo "MVP1 software gate PASS; evidence: $evidence"
