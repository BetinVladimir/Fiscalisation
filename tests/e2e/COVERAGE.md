# MVP1 Regression Coverage

Updated by REGRESSION-01.1.

## Module suites

| Module | Suite | What it covers |
|--------|-------|----------------|
| `fiscal-backend` | `go test -count=1 ./...` | Domain logic, MQTT client, API handlers, persistence integration, export service, reconciler, payment capability, SUPTO/UNP, device binding, JWT auth, line limits |
| `SmartDevices/bluecash-app` | `gradlew :bluecash-app:testProdDebugUnitTest` | BlueCash BLE protocol, Datecs/Daisy CP1251 frame codec, operation journal, command processor, sale flow (UNKNOWN/STORNO invariants), key rotation |
| `IoT/firmware/edge-agent-s3` | `run_native_serial.sh` | S3 serial exchange (SEQ wrap, SYN, NAK, deadline, status byte), durable storage atomic reservation, profile executor intents |
| `IoT/protocol-abstraction` | `run-tests.sh` | Protocol abstraction host-side emulator tests |
| `POS/pos-app` | `npx jest --no-coverage` | TypeScript POS line validation, webhook status mapping, dispatch, BLE link guard, lock→Fiscal→commit flow |
| `tests/mvp1/` | `pytest tests/mvp1/ -q` | All MVP1 acceptance tests: Go/Kotlin/S3/script/POS/BLE/EMU/CT/VENDOR cards |

## Status legend

- **PASS** — all tests in the suite ran and passed, count > 0
- **BLOCKED** — required tool or env var unavailable (PG_INTEGRATION_URL, gradlew, npx…)
- **FAIL** — at least one test failed, or zero tests reported
