# MiniPOS: actual HTTP authority pipeline regression

2026-10-18. Software integration evidence, not physical HIL or certification evidence.

The browser regression executes actual App and IndexedDB/WebCrypto key lifecycle;
actual MiniPOS JWT authentication, persisted employee identity, caller-owned app
session and logical shift; actual PostgreSQL encrypted tenant integration
credential; root HTTP credential authentication/capability/tenant mapping; actual
root issuer and persisted atomic no-I/O sale preparation; browser P256 DER proof;
actual mbedTLS verifier, IntentProcessor and SQLite. Only physical adapter
transport and fiscal executor are software substitutes. The deliberate unavailable
cloud command endpoint causes normal LAN selection; it is not an authorization
bypass. Signing uses a newly generated disposable test root. ESP32-S3 production
keys remain internal NVS; no ATECC hardware is assumed.

## Regressions found and fixed

- Generic web HTTP requests omitted mandatory X-Api-Version, preventing actual
  operator-session bootstrap. A default is now supplied only if the caller has
  not explicitly supplied a version.
- createOrder/openShift omitted mandatory mutation Idempotency-Key. They now
  supply UUIDs; existing explicit operation keys are preserved.
- Source payment normalization discarded payment_id and terminal_policy. Root
  correctly rejected the resulting incomplete plan. Both fields are preserved;
  execution freeze requires unique valid payment UUIDs before changing state.

The test asserts distinct source/root order and register namespaces, exact signed
server UNP, exact operator identity/code, immutable per-tender identity, no MQTT
outbox dispatch during local preparation, one local submission, actual firmware
accepted/result events and foreign app session rejection. It does not claim a
physical fiscal receipt or verified bank approval.

## Reproduce

Requires Go, Node dependencies/Chromium, Python/C++ host test dependencies, actual
mbedcrypto library and a **new empty disposable PostgreSQL database**. Do not use
an application database: the test inserts fixed test identities. The runner never
truncates or drops databases/tables. Canonical table DDL is read from sibling
BeeloyDB/model, not replaced with mocked SQL tables.

```sh
EXECUTION_PIPELINE_DISPOSABLE_DB=YES \
EXECUTION_PIPELINE_POSTGRES_URL=postgres://testuser@127.0.0.1:55487/fresh_test_database \
EXECUTION_PIPELINE_INIT_SCHEMA=YES \
BEELOY_DB_MODEL_ROOT=/absolute/path/BeeloyDB/model \
MBEDTLS_CRYPTO_LIBRARY=/absolute/path/libmbedcrypto.a \
python3 minipos/miniposweb/tests/browser/run-execution-http-pipeline.py
```

The runner prints a private temporary evidence directory and terminates its own
HTTP children. That directory contains freshly generated **test** keys and JWTs;
do not publish it wholesale. `pipeline-evidence.json` is the redacted behavioral
summary with source/root IDs, exact UNP, submission count and firmware events.

Validation: actual pipeline 1 PASS; source domain/API PASS; App discount absent,
zero and nonzero plus reverse assertions retained. Baseline failures were real
HTTP 400 missing version, HTTP 400 missing idempotency and root HTTP 400 invalid
payment plan; after fixes the same actual chain passes. Physical device acceptance,
secure production clock, actual fiscal shift and bank/fiscal proof remain separate
HIL prerequisites.
