# N5/N6: BlueCash local journal and hybrid storage coverage

Status: executable offline extraction and source-derived dictionary with host SQLite tests. This does not establish legal sufficiency, device provenance, signature validity, or complete НАП readiness. Host fixtures are synthetic tests, not production or physical-device evidence.

## Actual local storage contract

`SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt` uses SQLiteOpenHelper, not Room: `beefiscal-bluecash-journal.db`, user_version 3, WAL enabled and synchronous FULL. The exporter derives application DDL from that actual source, checks the complete SQLite schema and user_version, and refuses unreviewed drift. Five application tables, SQLite's sqlite_sequence and Android's android_metadata are included. Android locale metadata follows [AOSP SQLiteConnection](https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/database/sqlite/SQLiteConnection.java); actual OEM/platform schema must still be rehearsed. Unexpected/missing metadata fails closed rather than being silently discarded.

All retained rows, historical operation identities, exact payloads, nullable fields, signatures, RRN/authorization references, ACK checkpoint, UNP counters and engine high-water marks are exported. Integer values remain decimal strings, preserving values above JavaScript's safe integer range. The generated dictionary is `20_BLUECASH_SQLITE_DICTIONARY.generated.md`. The common S3 extractor's default profile remains unchanged.

The journal can prune older acknowledged events; first retained prev_hash may reference missing history. Exported signatures and hashes are data, not verified proof. No continuity, genesis completeness, backend receipt or successful physical fiscal operation is inferred from a row. UNKNOWN remains UNKNOWN.

## Acquire a snapshot before offline export

Acquisition is a separate authorized operational task; this CLI neither connects to Android nor acquires its private database. Record application build, source commit, device identity, platform, operator, capture time and acquisition method in the dossier. Access must use an approved application/platform backup mechanism; do not weaken release sandboxing or expose credentials to obtain it.

Never copy only the main database while WAL writers are active. Committed latest transactions can exist solely in `-wal`. Never delete sidecars to make export pass. Obtain one consistent snapshot using SQLite online backup from a connection able to read the actual live WAL, or stop all writers, perform a successful SQLite WAL checkpoint, close all connections, and acquire the resulting quiescent main file. A checkpoint failure/busy result is not permission to discard WAL. Do not independently copy live main/WAL/SHM files at unrelated times. If the available acquisition mechanism cannot ensure consistency, mark acquisition BLOCKED.

Preserve the original acquired evidence and acquisition hashes. Run offline export against a separate quiescent working copy; it must have no `-wal`, `-shm` or `-journal` companions. The tool refuses sidecars, checks source hash before/after, uses immutable read-only SQLite and requires an explicit declaration. That declaration is not authenticated proof that acquisition was correct. This package tests actual SQLite backup with an uncheckpointed committed WAL transaction; a host main-file-only copy is not the accepted acquisition method. A physical Android acquisition rehearsal remains required.

## Reproduce

From repository root, using a new output directory:

```sh
python3 -m unittest discover -s tests -p '*sqlite_export.py'
python3 scripts/export_bluecash_sqlite.py export \
  --database /evidence/working/beefiscal-bluecash-journal.db \
  --output /evidence/dossier/bluecash-local \
  --device-id DEVICE_DECLARED --source-commit SOURCE_COMMIT \
  --acquisition-note 'Authorized snapshot method, timestamps and acquisition record ID' \
  --quiescent-copy
python3 scripts/export_bluecash_sqlite.py verify --directory /evidence/dossier/bluecash-local
```

The source must match the captured build; use global `--schema-source` before the subcommand when a preserved source is required. Existing outputs are not overwritten. Success is exit 0, foreign-key issues exit 2, refusal exit 1. The manifest publishes last, includes input/source/tool hashes and unauthenticated acquisition declarations. Verification detects inventory/hash damage, not a maliciously rewritten manifest. Output contains sensitive business/personal data and receives restrictive file permissions. Transfer and retention require the dossier's access controls.

## Hybrid inventory and correlation boundaries

