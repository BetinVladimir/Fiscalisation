# MiniPOS administrator bank reconciliation

A committed reconciliation records BANK_DECISION_RECORDED_FISCAL_UNRESOLVED. It does not prove a fiscal receipt, release fiscal recovery or mark the original sale successful.

## Required administration

1. Grant the authenticated source credential the explicit `payment_reconciliations:authorize` capability. Sale authority does not grant administrator authority.
2. Bind the root operator to the exact source system, identity issuer and subject through the existing operator identity mapping.
3. As root ADMIN, POST `/public/v1/payment-reconciliations:bind-administrator` with operator_id, external_system_id, source_operator_id, identity_issuer, identity_subject, enabled and expected_version. This separate mapping is versioned/audited. Disabling it denies cached issuance retries too.
4. Require authenticated source ADMIN claims and persisted employee ADMIN, an active caller-owned app session/open logical shift, linked root workstation session and immutable accepted operation/payment evidence. A logical shift does not prove physical fiscal state.

POST `/public/v1/minipos/payment-reconciliations:authorize` accepts only shift_id and decision. Servers derive tenant/actor/app/session context. Audit headers grant no authority. The grant is bounded by source/session/operator expiry and the existing five-minute window. Root verifies durable route, expected payment and all source versions atomically before grant/replay persistence.

## Delivery and recovery

The ADMIN view displays signed scope, evidence and expected/observed rows before separate confirmation. A distinct IndexedDB outbox stores the immutable command and atomically marks it attempted before I/O. Uncertain delivery is lookup-only under the same operation ID, including restart/concurrent tabs. Logout or authoritative denial tombstones the scope; late responses cannot revive it. It never uses a sale-intent cast or fiscal-success projection.

HTTP uses the existing local token. BLE uses existing secured framing/CBOR; uncertain BLE delivery uses same-edge read-only HTTP lookup because current BLE has no operation lookup. If unavailable, status remains unresolved. New ADMIN issuance requires an online authenticated source control plane. Hardware keys remain ESP32-S3 internal NVS; no ATECC is assumed.

## Verification and limits

Root domain/API/integration and source domain/API/integration suites pass. Negative coverage includes missing capability, foreign tenant/source/issuer/app/session, revoked mapping, expiry, changed replay, source ADMIN disagreement and atomic persistence faults. Actual delegated Go grant → production mbedTLS/IntentProcessor/SQLite → signed event consumer passes 11 executable modes. Browser tests use a real Go-issued grant and IndexedDB transactions; review/confirm, restart, concurrency, revocation and fiscal-unresolved wording are covered, retaining discount/reversal checkout assertions.

Physical bank/device evidence, Android/iOS operator workflow, storage loss and device clock remain HIL checks. UI text currently English; localization remains a product follow-up. Browser hash/scope checks support review; firmware performs root-signature verification. Software emulation establishes neither vendor nor certification approval.
