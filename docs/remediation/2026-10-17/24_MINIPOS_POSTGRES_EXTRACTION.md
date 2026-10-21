# N5/N6: MiniPOS PostgreSQL allowlisted financial projection

Status: executable tenant-scoped read-only projection of seven actual MiniPOS runtime tables, with actual disposable PostgreSQL tests. This extends the separate Fiscal financial exporter; neither profile is a full cloud backup or legal/NAP sufficiency determination. Synthetic fixtures are not actual device or external review evidence.

## Actual source and ownership boundary

MiniPOS uses `minipos/beeminipos-backend/internal/persistence/postgres.go`, separate PostgreSQL tables and credentials from Fiscal. Runtime rows use `organization_id` and `app.organization_id`, role `beeminipos_tenant`; Fiscal uses `tenant_id` and `app.tenant_id`, role `beefiscal_tenant`. These namespaces must not be substituted or joined merely because identifier strings resemble each other. The exporter never calls either backend's `persistence.Open`, migrations, domain mutation or checkout endpoint.

The existing standalone Go snapshot engine now accepts explicit `-profile minipos`. Default `fiscal` retains its existing format/dictionary behavior. MiniPOS uses distinct archive/schema formats (`beeminipos-financial-pg-extract-v1`, `beeminipos-financial-pg-schema-v1`), table inventory, tenant setting/predicate and exclusions. There is no import or runtime call into MiniPOS services. A Fiscal contract cannot authorize MiniPOS extraction, or vice versa.

`25_MINIPOS_PG_DICTIONARY.generated.json` derives from PostgreSQL16.10 initialized with all actual `database/minipos` migration files. `26_MINIPOS_PG_SCHEMA_SOURCES.generated.json` records those input SHA256 hashes. The complete inspected catalog for the selected tables includes every physical column, constraints, indexes and RLS expressions, including excluded columns. A new/changed excluded column still triggers schema-contract refusal.

## Allowlisted data and deliberate omissions

| Table | Exported business evidence | Excluded data |
|---|---|---|
| minipos_runtime_orders | Explicit order/org/external/shift/register/operator IDs, state, numeric total/currency, version/fiscal version, timestamps, fiscal sale/operation/reference and reversal evidence; projected lines and payments | Entire generic payload; unknown/nested JSON keys; payload-only execution/session/authorization associations |
| minipos_runtime_checkout_results | Explicit replay key, org/order/state/version, fiscal sale/operation/reference | Generic payload |
| minipos_runtime_checkout_hashes | Replay key, organization, request digest | No credential bodies |
| minipos_runtime_products | Explicit product identifiers, SKU/barcode/name/unit, numeric price/currency/tax group, status/version/timestamps | Generic payload |
| minipos_runtime_tax_groups | Explicit ID/org/code/name/rate/status/version/timestamps | Generic payload |
| minipos_runtime_employees | Explicit ID/org/name/operator code/status/version/timestamps | Generic payload and roles |
| minipos_runtime_shifts | Explicit shift/org/register/employee/state/version/open/close times and Z operation/reference | Generic payload and close_error |

`export_columns` defines row-array order; physical `columns` describe the database, not the projected row order. `json_projections` documents nested exported paths. Every included scalar is PostgreSQL typed text or SQL null, preserving bigint/numeric precision; no float64 conversion. Covered MiniPOS tables contain no bytea column, so no fake BLOB coverage is asserted; actual Fiscal bytea regression remains tested separately.

Order lines retain only line_id/product_id/name/quantity/tax_group plus unit_price and discount amount/currency. Payments retain payment_id/type/terminal_policy plus amount amount/currency. Unknown keys at every object level are discarded by an explicit projection. Known nonnull scalar values must be JSON strings; malformed money objects/scalars fail the entire export before publication. Optional absent keys remain absent; explicit null remains null; element order and exact known money strings remain. This is a normalized allowlisted projection, not an exact original JSONB or full-row backup. An unknown key is not inferred as harmless and does not silently expand future export scope. Ordinary business text can itself be sensitive; this is not a general free-text secret detector.

Excluded stores: identity_bindings/operator_sessions, configurations and adapter URLs, API replays, webhook inbox/outbox, compatibility rows/meta and legacy/reporting-model tables; Fiscal PostgreSQL and object bodies; S3/BlueCash/browser/native stores. Configurations and raw payloads are not bulk-exported as a shortcut. Employee names and fiscal business records require dossier access/retention controls. No Android/ESP32 keys or NVS are accessed; S3 keys remain inside ESP32-S3, with no ATECC dependency.

## Snapshot and acquisition contract

The engine uses one REPEATABLE READ READ ONLY transaction, local organization setting, FORCE RLS plus an explicit organization predicate on every selected table, non-owner/non-superuser/non-BYPASSRLS connection and exact reviewed schema/policy comparison. ACCESS SHARE locks prevent concurrent DDL while exporting. All seven tables share the same MVCC snapshot; later concurrent commits do not produce mixed generations. Local settings disappear with the transaction. Staged output is private, requires a new operator-controlled target directory and publishes manifest last. No database write or persistent configuration is performed.

