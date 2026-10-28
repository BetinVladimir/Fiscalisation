# Integrated native, ADMIN and dossier status — 2026-10-19

Base committed revision: 361a263. The following evidence applies to the current integrated working tree and must be renewed after further changes.

- Native checkout/source-order migration and persistent signed journal: 62/62 Node tests pass; native TypeScript passes. Log: /private/tmp/fiscal-native-ble-merged-20261019.log. Node transport tests use simulated radio callbacks and do not establish physical BLE behavior.
- Browser ADMIN review/durable queue plus existing checkout: 252/252 tests pass. Log: /private/tmp/fiscal-admin-native-merged-web-20261018.log. Browser TypeScript passes.
- Root domain/API/integration and MiniPOS source domain/API suites pass after integration. These are separate from a complete authenticated ADMIN HTTP pipeline.
- API generation produces 147 request and 147 successful response contracts across two services. Full make contract-test passes: locked sources, generated drift, native TypeScript/client smoke, reviewed runtime73 operations and canonical73 operations. Log: /private/tmp/fiscal-contracts73-integrated-20261019.log.
- Both independent PostgreSQL financial profiles pass actual temporary PG16.10 tests: MiniPOS5 subtests and Fiscal8. See doc24 for isolation/projection limits. Full resource/audit export remains under development.

## Required remaining evidence

The actual authenticated ADMIN HTTP → browser confirmation → firmware pipeline is not yet accepted: its current end-to-end rehearsal reports delegated authorization403, under investigation. Existing issuer/firmware and UI tests cannot close this item. Actual native OS-key → issuer → S3 interoperability remains in progress. Android/iOS builds reported by isolated integration must be matched to final merged sources/artifact hashes before release.

Audit migration019 reads prev_hash while the domain writes previous_hash; migration017 typed projections use the same legacy key. A forward-compatible migration and actual PostgreSQL regression are required. Historical bytes/hash semantics and tenant-filtered global-chain limits must be preserved. Do not silently rewrite audit events or claim a tenant-filtered chain is complete.

Physical USB/COM/BLE HIL, real fiscal/acquirer results, production trust/clock/encryption/revocation, power-loss tests and applicable НАП external dossier review remain OPEN. ESP32-S3 keys are internal NVS; ATECC608A/608 is unused. No software result here establishes certification or legal acceptance.

## Additional merged checks — 2026-10-19

Native UI:91/91 tests across14 suites pass (`/private/tmp/fiscal-native-ui-merged-20261019.log`). Production Metro web export passes (`/private/tmp/fiscal-native-metro-merged-20261019.log`), verifying the actual merged App and its header/import assets. These are packaging/UI evidence, not Android/iOS radio tests. Audit021 and actual-domain-JSON forward regression now pass with Fiscal8 tests; see doc29. ADMIN fullchain located an actor namespace wire-size mismatch; coordinated issuer/firmware bounds and boundary regressions are pending integration.

## Integrated verification update — 2026-10-21

The earlier ADMIN403 and native OS-key pending statements above are historical. The coordinated authority actor limit (256 UTF-8 bytes, without truncation) is now integrated. Root domain/API/integration tests pass (`/private/tmp/fiscal-actor-admin-root-integrated-20261021.log`); full IoT tests pass (`/private/tmp/fiscal-actor-admin-integrated-iot-20261021.log`). The S3 build passes with RAM56,716/327,680 and flash1,999,316/3,145,728 bytes (`/private/tmp/fiscal-actor-admin-s3-build-20261021.log`).

Actual iOS Security.framework key-store → Go issuer → production JS signing transcript → S3 mbedTLS/IntentProcessor/SQLite interoperability passes, including app termination/relaunch and negative authorization vectors (`/private/tmp/fiscal-native-ios-integrated-20261020.log`). This is an OS-key harness result, not physical BLE or full Expo UI acceptance.

The integrated ADMIN HTTP rehearsal reached the actual UI but stopped on outdated accessible field labels after editorial changes. The E2E locators now match the observed Payment ID, Terminal reference, Retrieval reference number (RRN), and Authorization code labels. A fresh disposable PostgreSQL/browser/firmware run is pending; no full-chain pass is claimed yet.

Migration021 is committed as76574ef; actual PostgreSQL upgrade and conflict/rollback checks pass (doc29). Historical typed projections and full resource/audit coverage still require separate acceptance. Electronic receipt implementation and the offline-equivalence gate remain OPEN (doc30 electronic receipt gap).

### ADMIN full HTTP rehearsal accepted — 2026-10-21

The fresh disposable PG16.10 → source ADMIN JWT/encrypted integration capability → root delegated issuer → actual browser review/explicit confirmation → production S3 processor/mbedTLS/SQLite harness passes:1/1 Playwright scenario,8.0 seconds (`/private/tmp/fiscal-admin-http-integrated-20261021-r3.log`). Both obsolete UI locators were corrected against the observed accessible snapshot: field labels and review heading. No production authority behavior was relaxed. Assertions require exactly one authorization and one submission, preserve CARD_UNKNOWN→CARD_APPROVED bank evidence, reject a foreign app instance, and prohibit a root device-command outbox entry. This supersedes the pending HTTP statement above. The physical adapter transport is still represented by the native harness; real-device HIL remains OPEN.

### Live execution-authority boundary regression — 2026-10-21

Actual Go-signed grant → production S3 mbedTLS verifier now additionally tests nbf inclusive/exp exclusive, last valid second, absent trusted clock, binding generation increment/decrement, changed UNP range, issuer-key mismatch and clearing a previously accepted output on rejection. All vectors pass. Versioned log and tested source hash are in evidence/admin-http-2026-10-21. This is a bounded verifier regression; full offline equivalence remains OPEN.
