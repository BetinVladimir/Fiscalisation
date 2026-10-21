# N5/N6: consistent tenant-scoped Fiscal PostgreSQL financial snapshot

This executable component covers six authoritative Fiscal runtime financial tables. It is not a full cloud backup, normative export approval or proof of НАП readiness. Test fixtures are synthetic, never real physical-device/legal evidence. No production database was accessed.

## Existing implementation and chosen boundary

Both `fiscal-backend/internal/persistence/postgres.go` and `minipos/beeminipos-backend/internal/persistence/postgres.go` use independent PostgreSQL/pgx databases, not SQLite. Fiscal `domain/exports.go` offers application-level reporting exports, but creates artifact/resource/operation records; it is not a read-only acquisition method. `persistence.Open` also executes startup DDL. This standalone `cmd/dossier-export` invokes neither path.

Included complete rows: fiscal_runtime_sales, fiscal_runtime_operations, fiscal_runtime_shifts, fiscal_runtime_artifacts (actual bytea bodies), fiscal_runtime_sync_acks and fiscal_runtime_unp_sequences. Exact current columns, constraints, indexes and RLS policy expressions appear in `22_FISCAL_PG_DICTIONARY.generated.json`, derived from an actual PostgreSQL 16.10 instance initialized with repository `database/fiscal` migrations. Input hashes are `23_FISCAL_PG_SCHEMA_SOURCES.generated.json`. Fixtures contain no credentials beyond disposable database test accounts.

Explicitly excluded: generic resources (can contain device credentials), session/auth/bootstrap/activation stores, replay/outbox/audit generic payloads pending a secret-safe contract, outbound email, compatibility state rows/meta, legacy/reporting-model tables, external object-store bodies and MiniPOS/local/browser/native databases. Their absence is not proof of emptiness. Excluding generic audit payloads is an open dossier gap; no claim of complete audit extraction is made. Embedded financial JSONB is exported unredacted as business evidence and must receive dossier access controls; credentials must never be written to these financial records.

## Transaction and isolation contract

Export opens exactly one REPEATABLE READ READ ONLY transaction. Snapshot ID and transaction timestamp are captured before the first table export; every table shares that MVCC snapshot. Concurrent committed writes remain visible to later transactions, while this export retains the earlier consistent state. No application endpoints, advisory allocation, journal append, receipt execution, migration, INSERT/UPDATE/DELETE or persistent session change is invoked. Local tenant/timezone/date/bytea formatting settings apply only to this transaction.

Connection must be a restricted non-owner, non-superuser, non-BYPASSRLS reader. The six tables must enable and FORCE RLS. Tenant is set transaction-locally and an explicit tenant predicate is also applied to every query. A reviewed exact schema/policy contract is compared inside the same transaction; drift, missing table or disabled RLS refuses the whole extraction. Dedicated SELECT-only credentials are recommended: the runtime `beefiscal_tenant` role has write grants, although this tool's transaction prohibits writes. The tool does not create users/grants or require a production owner connection.

The supplied dictionary names `public`, matching the actual repository Docker migration path. Runtime SQL also contains `fiscal.` qualifications; these namespaces must not be assumed interchangeable. The CLI requires explicit deployed schema (`fiscal` default or `public`). A fiscal-schema candidate requires a separately reviewed dictionary generated on its clean migration fixture, never by learning the live candidate schema and immediately trusting it. Contract generation has no data export and is for disposable migration fixtures only.

## Format and provenance limits

Each table is newline-delimited JSON arrays in dictionary column order. Every nonnull cell is PostgreSQL's type-specific text output, represented as a JSON string; SQL NULL is JSON null. Bigint and numeric remain decimal strings, bytea uses hex `\x...`, JSONB is PostgreSQL canonical semantic text, timestamps UTC/ISO. This preserves database values, not original JSON lexical bytes, physical page layout or original client formatting. No generic cast to float64 occurs. Rows are ordered by the complete textual column tuple; equal duplicate rows have identical output. The dictionary is necessary to interpret types or implement a controlled replay; this exporter does not execute restore SQL.