A dedicated SELECT-only reader should be provisioned separately. The runtime role's existing write grants do not bypass this tool's READ ONLY transaction. Never pass production owner credentials. Source commit/acquisition provenance are declarations; snapshot ID is descriptive and does not survive transaction completion as a reusable exported snapshot. Operational account provisioning, authorized acquired replica selection and replica-lag declaration remain outside this tool.

Repository migration fixtures create `public` tables, while runtime SQL also uses `minipos.` qualifications. Select explicit deployed schema: `public` for the supplied fixture dictionary, `minipos` for a separately reviewed schema-specific fixture contract. The default `-schema fiscal` does not become MiniPOS automatically; wrong namespace fails closed. Do not generate a live candidate dictionary to bypass drift.

## Run and verify

From repository root, without production DB credentials:

```sh
make nap-minipos-postgres-extraction-test
make nap-postgres-extraction-test
cd fiscal-backend
go build -o /private/tmp/fiscal-dossier-export ./cmd/dossier-export
```

The first target creates a new disposable PostgreSQL16.10 database, applies all actual MiniPOS migrations read-only, seeds synthetic records, tests and removes its own container. The second rehearses the existing independent Fiscal profile. Without the corresponding integration environment variables, Go tests SKIP actual database checks; SKIP is not acceptance. Use the Docker runners.

Authorized extraction using a securely supplied DOSSIER_DATABASE_URL and reviewed acquired source/replica:

```sh
/private/tmp/fiscal-dossier-export -profile minipos -schema public \
  -tenant ORGANIZATION_ID -source-commit CAPTURED_SOURCE_COMMIT \
  -contract docs/remediation/2026-10-17/25_MINIPOS_PG_DICTIONARY.generated.json \
  -output /evidence/new-minipos-financial-snapshot
/private/tmp/fiscal-dossier-export -verify /evidence/new-minipos-financial-snapshot
```

The compatibility flag is named `-tenant`; with the MiniPOS profile it sets only app.organization_id and filters organization_id. The manifest format identifies this namespace. DSN must not be committed, echoed, shell-traced or supplied as a CLI argument. Verification opens no DB and checks exact inventory, hashes and projected row/column counts. It authenticates neither maliciously rewritten manifests nor deployed build identity. Generate a new dictionary only on a clean reviewed fixture, with `-profile minipos -dictionary -schema ... -output NEW.json`.

## Acceptance and cross-store reconciliation

Actual PostgreSQL tests verify org A/B isolation, concurrent updates to both order and checkout state while snapshot exports prior values, numeric 9007199254740993.17, bigint >2^53 and int64 max, known Unicode/money preservation, excluded payload/roles/error/API-replay/nested-key sentinels absent, malformed known money refusal, no source changes, readonly/no-clobber/schema/RLS guards, wrong-profile refusal and archive tamper. Dictionary/source inventory tests detect unreviewed changes. Existing Fiscal actual-PG tests must also pass after this shared-engine extension.

Correlate MiniPOS namespace + organization + order/external ID; typed fiscal_sale_id/fiscal_operation_id/fiscal_reference can link to Fiscal's tenant-scoped identifiers only with an explicit trusted organization-to-tenant mapping. Checkout replay key is idempotency correlation, not an authentication credential or unique cross-service operation by itself. Keep original/reversal references and shift Z evidence distinct. Raw payload-only receipt_session_id, execution_authorization_id, fiscal_register_id and checkout_operation_id are deliberately not inferred from generic data; a future explicit scalar projection requires its own reviewed contract/tests. Missing association is reported OPEN, never guessed.

Remaining hybrid gaps: complete audit/resource and legacy/reporting-model extraction with secret-safe projections, payload-only authority/session joins, external object bodies, browser/native allowlisted exports, executable cross-store reconciliation/missing-history reports, real capture rehearsals, authenticated provenance and independent dossier/legal review. No blanket full-cloud/N5/N6 completion or НАП readiness claim follows from these seven tables.

## Main-tree integrated verification — 2026-10-18

Both `make nap-minipos-postgres-extraction-test` and `make nap-postgres-extraction-test` completed with exit0 on separate disposable PostgreSQL16.10 containers. All five MiniPOS snapshot subtests and all eight Fiscal snapshot subtests passed, along with both migration inventories. Each runner intentionally skips the other database snapshot test; the two actual executions together provide profile coverage. Logs: `/private/tmp/fiscal-minipos-pg-integrated-20261018.log` and `/private/tmp/fiscal-pg-profile-regression-20261018.log`. This verifies the declared projections only; remaining dossier gaps above stay OPEN.
