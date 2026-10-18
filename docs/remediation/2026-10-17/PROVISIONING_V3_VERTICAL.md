# Secure S3 onboarding vertical: implementation and acceptance

This is implementation evidence from host tests and compilation, not physical HIL evidence or certification approval.

## Trust and transaction contract

The manufacturing `platform_device` record supplies the enrolled P-256 public key. The assigned tenant, active adapter resource public key and actual DeviceIdentity KID must agree. A BLE-advertised key is never accepted as an enrollment authority. DeviceIdentity keeps its key in ESP32-S3 NVS; this version uses no external ATECC device.

An ADMIN prepares an existing signed schema3 binding through `POST /devices/{device_id}/provisioning-deliveries`. Wi-Fi and fiscal operator passwords travel in the authenticated HTTPS request, never in BLE plaintext. The server returns only an issuer-signed BFP2 AEAD ciphertext, associated binding hash, generation, package hash and delivery ID. No private device key is delivered. The runtime TLS certificate uses the enrolled device key; the firmware creates its PEM representation only in local RAM. MQTT command/ack keys remain derived from the active credential ID, so delivery does not silently rotate the active identity.

BFP2 binds the enrolled key/KID, fresh device nonce, generation, binding hash and issuer. Firmware verifies the issuer signature before ECDH/AES-GCM, verifies the inner canonical schema3 signature and identity, checks the client certificate public key against DeviceIdentity and rejects unknown secret fields. The canonical binding carries configuration references; secret values remain inside ciphertext. The durable state is one encrypted NVS blob. NVS commit/readback failure cannot produce an installation proof from uncommitted RAM state. Failed or replayed installation clears decrypted output.

The actual BLE service is `7b6f0000-7c6d-4c7a-9e4f-424545464953`, control `0001`, info `0005`, installation status `0006`. Provisioning uses ordered BFPF frames, maximum 182 bytes per app write (185 ATT MTU), total packet at most 34,000 bytes. Raw BFPE and the old AUTH_V2/plaintext setWifi/bind stubs are outside this executable path. Default native MTU below 185 is rejected explicitly. Command/session transport remains on its existing characteristics.

Installation does not activate the server route by an untrusted mobile flag. DeviceIdentity signs `beefiscal-provisioning-installed-v1\n{kid}\n{binding_sha256}\n{generation}\n{package_sha256}` with single SHA256, low-S P1363. `POST /devices/{device_id}/provisioning-deliveries/{delivery_id}:confirm` validates that proof against the manufacturing key and exact persisted issuance before applying the binding. `by-proof:confirm` recovers a lost issuance/final ACK after app reload by the same exact subject. An interrupted BLE write is never automatically replayed: reconnect/read/verify/confirm or stop. Installation is distinct from physical fiscal/payment readiness.

## Authority allocation

A newly signed S3 binding reserves its range through `ReserveFiscalUNPRange` using the selected physical fiscal resource's `fiscal_device_number` and fiscal-memory number. Board serial, caller-supplied adapter ranges and FMIN are not UNP serial prefixes. The allocator is shared with central sales. Legacy FM leases remain fenced by `LEGACY_UNP_LEASE_FENCE_REQUIRED`; changing a backend status cannot erase already cached physical authority. A failed later issuance may burn a reserved range but cannot reuse it.

## Verification commands

- `cd fiscal-backend && go test ./internal/domain ./internal/api`
- `cd IoT/firmware/edge-agent-s3/idf && bash tests/run_provisioning_host.sh` (actual SDK mbedTLS/cJSON; `MBEDTLS_SOURCE` and `CJSON_SOURCE` may select installed source roots)
- `cd IoT/firmware/edge-agent-s3/idf && bash tests/run_native_ble.sh`
- `cd IoT/firmware/edge-agent-s3/idf && platformio run`
- `cd BeeFiscalApp && npm test && npx tsc --noEmit`
- `cd BeeFiscalApp && npx jest --config jest.ui.config.cjs --runInBand --modulePathIgnorePatterns '/.store-build/' --runTestsByPath __tests__/provisioning-dialog.test.tsx`

The host installer uses disposable test-only keys and real Go issuer/X.509 + actual firmware crypto/parser, stages NVS writes and injects commit failure, recreates the controller for lost ACK, rejects changed nonce/header/secret/ciphertext and verifies the actual firmware installation signature back in Go. These fixtures are not enrollment, legal or HIL proofs.

## Remaining acceptance work (OPEN)

1. Physical Android/iOS commissioning, negotiated MTU/read fragmentation, packet interruption/disconnect and power-loss NVS checks; Datecs COM/Daisy USB/BluePad BLE readiness after reboot; actual authenticated MQTT/sync cycles.
2. Hardware setup-entry/reset/reconfiguration policy for an already commissioned device. The current vertical installs an unprovisioned adapter and recovers committed installation. It does not claim live operational MQTT raw BFPE reconfiguration succeeds. A higher generation requires deliberate authenticated setup-entry and an explicit physical-authority fence where necessary.
3. Factory enrolled key injection and configured P-256 issuer trust-anchor deployment are external prerequisites. An auto-generated, unenrolled DeviceIdentity is rejected by the server. Do not replace trust anchors with app- or BLE-supplied keys.
4. The separate legacy smart-device activation contract must distinguish adapter board identity from verified fiscal serial. The previous FMIN allocator caller must not be treated as a safe S3 issuance path. Ambiguous embedded device profiles need vendor identity proof; do not infer fiscal serial from board serial.
5. Production BLE policy requires physical setup access controls and hardware validation; unauthenticated readers see only public installation metadata/proof, and only issuer-encrypted packets can install. This does not assert secure radio pairing, device tamper resistance or trusted time from ordinary SNTP.
6. TLS connectivity requires reachable Wi-Fi, CA/issuer deployment and clock synchronization; no TLS validation bypass is permitted. Offline authority expiry remains subject to the separate trusted-time execution contract.
