# Тесты к добавлению и критерии предварительной приёмки

[Навигация](README.md) · [План этапов](04_REMEDIATION_PLAN.md) · [GAP register](02_GAP_REGISTER.md)

Это спецификация **будущих** тестов; перечисление не означает, что они уже добавлены или исполнены. Тестовые файлы/каталоги ниже — предлагаемое место реализации. В текущем этапе код не менялся.

## Требования к тестовому стенду

- Contract fixtures versioned и независимы от encoder под тестом. Для binary golden vectors записаны vendor document/version/section и происхождение; assertions проверяют точные bytes и typed result.
- Host compilation actual IDF runtime translation units с injectable UART/USB/BLE/clock/storage port; не подмена всей business логики fake orchestrator. Device simulator задаёт bytes/failures, а не заранее нужный COMMITTED.
- Real MQTT broker с publisher `fiscal-backend/internal/mqttclient`, actual consumer adapters и ACL; PostgreSQL нужной версии с полной migration chain и non-owner roles; SQLite database с reopen и failpoints.
- Наблюдаемые счётчики: vendor bytes/calls, purchase/VOID/refund counts, receipt open/close counts, UUID/UNP, tender references, event hashes, ACK cursor, order/sale state и WebHook inbox/outbox. Итог «тест прошёл» без этих assertions не принимается.
- В failure cases допускается UNKNOWN/RECONCILE с доказанным отсутствием повторного действия. Ложный success, выдуманный receipt reference и потерянный approved tender недопустимы.

## T-01 — Сквозной MQTT contract

**GAP:** 001/002/024. **Где:** новый cross-runtime suite `tests/e2e/mqtt-runtime-contract/` плюс Go/Kotlin/IDF consumer fixtures.

Сценарии: binding из настоящего backend → команда опубликована тем же publisher → exact consumer receives → ACCEPTED/terminal event → sync commit → signed business ACK → device cursor. Отдельно S3 и BlueCash; adapter ID намеренно отличается от fiscal device ID; internal sale ID отличается от external ID. Проверить suffix, fragmented MQTT data offsets, invalid topic/body identity, wrong tenant, generation, expired signature, QoS1 duplicate и retained command policy. Повтор доставки даёт 1 vendor execution, 1 business result и корректный idempotent ACK; чужая команда —0I/O. Negative publish topic должен действительно не доходить/отвергаться.

## T-02 — Factory-reset BLE provisioning

**GAP:** 003/004/005/031. **Где:** `BeeFiscalApp` adapter tests + IDF provisioning host tests + factory CLI tests; затем Android/iOS instrumentation.

Проверить UUID/service-info соответствие, device proof/capability, wrong device/key/signature, expiry/revocation и generation rollback. Wi-Fi/config/secrets installation с readback без вывода секретов; reboot с сохранением binding и identity. Передавать binding >512bytes, fragment reorder/duplicate/missing/corrupt/digest mismatch, disconnect/resume/timeout; incomplete transfer не меняет config. Missing trust/SD/secret/time/network не даёт ложного READY. Полный критерий — чистая плата после документированной flash достигает broker и probe без ручного NVS редактирования.

## T-03 — S3 GATT transport boundaries

**GAP:** 005…008. **Где:** actual `ble_runtime.cpp` с fake NimBLE API и OS-level smoke.

Посчитать длины AD и scan response; injected advertising error видим. MTU23/185/247/517, short write, notify failure, CCCD unset/failed, disconnect mid-message, duplicate identical/conflicting frame, wrong offset/size/digest/ID, max8192 и8193bytes. Проверить per-connection state/TTL/memory cap/flow BUSY/cancel/resume, отсутствие continuation чужой сессией. При неподдерживаемом MTU блокировать до monetary send; header+body <=MTU-3. Long notification не должна молча теряться.

## T-04 — Native BLE Android/iOS lifecycle

**GAP:** 007/008/027. **Где:** `minipos/BeeMiniPOS/src` adapter tests и platform instrumentation.

Native library возвращает MTU меньше requested; iOS ограничивает write value; subscription callback приходит с ошибкой; permission denied/revoked, Bluetooth off, scan timeout, stale cached service, background/foreground, lock screen, OS kill и session expiry. Оплата завершается через90–120s; UI допускает ожидание/reconcile, не новую продажу. Reset отменяет waiters, очищает assembler и disconnects без stale notification success. OPEN_MVP и secure режимы имеют явные разные expectations; неподдержанный secure mode не становится READY.

## T-05 — Независимые Daisy/Datecs codec golden vectors

