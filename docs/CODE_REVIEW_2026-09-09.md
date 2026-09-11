# Code Review Report — Fiscalisation Project

## Date: 2026-09-09

---

## Executive Summary

- **CRITICAL (Infrastructure):** `compose.fiscalisation.prod.yaml:19` sets `CORS_ALLOWED_ORIGINS: "*"` while `APP_ENV: prod`. The `Config.Validate()` in `fiscal-backend/internal/config/config.go:48` explicitly rejects this combination, meaning the production container **will refuse to start**. This is a deployment blocker.
- **CRITICAL (Regulatory / SUPTO-29-20):** The `ALLOW_STUB_ADAPTERS` env var defaults to `"true"` in the base `compose.fiscalisation.yaml:58`. Unless the prod overlay reliably overrides this (it does: line 17 sets `"false"`), any accidental launch from the base file alone runs the simulator in production — a Приложение №29, т. 20 violation.
- **CRITICAL (Regulatory / N-18 Art. 26):** The UNP format regex (`bgFMINPattern`) at `regulatory_identifier.go:20` allows alphanumeric (`[A-Za-z0-9]{8}`), but according to Приложение №29, т. 9, the third component must be exactly 7 **Arabic digits** (`[0-9]{7}`). The sequence component is correctly constrained, but the FMIN and operator-code patterns allow letters mixed with digits. If a device receives an alphabetic FMIN (e.g., `AB123456`) this is consistent with the regulation. However, the `DefaultBGFiscalProfile` currency is **`"EUR"`** (set in January 2026 context), which is correct for the EUR adoption date.
- **HIGH (Regulatory / N-18 Art. 26 §17, App. 29 §10):** Payment type support is restricted to `"CASH"` and `"CARD"` in `service.go:1010`. Наредба N-18 requires support for additional payment types: check (`чек`), voucher (`ваучер`), deferred payment (`резерв 2 — отложено плащане`), NHIF (`резерв 1 — НЗОК`), and internal consumption (`резерв 2 — вътрешно потребление`). None of these are implemented in the domain or POS clients.
- **HIGH (Regulatory / App. 29 §18):** The SUPTO_18_1 export table (`exports.go:260–261`) is missing several mandatory fields from Приложение №29, т. 18.1: `дата на приключване` (close timestamp), `дължима сума по продажбата` (due amount), `обща сума без ДДС` (net amount), and invoice fields. The current export emits only `sale_id, external_id, unp, location_id, register_id, operator_id, state, created_at` — a significant gap for NRA audit compliance.

---

## 1. Regulatory Compliance — Наредба N-18 + Приложение №29

### 1.1 Compliant Areas

**UNP Generation (App. 29, т. 9)**
- Format `XXXXXXXX-ZZZZ-0000001` is correctly implemented in `regulatory_identifier.go:50–73` with strict regex validation, 8-char FMIN, 4-char operator code, and 7-digit monotonically increasing sequence.
- PostgreSQL unique constraints in `013_supto_identifier_foundation.sql:64–65` enforce no-reuse: `UNIQUE (tenant_id, unp)` and `UNIQUE (tenant_id, fiscal_device_number, unp_sequence)`.
- The advisory lock (`pg_advisory_xact_lock`) in `postgres.go:68` correctly serializes concurrent allocations per FMIN stream — satisfying the monotonicity requirement.
- The 2-hour readiness lease max (`BGReadinessLeaseMax = 2 * time.Hour` in `regulatory_identifier.go:16`) and DB constraint (`CHECK (valid_until <= checked_at + interval '2 hours')` in `013_supto_identifier_foundation.sql:94`) directly enforce App. 29, т. 8.

**Operator Data Requirements (App. 29, т. 6)**
- Operator resource validation at `admin.go:177` enforces `code` length = 4, non-empty `first_name`, `last_name`, and `active_from`.
- Operator active-period checking in `service.go:183–199` correctly validates `active_from`/`active_to` RFC3339 timestamps for both ID and code lookups.
- Audit trail of login/logout captured via `operator_security_events` table (migration `017`) with append-only trigger.

**Authentication (App. 29, т. 7)**
- Workstation sessions are uniquely bound to `actor_subject` (OIDC subject) + `app_instance_id`. Sessions expire at 8 hours and are durably revocable (`service.go:716–733`).
- HMAC-SHA256 token validation with algorithm enforcement (only `HS256` accepted) at `auth.go:175`.
- OIDC fallback is correctly implemented in `auth.go:50–56`.

