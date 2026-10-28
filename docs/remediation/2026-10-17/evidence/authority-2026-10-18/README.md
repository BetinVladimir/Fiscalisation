# Authority integration evidence — 2026-10-18

Base commit: `9702413`. Working-tree source snapshot: SOURCE_SHA256SUMS.

- `iot.log`: full `make iot-test`, terminal exit 0. Actual ESP-IDF mbedTLS prebuilt library override was used; this library was independently built from installed ESP-IDF source and exercised by the actual crypto verifier. No test was skipped.
- `backend.log`: `go test ./internal/domain ./internal/mqttclient ./internal/api`, terminal exit 0.
- Source manifest covers runtime/test sources, backend internals, MiniPOS wire source/tests, contracts and Makefile. Docs and generated build outputs are excluded.

S3 cross-build completed with exit 0 (`s3-build.log`): RAM 39940/327680, Flash 1659640/3145728. Firmware artifact hashes are recorded in BUILD_SHA256SUMS. Provisioning crypto foundation is present but complete onboarding is not yet integrated. MiniPOS authorization/client proof, physical USB/COM/BLE HIL, trusted clock/rollback, legacy offline lease migration and applicable NAP confirmation remain open. No hardware approval or certification is claimed.

Build log trailing whitespace is normalized; the raw log remains `/private/tmp/fiscal-mqtt-validity-integrated-s3-20261018.log`.
