# Authoritative offline SALE_FINALIZE v2

2026-10-17. This package implements the backend half of the coordinated firmware authorization/evidence contract. It is not a claim of CARD hardware readiness or certification.

## Original defect and executable red evidence

Actual MiniPOS emits SALE_FINALIZE; original edge_sync materializer requires an existing PAYMENT_PENDING sale. Offline sale is absent. The isolated regression at `/private/tmp/fiscal-offline-finalize-20261017/work/fiscal-backend/internal/domain/offline_finalize_gap_test.go` produced `actual offline SALE_FINALIZE cannot materialize missing sale: finalize sale not found` (red.log). Current `TestOfflineFinalizeV2MissingSaleCreatesAfterSignedRestart` replaces that missing behavior with production hardware-signed acceptance, restart and terminal projection.

## Immutable authorization before device IO

POST `/public/v1/execution-authorizations` accepts only workstation_session_id and client_signing_public_key_der_base64. Tenant/actor come from authenticated request. Persisted session and operator resource must both map to the same actor_subject; session owns operator resource ID and code. ADMIN-only POST `/public/v1/operators:bind-identity` uses expected_version CAS and records the actual administrator audit actor. Client employee ID is not assumed to be fiscal operator ID.

Issuer loads current active route, immutable issued_binding canonical payload+SHA256, generation and range, and checks both enrolled fiscal identities. It signs an immutable resource only if all session/operator/register/binding/issued binding/device versions still match under repository lock. Root P256 issuer is the binding trust anchor; SPKI P256 client key is bound in authorization. Domain separator `BEELOY-EXECUTION-AUTHORIZATION-V1\n` precedes Go canonical JSON before SHA256/ES256 DER. Envelope: algorithm ES256_DER, key_id, payload(base64url), sha256(hex), signature(base64url DER). Lifetime <=15min. Root rotation currently requires retaining/verifying the appropriate historical trust anchor; current issuer only verifies its configured root.

Authorization fields: kind EXECUTION_AUTHORIZATION/version1, authorization_id, workstation_session_id, tenant/location/register/edge/fiscal IDs, fiscal_device_number, fiscal_memory_number, binding_sha256/binding_generation, operator_id(resource ID), operator_code(4), actor_subject, app_instance_id, client_signing_public_key_der_base64, scope fiscal.execute, allowed_commands SALE_FINALIZE, nbf/exp, authority {binding_sha256,range_start,range_end,unp_prefix}.

unp_prefix is fiscal DEVICE SERIAL8, not fiscal memory number. Actual UNP is SERIAL8-OPERATOR4-SEQUENCE7. Global leased sequence is shared across operators. Activation and central server allocator must use the same serial-ledger key and disjoint ranges; old FMIN-prefix provisioning is deliberately rejected by issuance. Onboarding owner implements the schema3 producer and migration.

Client proof canonical transcript contains kind EXECUTION_INTENT_PROOF/version1, authorization_sha256, operation_id, receipt_session_id, payload_sha256, server_sale_id nullable, client_sale_surrogate_id nullable, binding_sha256, binding_generation, command_type. Domain separator `BEELOY-EXECUTION-INTENT-PROOF-V1\n` before SHA256/ES256 DER. Firmware verifies authorization/client PoP before monetary IO; backend trusts only that typed verified acceptance in signed hardware events, not client assertions.

## Signed acceptance and results

Payload.acceptance_evidence: version2, ingress BLE/HTTP, receipt_session_id, server_sale_id nullable, client_sale_surrogate_id, payload_sha256, unp, binding_sha256, generation, fiscal_device_id, authority as above, session_evidence {authorization_id,authorization_sha256,client_proof_sha256}, proof_state AUTHORIZED. Backend verifies actual issuer signature, historical issued snapshot, accepted time validity/revocation, operator ID+code, route, payload digest and allocated UNP range. Later mutable rebind does not rewrite historical accepted evidence. Incomplete/legacy SALE_FINALIZE is rejected without ACK; an explicit operator recovery/migration workflow remains necessary for old journals.

Terminal result requires exact operation/edge/receipt/payload hash/UNP and full payment_evidence inventory: payment_id,type,amount_minor,state,terminal_reference,rrn,auth_code,reversal_reference,reversal_rrn,reversal_auth_code. CARD approval is per tender, cash completion requires FISCALIZED_CASH, compensated CARD requires proven individual reversal. UNKNOWN remains uncertain. No operation-level final CARD reference is copied across tenders.

The deterministic sale ID hashes tenant+receipt_session_id. Top-level server_sale_id, when supplied, must refer to a real existing sale; canonical legacy payload.server_sale_id is never interpreted as authoritative internal ID. Existing merge requires exact tenant/location/register/operator/UNP/lines/tenders/device snapshot in PAYMENT_PENDING. Payment arithmetic, discount currency, totals, overflow, duplicates and effective tax policy are checked before projection.

## Atomicity and replay

