# Typed CBOR and end-to-end BLE reply origin

## Typed command prerequisite integrated

Actual BLE command decoding previously passed through cJSON double values. go_json integer validation then rejected a legitimate binding_generation because its original numeric token had disappeared. The old CBOR decoder also applied its8192 length cap to integer magnitudes. Actual Go-issued/PoP command→real CBOR→processor reproduction exposed this gap; tests with a substituted BLE decoder did not prove that route.

The command and reconciliation BLE paths now decode directly into typed go_json::Value, retaining checked int64 integers. Size/depth/minimal encoding, duplicate keys, trailing bytes, unsupported types and strict UTF8 are checked; byte-string→hex behavior remains explicit. The business cJSON projection still rejects integers outside ±(2^53−1) before lossy conversion; the typed reader itself supports full int64. JSON lexical integer validation is unchanged. The legacy cJSON CBOR API and32-bit result encoder remain separate limitations; this prerequisite does not claim those are fixed.

Root strict GCC tests PASS: independent hexadecimal typed-reader vectors; actual authority/SQLite; actual reconciliation/Go signature interop; actual Go-wire/sync/large sequence regression. Three host decoder stubs were removed and linked to the real decoder. Root additionally fixed go_wire_host to retain that dependency for its actual authority executable. [Hash-only evidence](evidence/typed-cbor-root-2026-10-21/manifest.json).

## Remaining integration

Reply owner propagation through actual queue→processor→delivery→publisher is a separate reviewed package. Root full vertical verification and exact-source S3 CI are still required after that integration. No authenticated BLE channel, TLS, trusted-time provider, physical device or НАП acceptance is established here. Keys remain in ESP32-S3 NVS; ATECC608 is unused.

## Reply origin integrated and root host-verified

BleReplyOwner is an ephemeral by-value delivery context captured at guarded BFF1 admission, copied by both queue paths and forwarded through all four processor result branches. The actual app delivery body is now a shared production function; publisher requires an explicit owner and rejects missing/stale ownership without falling back to the current connection. Strict UUID dash/hex validation precedes frame allocation. No origin is persisted in receipt plans, journal rows, signed payloads or NVS.

Root GCC actual Go issuer→real CBOR/BFF1→CommandQueue→authorization→IntentProcessor/SQLite→production delivery→publisher passes13 scenarios. The same13 pass ASan/UBSan under Clang. Queued/executing A may finish durably after reset/reuse by B, but B receives no A notification. Missing owner and SDK notify failure do not alter stored financial outcomes or repeat device execution. Both queue wait paths, duplicate/unavailable results, actual signed reconciliation, HTTP non-BLE behavior and no unsolicited recover_pending result are covered. Root GCC native BLE three runs, actual reconciliation callback interop and44 Python checks pass. [Hash-only evidence](evidence/ble-origin-root-2026-10-21/manifest.json).

Exact-source full IDF/host/provenance gate must run after this change. Legacy OPEN_MVP/BFF1 response authentication, standard TLS/GATT channel, trusted time provider, real-device/HIL and applicable НАП acceptance remain OPEN. Generation tokens isolate local lifetimes and do not authenticate peers. The separate queue timeout policy still waits for worker completion to avoid dangling result pointers; this change does not make that wait a hard deadline.

Next accepted encoder follow-up is [62](62_TYPED_CBOR_RESULT_ENCODER_CONTRACT.md). Existing persisted result JSON already carries exact int64 amounts; its cJSON/valueint32 bridge can prevent valid result delivery. The new typed encoder/strict result bridge is developed separately and must prove actual client bigint compatibility. This document does not mark that follow-up complete.

Full exact-source S3 gate accepted for973a136: [run39552477296](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39552477296) SUCCESS. Root downloaded/verified all7 artifacts and101 Git source hashes; full Make command coverage and actual13 BLE origin/typed CBOR markers checked. [Reviewed provenance](evidence/s3-cbor-origin-ci-2026-10-21/manifest.json). Local ignored software candidate: `.store-build/s3-idf556-ci-973a136-20261021`. This closes the software target gate for these two packages only; remaining channel/provider/encoder/HIL/legal gates stay OPEN.
