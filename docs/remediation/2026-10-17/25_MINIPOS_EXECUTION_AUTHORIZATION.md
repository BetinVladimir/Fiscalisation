# MiniPOS browser execution authorization vertical

Status: implementation in isolated overlay, not device readiness evidence.

## Trust contract

MiniPOS browser authenticates only to its own backend. The backend derives employee identity from validated JWT issuer and subject plus persisted employee identity binding, active operator session, app instance, open shift, and configured fiscal register. A browser employee UUID or operator code never identifies the fiscal operator by itself.

The MiniPOS backend calls `/integration/v1/execution-authorizations` using its existing encrypted tenant/source credential. Fiscal backend requires the explicit `execution_authorizations:issue` capability. Existing audit actor headers do not grant this capability. The credential determines tenant and integration system; request body cannot select these identities.

Delegation body contains source employee/register IDs, identity issuer and subject, authenticated source session SHA256, app instance, authenticated expiry, client public SPKI, and optional real fiscal sale ID. Source values are produced by MiniPOS backend, never forwarded from a browser DTO. Fiscal backend resolves external source resources and requires ADMIN-configured identity equality using an unambiguous issuer+subject namespace. Old bare-subject mappings cannot authorize delegation. ADMIN mapping, capability assignment and synchronized source resources are explicit setup prerequisites; none is automatically granted.

A persisted delegated workstation session binds the entire authenticated source context. Exact retry reuses the session; changed context under the same source session/app/register identity conflicts. A short-lived root-signed authorization is issued only through the existing strict issuer. Session expiry cannot exceed authenticated source expiry; authorization lifetime cannot exceed session expiry. Issuance failures must not manufacture fallback authority.

## Browser custody and lifecycle

The browser stores a P256 nonextractable private CryptoKey in IndexedDB, scoped to backend, fiscal tenant, namespaced actor, app instance and authenticated session. Public SPKI is sent for issuance. The root ES256_DER envelope is preserved exactly. PoP signs the Go-canonical transcript with the protocol domain separator and converts WebCrypto P1363 output to ASN.1 DER. Firmware verifies both root and client signatures before reservation or device I/O. ESP32-S3 keys remain in its NVS; no ATECC is assumed.

Offline authorization cache is separate from online sale-specific tickets. Expired/wrong-session/route tickets cannot build new intents. Logout invalidates live context and fences in-flight cache writes. Existing immutable outbox intents retain their exact authorization and proof on retry; they are never silently refreshed, resigned, or converted to a different sale. Receipt/outcome lookup is separate from resending money commands.

## Online preparation

MiniPOS order ID is not fiscal sale ID. Online execution requires a persisted actual fiscal sale and prepared operation, receipt ID, exact BuildFinalizePayload and immutable SERVER_ASSIGNED authority. The prepare-only boundary produces these before local dispatch without publishing a second money command. Browser cannot supply trusted UNP, totals hash, operation, or receipt identity. Offline has null server_sale_id and a bounded client surrogate.

Only SALE_FINALIZE is covered by existing root allowed_commands. Reports, cash commands and reversal require explicit role-derived command scope and correct payload contracts; a sale ticket cannot be reused for them.

## Acceptance evidence

1. Actual red: old buildIntent accepts SALE_FINALIZE without authorization.
2. Source capability missing/foreign tenant/issuer collision/missing ADMIN map fail before root issuance.
3. Persisted employee/session/shift mismatch, revoked session, wrong configured register fail before delegation.
4. Exact delegation retry reuses session; changed key/context replay conflicts; expiry bounded by authenticated source.
5. Browser real IndexedDB retains one nonextractable key across reload and concurrent tabs; logout and stale in-flight writes cannot revive it.
6. Actual browser WebCrypto DER proof passes independent OpenSSL verification and the actual firmware verifier; mutated plan/actor/op/receipt/expired auth fails before device I/O.
7. Immutable outbox retry preserves intent bytes and proof, including after logout; creation of new intent is forbidden without active scope.
8. Online actual fiscal ID, prepared plan and assigned UNP survive browser→firmware→backend roundtrip. MiniPOS ID substitution fails.

## Logical shift and hardware limits

`OpenShift` is an existing logical database transition and performs no fiscal device command. Delegated links explicitly record `source_assertion=MINIPOS_LOGICAL_OPEN` and `physical_verified=false`. Existing `OpenSaleWithFirstLine` readiness and workstation-clock requirements remain in force. A MiniPOS shift assertion is not evidence that a fiscal device shift is open, that its clock is correct, or that an operator has physical fiscal privileges. These require the independent device evidence/HIL checks.

## Implemented prepare/dispatch boundary

MiniPOS freezes its persisted order lines and chosen tenders atomically. The root resolves the actual external register/operator mapping, creates a separate actual fiscal sale, and freezes the complete sale/tender/operation/receipt state without publishing a money command. Source catalog `product_id` remains in the MiniPOS order and is never relabelled as a fiscal product code. The browser receives the exact `BuildFinalizePayload` and root-signed SERVER_ASSIGNED authority. Cloud dispatch accepts only a persisted source-owned order/authorization reference; it prepares and stores the same exact plan in the durable command outbox. Local HTTP/BLE and MQTT share the same operation and receipt identities; existing device idempotency handles duplicate ingress.

The existing queued finalize implementation lost appended tenders by rereading the old sale in `ReserveSalePaymentCommand`. `ReserveSaleFinalize` atomically stores the full tender plan, sale operation link, operation, and optional command, with persistence rollback and generation CAS supplied by the existing repository store.

## Validation in isolated overlay

- Full MiniPOS backend `go test ./...`: PASS (local HTTP listener permission required).
- Fiscal domain/integration/API/MQTT suites: PASS; delegation persistence fault/restart and concurrent exact replay tests included.
- Web unit tests: 124 PASS; TypeScript and Vite production build PASS.
- Real Chromium: six tests PASS, including actual Go issuer with browser-owned persistent nonextractable P256 key for offline and server-assigned online plans; client DER signature independently verified with OpenSSL.
- Both browser-produced intents pass actual mbedTLS/IntentProcessor/SQLite firmware harness (valid plus missing/signature/binding/expired/payload negatives). No physical device I/O occurred.

This evidence does not yet establish a single deployed MiniPOS JWT→real HTTP root issuer→browser UI→physical device roundtrip. Source JWT/session derivation and integration capability boundaries have executable independent tests, while the root/browser/firmware cryptographic chain is actual cross-language execution. Hardware routing, NVS trust provisioning, browser BLE support on iOS, device clock and physical receipt/payment evidence remain HIL obligations.

## Authoritative refusal versus outage

The integration HTTP client preserves upstream HTTP status in a typed error. Root capability refusal and revoked/missing/ambiguous administrator identity/source mapping produce authoritative 403; MiniPOS propagates 401/403 rather than hiding them as 409. A browser authorization manager responds by tombstoning its session-scoped key and removing cached tickets, fencing contexts and issuance requests already in flight. Reauthentication with a new source session is required after this refusal. Network failures and 5xx do not revoke a valid offline ticket; signature expiry, source session, actor/app scope and active-key checks still apply. A setup/conflict 409 is not relabelled as identity revocation and does not erase the key. The current request still fails; an online prepared sale cannot fall back to a different offline sale.

Follow-up verification: actual HTTP client typed-status tests, source 401/403/400/409/5xx mapping tests, root namespaced mapping refusal403 test, and eleven Chromium lifecycle/crypto tests PASS. Cache expiry remains effective during network outage; confirmed denial prevents old context signing and cache/key revival.
