# BLE ingress connection fence — 2026-10-21

## Defect and fix

The production receive_command callback checked startup and MTU but did not require the incoming handle to match the active peripheral connection. A valid-shaped BFF1 frame could reach the command sink with no active connection, a foreign handle or a disconnected handle in the native runtime harness. This is a transport admission defect; downstream execution signatures remain a separate mandatory boundary.

The callback now rejects absent active connection and handles different from active_connection before MTU, allocation, frame reassembly and sink dispatch. Test setup for connected frame-size checks now explicitly establishes a connection.

## Acceptance evidence

The added disconnected admission assertion failed before the fix (exit134). After the fix, the actual ble_runtime.cpp native host suite passes, including absent/foreign/disconnected handle rejection, reconnection acceptance, existing MTU/provisioning/startup checks and outbound reconnect-generation fencing. The same suite passes AddressSanitizer/UndefinedBehaviorSanitizer. Logs and source hashes: evidence/ble-ingress-2026-10-21.

## Remaining scope

The BLE stack is mocked, including the native test SHA function; this result does not establish on-air authentication/encryption or signature verification. Cryptographic authority has a separate actual-mbedTLS suite. A reused numerical handle cannot identify an old session by itself; session-bound authenticated encryption, autonomous revocation delivery and complete offline equivalence remain OPEN. Physical Android/iOS BLE HIL is required. ESP32-S3 keys remain internal NVS; no ATECC608 module is required.

Production ESP32-S3 PlatformIO build passes after the fix (`/private/tmp/fiscal-ble-ingress-s3-build-20261021.log`,13.07s). This verifies compilation/linking, not radio behavior.

## BFF1 final-marker and failed-assembly regression

A further red test reproduced premature-final acceptance. Production S3 now requires the marker byte to be0 or1 and final to be true exactly when offset+body reaches total. Early final, missing final and invalid marker are rejected. Any rejected parsed/flattened frame clears its pending assembly; a later continuation cannot revive it. A new offset0 frame can recover. Existing command admission and reconnect tests remain in the suite. The expanded suite passes AddressSanitizer/UndefinedBehaviorSanitizer (`/private/tmp/fiscal-ble-final-green-20261021.log`). This matches the existing client BFF1 marker invariant. Crypto/radio limitations above still apply.

Final expanded-runtime evidence and source hashes: evidence/ble-final-2026-10-21; S3 build PASS14.27s. The earlier ingress-only evidence remains a historical snapshot.

## ATT-length rejection follow-up

A further regression reproduced that an invalid-length ATT write returned an error without discarding a previously buffered fragment. The active-connection length-error branch now clears payload/total. A test submits a first fragment, then a header-only invalid write, then a valid-looking continuation and requires zero additional sink calls. Red log:`/private/tmp/fiscal-ble-length-red-20261021.log`; expanded sanitizer suite PASS:`/private/tmp/fiscal-ble-length-green-20261021.log`.

The concurrently running full IoT rehearsal compiled the new red test before this follow-up fix and stopped on that assertion. It is not a passing full-suite result. A fresh full IoT run was started after the fix with log:`/private/tmp/fiscal-ble-length-full-iot-20261021.log`; acceptance pending its terminal status. Runtime/tests will remain unchanged during that run.

Final follow-up acceptance: fresh `make iot-test` completed exit0 on the unchanged runtime/test snapshot; S3 build completed exit0 (94.34s). Versioned logs and source hashes: evidence/ble-length-2026-10-21. This supersedes the pending full-IoT statement. A read-only serial inventory found only debug-console and Bluetooth-Incoming-Port; no USB/COM fixture was available for HIL.