**FU Connectivity Checks (App. 29, т. 8)**
- Readiness is verified at sale open (`service.go:765`) and freshly re-verified at payment (`service.go:1016–1022`).
- Lease signature is HMAC-SHA256 over the full identity tuple at `service.go:443–446`, preventing forgery.
- Clock drift correction with 30-second tolerance enforced in `service.go:463–478`.
- Daily clock sync is required before opening a sale (`service.go:762`).

**Storno / Reversal (N-18 Art. 31(4), App. 29 §13)**
- Reversal is fail-closed: original fiscal reference required, sale must be COMPLETED, reason must be in the allowlist (`reversal_policy.go:9–14`).
- Europe/Sofia civil calendar deadline ("through day 7 of following month") correctly implemented in `reversal_policy.go:32–34`.
- Sale transitions to `CANCELLED` (not deleted) and reversal operation retains `original_fiscal_reference`.

**Cancelled Sale Preservation (App. 29 §12)**
- Cancelled sales retain all data. The `cancelSaleForTenant` in `service.go:1217–1229` only allows cancellation when no payments have been made. The sale state becomes `"CANCELLED"` and is persisted.
- `sale_events` table is append-only (trigger in `017_supto_full_event_model.sql:122–124`).

**Banned Fiscal Wording (App. 29 §14, N-18 Art. 7а)**
- `document_policy.go:36–41` implements four regex patterns enforcing bans on "фискал", "fiscal receipt", "касова бележка", and "фискален бон" in non-fiscal document templates.
- `DocumentClassFor` at `document_policy.go:43–58` enforces server-side document class selection — POS cannot override this.

**Test/Training Mode Prohibition (App. 29 §20)**
- `config.go:39–40` rejects `ALLOW_STUB_ADAPTERS=true` in `APP_ENV=prod`.
- `config.go:42–43` rejects `SIMULATOR_CARD_TERMINAL_AVAILABLE=true` in `APP_ENV=prod`.

**Audit Log (App. 29 §15–16)**
- Hash-chained audit log in `fiscal_runtime_audit` with immutable trigger (`019_audit_immutability.sql`).
- `AUDITOR` role correctly restricted to read-only access in `auth.go:133, 141` — cannot call mutation endpoints.
- Audit events include: login/logout, sale open/cancel/complete, line changes, reversals.

**Export Format Support (App. 29 §18)**
- Export types SUPTO_18_1 through SUPTO_18_9 and KLEN/FISCAL_MEMORY are recognized.
- JSON, CSV, and XLSX formats are implemented.
- Periodized export (BGN/EUR split at `2025-12-31T22:00:00Z` boundary) is correctly implemented in `exports.go:147–158` using a half-open `[from, to)` interval.

**EUR Currency Adoption**
- `bgEuroAdoption` pinned at `exports.go:48` to the correct legal instant `2025-12-31T22:00:00Z UTC` (= 2026-01-01T00:00:00 Europe/Sofia).
- Policy enforces `currency = 'EUR'` in DB (`013_supto_identifier_foundation.sql:8`).

**Data Integrity / Immutability**
- `sale_events`, `operator_security_events`, `regulatory_identifier_bindings`, `unp_allocations` are all append-only with both DB triggers and RLS enforcement.
- FORCE ROW LEVEL SECURITY on all sensitive tables ensures the app role cannot bypass tenant isolation even with explicit SQL.

---

### 1.2 Non-Compliant / Missing / Risk Areas

#### P1 — Payment Types Incomplete (N-18 Art. 3(2), Art. 3(7), Art. 3(15); App. 29 §10)

**File:** `fiscal-backend/internal/domain/service.go:1010`
```go
!contains([]string{"CASH", "CARD"}, p.Type)
```
Наредба N-18 mandates support for:
- `резерв 1 — НЗОК` (NHIF pharmacy payments, Art. 3(15))
- `резерв 2 — вътрешно потребление` (own-consumption of fuel, Art. 3(7))
- `резерв 1 — отложено плащане` / `резерв 2 — отложено плащане` (deferred payment in ИАСУТД, Art. 25(5))
- Cheque (`чек`), voucher (`ваучер`) — required payment tender types per Appendix 1

The current domain only handles CASH and CARD. The edge-agent and FU driver layer may support more types, but the core business logic rejects any non-CASH/CARD payment at the entry point. This blocks compliance for pharmacy, fuel, and deferred-payment use cases.