**GAP:** 009/010/029. **Где:** новые actual-runtime C++ tests рядом с IDF host tests и `SmartDevices/bluecash-app/src/test/.../DatecsProtocolTest.kt`.

Daisy status74 TX/RX из V-02, BCC nibbles A…F,0DATA/maxDATA, response6status. Datecs V-01 sample command51 и заранее вручную рассчитанные empty/data frames, response8status; payload496/480 boundaries, SEQ20/7F/80/FF. Wrong/minus/oversized LEN, truncation, extra bytes, wrong BCC, separators и malformed nibble должны отвергаться. Encoder/decoder roundtrip отдельно, но не заменяет vendor vectors. C++/Kotlin используют один versioned fixture corpus. Включить mutation negative control: offsets0x26/0x2F должны сделать suite красным.

## T-06 — Physical stream parser, timeout и SEQ

**GAP:** 011/012. **Где:** fake serial driver для actual `profile_executor.cpp` и Kotlin `DatecsSocketProtocol`.

Byte-by-byte, arbitrary split/coalesced frames, SYN before/during response, NAK, noise, wrong SEQ/CMD, duplicate/late response, disconnect и частичная запись. NAK/timeout retry сохраняет bytes/SEQ с ограниченным числом попыток; progress SYN не портит DATA и укладывается в общий deadline. Две последовательные single-command операции имеют правильный progression, не исполняются как cached reply. Status по каждому documented bit плюс Datecs negative ErrorCode; stuck stream не зависает. Для unknown после possible-send нет автоматического нового semantic command.

## T-07 — Profile payload/result conformance

**GAP:** 012/015/021. **Где:** `CommandPayload`/Kotlin payload tests + actual executor byte-trace tests.

Внешние fixtures для open/sale/discount/payment/close/storno/X/Z/cash/identity/time/EJ. Bulgarian text, CP1251 для Daisy, model-specific Datecs encoding; byte limits, delimiters/control chars, unmappable text. VAT mapping каждого profile, operator/password/till из binding, cash/credit/debit slots, storno reason0/1/2 и original document metadata. Настоящие close/identity/time responses должны попасть в typed result; synthetic EDGE ID не принимается за fiscal document. Discount участвует в fiscal bytes и итоговой сумме. Unsupported capability отклоняется до card purchase.

## T-08 — BluePad vendor BLE и correlation

**GAP:** 013/014. **Где:** host tests actual `bluepad_ble_central.cpp` с fake GATT callbacks; затем vendor device.

Service discovery/identity, POWER30→WAKE→31, обе подписки, failure/late CCCD, sleepy terminal, disconnect/reconnect. Chunks<=19, строго один write до completion, failure после0/1/N chunks; после possible-send certainty UNKNOWN. Wrong/late transaction notification, different STAN/RRN/amount, malformed TLV/length/checksum, missing result fields и buffered old bytes. Lookup использует сохранённый typed reference, не digits от UUID. Payment app version зафиксирована; разные reference fields не преобразуются молча друг в друга.

## T-09 — Card saga, split и компенсация

**GAP:** 014/019/020. **Где:** actual ProfileExecutor/IntentProcessor/SQLite composition tests, Kotlin analogous suite.

CASH, CARD, CASH+CARD, CARD+CASH, CARD+CARD, минимум3tenders. Первый approve, второй decline/timeout, fiscal open/line/payment/close failure и card reversal unknown. Отдельный durable state/STAN/RRN/auth/amount каждого tender; crash до/после каждого purchase response/checkpoint. При restart сначала lookup, никогда повтор purchase. Compensation idempotent для всех доказанных approved, не для cash/unapproved. Уже закрытый fiscal receipt не «отменяется» только потому, что lost response. Нет общего RRN, перезаписывающего split.

## T-10 — Monetary validation, authority и UNP

**GAP:** 015/016/025. **Где:** actual IDF decode/reservation tests; Go/TS/Kotlin differential payload corpus.

Invalid currency/amount precision/negative/overflow, zero quantity/unsupported VAT, invalid discount, sum(payments) !=netTotal, duplicate tender IDs, invalid operator/password/profile. Ни одного vendor call при input invalid. UNP counter1/9999999/range exhaustion/generation rollback, parallel identical/different UUID, same UUID changed digest; cloud/local authority не расходятся. Reserved UNP сохраняется в recovery snapshot. Expired/future command/clock invalid policy проверяется до I/O; diagnostic commands не расходуют sale range без контракта.

## T-11 — Durable failpoints и recovery

**GAP:** 016…020/025. **Где:** actual IDF SQLite storage + intent/profile runtime host fault harness.

