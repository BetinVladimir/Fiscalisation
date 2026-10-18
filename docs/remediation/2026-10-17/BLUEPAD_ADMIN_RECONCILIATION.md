# Trusted BluePad administrative reconciliation

## Implemented authority and boundaries

`POST /public/v1/payment-reconciliations:authorize` requires an authenticated ADMIN, an operation/payment identifier, observed bank outcome and immutable acquirer case/reference plus SHA-256. The server derives the expected complete payment row, amount, receipt, tenant, terminal, device route and historical issued-binding identity from the last verified device result. Clients cannot choose those authority fields. The original outcome must remain UNKNOWN/RECOVERY_REQUIRED.

The server persists a unique immutable authorization before returning it. The P-256 DER envelope has a separate signature domain `BEELOY-PAYMENT-RECONCILIATION-V1\n`, kind PAYMENT_RECONCILIATION, scope payment.reconcile, and a maximum five-minute validity interval. This capability is independent of cached SALE_FINALIZE grants. Its signed expected/observed rows include every purchase and reversal reference; amount_minor is a canonical positive decimal string, preserving int64 exactly. Evidence stores both acquirer reference and content hash. The ADMIN is accountable for verifying that external evidence; neither its hash nor a signature automatically proves a physical fiscal document.

Send the returned command unchanged using the existing authenticated local S3 HTTP command endpoint, with `Idempotency-Key` equal to client_operation_id. The ESP32 verifies the compiled root trust anchor, signature domain, canonical bytes, digest, interval, complete binding/terminal/tenant route and command aliases before storage access. Missing trust or unsynchronized clock fails closed. BLE uses the existing canonical CBOR entry point. MQTT delivery likewise requires the root-signed exact capability; it cannot turn an unsigned payload into an administrative decision.

The transaction compares the complete current payment row and saga phase, verifies the terminal guard belongs to the original operation, updates the permitted bank decision, inserts immutable reconciliation evidence and an outbox event, and creates a separate COMMITTED administrative operation result. Any failure rolls back the complete transaction. Replaying the grant against an already changed row is rejected; HTTP polling/idempotent transport can return the existing administrative result. The original financial operation remains RECOVERY_REQUIRED. An approved bank payment with unresolved fiscalization/compensation retains the terminal lock. No terminal query, reversal, new purchase or physical fiscal proof is invented by this workflow.

The backend accepts PAYMENT_RECONCILED only in a valid device-signed journal/batch, with exact immutable grant, expected original operation, terminal, amount, references, ADMIN/evidence identity and occurrence within the signed interval. Cursor advancement and immutable audit resource commit together under repository/store CAS. This audit does not project a completed sale or replace a fiscal result. A subsequent real durable recovery snapshot is required before any further administrative decision on the same original state.

## Operator procedure

1. Identify the quarantined payment from its immutable device result. Match terminal, amount and merchant/bank references to an authoritative acquirer record.
2. An authenticated ADMIN requests the exact decision capability, reviews its decoded expected and observed rows, and retains the acquirer evidence at the signed reference/hash.
3. Send the unchanged capability to the matching S3 using its authenticated local transport. Record the administrative operation ID and wait for its stored result and backend journal acknowledgment.
4. Reconcile the remaining fiscal/compensation workflow separately. A bank approval alone never releases an uncertain fiscal operation. Missing bank authority, missing fiscal receipt proof, or unsupported reversal remains a blocked operation requiring the acquirer/vendor workflow.

## Verification

- Actual Go issuer tests: trusted role/actor, tenant, original payment and operation ownership, decision/evidence validation, immutable grant and source snapshot CAS.
- Actual S3-code host tests use real mbedTLS and SQLite, reopen the journal before dispatch, verify exact signed commands before any executor call, and reject tampering, wrong binding/terminal/tenant, interval boundaries, aliases, changed rows and correctly signed invalid decisions.
- Actual Go issuer → C++ IntentProcessor/mbedTLS/SQLite → actual Go hardware-signed SyncBatch consumer interoperability. Separate signing domains and typed decimal amounts are exercised on real production code.
- Existing scoped-terminal restart/concurrent/fault tests retain approval-with-unresolved-fiscal quarantine; pinned ESP32-S3 compiler checks the modified production files.

## Remaining external acceptance

Real acquirer evidence acquisition and operator review, physical fiscal recovery proof, and actual Android/iOS/local-network transport/HIL remain acceptance requirements. This package does not add unsupported query/reversal capabilities or an external secure element; device keys remain inside ESP32-S3/NVS. The authenticated UI integration is tracked separately from the trusted issuer/device/consumer vertical.