**Recommendation:** Add an extensible payment type enum validated against a policy catalog (similar to `TaxGroup`), not a hardcoded two-element list.

#### P2 — SUPTO 18.1 Export Fields Incomplete (App. 29 §18.1)

**File:** `fiscal-backend/internal/domain/exports.go:260–261`

Current SUPTO_18_1 export headers:
```
sale_id, external_id, unp, location_id, register_id, operator_id, state, created_at
```

Missing mandatory fields per Приложение №29, т. 18.1:
- `дата на приключване на продажбата` (completion timestamp — `completed_at`)
- `време на приключване` (completion time HH:MM:SS)
- `дължима сума по продажбата` (total due)
- `обща сума без ДДС` (net amount without VAT)
- `ДДС — сума` (VAT amount)
- `отстъпка` (total discount)
- `код и наименование на търговски обект` (location code and name, not just ID)
- `код на работно място` (workstation/register code, not just ID)
- `код на оператор` (operator code)
- `фактура — номер / дата` (invoice number and date, where applicable)
- `клиент код / клиент име` (customer code/name, where available)

Similarly, SUPTO_18_4 (`exports.go:269`) maps to "storno" data but outputs only `sale_id, unp, fiscal_operation_id, receipt_artifact_id, fiscal_device_json, state` — it does not match the App. 29 §18.4 storno table format at all (missing: `operator_code`, reversal timestamps, FU number, product lines).

**SUPTO_18_2** (`exports.go:263`) is missing `платена сума без ДДС` and `ДДС сума` in individual rows (outputs combined `amount_json` only, which is permissible per App. 29 §18.2 footnote — but the footnote requires the combined field to be labeled "Платена сума - в лв.", not a JSON blob).

#### P3 — Only Tax Group "B" (20%) Seeded (N-18 Art. 26 §1.7; Policy Catalog)

**File:** `fiscal-backend/internal/domain/policy.go:46`
```go
groups: []TaxGroup{{Code: "B", Rate: "20.00", ValidFrom: from, PolicyVersion: version}},
```

The policy catalog intentionally seeds only group B. Bulgaria uses tax groups A (0%), B (20%), C (9%), D (0% for certain exemptions), and historically E-H for specialized categories. Allowing only group B means:
- Zero-VAT sales (food, medicines, books) cannot be processed — these are mandatory under N-18.
- Reduced-VAT (9%) hospitality/accommodation sales are blocked.
- Any product with a non-B tax group will be rejected at `service.go:907`.

The comment says "other A-H mappings must enter through a reviewed policy update." This is a conscious design decision, but it means the system cannot handle common BG VAT rates in production until this is unblocked.

#### P4 — Receipt Fields Missing on API Response (N-18 Art. 26 §1)

**File:** `fiscal-backend/internal/domain/service.go:1272`

The `receiptForTenant` return map includes: `sale_id, operation_id, unp, state, fiscal_reference, issued_at, total, artifact_id, fiscal_device, fiscal_device_number, fiscal_memory_number, lines, payments`.

Missing mandatory receipt fields per N-18 Art. 26 §1:
- Merchant name and correspondence address (`наименование и адрес на лицето по чл. 3`)
- Location name and address (`наименование и адрес на търговския обект`)
- Merchant EIK (`ИН по ДОПК`)
- VAT registration number (`ИН по ЗДДС`)
- Cashier name or number (the receipt returns `operator_id` — a code, which is acceptable, but the name should also be available)
- Sequential receipt number (`пореден номер на касовата бележка`) — `fiscal_reference` is the FU-assigned document number, which serves this purpose, but it is device-side only
- QR code data (Art. 26 §1.16) — not included in receipt artifact
- Fiscal logo indicator (Art. 26 §1.10) — not emitted

The receipt artifact is a backend JSON structure, not a printed receipt, so some of these are generated by the FU hardware. However, the API receipt endpoint is used by POS clients to reconstruct and display/forward the receipt. Missing the merchant address, EIK, and VAT number means the POS cannot render a legally compliant receipt image from the API alone.

#### P5 — CORS Wildcard in Production Compose (Security / Regulatory Config)

**File:** `compose.fiscalisation.prod.yaml:19`
```yaml
CORS_ALLOWED_ORIGINS: "*"
```

With `APP_ENV: prod` on line 16, `Config.Validate()` at `config.go:48` will reject this:
```go
if c.AppEnv == "prod" && (c.CORSAllowedOrigins == "*" || !secureOrigins(c.CORSAllowedOrigins)) {
    return errors.New("CORS_ALLOWED_ORIGINS must be an explicit HTTPS origin list in PROD")
}
```
**The service will fail to start in production.** This is a deployment blocker.

