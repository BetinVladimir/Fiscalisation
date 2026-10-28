# Authority actor wire limit and actual ADMIN pipeline

Execution actor_subject and payment reconciliation actor_id support **1–256 UTF-8 bytes**, not 256 code points. Identity remains the exact trusted mapping / issuer-subject namespace: no truncation, implicit hash or fallback identity is allowed. Root rejects an oversized or malformed UTF-8 actor before signing or persisting either grant. Provisioning administrators must configure a supported identity namespace; denial requires explicit configuration correction, not relaxed credentials. Existing execution firmware already limits actor_subject to 256 bytes; reconciliation firmware is aligned from 128 to 256.

The full real MiniPOS ADMIN HTTP pipeline exposed a valid 136-byte integration actor rejected by the old reconciliation verifier. Independent signed vectors cover 136 bytes, 256 bytes using 128 Cyrillic letters and rejection at 257 bytes. Root execution and reconciliation issuers cover the same boundaries and invalid UTF-8; invalid ADMIN grants leave no persisted authorization.

The opt-in runner `minipos/miniposweb/tests/browser/run-execution-http-pipeline.py` accepts `RECONCILIATION_HTTP_PIPELINE=YES`, with the same explicit disposable PostgreSQL / canonical-schema setup as remediation 26. It uses actual source JWT middleware and persisted ADMIN, encrypted integration credential carrying the separately configured reconciliation capability, explicit root ADMIN mapping, historical accepted UNKNOWN payment evidence, real root HTTP issuer and actual browser App review/confirmation. Production native firmware verifier/IntentProcessor/SQLite processes the returned exact command. Physical transport is emulated; the harness asserts zero purchase/reversal calls and unresolved original fiscal operation.

Test hosts enable hardware-signature verification explicitly, matching production APP_ENV=prod or configured EMQX. This is required for v2 accepted payment context; disabling it cannot authorize reconciliation. No debug grant endpoint or privileged actor from an unsigned caller header is introduced. Fresh test authority material stays in private temporary files and must not be published as production configuration.

## Reproduction

On a new EMPTY disposable PostgreSQL database, use the same canonical model/schema prerequisites and test mbedcrypto dependency documented in remediation 26, then run:

```sh
RECONCILIATION_HTTP_PIPELINE=YES \
EXECUTION_PIPELINE_DISPOSABLE_DB=YES \
EXECUTION_PIPELINE_INIT_SCHEMA=YES \
EXECUTION_PIPELINE_POSTGRES_URL="$TEST_EMPTY_POSTGRES_URL" \
BEELOY_DB_MODEL_ROOT="$TEST_CANONICAL_MODEL_ROOT" \
MBEDTLS_CRYPTO_LIBRARY="$TEST_MBEDCRYPTO_LIBRARY" \
python3 tests/browser/run-execution-http-pipeline.py
```

Run from minipos/miniposweb with installed web dependencies, Go/C++ toolchains and psql available. The runner never drops/truncates a database. The successful fresh run produced exactly one signed ADMIN grant and one local submission; the browser reviewed expected CARD_UNKNOWN versus observed CARD_APPROVED before dispatch, and foreign app/session authorization was denied401. Original fiscal recovery stayed unresolved and no purchase/reversal executor was called. Full chain success is software evidence, not physical bank/fiscal certification.