| Actual store/source | Data and cross-store identifiers | Executable extraction coverage |
|---|---|---|
| S3 `durable_storage.cpp`, SQLite | operation_id, receipt/sale surrogate, per-tender payment_id, event sequence/ACK, authority generations and UNP | Existing `export_s3_sqlite.py`; dictionary must regenerate against final integrated source whenever schema changes, regardless of schema_version 3 |
| BlueCash `AndroidJournal.kt`, SQLiteOpenHelper | journal_event.operation_id/event_id/journal_seq, payload, event_hash/prev_hash, ACK ID; payment_transaction.operation_id/RRN; ble_local_sale.surrogate_id; UNP prefix/counters | This package; full retained local DB, no joins inferred across tenants/devices |
| Fiscal backend `internal/persistence/postgres.go`, PostgreSQL/pgx | Repository collections sales, operations, devices, shifts, unp, replays, outbox, ble_sessions, sync_acks, connectivity_probes, resources, artifacts, edge_pending, activation challenges/requests; audit and other typed rows require separate inventory | OPEN: no cloud extractor supplied. A transactionally consistent read-only tenant-scoped export contract must enumerate typed tables, JSON resources, audit, outbound email and referenced artifact/object bodies; FORCE RLS and tenant reader permissions must remain enforced |
| MiniPOS backend `internal/persistence/postgres.go`, separate PostgreSQL | JSONB state rows/meta; products, tax_groups, employees, identity_bindings, operator_sessions, shifts, orders, checkouts, checkout_hashes, api_replays, webhook_inbox, configurations | PARTIAL: executable `-profile minipos` seven-table allowlisted financial projection, doc24; broader legacy/auth/configuration data remains OPEN |
| MiniPOSWeb `src/storage/outbox.ts`, IndexedDB `beeloy-miniposweb` / `fiscal-outbox` | client_operation_id, immutable intent envelope/digest, route/possible-send claim, certainty/result and tenant scope | OPEN: browser export with real IndexedDB transaction and scope tests, no claim erasure or resend during extraction |
| MiniPOSWeb `referenceCache.ts`, IndexedDB `beeloy-miniposweb-reference` / `snapshots` | Cached reference snapshots; distinct from fiscal evidence | OPEN: separate cache inventory/export |
| BeeFiscalApp `emailOtpAuth.ts`, `ui/preferences.tsx` | Native SecureStore; web preferences/auth ancillary fields in localStorage, session key held in memory on web | Excluded. A targeted nonsecret settings dictionary requires explicit allowlist; never bulk-export secure storage |
| BlueCash `DeviceBootstrap.kt`, `SpaDeploymentManager.kt` | Bootstrap/identity/deployment SharedPreferences, AndroidKeyStore private keys | Excluded. Credentials and keys are not dossier business-record exports |
| S3 NVS DeviceIdentity | Internal ESP32-S3 identity/configuration keys | Excluded; no ATECC external key store is used |

Correlation is an evidence task, not a destructive merge: retain store/device/tenant/source provenance on each row. Match operation UUID first, then explicit payload server_sale_id/client_sale_surrogate_id and receipt/session/binding IDs only when present. RRN is a terminal correlation reference, not globally unique identity or proof of fiscal completion. Local journal sequence and cloud outbox sequence are different namespaces; never equate them. Exact UNP agreement needs matching fiscal device/operator/range context. Missing cloud records, pruned local history or unmatched identifiers must be reported as gaps, not fabricated or inferred success.

## Preliminary acceptance and remaining work

Automated acceptance: all original 11 S3 tests plus BlueCash actual-SQLite replay, >2^53 money, retained ACK prefix, version/schema refusal, archive tamper, secret-sibling exclusion, CLI/dictionary reproducibility and WAL backup preservation pass. No runtime/schema modifications are part of this package.

Still required: Android/OEM acquired snapshot rehearsal; captured-build source mapping; signature/public-key and retained-chain verification contract; cloud transaction snapshots and actual executable exports with tenant-isolation tests; browser/native allowlisted exports; cross-store reconciliation reports and missing-record handling; external dossier review. These remain open and prevent a claim that hybrid N5/N6 or НАП preparation is complete.