#### P6 — Operator "Full Name" Validation Incomplete (App. 29 §6)

**File:** `fiscal-backend/internal/domain/admin.go:177`

App. 29 §6 requires "най-малко две имена" (at minimum two names). The current validation only checks `first_name != ""` and `last_name != ""` — it does not validate minimum character length or prohibit purely numeric/symbol names. This is a minor compliance gap but should enforce that each name component contains at least one letter.

#### P7 — No Data Transmission to НАП (N-18 Art. 7(3), Art. 25(9))

Наредба N-18 Art. 7(3) forbids operation "without an established remote connection to НАП." Art. 25(9) requires automatic data transmission to НАП with each fiscal receipt via the "дистанционна връзка."

The project relies on the FU hardware (Datecs/Daisy) to manage the NАП GPRS/TCP channel — this is appropriate for a СУПТO. However, there is no evidence in the backend code that the system verifies the NАП transmission status from the FU or that it blocks sales when the FU cannot communicate with НАП. This is classified as `EXTERNAL_BLOCKED` in `bg-requirements-trace.json` (BG-013), but it should be clearly documented as a gap that requires HIL evidence.

#### P8 — Training Mode Receipt Prohibited (App. 29 §20; N-18 implicit)

The requirement at App. 29 §20 explicitly states "Softuerat ne pritezhava vazmozhnost za rabota v testovi rezhim, rezhim za obuchenie ili drug podoben." The production guard exists (`config.go:39–40`), but the `Simulator.Execute()` in `service.go:55–68` returns realistic-looking fiscal references (7-digit numbers derived from the operation SHA-256). If `ALLOW_STUB_ADAPTERS=true` leaks into a non-dev deployment, simulated receipts would be indistinguishable from real ones at the API level. The guard in `Validate()` is the only protection.

#### P9 — Z-Report Closure Not Integrated with Sale Blocking (N-18 Art. 30–33)

**File:** `fiscal-backend/internal/domain/service.go:335`

The Z-report operation type is accepted (`"Z"` in the allowed list), and the operation is routed to the FU driver. However, there is no domain logic that:
- Records the Z-report sequence number or daily financial total
- Blocks new sales if the preceding business day's Z-report was not performed
- Tracks which operator performed the Z (N-18 Art. 34 requires cashier identification on Z-reports)

The Z-report trigger is entirely delegated to the FU. For a СУПТO, the software should enforce that a Z-report is performed before crossing a business-day boundary.

---

### 1.3 Unclear / Needs Verification

- **App. 29 §9.1 — UNP generation moment:** The spec defines "the moment of entering the first item in the screen form" as the UNP generation trigger. The code generates UNP in `OpenSaleWithFirstLine` which is the correct atomic operation, but the Appendix also allows UNP generation "at the moment the data is saved to DB." The current implementation delays UNP assignment until a separate `OPEN_SALE_WITH_FIRST_LINE` event — need to verify whether the UNP is visible in the UI at the exact moment of DB record creation or on the response round-trip.

- **App. 29 §5 — Reliable time source:** The spec requires "по възможност надежден източник на точно астрономическо време" (where possible, a reliable astronomical time source). The system uses `time.Now().UTC()` — NTP sync is assumed to be an OS-level concern. This needs infrastructure documentation confirming NTP is configured on production hosts.

- **N-18 Art. 26 §4 — Receipt currency:** Art. 26(4) states "всички регистрирани и натрупвани суми се изразяват във валутата на Република България" (all amounts shall be expressed in the currency of the Republic of Bulgaria). Since 01.01.2026, the official currency is EUR. The system correctly uses EUR for all amounts. Verified compliant.

- **App. 29 §22 — Import of sales via Annexes 41/42:** The edge sync pathway (`edge_sync.go`) handles offline sale import. The regulatory linkage between the offline UNP authority and Annexes 41/42 is not fully documented in code comments. The `supto-annex29-trace.json` marks this as `production_blocked: true`.

---

## 2. Code Quality

### 2.1 Critical Issues

#### C1 — Production Startup Failure (CORS + prod config contradiction)

**Files:** `compose.fiscalisation.prod.yaml:19`, `config.go:48`

