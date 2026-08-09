# Release and rollback plan

## Principle

Application binaries may roll back; fiscal history never rolls back. Completed sales, fiscal operations, audit events, receipt artifacts, UNP allocations and acknowledged Edge journal records are immutable.

## Before deployment

1. Verify the signed manifest, independently trusted public key, SBOM, provenance and approved scan.
2. Record current and candidate image digests, OpenAPI lock hash, migration versions and configuration checksum.
3. Take and verify independent Fiscal and MiniPOS backups.
4. Confirm the previous application image can read the post-migration schema. If not, use restore-to-new-database rollback instead of binary-only rollback.
5. Keep Caddy configuration and certificate data backed up separately.

## Rollback triggers

- duplicate/lost fiscal-integrity defect;
- inability to determine final-device state safely;
- cross-tenant/RLS exposure;
- corrupt journal, audit chain or receipt artifact;
- sustained startup/readiness failure after the deployment window;
- Critical/High security finding affecting the candidate.

## Procedure

1. Stop new fiscal commands at ingress while preserving status/read endpoints.
2. Snapshot logs, metrics, database WAL position, Edge cursor and release evidence.
3. Resolve all in-flight/UNKNOWN operations by authoritative device/payment lookup; do not retry them during rollback.
4. Redeploy the exact previous image digests. Keep current databases only when schema backward compatibility was proven.
5. Otherwise restore both products independently into new database instances, run migrations to the previous compatible level and verify artifact/audit hashes before switching Caddy upstreams.
6. Resume read traffic, then controlled cash traffic; enable card only after the bound terminal probe succeeds.
7. Record the incident, affected operation IDs and final reconciliation outcome.

## Forbidden rollback actions

No destructive SQL against completed fiscal records, no UNP counter decrement, no deletion of unacknowledged Edge data, no replacement of signed evidence, and no switch from CARD to CASH without a new explicit operator action/payment ID.
