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