As noted in Executive Summary — the prod compose file sets `CORS_ALLOWED_ORIGINS: "*"` but the validator rejects wildcards in prod. The service will log `CORS_ALLOWED_ORIGINS must be an explicit HTTPS origin list in PROD` and exit with `log.Fatal`. Fix: set the correct origin list in `compose.fiscalisation.prod.yaml`.

#### C2 — Race Condition in Sale Total Calculation (Go)

**File:** `fiscal-backend/internal/domain/service.go:1046–1062`

In `payForTenant`, the paid amount is summed from `sale.Payments` (the in-memory snapshot loaded before the payment reservation):
```go
for _, x := range sale.Payments {
    ...
    paid += v
}
if amount <= 0 || paid > total || amount > total-paid {
    return Operation{}, errors.New("payment amount exceeds balance")
}
```

The `ReserveSalePayment` function then writes a new DB row. Between the read and the reservation, a concurrent payment could be accepted. The PostgreSQL persistence layer uses optimistic locking via version numbers (`ReserveSalePaymentExpected`), but the in-memory `MemoryRepository` does not serialize across goroutines for the payment step — the `payment_test.go:TestConcurrentPaymentDoubleSpend` test covers the concurrent case for the memory repo. This should be verified with load testing against the Postgres path.

#### C3 — Error Shadowing in reversal path

**File:** `fiscal-backend/internal/domain/service.go:260–273`

```go
op := Operation{..., Simulated: true, ...}
if queued, ok := driver.(durableQueuedDriver); ok {
    command, prepareErr := queued.Prepare(op, sale, PaymentRequest{})
    if prepareErr != nil {
        return Operation{}, prepareErr
    }
    sale, e = s.repo.ReserveSaleReversalCommand(...)
    if e != nil {
        return Operation{}, e
    }
    if e = queued.Publish(command); e != nil {
        return op, nil  // <-- returns partially-committed op with no error
    }
    return op, nil
}
```

When `queued.Publish` fails, the function returns `op, nil` — a success response to the caller despite the publish failure. The intent is that the command is durable and will be republished, but callers receive no indication the fiscal operation is in a pending/uncertain state. The `op.State` is `"EXECUTING"` at this point — the caller should treat this as pending, but this is a subtle API contract that could be misunderstood.

#### C4 — Unchecked JSON marshal errors in exports

**File:** `fiscal-backend/internal/domain/exports.go:263–267`

```go
amount,_:=json.Marshal(payment.Amount)
...
price,_:=json.Marshal(line.UnitPrice)
discount,_:=json.Marshal(line.Discount)
```

JSON marshal errors are silently discarded. While `json.Marshal` rarely fails on struct types, this pattern means corrupted export data could be silently produced. Use proper error handling.

#### C5 — No request body size limit for edge sync batches

**File:** `fiscal-backend/internal/api/handler.go` (edge sync endpoint)

The `decodeStrict` function applies `io.LimitReader(w, r.Body, 1<<20)` (1 MB). Edge sync batches can theoretically contain many days of offline operations. If a device accumulates a large backlog, this limit could cause legitimate syncs to fail silently. The limit should be configurable or documented.

---

### 2.2 Significant Issues

#### S1 — Single Tax Group in Production Policy

As noted under P3, only tax group B (20%) is seeded. This is intentional but blocks production deployment for any non-standard VAT rate. The gateway to adding new groups needs to be clearly defined (presumably a policy PR with the source hash).

#### S2 — `exports.go` Code Style: Dense One-Liners

**File:** `fiscal-backend/internal/domain/exports.go:243, 261, 262, 263–267, 281, 288–293`

Many functions are compressed to single lines with multiple statements, making code review and error tracking difficult:
```go
headers=[]string{...};for _,r:=range rows{records=append(...)}
```
This style is inconsistent with the rest of the codebase and makes it hard to add error handling or debugging instrumentation.

#### S3 — `ValidateReceiptPlan` does not validate payment type

**File:** `fiscal-backend/internal/domain/receipt_saga.go:66–86`

`ValidateReceiptPlan` checks that payments are non-empty UUIDs with positive EUR amounts, but does not validate the payment `Type` field. A receipt plan with `type: "INVALID"` would pass validation here and only fail at the driver level.

#### S4 — Memory Repository Used in Production Path When DB URL Is Empty

**File:** `fiscal-backend/cmd/fiscal-backend/main.go:28–47`

If `DATABASE_URL` is empty (e.g., misconfigured prod deploy), the service silently falls back to an in-memory repository. All data is lost on restart. The `config.go:51` check requires `DATABASE_URL` in prod, so this would be caught by `Validate()`, but only if `APP_ENV=prod` is set. Any non-`prod` deployment without a DB URL will silently use memory storage.