Manifest includes tenant/schema, MVCC snapshot ID, transaction timestamp, declared source commit, per-file SHA256/counts and exclusions. Snapshot ID is descriptive, not an exported snapshot that survives transaction end. Source commit and acquisition provenance are declarations, not device/server signatures. Generated schema input hashes aid review but do not independently prove deployed binary identity.

Output must be new, in an operator-controlled parent directory. Files/directories receive 0600/0700 permissions. Data stages privately; no-clobber output reservation happens after DB reads complete; manifest publishes last. Existing outputs/symlinks are refused. Failures roll back and remove incomplete tool-created outputs. Long snapshot durations can retain MVCC tuples: schedule authorized runs and review operational load. The tool's context deadline is ten minutes.

Archive verification checks exact filenames, regular nonsymlink files, hashes, table/column/row inventory and dictionary linkage. It does not authenticate a maliciously rewritten manifest. Preserve original capture, independent hashes, custody record and externally reviewed dossier evidence.

## Reproduce without production credentials

From repository root:

```sh
sh scripts/dossier-postgres-integration.sh
cd fiscal-backend
go build -o /private/tmp/fiscal-dossier-export ./cmd/dossier-export
```

The runner creates a new isolated PostgreSQL16.10 container, mounts actual migrations read-only, supplies synthetic tenant/financial data, runs integration tests and removes only its own container. It never reuses existing developer or production databases. Running `go test` without both DOSSIER_TEST_ADMIN_URL and DOSSIER_TEST_READER_URL skips actual integration; that is not acceptance. Use the runner for real verification.

Authorized acquired database/replica extraction (URL provided securely by the execution environment, never committed, echoed or passed as CLI argument):

```sh
/private/tmp/fiscal-dossier-export -schema public -tenant TENANT_ID \
  -source-commit CAPTURED_SOURCE_COMMIT \
  -contract docs/remediation/2026-10-17/22_FISCAL_PG_DICTIONARY.generated.json \
  -output /evidence/new-financial-snapshot
/private/tmp/fiscal-dossier-export -verify /evidence/new-financial-snapshot
```

The first command reads DOSSIER_DATABASE_URL. Avoid shell tracing and process/environment disclosure. Verification requires no database connection. A new dictionary for a clean reviewed migration fixture uses `-dictionary -schema ... -output NEW_DICTIONARY.json`; never overwrite the reviewed candidate contract to bypass drift.

## Acceptance, cross-store IDs and remaining coverage

Actual PostgreSQL integration verifies two-tenant isolation, snapshot state before a second connection commits, exact bigint >2^53 and JSONB integer near int64 max, binary body preservation, readonly write rejection, source unchanged by ordinary export, superuser rejection, new-output enforcement, schema drift refusal, archive tamper and committed dictionary reproducibility. No physical fiscal/POS proof is fabricated.

Use store namespace plus tenant plus operation/sale ID for reconciliation. Runtime sale ID and fiscal_operation_id can link to backend operation; receipt_artifact_id links to tenant-scoped runtime bytea artifact. Original fiscal reference/reversal references must retain their meanings. S3 client_operation_id, BlueCash operation_id and MiniPOS checkout/order IDs are joined only through explicit actual payload associations, not equal-looking strings. Local event sequence, backend ACK sequence, UNP counter and row version are independent counters. UNKNOWN cannot become success through export. See doc19 for local/mobile/browser inventory.

Still OPEN: MiniPOS stores beyond doc24's seven-table allowlisted profile; complete legacy/reporting tables and secret-safe resource/audit extraction; external artifact/object bodies and retention gaps; cross-store reconciliation implementation; captured source/build identity and externally authenticated provenance; operational read-only account provisioning and replica-lag declaration; real capture rehearsals and dossier/legal review. Hybrid N5/N6 remains incomplete until these are addressed.

## Integrated verification — 2026-10-18

On the main working tree, `make nap-postgres-extraction-test` completed with exit code 0 against a disposable PostgreSQL 16.10 instance. `TestPostgresSnapshot` and all eight subtests passed, including tenant/bigint/blob isolation, privileged and owner refusal, disabled RLS, no-clobber, schema drift and read-only behavior. `TestMigrationSourceInventory` also passed. Execution log: `/private/tmp/fiscal-pg-export-integrated-20261018.log`. This establishes the six-table software extraction boundary above; the remaining coverage stays OPEN.
