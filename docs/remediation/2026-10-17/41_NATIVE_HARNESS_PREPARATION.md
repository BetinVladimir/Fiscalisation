# Native full-App harness preparation

The Android harness now prepares a frozen temporary copy with the real Expo AppEntry and application sources, shared UI/design inputs and selected resolved dependency manifests. Build/run records check source and APK hashes, package/scheme, debug signer, local API origin and installed APK identity. This is preparation for scoped full-App UI tests against synthetic localhost API fixtures, not a completed native build or real backend/device acceptance.

## Repaired preparation gaps

- Reject non-normalized/symlink escapes, foreign snapshot roots, synthetic entrypoints, stale APKs and overwritten build records.
- Exclude dotenv files; allowlist build environment with isolated HOME/Gradle/cache. Caller secrets and NODE_OPTIONS/JAVA_TOOL_OPTIONS are not forwarded into builds.
- Use one cross-product lease for the assigned emulator. Evidence-directory failure does not leak the lease; cleanup releases it even if adb reverse removal fails. Runners never start/reset/terminate an emulator and do not install arbitrary APKs; the assigned isolated package must already match the verified APK.
- Bound build/Maestro process groups and adb/tool calls. Timeout terminates uncooperative child processes. Executed runner/fixture/guard/server source hashes are included in evidence.
- Redact auth/access/refresh/selection tokens and private-key fields when persisting ledgers. Real cryptographic signing inside the MiniPOS fixture uses an ephemeral synthetic key; this does not establish Go-issuer/S3 trust.
- Limit HTTP request bodies to64KiB and one2-second read deadline. Reject invalid UTF8/malformed requests, record rejection in the ledger and close the connection. Rejected input prevents acceptance rather than disappearing from evidence.

[Evidence](evidence/native-harness-2026-10-21/README.md):30 tests and26 syntax checks PASS. `native-harness-test` is included in the regression target. Old prepared snapshots cannot be relabelled current after these changes; fresh snapshots/builds are required.

## Still required

1. Create fresh source snapshots after the current financial/role fixes, build exact merged Android artifacts and record source/toolchain/APK/firmware identities. The harness samples dependency manifests; complete dependency/toolchain provenance is not yet hermetic.
2. Run auth, operational, payment/recovery and visual/accessibility scenarios using owned emulators and actual Android/iOS devices. Existing YAML covers a bounded subset, not the entire application or all platforms. Debug-signed isolated artifacts are not store/release packages.
3. Replace synthetic API assumptions with the real backend/S3 integration contour, including authoritative checkout outcome and source-order projection. Retain separate fixture-unit and real-integration results.
4. Complete encrypted/authenticated operational BLE, offline result durability/revocation/equivalence, real Datecs COM/Daisy USB/BluePad BLE HIL and applicable external НАП review. Device keys remain internal ESP32-S3 NVS; ATECC608 is unused.