CommitEdgeSync checks expected ACK cursor/hash, sale/operation versions, immutable accepted/result digests and cross-operation identifiers under one lock, then persists with existing store generation CAS. Sale, operation, receipt artifact, evidence, audit, webhook and ACK commit together. Save failure plus reload failure restores staged in-memory changes. Accepted context is retained after completion for replay and UNKNOWN→FISCALIZED reconciliation. Exact duplicate result has no version/audit/webhook side effects; conflicting terminal evidence fails.

No new database collection is required: accepted evidence/lifecycle extends existing edge_pending JSON storage. Generic issued_binding/execution_authorization/revocation resources are immutable (exact retry only), including defensive deep copies.

## Tests and integration boundaries

`go test ./internal/domain ./internal/mqttclient`: signed restart/create, authority forgery/context mismatches, plan/tender guards, recovery transitions, immutable historical rebind, persistence failure with failed reload, deterministic concurrent commit CAS, reused receipt/UNP/tender, exact server merge/mismatch, immutable resource identity mapping. `go test -race` covers those packages. API tests cover ADMIN mapping and rejection of free authority fields/foreign session; runtime OpenAPI owner must add the two routes before combined API gate passes.

Actual Go issuer → actual mbedcrypto authorization verifier → actual IntentProcessor HTTP → actual SQLite producer → actual hardware-signed Go SyncBatch consumer passed. The opt-in `TestOfflineFinalizeV2ActualFirmwareConsumerInterop` consumes `EXECUTION_AUTH_ACTUAL_DIR/{oracle.json,actual-events.json}` exported by the independent firmware host executable. This proves serialization/crypto/business interop, not physical receipt/payment IO: its executor is explicitly a software test adapter.

Online preassigned UNP is a separate followup: MQTT SALE_FINALIZE must carry authenticated SERVER_ASSIGNED source and existing sale UNP; local online authorization must bind optional unp_authority {source:SERVER_ASSIGNED,server_sale_id,unp}. This baseline never weakens existing UNP equality or silently reallocates online UNPs. Current v2 consumer admits local LEASED acceptance only. Firmware/client/onboarding owners must integrate full authorization flow, fresh serial-based provisioning and trusted mapping before end-to-end readiness. Physical receipt proof, query/reversal vendor evidence, device HIL, key-rotation and historical journal migration remain explicit gates.

## Online SERVER_ASSIGNED followup

Online authorization request optionally includes server_sale_id; no caller UNP, digest or monetary plan is accepted. The issuer requires the exact tenant/register/location/operator/device snapshot of an existing PAYMENT_PENDING sale and its EXECUTING SALE_FINALIZE operation. Shared `BuildFinalizePayload` produces the same canonical plan for MQTT and local authorization. Signed optional unp_authority contains source SERVER_ASSIGNED, server_sale_id, unp, payload_sha256, operation_id and receipt_session_id. Sale/operation versions join issuance CAS. Altered lines, tenders, operation or receipt cannot be authorized after issuance, even with the legitimate client private key.

Assigned sequence must be outside the active offline lease; firmware additionally fences retired leases and unique physical serial/sequence claims. Assigned SALE_FINALIZE uses unp_sequence0 and does not consume the leased counter. Local offline without signed unp_authority must have no real server_sale_id and no caller UNP/source. Its allocated sequence remains positive. Backend never accepts seq0 as an offline allocation.

MQTT acceptance uses ingress MQTT, proof_state SERVER_COMMAND, null session_evidence/client_sale_surrogate_id and unp_source SERVER_ASSIGNED. Backend compares its full canonical payload, digest, receipt, operation, tenant/register/adapter/generation and accepted validity against the durable prepared device_command_outbox body; then verifies historical issued binding and exact existing sale snapshot. Prepared nonempty body/topic/device/tenant cannot change through PutResource. It does not infer a local authorization or reroute by the current register. Existing immutable sale lines/tenders/UNP still must match at terminal projection.

Actual online Go-issued authorization → real mbedTLS verification → actual IntentProcessor/SQLite → actual signed backend consumer passed with assigned AB123456-A001-0005000, unchanged offline counter, seq0 and duplicate no extra IO. The consumer test accepts exact independently exported oracle/events via EXECUTION_AUTH_ONLINE_ACTUAL_ORACLE and EXECUTION_AUTH_ONLINE_ACTUAL_EVENTS. Software executor remains explicit; this is not physical money evidence.

Added regression inventory: online immutable merge; changed lines/tenders/op/receipt/UNP/source/sequence; foreign tenant/register/operator/device, completed sale, overlapping lease, missing/wrong operation/receipt; concurrent sale and operation version changes during signing; MQTT prepared authority mutation and altered accepted context. Combined domain/MQTT/API tests and race gate run against onboarding owner's generated specification overlay. ADMIN binding uses empty-body204 including idempotent replay.

Activation still requires coordinated serial-ledger migration: historic FMIN-key allocations cannot be copied silently as new serial authority. The onboarding/allocator owner must issue a fresh fenced serial-based range using shared central serial high-water, and account for already issued legacy leases before hardware enablement.
