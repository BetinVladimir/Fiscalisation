# PostgreSQL: canonical namespace and typed audit integration

Status: software checks below PASS on2026-10-21. Overall device/release/legal acceptance remains OPEN. Evidence: [raw logs and source hashes](evidence/postgres-canonical-2026-10-21/README.md).

## Implemented

- Migration022 projects real opaque sale/event/tenant IDs and current operator/device snapshot fields into `fiscal.*`. Existing UUID references become text losslessly. Historical source audit and original derived rows stay immutable; reviewed corrections are append-only, tenant-scoped and digest-bound. See [migration and reconciliation procedure](35_TYPED_AUDIT_PROJECTION.md).
- Replayed diagnostics return `exact` and contribute zero new diagnostics; first historical diagnostic contributes one consistently in dry-run and apply. Conflicting source hashes/reasons remain errors. Reconciliation counts at most100001 rows and refuses oversized ranges before serializing digest inputs.
- Disposable fixture preparation maps historical local DDL into actual `fiscal`/`minipos` namespaces. Migration022 is copied unchanged. The required runtime login script is copied byte-for-byte with mode and hash; unknown scripts/migrations fail closed. Original migrations001–021 and external BeeloyDB sources are unchanged.
- Persistence tests clone their own marked disposable template instead of deleting shared immutable audit rows. Cleanup drops only an owned random database. Actual audit deletion is rejected and state/generation remain unchanged.
- Three database dictionaries were regenerated from real PostgreSQL: Fiscal financial6 tables, MiniPOS7 tables, Fiscal evidence2 tables. Reviewed source manifests include022 and the fixture preparer hash. These are scoped profiles, not a full schema or Annex29 inventory.
- Compose fixtures now consume generated canonical namespace initialization directories; backup/restore queries explicitly address the correct schemas.

## Verification

| Check | Result | Scope |
| --- | --- | --- |
| Typed projection runner | PASS | Actual domain serialization +46 PostgreSQL assertions |
| PostgreSQL persistence runner | PASS | Fiscal persistence/integration and MiniPOS persistence |
| Fiscal dossier profile | PASS | Actual PG snapshot, RLS, drift and source inventory |
| MiniPOS dossier profile | PASS | Separate actual PG profile |
| Fiscal evidence dossier profile | PASS | Separate actual PG audit/evidence profile |
| Backend `go test ./...` | PASS | Generic tests; conditional database skips are not database acceptance |
| Fixture preparer tests |6 PASS | Required scripts, source preservation, namespace mapping, fail-closed inputs |
| Product boundary fixture tests |4 PASS /16 assertions | Generated fixture wiring |
| Presentation build/report guards |26 PASS |6 Python +20 Node tests; separate UI scope |

The initial generic Go run was blocked by sandbox cache/listener restrictions; the authorized rerun passed. Earlier integration failures exposed shared audit cleanup and stale migration inventories; these were fixed and the final actual PostgreSQL runs passed. Raw final logs are linked above. Full Compose validation and remaining ordinary browser regressions are tracked independently and are not claimed by this package.

## Remaining acceptance work

1. Review migration022 with the owner of canonical BeeloyDB: external consumers must accept text reference columns; assess locking, rollout/recovery and existing public-only database conversion. Execute against a restored representative database before deployment. The disposable fixture adapter is not a production migrator.
2. Add reviewed catalog profiles for all typed projection tables/views before claiming a complete schema dictionary. Resolve diagnostic categories and absent metadata instead of treating a diagnostic row as full event-model coverage. Country-profile fallback compatibility remains a limitation.
3. Validate full two-product Compose flows and backup/restore after these namespace changes. Continue ordinary browser/native regressions and produce final app/firmware artifacts.
4. Close authenticated/encrypted BLE result/session, offline revocation/equivalence and electronic receipt gaps; complete real Datecs COM, Daisy USB, BluePad BLE and Android/iOS HIL. Keys remain inside ESP32-S3 NVS, with no ATECC module requirement.
5. Complete applicable НАП evidence and external review; none of the checks here constitutes hardware, vendor or legal approval.