Инжектировать prepare/step/commit/fsync/full-media/I/O error перед каждой записью и crash после каждой записи, а также до/после vendor send/response. Reopen той же БД; operation/receipt/payments/event_outbox согласованы. До успешного EXECUTING checkpoint —0side effects. Terminal state без terminal outbox невозможен. Divergent event_id payload отвергается. Delayed USB/BLE readiness активирует повтор recovery; чужой open receipt не cancels. Recovery охватывает все tenders и >128 pending через bounded повторные проходы. Capacity/retention не удаляет unacked данные или dedup identity, не возвращает ложный success.

## T-12 — Cross-language sync, lost ACK и chain

**GAP:** 002/022/023. **Где:** new fixture corpus `contracts/fixtures/edge-sync/` и Go verifier/C++ actual builder/Kotlin tests.

Сравнить exact unsigned event/batch bytes и hashes/signatures через настоящие production functions. Nested maps с переставленными keys, null/empty/missing, Unicode, <>&, escaping, numeric boundaries. Server commit → lost ACK → append new events → process reboot: retransmit неизменённого frozen batch, не расширенного диапазона. Duplicate ACK, out-of-order/stale ACK, wrong hash/edge/batch/signature/key, forward seq и re-signed attacker payload. Cursor/hash меняются только для правильного текущего ACK, transaction atomic. Проверить chain после prune/anchor и длительной offline очереди.

## T-13 — Domain materialization sale/reversal/offline

**GAP:** 021/024. **Где:** fiscal-backend domain/integration и MiniPOS checkout/webhook tests.

Internal !=external !=client IDs, adapter !=fiscal endpoint, cloud aggregate sale, offline SALE_FINALIZE, duplicate/split-batch events, event before ACCEPTED/reordered policy, reversal success/failed/unknown. Существующая/новая sale materializes ровно1раз, operation state и REVERSED event корректны. Real receipt evidence сохраняется, UNKNOWN не делает order COMPLETED. Cross-tenant substitutions отвергаются до commit. WebHook+poll converge на той же version/receipt/UNP.

## T-14 — Durable UNKNOWN и route switching

**GAP:** 026/027. **Где:** `minipos/miniposweb/tests/route-controller-recovery.test.ts` (новый) + native checkout equivalents.

Submit accepted but response lost; reload; повтор execute того же UUID; lookup timeout/401/403/429/503, non-authoritative404, offline cloud/available BLE. Во всех uncertain cases суммарный submit count остаётся1. Persisted send marker появляется **до** send. Authoritative NOT_SENT допускает контролируемую попытку с тем же UUID/digest; payload mismatch блокируется. Concurrent tabs/processes и двойной tap не создают две оплаты. Terminal replay возвращает прежний result, не новый submit. Uncertain recovery после logout сохраняет record в правильном tenant/register scope без раскрытия другому пользователю.

## T-15 — MiniPOS backend contract regressions

**GAP:** 028. **Где:** existing `internal/api/handler_test.go` + domain/fiscalintegration tests.

Два положительных employee/operator fixtures с реальным subject mapping и отрицательные wrong/inactive/cross-org operator; recovery filter не обходит identity. Missing/stale/current If-Match/version, alias routes и canonical errors. Checkout external Fiscal malformed response/202/timeout/lost response/retry/poll/WebHook. API suites проходят с `-count=1`, отсутствие тестов у integration adapter закрывается boundary tests.

## T-16 — Real PostgreSQL migrations, RLS, concurrency

**Связано:** W5/W7, GAP-028/030. **Где:** `scripts/postgres-integration.sh`, backend persistence integration suites.

Fresh full schema и upgrade со старых typed/compatibility snapshots, совпадение SQL и embedded migration chain. Wrong/missing tenant context non-owner, cross-tenant FK/insert/list/artifact/ACK, transaction rollback/CAS conflict между replicas, unique idempotency claim и restart pending checkout. Atomic sale+outbox/inbox+order; повтор WebHook и offline import. Rollback drill сохраняет историю. Проверка реальной БД с обязательным exit status, не auto-SKIP из-за отсутствия DATABASE_URL.

## T-17 — Outbox/reference/offline reconciliation UI

**GAP:** 026/027; пользовательские уже изменённые scope guards не объявлять исправляемым дефектом без failing test.

