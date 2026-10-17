# Authority regression tests

`issuer_fixture.json` was produced by the actual fiscal-backend execution authorization issuer (P256 issuer signature and client proof). The fixture contains public keys and signatures only; fixed `now` is injected into the real mbedTLS verifier. The host suite runs actual IntentProcessor and SQLite; its ReceiptExecutor deliberately simulates a committed physical receipt and does not claim hardware coverage.

`run.py` verifies exact serial/operator/seven-digit sequence, restart persistence, lease mutation/overlap rejection, no reuse across operators, exhausted-sale lease with still-working non-sale/reversal reservations, atomic recovery UNP, valid local authorization, duplicate suppression, and pre-I/O rejection of missing/expired/tampered authorization, payload, or binding context.

The reusable mbedTLS bootstrap builds the pinned installed ESP-IDF sources. `MBEDTLS_SOURCE_DIR`, `MBEDTLS_INCLUDE_DIR`, and `MBEDTLS_CRYPTO_LIBRARY` permit a prebuilt matching toolchain; the OpenSSL shims are never used for authorization verification.

Online authority fixture is also produced by the actual backend issuer, using the persisted pending server operation and canonical BuildFinalizePayload. Its seven signed `unp_authority` fields bind source, real sale ID, exact UNP, payload digest, operation and receipt IDs before physical I/O. Online has zero lease consumption; local offline authorization cannot select a caller UNP. The SQLite matrix additionally exercises unique serial/sequence sale claims, lease history overlap, future lease rejection, claim fault rollback and a deterministic two-connection concurrent claim race.