#### S5 — UNP Sequence Reset Risk on 9,999,999

**File:** `fiscal-backend/internal/domain/regulatory_identifier.go:16`, `persistence/postgres.go:75`

`BGUNPSequenceMax = 9_999_999`. When the sequence is exhausted for a given FMIN, `AllocateUNP` returns an error. There is no alert, no graceful blocking, and no mechanism to rebind to a new FMIN. If a high-volume business reaches the limit, new sales are blocked with an opaque error. App. 29 §9 allows defining new ranges in multi-SUPTO scenarios — this should be handled with a warning threshold and management notification.

#### S6 — BLE Session Security Mode Labeled "OPEN_MVP"

**File:** `fiscal-backend/internal/domain/service.go:659`

```go
"security_mode": "OPEN_MVP",
```

This string is returned in the BLE session response and may be read by the edge agent or POS to select encryption parameters. Leaving a production-visible field labeled "OPEN_MVP" is a risk — a future change in security mode might not update this label, or a security reviewer might flag it as an incomplete implementation.

---

### 2.3 Minor Issues

#### M1 — `newID` generates UUIDs with a fixed `prefix-` prefix format

**File:** `fiscal-backend/internal/domain/service.go:21–26`

`newID("sale")` generates `sale-XXXXXXXX-XXXX-...` — not a standard UUID v4. This is intentional for human readability, but callers outside the domain (e.g., OpenAPI validators expecting `format: uuid`) may reject these IDs. The `apiUUIDPattern` at `handler.go:51` expects standard UUID format, suggesting these domain IDs are not the same identifiers exposed in the API.

#### M2 — `contains` function not using generics (Go 1.21+)

**File:** `fiscal-backend/internal/domain/service.go:1447–1453`

Go 1.21+ has `slices.Contains`. The hand-rolled version is fine but could be replaced for clarity.

#### M3 — `compose.fiscalisation.yaml` has `sslmode=disable` in DATABASE_URL

**File:** `compose.fiscalisation.yaml:58`

```
DATABASE_URL: "postgres://fiscal:...@postgres:5432/fiscal?sslmode=disable"
```

The prod override correctly uses `sslmode=require`. However, the base compose file's `sslmode=disable` can accidentally be used in staging/QA environments that do not apply the prod overlay. Consider using `sslmode=prefer` as the base default.

#### M4 — XLSX generator uses column letters `'A'+j` which overflows beyond column Z

**File:** `fiscal-backend/internal/domain/exports.go:331`

```go
fmt.Sprintf(`<c r="%c%d"`, 'A'+j, i+1, ...)
```

If `j >= 26`, the column reference character wraps past `'Z'` into non-letter characters (e.g., `'['`). SUPTO_18_1 has 8 columns; the max SUPTO type has ~13 columns. This is within range for current exports but will silently corrupt output if a new export type with >26 columns is added.

---

## 3. Test Coverage

### 3.1 Well-Covered Areas

- **UNP generation and uniqueness:** `regulatory_identifier_test.go` and `unp_allocator_integration_test.go` cover format validation, monotonicity, and 128-way concurrent uniqueness.
- **Reversal policy:** `reversal_policy_test.go` covers all reason codes, timezone boundary (Europe/Sofia), and deadline calculation.
- **Payment lifecycle:** `payment_test.go` covers split payments, overpayment rejection, exact total, concurrent double-spend, and FISCAL_RESULT_UNKNOWN handling.
- **Auth/RBAC:** `handler_test.go` and `auth_test.go` cover role-based access for all major endpoints.
- **Readiness lease:** `readiness_time_test.go` covers 2-hour expiry, FMIN mismatch, and signature verification.
- **BLE session:** `ble_test.go` covers full ticket lifecycle including future/past operator rejection and revocation.
- **Periodized export:** `periodized_export_test.go` and `exports_test.go` cover the BGN/EUR boundary, half-open interval, and currency mismatch rejection.
- **Document policy:** `document_policy_test.go` covers banned wording regex and customer document class enforcement.

### 3.2 Coverage Gaps