Reload/OS kill/private browsing/quota exhausted, IndexedDB migration/transaction abort; tenant/location/register/user switching, stale cache и inactive product/operator. UNKNOWN остаётся видимым и reconciles без resubmit; поздний webhook/poll monotonic, duplicate completion не удваивает order. Sync error и suspended storage имеют actionable UI. Проверить live текущие scope helpers, не старую версию из Git HEAD. E2E Android/iOS native и desktop/tablet web для денежных форм.

## T-18 — Admin/auth/security capability

**GAP:** 003/008/033; `AdminApp`, `BeeFiscalApp`, MiniPOS.

Token expiry/logout/revoked role, tenant switch, stale async response, expired activation, wrong device, secret redaction и passwordless identity mapping. Operator не меняет signed binding/payment profile; stale generation rejected. UI не показывает READY на successful connect без probe; STUB/unsupported profiles не доступны как production ready. BLE OPEN_MVP отображается как выбранный профиль; secure reference не даёт ложных гарантий actual firmware. Добавить отдельный AdminApp test runner либо явный shared suite, не считать typecheck behavioral test.

## T-19 — WebHook/API orchestration faults

**Где:** fiscal-backend webhook/outbox и MiniPOS inbox/fiscalintegration.

Signed raw body/timestamp/kid/rotation, replay/out-of-order/duplicate, wrong tenant, 410 endpoint disable, DNS/redirect policy по текущему контракту, network loss before/after commit, process restart. Завершённая device операция не печатается повторно из-за недоставленного webhook. Polling authoritative; order/fiscal version монотонны; durable inbox-before-apply и outbox atomic с domain commit. Проверить все включённые mutation paths роли/idempotency/CAS через OpenAPI.

## T-20 — Deploy/runtime health и transport failure

**GAP:** 004/025; W7.

Actual broker mTLS: wrong/expired/revoked cert, wrong CA/clientID/tenant ACL/topic, reconnect duplicate sessions; неверное время и network loss не создают ложный READY. Final-device probe после UART install/USB enumerate обязателен: unplugged fiscal device нельзя считать живым по installed driver. Lost SD/full SD, network health отдельно от endpoint health; detached USB/short queue/CRC and partial serial write → правильный certainty. Зафиксировать USB descriptors/CDC line coding/VID/PID/interface, board power/pinout, transceiver. Проверить SPA deployment signature/hash/version/activation/rollback и недоступность download без потери fiscal runtime.

## T-21 — Минимальный физический smoke перед основными циклами

**GAP:** 006/007/013/027; внешнее evidence.

| Профиль | Управление | Обязательный цикл |
|---|---|---|
| S3 + Datecs DP-150MX COM + BluePad-50 Plus BLE | Android MiniPOS; затем iOS MiniPOS | BLE provisioning → protocol probe → MQTT cash/card/split → direct BLE cash/card/split → status/poll/WebHook reconciliation |
| S3 + Daisy Compact S01 USB | Android; затем iOS | BLE provisioning → USB probe → MQTT cash → direct BLE cash → device disconnect/reconnect; CARD только при реально добавленном поддерживаемом payment endpoint |
| BlueCash-50 Android device application | Внешний MiniPOS Android и iOS | demoDebug APK install/activation → MQTT и direct BLE → cash/card/split → typed fiscal result и durable sync; встроенный pinpad путь отдельно от внешнего BluePad/S3 |

Для каждого применимого пути: reversal/refund с original reference, X/Z, cash-in/out, printer test, response loss, app/adapter reboot. Card-only проверки выполняются на согласованном vendor/acquirer test setup; production receipt/payment не использовать как случайный smoke. Записывать exact model/OS/fw/app/SDK/build, device serial anonymized ID, command UUID/UNP, terminal receipt, firmware logs и backend events. iOS web/PWA не принимать как доказательство native BLE — использовать native build. Основные endurance и power-cut сценарии выполняются по исходному hardware plan после software gates.

## T-22 — Reproducible artifacts и factory/release gate

**GAP:** 004/029/031/033/034.

Clean checkout/offline dependency manifest, pinned toolchain/JAR hashes, explicit BlueCash flavor (например `:bluecash-app:testDemoDebugUnitTest`, `:bluecash-app:assembleDemoDebug`, после проверки списка Gradle tasks), правильный IDF target вместо default Arduino scaffold. APK install/start/logcat, native Android/iOS build/entitlements/permissions, firmware boot после exact flash manifest. Wrong chip/offset/image type/signature/hash блокируются до flash. Release остаётся заблокирован при missing HIL. Внешний trusted reviewer key, required cases/device/build hashes/artifact digest verification; fake new key/PASS-only/expired/case omission/tampered artifact должны fail.

## T-23 — Gate quality и runtime composition

