# Provisioning / BlueCash integrated evidence — 2026-10-18

Base commit: e66b4f7 plus the source snapshot recorded in SOURCE_SHA256SUMS. Build hashes identify local artifacts; binaries are not committed. This is a software verification snapshot, not hardware or НАП acceptance.

- Full `make iot-test`: exit 0, including mandatory actual provisioning host runner, recovery and vendor codec suites.
- Go issuer → production S3 crypto/NVS installer → signed proof → Go verification: PASS.
- BeeFiscalApp: 24 unit tests; TypeScript check: exit 0; actual BLE host: PASS.
- Go → production Kotlin canonical JSON: 10660 vectors PASS.
- Backend domain/MQTT/API: exit 0.
- BlueCash Android: 43 unit tests, assembleProdDebug: exit 0.
- ESP32-S3 target build: exit 0; RAM 56636/327680; flash 1978268/3145728.

Full gate previously exposed missing provisioningseal dependencies in isolated oracle copies and a stale startup source guard; both were corrected before the successful run. The startup guard verifies setup returns before operational ingress; behavioral crypto/NVS replay and failure coverage runs separately in the mandatory native installer suite. This does not prove a physical boot/power-cut scenario.

Keys remain in ESP32-S3 NVS; ATECC608A is not used. MiniPOS delegated execution, BluePad purchase correlation, physical revocation, hardware Android/iOS USB/COM/BLE and applicable НАП evidence remain open.