- **Payment type validation:** No tests for non-CASH/CARD payment type rejection or future expansion. No tests verify that the payment type is transmitted to the FU driver correctly.
- **SUPTO_18_1 through _9 field completeness:** `exports_test.go` tests that exports are produced, but no test verifies that all mandatory App. 29 §18 fields are present with correct values. This gap means the missing fields identified in P2 could go undetected.
- **Z-report blocking behavior:** No test verifies that attempting to open a sale when a Z-report is overdue is blocked (because this logic does not exist in the domain).
- **CORS wildcard rejection in prod:** No test in `config_test.go` covers the `CORS_ALLOWED_ORIGINS = "*"` + `APP_ENV = prod` combination.
- **Receipt artifact fields:** No test validates the full `receiptForTenant` response against N-18 Art. 26 §1 mandatory fields.
- **UNP sequence exhaustion:** No test covers behavior when `BGUNPSequenceMax` is reached.
- **Operator name minimum character validation:** No test checks that a 1-character first/last name is rejected.
- **Edge sync large payload handling:** No test for the 1 MB body limit with large batches.

---

## 4. API Contracts

### 4.1 Issues Found

**`contracts/generated/openapi-public-v1.d.ts` — Payment type enum not enforced**

The OpenAPI contract for payment requests should enumerate the allowed payment types. If the backend only accepts `"CASH"` and `"CARD"`, the API contract should reflect this. Currently, the TypeScript types at `miniposweb/src/types.ts:12` define `type: "CASH" | "CARD"` — consistent with the implementation but not documented in the OpenAPI spec as a constraint.

**`contracts/bg-requirements-trace.json` — BG-003 (receipt fields) marked PARTIAL**

BG-003 evidence states "gap: vendor golden printed receipt comparison." The backend receipt API response is missing the merchant address and EIK fields (P4 above). The partial status is acknowledged but the gap description underestimates the scope — it is not just a "vendor comparison" issue; the API response genuinely lacks regulatory required data.

**`contracts/supto-annex29-trace.json` — 19 of 23 requirements marked `production_blocked: true`**

This is a faithful self-assessment. The most significant external blockers are:
- SUPTO-29-04: Secure boot / signed release (external audit)
- SUPTO-29-07: Real passwordless IdP deployment
- SUPTO-29-10: Physical payment/receipt HIL
- SUPTO-29-14: Annex 29 external catalog review

All are correctly identified. No false-positive PASS status was found in the trace.

**`contracts/openapi-runtime-v1.yaml` — UNP field not marked required in all sale responses**

The runtime OpenAPI spec should enforce that `unp` is always present in a completed sale response. If a sale is completed without a UNP (edge case in the pre-SUPTO path), the POS would have no regulatory identifier to display.

---

## 5. Infrastructure & Security

### 5.1 Issues Found

**SEC-1 (Critical): `CORS_ALLOWED_ORIGINS: "*"` in prod compose**
See C1 above. This is both a security vulnerability (allows cross-origin requests from any domain to the fiscal API) and a deployment blocker (config validation rejects it).

**SEC-2 (High): `sslmode=disable` in base compose DATABASE_URL**
`compose.fiscalisation.yaml:58` uses `sslmode=disable` as a default. In a staging environment that does not apply the prod overlay, DB connections are unencrypted. Change the base default to `sslmode=prefer` or `sslmode=require`.

**SEC-3 (Medium): No rate limiting on device bootstrap endpoints**
`/device-bootstrap/v1/challenges` and `/device-bootstrap/v1/activation-requests` are outside the auth middleware (they must be, as they are used before credential issuance) but they have no rate limiting. A bot could enumerate `device_instance_id` values or exhaust challenge tokens.

