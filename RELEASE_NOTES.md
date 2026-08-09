# BeeFiscal / BeeMiniPOS software MVP release candidate

Release profile: `BG_MVP_FUNCTIONAL_NONPROD`  
Acceptance record: [`contracts/mvp-acceptance-v1.json`](contracts/mvp-acceptance-v1.json)

## Included

- Two isolated Fiscal and MiniPOS Compose projects, each with DEV/PROD overlays, Caddy and a separate PostgreSQL database.
- 92 OpenAPI operations with generated request/response runtime enforcement and typed clients.
- Fiscal sale, EUR totals, explicit cash/card split payments, append-only Fiscal and MiniPOS reversal, shifts, cash movements, reports, exports, immutable audit/outbox and reconciliation.
- MiniPOS touch checkout exposes exact ordered CASH+CARD split tender through `checkout-batch`. Its REST adapter consumes the enforced `MiniPosFiscalOperation`: only `FISCALIZED` succeeds, confirmed `FAILED` is final, and `UNKNOWN` remains frozen for GET-only reconciliation.
- The original fiscal receipt reference is persisted separately from the reversal reference in MiniPOS orders and checkout replay evidence, survives restart/backup/restore and is populated by both synchronous checkout and signed offline-sync webhooks. Fiscal operation/webhook envelopes use `fiscal_reference`; the public MiniPOS order uses the canonical OpenAPI field `receipt_reference`.
- The signed Fiscal webhook receiver uses a closed OpenAPI envelope for operation type, external/sale IDs, receipt reference and error evidence; undocumented fields fail before domain mutation.
- Canonical and additive runtime response schemas are composed fail-closed when they share a method/path/status. In particular, an order response that satisfies the canonical schema but omits runtime-owned `allowed_actions` is rejected before reaching the client.
- BLE authority issuance and refresh now revalidate the tenant-owned active operator in addition to the active register and final-device binding. Unknown, future or deactivated operators cannot obtain or extend local `fiscal.execute` authority.
- MiniPOS PROD bundles ignore both public MiniPOS and Fiscal static-token environment variables and use the personal OIDC access token for both public APIs. Browser E2E injects two forbidden token markers and scans the compiled artifact to prove neither secret is bundled; security regression now contains 32 executable cases.
- Registry-aware cash and card simulator flows. CARD requires an effective ACTIVE terminal binding and never falls back to cash.
- REST-issued BLE sessions, encrypted framing, replay protection, Edge SQLite journal, signed sync acknowledgements and three-month ACK-gated retention.
- BLE session lifecycle is bound to the issuing OIDC subject and a MiniPOS-prepared X25519 public key signed into the ticket; Edge HELLO verifies proof of possession and every established gateway channel rechecks expiry/revocation before frame decryption. MiniPOS removes stale READY state, cancels pending results and supports explicit re-issuance after expiry, failed connect, logout or configuration change. Refresh rotates nonce/fencing authority and revoke is a contract-checked idempotent empty `204`. Existing unbound session rows are deliberately invalidated and must be reissued.
- BeeMiniPOS and BeeFiscalApp Web/Android/iOS build surfaces; Daisy SMART S Android contract STUB.
- Daisy Compact protocol semantics now cover all 45 commands classified as `Supported`. Beyond the fiscal/readiness/EUR and sale/report set, commands 110/112/114/116/119/128/138/146/153 provide typed current-day payments, operator/refund totals, FM `P/F/E` queries, QR and issued-document/SHA1 evidence, 26 device constants, department sales/corrections and acknowledged text-report lines. Golden and negative C++ vectors remain software evidence, not USB/HIL approval.
- All 16 Daisy commands classified as `Optional` now also have typed semantics and golden/negative vectors: display/printer/drawer operations, invoice customer fields, barcode types 1–13, customer QR templates, display configuration, customer directory and Compact-family battery telemetry. Their disposition and model restrictions remain unchanged; this is not hardware activation evidence.
- The isolated two-Compose regression harness uses a separate PostgreSQL Unix-socket volume per stack and verifies the socket DSN before tests. This removes Docker Desktop DNS/VPN/bridge blackholes from backend-to-database communication while retaining separate database containers, internal `dbroute` networks and no PostgreSQL presence on shared ingress. DEV/PROD TCP topology is unchanged, and targeted two-Compose E2E passes.
- A unified `make full-regression` passes with exit code 0 for the final 45 core + 16 optional Daisy slice, including independent TCP PostgreSQL/RLS tests and the Unix-socket-isolated two-Compose cash/card/split/reversal/Z/UNKNOWN/restart/backup/restore journey.
- The mandatory regression gate now includes `go vet` for all three Go modules with isolated writable caches; the initial Fiscal, MiniPOS and Edge baseline is clean.
- Fiscal and MiniPOS PROD startup now rejects HTTP public routes, wildcard/non-origin CORS values and webhook keys shorter than 32 bytes. The PROD Compose profiles require the matching public/CORS variables, and `.env.example` was replaced with a complete DEV-only two-stack profile that is rendered by the regression gate.
- The unified regression passes with these guards enabled: three race/vet suites, 33 security cases, all contracts and platform builds, PostgreSQL/RLS, plus the complete two-Compose Caddy E2E all finished with exit code 0.
- MiniPOS HTTP connections now have explicit header/read/write/idle deadlines rather than Go's unbounded body/write defaults. The policy is executable security evidence, and two consecutive unchanged-tree full regressions passed with the resulting 34-case security matrix.
- Stage 1 governance is now machine-readable and regression-gated: two independent product boundaries, ten owned module domains, identifier policy and all 14 P0 decision records must remain complete, evidence-backed and production-blocked where external acceptance is pending.
- Fiscal Web CORS preflight now advertises the canonical webhook-endpoint DELETE operation. Approved origins can use the full OpenAPI mutation surface; foreign origins remain denied. A positive/negative preflight test raises the security matrix to 35 cases.
- BeeMiniPOS and BeeFiscalApp now bound every public-API fetch to four seconds, including the primary MiniPOS backend path. Hung connections abort deterministically and release timer/signal resources instead of freezing cashier/admin busy state; executable timeout and UI gates cover the policy.
- BeeFiscalApp no longer embeds or accepts a public static admin token in PROD. It now uses its own OIDC Authorization Code + PKCE client/scheme, blocks every API call before personal login and exposes logout; the real production Web bundle is scanned and exercised in Playwright.
- The post-OIDC unified regression passed end to end with 36 security cases and 25 UI acceptance cases, including both production bundle scans and pre-login fail-closed checks.
- MiniPOS and BeeFiscalApp now refresh expiring OIDC sessions in memory and fail closed to personal login when refresh authority is absent or rejected. Provider tokens are never persisted; deterministic expiry boundaries are covered by the 37-case security and 27-case UI matrices.
- The unified full regression passed after this lifecycle cutover, including all platform bundles and the two-stack Caddy/PostgreSQL recovery E2E.
- Malformed token lifetime metadata and production API `401` now invalidate UI authority immediately. MiniPOS also tears down/revokes its BLE session on personal-authority loss rather than leaving the local route READY.
- The post-hardening unified regression passed with all 29 UI acceptance cases and the complete two-stack recovery journey.
- Operator-owned open shifts are now recoverable through the public API after app/backend restart. Recovery is filtered by employee/register/state, denies another OIDC employee, clears stale cart context on auth loss and is covered by a real browser reload plus two-Compose restart assertion; UI acceptance contains 30 cases.
- The unified regression passed after the 91-operation recovery cutover, including PostgreSQL restart and both public Caddy routes.
- Blocked Z-report closure now has a dedicated server-authorized reconciliation path. It reads the original operation, cannot repeat Z when its ID is known, blocks all sales until final evidence and closes only with a final fiscal reference; security/UI gates are 38/31.
- The unified regression passed after the 92-operation Z-reconciliation cutover.
- CASHIER read access is now employee-scoped: employee directory and tenant-wide report are denied, order lists exclude other employees, and production UI neither fetches nor renders management data without supervisor/admin authority. Security/UI matrices are 39/32.
- The unified full regression passed after this least-privilege increment: all 92 request/response contracts, 39 security and 32 UI cases, platform exports, PostgreSQL/RLS and the independent two-Compose/Caddy recovery journey completed with exit code 0.
- OIDC/OAuth software enforcement, FORCE RLS, release SBOM/provenance/signature verification and signed OTA policy.

## Compatibility disposition

- Daisy SMART S: `STUB_ONLY`, impossible to activate in PROD.
- Daisy Compact S 01 and DP-150 MX: protocol semantics documented and tested without claiming hardware approval.
- BlueCash-50 and BluePad-50 Plus: `UNSUPPORTED/EXCLUDED_FROM_PROD` until vendor/acquirer evidence exists.
- Currency: EUR for current operations; the periodized compliance export preserves the legal BGN/EUR boundary.

## Known limitations and NO-GO

This candidate is not a Bulgarian pilot or production release. Real hardware, vendor/acquirer, passwordless IdP deployment, vulnerability scan, protected signing key, NAP/BIM/legal and authorized-service evidence remain required. Simulator, STUB and accelerated soak results cannot close those gates.

## Verification

Run `make full-regression` and `make handover-test`. Release package creation and independent signature verification are documented in [`docs/release-evidence.md`](docs/release-evidence.md).

## Rollback

Use [`docs/rollback-plan.md`](docs/rollback-plan.md). Never roll back by editing or deleting a completed fiscal sale, operation, audit event or acknowledged Edge record.
