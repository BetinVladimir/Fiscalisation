# GAP016: durable authority and authenticated offline acceptance

Scope: actual S3/BlueCash runtime only. Go backend is the interoperability oracle and consumer; historical Go edge-agent is excluded.

## Implemented contract

- The signed authority prefix is the eight-character fiscal device serial, independent of fiscal memory number. Monetary execution checks prefix equals typed binding fiscal device number.
- Only SALE_FINALIZE consumes a leased sale counter. UNP is `SERIAL8-OP4-0000001`; operator comes from verified execution authorization for local ingress or authenticated canonical MQTT payload. No inferred/default operator.
- REPORT_X/Z, identity, time, printer, CASH_IN/OUT reserve operation sequence only, with zero UNP sequence and empty UNP. Exhausted sale ranges do not prevent these operations. REVERSAL persists original document UNP and consumes no new sale counter.
- Reservation, exact execution UNP, authority context, operation/receipt/payment rows, accepted outbox evidence, and counter advancement commit atomically. Recovery reads the durable execution UNP rather than caller-supplied canonical payload UNP.
- Same generation cannot mutate authority. Same unchanged context in a new generation preserves its counter. Changed ranges must not overlap historical ranges for the same serial. Changing device and returning to an old range also rejects. Legacy fourteen-character operator prefixes fence overlapping serial ranges rather than silently reusing them.
- Local monetary commands require real P256 issuer signature and client proof of possession before reservation or physical I/O. Verification binds exact canonical payload digest, operation/receipt, nullable real server sale ID and surrogate, operator resource ID/code, tenant/location/register/device, FD/FM, binding digest/generation, leased authority, command scope, validity window, and issuer trust key. Missing production trust anchor fails closed.
- Accepted evidence v2 records nullable real server sale ID separately from local surrogate; exact UNP; binding hash and generation; leased context; verified authorization/client proof digests. `AUTHORIZED` is emitted only after actual cryptographic verification.
- Result evidence contains receipt session, payload digest, exact UNP, and per-tender integer minor amount/state/original and reversal references from bound SQLite rows. CARD evidence never copies aggregate references. CASH is `FISCALIZED_CASH` only after committed physical execution.

## Evidence

Baseline actual storage binary fails independent expected `SERIAL8-OP4-pad7` vector. Updated authority runner passes twelve cases: six SQLite allocation/history/atomicity/policy/operator cases and six actual backend-issued P256 authorization through actual IntentProcessor/SQLite cases, including duplicate suppression and pre-I/O negative cases. Independent Go-signed crypto corpus passes seventeen cases with actual mbedTLS. Existing storage durability matrix passes seventeen cases. Native protocol/saga, saga storage, per-tender recovery, money vendor bytes, and fiscal receipt readout matrices pass. Actual Go command/event/batch/ACK interoperability passes against updated backend producer. Actual produced accepted/result evidence is consumed successfully by actual backend signed hardware SyncBatch consumer using the identical issuer fixture.

The real mbedTLS bootstrap builds installed pinned ESP-IDF sources (successfully exercised without prebuilt override), and permits explicit matching prebuilt include/library overrides. OpenSSL-backed shims are not used for authorization verification.

## Acceptance before HIL

1. Full integrated `make iot-test` and actual ESP-IDF S3 cross-build pass with backend issuer/consumer, updated binding parser, trust anchor configuration and frontend authorization contract together.
2. Production trust key/key ID and correct trusted clock are provisioned. Local monetary commands without them reject before counter allocation or I/O. Current issuer authorizes SALE_FINALIZE; other local monetary command scopes must be explicitly issued before use.
3. Binding authority leases must be globally allocated without overlap and historical device reset/flash rollback must not reintroduce a consumed lease. The local SQLite history protects the retained database; it cannot reconstruct ranges lost before upgrade or after database replacement.
4. Existing pre-migration receipts without durable authority metadata remain unknown for physical proof. No synthesized UNP or automatic rewriting of historical accepted/frozen data.
5. Physical fiscal serial/FM/device proof, Datecs/Daisy cash+reversal, BluePay ambiguous-payment recovery, BLE/HTTP authorization transport, MQTT reconnect, power loss and clock rollback are HIL checks. Host fakes do not constitute device certification or SUPTO acceptance.

## Проверка в основном дереве

Пакет интегрирован 2026-10-17. `execution_auth_host/run.py` повторно собрал mbedTLS из установленного ESP-IDF и завершился exit 0: 17/17 crypto cases. Authority runner в основном дереве: 12/12 PASS (6 SQLite + 6 actual issuer → IntentProcessor → SQLite). S3 cross-build после исправления трёх GCC misleading-indentation ошибок: exit 0, RAM 39932/327680, Flash 1654632/3145728. Лог: `/private/tmp/fiscal-authority-integrated-s3-20261017.log`. Backend v2 пакет интегрирован; actual domain и MQTT suites: exit 0. Общий `make iot-test` запущен, результат ещё не подтверждён.

Открыто: activation allocator использует FM вместо serial для authority prefix; online SERVER_ASSIGNED contract реализуется отдельно. Эти ограничения не позволяют считать полный сценарий onboarding/продажи готовым к HIL.