**SEC-4 (Medium): Readiness lease signing key minimum length = 16 bytes**
`service.go:415`:
```go
if len(s.bleSigningKey) < 16 {
    return ReadinessLease{}, errors.New("readiness signing unavailable")
}
```
The prod config check at `config.go:78` requires `len >= 32`. The domain check at 16 is a weaker fallback. In dev environments (where prod validation doesn't run), a 16–31 byte key would be accepted for cryptographic signing — insufficient for HMAC-SHA256 security.

**SEC-5 (Medium): No TLS certificate pinning for FU MQTT connection**
`DeviceMQTTTLSURI` is validated as `ssl://...` in prod, and the device CA cert is mounted. However, the MQTT client implementation in `mqttclient/client.go` should be verified to perform certificate chain validation against the device CA, not just TLS handshake. If the MQTT client trusts the system certificate pool rather than the custom device CA, a compromised intermediate CA could issue a fraudulent FU certificate.

**SEC-6 (Low): `AUTH_HMAC_KEY` described as "for BeeMiniPOS tokens" in prod config comment**
`config.go:75` error message: `"strong AUTH_HMAC_KEY required in PROD for BeeMiniPOS tokens"`. This key is actually used for all API token signing — not just BeeMiniPOS. The comment is misleading and could lead operators to believe the key has a narrower scope than it does.

**INF-1 (Medium): No readiness probe for EMQX in compose**
`compose.fiscalisation.yaml:60`: `depends_on: {emqx: {condition: service_started}}`. EMQX takes longer to initialize its auth/ACL configuration than "started." The fiscal-backend may attempt to subscribe to MQTT topics before EMQX has loaded the JWT plugin, resulting in silent dropped subscriptions and missed device commands.

**INF-2 (Low): Outbound email worker uses fire-and-forget `RunEmailWorker`**
`main.go:105`: `go integrationService.RunEmailWorker(...)`. If the email worker panics, it silently terminates. Fiscal notification emails (e.g., activation confirmations) would stop without any alert. The worker should have a recover/restart wrapper.

---

## 6. Prioritized Action Items

1. **[DEPLOYMENT BLOCKER]** Fix `compose.fiscalisation.prod.yaml:19` — replace `CORS_ALLOWED_ORIGINS: "*"` with the actual production HTTPS origin(s). Without this fix the service cannot start in prod.

2. **[REGULATORY CRITICAL — App. 29 §10, N-18 Art. 3]** Expand payment type support beyond CASH/CARD. Add at minimum: deferred payment (резерв 2), NHIF (резерв 1 — НЗОК), voucher, and cheque. Update `service.go:1010`, `ValidateReceiptPlan`, and all downstream schema validators.

3. **[REGULATORY HIGH — App. 29 §18.1–18.5]** Redesign SUPTO export tables to include all mandatory fields from Приложение №29, т. 18. Priority: 18.1 (close timestamp, due amount, VAT, discount, client data), 18.4 (reversal format redesign). Add automated field-completeness tests.

4. **[REGULATORY HIGH — App. 29 §3; N-18 Art. 26]** Add merchant details (name, address, EIK, VAT number) to the receipt API response so POS clients can render a legally compliant receipt without requiring a secondary API call.

5. **[REGULATORY HIGH — Policy Catalog]** Unblock tax groups A (0%), C (9%), and D (specific zero-rate) by completing the reviewed policy update process. Document the evidence source and SHA256 for each new entry. Without this, the system cannot process zero-VAT or reduced-VAT sales.

6. **[REGULATORY MEDIUM — N-18 Art. 30–33]** Implement Z-report business day enforcement in the domain: record Z-report timestamp, block sales that cross a business day without a completed Z, and associate the Z with the responsible operator.

7. **[SECURITY MEDIUM]** Add rate limiting to `/device-bootstrap/v1/` endpoints to prevent challenge token exhaustion and serial enumeration attacks.

8. **[SECURITY MEDIUM]** Change `compose.fiscalisation.yaml` base `DATABASE_URL` from `sslmode=disable` to `sslmode=prefer` to prevent unencrypted DB connections in staging environments.

9. **[CODE QUALITY — exports.go]** Refactor dense single-line export functions with proper error handling (fix C4 — silent JSON marshal errors in export CSVs). Add error returns from all record-building inner loops.

10. **[TESTING]** Add tests for: SUPTO export field completeness against App. 29 §18 field list, UNP sequence exhaustion behavior, CORS wildcard rejection in prod config, payment type expansion, and receipt API field completeness.

11. **[MONITORING]** Add a UNP sequence utilization metric (e.g., Prometheus gauge) and alert at 90% of 9,999,999 per FMIN. Ensure management is notified before sequence exhaustion blocks new sales.

12. **[INFRASTRUCTURE]** Verify EMQX compose healthcheck uses `service_healthy` with a proper health check command that confirms the JWT auth plugin is active, not just `service_started`.

13. **[DOCUMENTATION]** Clarify the security scope of `AUTH_HMAC_KEY` in config comments. Document that NTP is a required infrastructure dependency for App. 29 §5 compliance.

---

*Review performed against: `fiscal-backend/internal/domain/` (90 Go files), `database/fiscal/` (19 SQL migrations), `contracts/` (traceability matrices), `compose.fiscalisation.prod.yaml`, `docs/SUPTO/Приложение № 29.md` (App. 29 full text), and `docs/SUPTO/Naredba N-18 2006 canonical clean.md` (N-18 Art. 1–52, Art. 25–26, Art. 30–34 reviewed). Frontend POS code reviewed for payment type surface area.*
