# Typed audit projection v2: forward migration and historical reconciliation

## Scope and evidence

Migration `database/fiscal/022_runtime_typed_projection.sql` targets canonical `fiscal.*`, with explicit schema-qualified tables/types/functions and safe `fiscal,pg_temp` function search paths. It closes the silent omission of real `sale-...` IDs by017/021 and the obsolete operator/device JSON paths. Reference UUID columns become text with `USING column::text`; existing UUID values, indexes, keys and original append-only rows remain preserved. No prefix stripping, hash alias or reconstructed historical casing is used. Actual `operator_code` and `fiscal_device` paths take precedence. Legacy snapshots are accepted only when actual fields are absent; conflicts produce explicit diagnostics.

A positive integral snapshot version, exact sale/tenant identity and valid regulated codes are required. Partial EDGE events and malformed snapshots are append-only `typed_projection_diagnostics`, never fake version1 sale snapshots. Diagnostics are an explicit coverage limitation, not evidence that the complete event model is implemented. Existing country-profile default and optional client-surrogate mapping retain compatibility; this migration does not infer absent source metadata or certify Annex29 completeness.

Run `scripts/typed-projection-postgres-integration.sh`. It creates a separate PostgreSQL16.10 with synthetic fixtures in the actual `fiscal` namespace (legacy local DDL is mapped during fixture initialization), executes the real Service.OpenSaleWithFirstLine + repository AuditEvent serializer, proves017/021 omission and missing operator/device fields, applies022, and checks46 SQL assertions including diagnostic replay counts and oversized-range rejection before digest aggregation, future opaque IDs, UUID preservation, source immutability, append-only corrections, dry-run/apply/retry, digest rejection, time-zone stability, legacy conflicts, malformed versions and FORCE RLS/effective-view isolation. It also tests caller `search_path=public` with a shadow `public.sale_events` and verifies the runtime trigger points to `fiscal.project_runtime_audit_to_supto_events`, preventing a public-schema green result from being mistaken for the actual runtime schema. The runner removes only its own container and temporary directory. It does not connect to a configured production database.

## Applying022

Public-only legacy database migration compatibility is **OPEN** and is not claimed by this package. A missing canonical `fiscal` schema fails closed; do not execute022 against an unconverted public-only database. Existing local PostgreSQL fixture runners must install their DDL in `fiscal` or explicitly pin legacy migrations through021. No external BeeloyDB source is modified by this package.

Use the existing controlled database migration process, owner credentials and maintenance window. ALTER TYPE requires exclusive catalog/table locks. Migration does not automatically reinterpret historical rows or write to original audit events. External SQL consumers must accept the changed reference column types. Refresh reviewed database dictionaries/catalog snapshots after migration, including three new tables and two views; retain catalog drift checks.

`typed_projection_corrections` stores a complete corrected derived projection at version2. `sale_events_effective` and `operator_security_events_effective` select the correction over the original row, with PostgreSQL16 `security_invoker` views and FORCE RLS on base/correction tables. Raw historical rows remain independently readable. Future source inserts require exact existing derived equality; divergence fails the source transaction rather than disappearing behind `ON CONFLICT DO NOTHING`.

## Explicit tenant-scoped reconciliation

Grant membership of `beefiscal_projection_reconciler` only to the authorized maintenance identity. The NOLOGIN role has SELECT and EXECUTE on the guarded entry point; no direct derived-table mutation privileges and no PUBLIC execute permission. Tenant authorization remains the responsibility of the trusted maintenance entry point; a configurable tenant GUC is not authentication.

Use one consistent snapshot and a reviewed bytewise `[range_from, range_until)` event-ID range. Example placeholders are deliberate; do not substitute an unreviewed whole-database range:

```sql
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SELECT set_config('app.tenant_id', '<exact tenant ID>', true);
SELECT fiscal.reconcile_runtime_projection_v2('<inclusive event ID>', '<exclusive event ID>');
COMMIT;
```

Review `source_count`, `source_sha256`, proposed `inserted`, `corrections`, `diagnostics`. Dry-run writes nothing. Apply deliberately with the returned digest in a new repeatable-read transaction; changed source inventory fails closed:

```sql
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SELECT set_config('app.tenant_id', '<exact tenant ID>', true);
SELECT fiscal.reconcile_runtime_projection_v2('<inclusive event ID>', '<exclusive event ID>', '<reviewed source_sha256>', true);
COMMIT;
```

Missing rows are inserted from original source identity/hash/time/payload; incorrect historical projections receive an append-only correction. Existing correction disagreement or changed original source evidence aborts the transaction. Repeating the same apply adds no projections, corrections or run record. A run record stores range, source digest/count and projection version. Digest/equality use UTC/ISO and C collation independently of session locale. Maximum range inventory is100000 events; partition larger reviewed ranges.

The tenant inventory digest is provenance of the selected rows, not verification of the original global cross-tenant audit hash chain, legal approval or hardware evidence. Resolve diagnostics separately before claiming full model coverage.
