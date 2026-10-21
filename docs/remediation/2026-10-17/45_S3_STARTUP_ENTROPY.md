# ESP32-S3 startup entropy and persistent identity

Status: software fix prepared and tested; existing-device key provenance and physical startup remain separate acceptance gates. ATECC608 is unused. Persistent identity remains inside ESP32-S3 NVS.

## Finding

`app_main.cpp` opened DeviceIdentity before RF initialization. `device_identity.cpp` supplied `esp_fill_random` directly to P-256 key generation, stored-key multiplication blinding and signing. The [ESP-IDF 5.4 ESP32-S3 RNG guide](https://docs.espressif.com/projects/esp-idf/en/v5.4/esp32s3/api-reference/system/random.html) requires an active primary entropy source for its true-random guarantee. Application startup had no demonstrated source enablement after bootloader handoff. This is a missing guarantee, not evidence that any particular existing key is predictable or compromised.

## Change

- `startup_entropy.h` scopes `bootloader_random_enable/disable` around the actual `DeviceIdentity.open()` call before profile, RF and network initialization. RAII closes the scope on success and failure. CMake explicitly requires bootloader_support.
- The identity owns a mbedTLS CTR-DRBG, seeded once inside that scope. New-key generation and stored-key blinding use it. Later ECDSA/provisioning proof/ECDH use that same DRBG, so crypto tasks started before BLE do not toggle the SAR entropy source or read unconditioned hardware randomness.
- Prediction resistance is off and automatic reseed interval is INT_MAX. An explicit lower limit of 2^20 RNG requests per object/boot fails closed before the automatic reseed threshold; a reboot obtains a fresh conditioned seed. Reaching this limit does not erase/regenerate the persistent identity or automatically replay any business operation. Operational health/error handling must treat a crypto failure as unavailable/uncertain and use existing journal recovery. Future live reseed requires a separately verified entropy-availability policy; do not add unconditional hardware reseeds.
- A mutex serializes access to the shared P-256 key/DRBG. Native destructors release/zeroize library contexts. No private key/seed is logged or exported by this change.

The existing deterministic crypto self-test precedes this initialization and does not generate a key. Setup nonce generation is a BLE information callback after the controller starts. Sync UUID/backoff randomness is not used as the identity signing RNG; its independent uniqueness behavior is not newly certified here.

## Automated evidence

`idf/tests/run_identity_entropy_host.sh` compiles the actual DeviceIdentity code with actual mbedTLS 3.6.2 and executes:

- true-source prerequisite/order through the actual startup guard;
- first-boot key generation and one durable NVS write;
- exact public-key/scalar preservation on reload;
- stored-zero-scalar rejection without replacement;
- NVS commit failure with entropy cleanup;
- 11,000 signatures before RF, crossing the library's default reseed request interval while asserting no subsequent hardware entropy call;
- simultaneous signatures from four host threads;
- the actual last allowed DRBG request and the next rejected request, public signing/proof failure with cleared output, no hidden hardware reseed/rekey, and recovery through a freshly startup-seeded object preserving the durable identity;
- later signing with simulated active RF, without toggling startup entropy.

The budget boundary fixture includes the actual production implementation and exposes its existing private counter in that test translation unit only. It positions the counter near 2^20 to execute the real callback boundary without a million signatures; no production setter/test hook is added.

The baseline actual identity code aborts the same test when signing outside the guarded startup interval: its direct hardware callback violates the prerequisite. Source gates verify the real app-main wiring and CMake dependency, complementing the executable guard test. The host entropy fixture does not prove target silicon noise quality.

The runner is included in `make iot-test`. Existing Go issuer → real mbedTLS device decryption and installer/install-proof interop remains green. The actual ESP32-S3 IDF build remains mandatory; firmware image/source hashes identify the tested change.

## Existing identity provenance and physical acceptance

The fix preserves every valid stored scalar. It does not retrospectively establish how an enrolled key was originally generated. Do not delete NVS or silently issue a replacement key to hide this gap.

For existing installations, record manufacturing/enrollment identity fingerprint, creating firmware/build and available provisioning evidence. If sufficient provenance cannot be established, mark the identity's provenance unresolved and plan an explicitly authorized maintenance/re-enrollment rotation: verify the new device proof, update the exact root device record/binding at a higher generation, retire the old credential and preserve audit history. Do not publish or extract private scalars for this assessment.

Physical acceptance must observe startup entropy enable/disable before RF/ADC access, first boot and retained-key boot, power failure during initial commit, unchanged identity after normal upgrade/restart, and actual signing/provisioning with BLE/Wi-Fi unavailable. Use disposable test devices for intentional identity erasure. Statistical output tests alone do not prove entropy strength. Existing-device provenance, secure-boot/flash/NVS protections and authenticated BLE channel readiness remain separately assessed; this change does not claim their completion.

## Integrated evidence (2026-10-21)

[Manifest and bounded logs](evidence/s3-startup-entropy-2026-10-21/manifest.json) record exact source and image hashes. Root full `make iot-test` PASS ran with the final production change before the additional budget-boundary fixture; the final extended entropy runner separately PASS. Actual target build PASS: static RAM 56,716/327,680 bytes; flash 2,000,820/3,145,728 bytes. BIN SHA256 `9ae3d988a975ed1bd9664757991829a7acd0f54202c1c3eb3ee3a26e51c11051`. These figures do not measure dynamic TLS heap. The restored original SDK reproduced the identical BIN hash.

ESP-IDF 5.4.0 / mbedTLS 3.6.2 is the measured build baseline, not a claim of current security maintenance suitability. Review official SDK advisories and a supported patched baseline before release or adopting TLS 1.3. No physical flash was performed.
