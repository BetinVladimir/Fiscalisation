# Release evidence and vulnerability gate

Stage 23 evidence is fail-closed. An unsigned package is useful for DEV inspection but remains `UNSIGNED_NO_GO`. A signed package does not become production-ready merely because its signature is valid: hardware, vendor and legal gates remain independent.

## Reproducible two-step flow

1. Choose one immutable RFC 3339 instant and generate the candidate SBOM:

   ```sh
   RELEASE_CREATED_AT=2026-08-09T12:00:00Z ruby scripts/generate_release_evidence.rb /tmp/release-scan-input
   ```

2. Scan `/tmp/release-scan-input/sbom.cdx.json`. Normalize the scanner result to the JSON shape below. `subject.sbom_sha256` must be the SHA-256 of those exact SBOM bytes.
3. Generate the signed package with the same timestamp and the normalized report:

   ```sh
   RELEASE_CREATED_AT=2026-08-09T12:00:00Z \
   RELEASE_VULNERABILITY_REPORT=/secure/vulnerability-report.json \
   RELEASE_SIGNING_PRIVATE_KEY=/secure/release-private.pem \
   ruby scripts/generate_release_evidence.rb artifacts/evidence/release-candidate
   ```

4. The accepting party supplies the trusted public key independently:

   ```sh
   RELEASE_TRUSTED_PUBLIC_KEY=/secure/release-public.pem \
   ruby scripts/verify_release_evidence.rb artifacts/evidence/release-candidate
   ```

The packaged public key is compared byte-for-byte with that trust input; it is not a trust anchor by itself.

## Normalized vulnerability report

```json
{
  "schema_version": "1.0",
  "scanner": {
    "name": "approved-scanner",
    "version": "exact-version",
    "database_updated_at": "2026-08-09T11:00:00Z",
    "status": "COMPLETED"
  },
  "subject": {
    "sbom_sha256": "64 lowercase hexadecimal characters"
  },
  "summary": {
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0,
    "unknown": 0
  },
  "findings": []
}
```

Verification rejects an incomplete scan, a vulnerability database older than 72 hours, a different SBOM digest, malformed/non-negative counters, a finding-count mismatch, any Critical or High finding, a modified provenance statement, an untrusted signature or an incomplete checksum inventory.

`provenance.intoto.json` is an in-toto Statement v1 with a SLSA Provenance v1 predicate. It binds the exact SBOM, source commit, release invocation and builder identity. Its SHA-256 and the optional scan report SHA-256 are inside the signed release manifest, so both are transitively protected by the Ed25519 signature.