**GAP:** 030/034. **Где:** CI и `scripts` regression tooling.

Manifest отображает actual `.cpp`/Kotlin entrypoints, linked drivers, build flags, profile и test IDs. Gate выполняет actual runtime units; source-string checks остаются supplemental. Negative controls: сломать frame LEN/topic mapping/pre-I/O checkpoint/ACK hash в тестовом mutation workspace — соответствующая suite обязана падать. Go reference и IDF используют общий contract corpus, но отдельные statuses. Отсутствующий compiler/broker/Postgres/device evidence не превращается в PASS через skip или pipeline tail exit code. Required case coverage и trust verifier — самостоятельные negative tests.

## T-24 — Tremol conformance при включении в scope

**GAP:** 032. **Где:** новые host tests для `TremolProtocol.cpp`/`TremolPrinter.cpp` с BG2507141400 fixtures.

STX/LEN/XOR/nibble/terminator, ACK/message/Retry/busy/STE, NBL20…9F wrap и stale-response matching. Boundary/truncation для uint8 lengths, oversized text/semicolon fields, malformed checksum nibble, variable-response buffers. Open/sale/discount/pay/close/storno/identity/time/EJ profile vectors и own-receipt recovery. До этого — capability disabled и отдельного HIL нет.

## T-25 — IoT build + расширяемая эмуляция USB/COM/BLE устройств

**Пакет:** W9; GAP-009…023/030. **Спецификация:** [W9 в плане доработок](04_REMEDIATION_PLAN.md#w9--покрытие-iot-сборки-эмуляторами-подключённых-устройств).

Production ESP-IDF cross-build и host test build из тех же runtime sources, mocks только на I/O/platform boundary. Stateful эмуляторы Datecs DP-150MX COM, Daisy Compact S01 USB и BluePad-50 Plus BLE проверяют точные команды/параметры/порядок и отдают vendor responses; independent golden oracle не использует production codec для ожидаемых bytes. Общая suite проверяет T-05…T-12 через actual processor/executor и ingress adapters; отдельно simultaneous BLE peripheral MiniPOS и central BluePad.

Cases: все объявленные capabilities; cash/card/split/storno/reports/cash movements/diagnostics по profile; initialization/wake/subscription/SEQ; byte fragmentation/corruption/partial write/late reply/disconnect; device executed + response lost + host restart с сохранённым device state. Assertions: wire trace, корректный typed result и journal, 0 повторных receipt/payment actions, per-tender evidence и безопасный UNKNOWN.

Registry-driven runner автоматически включает существующие и новые device/protocol profiles; каждый enabled runtime profile обязан иметь versioned vendor sources/fixtures, command/error/recovery coverage и исполненные required scenarios. Missing profile/case, unsupported unexpected command или SKIP обязательного case проваливают gate. Mutation checks для LEN/BCC/command/SEQ/wake/write completion/repeated purchase должны падать. Evidence — EMULATED_RUNTIME с source/build hashes и exact profile; отдельно требуется PHYSICAL_HIL. Подробные architecture/deliverables и Definition of Done — в W9.

## Разделение software и аппаратной приёмки

Для автоматического результата **до подключения устройств** использовать [A0–A11 и phase/case manifest](07_AUTOMATION_EXECUTION.md): HOST/SERVICE_INTEGRATION/CROSS_BUILD обязательны, PHYSICAL_HIL явно HIL_PENDING. T-02/04/20/22 делятся на cases этих phases; T-21 остаётся physical. Существующий критерий ниже относится к полной предварительной приёмке и не требует устройств для начала software fixes. Требуемый T-25 runner ещё предстоит реализовать; наличие описания тестов не считается executed evidence.

## Обязательный критерий предварительной приёмки

Применимые T-01…T-23 и обязательный T-25 зелёные на фиксированном release candidate; T-24 нужен только при включённом Tremol. Покрытие T-25 расширяется при каждом новом поддерживаемом устройстве/версии протокола. Нет незакрытых P0; P1 закрыты или соответствующий profile исключён и блокируется программно. Все required cases исполняются без unexpected SKIP, evidence привязано к exact artifacts. Fault matrix доказывает: **0 повторных списаний/чеков, 0 потерянных approved tenders, 0 terminal operations без recoverable result/outbox, 0 ложных success**, а ambiguous states остаются видимыми для reconciliation.

Обязательный acceptance отчёт: test/case ID → GAP → runtime/profile → build hash → command/fixture → expected/actual state/counters → logs/artifacts → reviewer. Coverage percentage и число PASS сами по себе не заменяют этот отчёт.
