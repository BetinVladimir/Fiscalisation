# ADMIN reconciliation transport deadlines — 2026-10-21

Route availability probes previously could wait forever; submit/lookup promises could also leave the review UI busy indefinitely. Cancellation alone is insufficient because a BLE/native adapter can ignore AbortSignal.

The dispatch helper now bounds probe to3 seconds, submit to15 seconds and lookup to10 seconds with a separate promise deadline. Timed-out probes may try another route before a write. Before submit, the chosen route/attempt is durably claimed in IndexedDB. A submit timeout retains UNKNOWN and the attempt; restart/retry performs lookup of the identical operation through that route, never another submit. Late completion is ignored. A concurrent proven COMMITTED result remains monotonic.

Actual IndexedDB fake-clock regressions cover non-cancellable probe fallback, submit timeout, lookup timeout, process-level store reopening and no resend. Two new tests fail against the prior helper; integrated helper+panel12 tests and TypeScript pass. Actual source ADMIN HTTP→root issuer→browser review→production S3 processor/SQLite harness also passes1/1 (8.6s). Physical adapter transport is still substituted by the native harness.

Full web regression is running separately. Evidence applies to the current working tree; complete native/radio/НАП acceptance remains outside this bounded change. No keys leave ESP32-S3 internal NVS and ATECC608 is not introduced.

Full-suite integration follow-up: first current web run reported295PASS/3FAIL in the checkout fixture. Its shift mock lacked employee_id/register_id/state after active shift selection became contract-scoped. The mock now supplies the actual employee/register and OPEN state plus CASHIER role. All three exact-money/signing/reversal checkout cases pass separately; financial assertions and production shift selection remain unchanged. Full-suite rerun is pending.

Final current-tree acceptance: full web47files/298testsPASS. Targeted12PASS, TypeScript exit0 and actual HTTP1PASS also confirmed. Versioned logs/source hashes: evidence/admin-deadlines-2026-10-21. This supersedes the pending statements above.
