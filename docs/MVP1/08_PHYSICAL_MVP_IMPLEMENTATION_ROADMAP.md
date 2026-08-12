# Roadmap реализации обязательного physical MVP1

Порядок обязателен: сначала полная логика на driver stubs с минимальными
проверками, затем единый regression/fault cycle, затем HIL каждого устройства и
bugfix до выполнения `MVP1_GO` из `index.md`.

## Этап 01 — canonical contracts и trace registry

**Сделать:** обновить OpenAPI aggregate checkout/device configuration/status;
добавить AsyncAPI/JSON Schemas MQTT и CBOR BLE; определить fiscal/payment endpoint,
composite edge route, capabilities, errors и receipt saga; устранить восемь
незарегистрированных Platform API operations. Во всех mutation contracts сделать
required `client_operation_id` UUID и провести его через Idempotency-Key, MQTT,
BLE, lookup, sync result и WebHook.

**Документы:** `index.md`, 01, 05, 06/P0-012/P0-013.

**Завершённость:** generated Go/TypeScript/Kotlin/C++ DTO и shared vectors.

**Приёмка:** contract drift/surface tests PASS; runtime surface 100%; никакого
client-supplied tenant/vendor routing.

## Этап 02 — DDL и repositories

**Сделать:** migrations/data dictionary для endpoint profiles, composite routes,
bindings/generations, readiness observations, receipt sessions, physical steps,
payment transactions, sync quarantine и evidence references; RLS/CAS/unique
active fiscal endpoint constraints. Добавить immutable idempotency record с
unique `(tenant_id, client_operation_id)`, operation type, sale/session binding,
canonical digest, state/result и retention; на adapter — индекс по binding
generation + client UUID.

**Документы:** 01/3.1, 05/2, 06/P0-003/004/008/011/012.

**Завершённость:** fresh и upgrade migrations, repositories без in-memory
authority.

**Приёмка:** PostgreSQL concurrency/crash/tenant isolation tests PASS; 100
concurrent requests одного UUID создают одну operation/outbox; digest conflict не
создаёт side effect.

## Этап 03 — Platform/Admin device API

**Сделать:** отдельный protected Platform API для manufacturing registry,
assign/unassign/suspend/resume/retire; tenant API для activate/bind/probe/rebind;
audit каждого изменения.

**Документы:** 06/P0-013, 07/SW-13, activation docs.

**Завершённость:** все platform operations зарегистрированы и доступны generated
AdminApp client только с platform scopes.

**Приёмка:** lifecycle/CAS/idempotency/cross-tenant/RBAC tests PASS.

## Этап 04 — BeeFiscalApp configuration workflow

**Сделать:** UI выбора location/register, fiscal vendor/model/adapter/channel,
optional terminal; compatibility filtering; BlueCash activation; edge composite
configuration; physical discovery/probe; diagnostics; detach/rebind/block.

**Документы:** `index.md` 2.1–2.2, 05.

**Завершённость:** сохранён versioned binding с actual serial/FMIN/firmware;
tenant не выбирается.

**Приёмка:** Android/iOS/Web UI tests и browser E2E на driver stubs; invalid
combination и stale generation fail closed.

## Этап 05 — backend route resolver и dispatcher

**Сделать:** per-register resolver, composite endpoint snapshot, capability
check, fencing, route freeze; единый durable dispatcher для probe/time/finalize/
reverse/cancel/X/Z/cash/lookup; signed result materializer.

**Документы:** 05, 06/P0-002/007/008/009/011.

**Завершённость:** BlueCash и edge routes сосуществуют; global driver не выбирает
business route.

**Приёмка:** multi-tenant/multi-register/duplicate/rebind/outbox restart tests.

## Этап 05A — lightweight ping и automatic transport controller

**Сделать:** `HEAD /connectivity/ping` через Caddy к in-memory backend liveness
handler без DB/MQTT/business middleware; MiniPOS connectivity state machine,
hysteresis/circuit breaker, pre-issued BLE authority, automatic REST→BLE и
BLE→REST; shared cross-transport IDs/lookup/dedupe.

**Документы:** `index.md` 2.4, 09, 01/инварианты 16–18, 05/6.

**Завершённость:** открытая sale продолжается между transports; cloud recovery
не создаёт параллельный physical step.

**Приёмка:** все 23 сценария документа 09 на stubs; ping load test доказывает
нулевые DB/MQTT/business calls; HIL failover включён в этап 16.

## Этап 06 — общий driver-stub слой

**Сделать:** deterministic stubs:

- `BlueCash50DriverStub` — fiscal + embedded card;
- `DP150MXDriverStub` — fiscal over COM semantics;
- `BluePad50PlusDriverStub` — card over BLE semantics;
- `DaisyCompactS01DriverStub` — fiscal over USB semantics.

Каждый поддерживает success, decline, timeout-before-send, unknown-after-send,
disconnect, status bits, malformed response и reboot recovery. Result обязательно
`simulated=true`.

**Документы:** `index.md` 5, 03.

**Завершённость:** одинаковый interface с production driver; static release gate
запрещает stub в physical profile.

**Приёмка:** contract/golden/fault tests для каждого command и profile.

## Этап 07 — receipt/payment saga

**Сделать:** aggregate `SALE_FINALIZE`, ordered tender plan, journal-before-I/O,
durable terminal evidence, cancel/storno/reversal compensation, lookup и recovery.
Один coordinator используется BlueCash и edge.

Каждая POS mutation имеет отдельный MiniPOS-generated client UUID; physical
steps finalize связаны с UUID родительской операции и имеют собственные step IDs.

**Документы:** 01/3.1, 03/P0, 06/P0-003/004/005/011.

**Завершённость:** один чек и максимум один side effect на payment leg.

**Приёмка:** crash на каждой boundary; cash/card/split/two-card/decline/unknown;
итог только COMMITTED/COMPENSATED/RECOVERY_REQUIRED.

## Этап 08 — BlueCash-50 production adapter

**Сделать:** заменить `*_NOT_INSTALLED`; подключить поддержанные Datecs fiscal и
Datecspay APIs; lossless line/payment mapping; physical probe/time; purchase/
lookup/reversal; receipt/cancel/storno/X/Z/cash; MQTT и GATT в один processor;
P1363-compatible Android Keystore signatures.

**Документы:** 02, 03, 06/P0-001/005/006/009,
07/EXT-01..04/04A/04B/04C.

**Завершённость:** release APK не содержит selected-path stubs.

**Приёмка:** JVM/instrumentation tests, затем полный BlueCash HIL.

## Этап 09 — edge-agent-s3 platform runtime

**Сделать:** FreeRTOS/runtime tasks, watchdog, SD SQLite migrations, composite
binding, TLS MQTT QoS1, GATT BLE, signed journal/sync, trusted time, storage
pressure и recovery; подключить protocol-abstraction.

**Документы:** 04/3.1–3.7, 06/P0-001/002/010/011.

**Завершённость:** MQTT и BLE используют один router/saga; reboot resume работает.

**Приёмка:** native/PlatformIO unit tests, ESP32 integration tests на stubs,
power-cycle journal tests.

## Этап 10 — DP-150 MX COM adapter

**Сделать:** board-specific UART/RS-232 profile, port settings и полный Datecs
fiscal catalog MVP; physical status/FMIN/time/cancel/storno/X/Z/cash/lookup.

**Документы:** `index.md` 3.2, vendor Datecs protocol, 03/06.

**Завершённость:** frame parser обрабатывает partial/corrupt/timeout и status bits.

**Приёмка:** stub integration, electrical review, DP-150 HIL cash/sale/reversal/
reports/disconnect/reboot.

## Этап 11 — BluePad-50 Plus BLE adapter

**Сделать:** BLE discovery allowlist, secure pairing/bonding, reconnect, Datecspay
purchase/status/lookup/reversal, durable terminal references; связать с DP-150
receipt saga.

**Документы:** `index.md` 3.3, 03/4, Datecspay vendor specs.

**Завершённость:** terminal может быть optional endpoint того же edge binding.

**Приёмка:** stub approve/decline/unknown, BLE disconnect/reconnect, real BluePad
HIL и combined DP-150+BluePad compensation matrix.

## Этап 12 — Daisy Compact S 01 USB adapter

**Сделать:** ESP32-S3 USB host lifecycle, VID/PID/interface selection, Daisy
native protocol, status/FMIN/time/sale/cancel/storno/X/Z/cash и reconnect recovery.

**Документы:** `index.md` 3.4, Daisy protocol, 03/04.

**Завершённость:** USB detach не вызывает blind retry; binding сверяет identity.

**Приёмка:** host/parser stub tests, electrical/power evidence и Daisy HIL.

## Этап 13 — MiniPOS complete physical workflow

**Сделать:** настройка не дублируется в MiniPOS; POS получает allowed actions и
route projection, выполняет смену, sale, cash/card/split, reversal, reports;
REST preferred и automatic authorized BLE fallback; возврат после устойчивого
ping+sync; webhook/poll recovery UI.

**Документы:** `index.md` 2.3, 01, 02, external POS protocol.

**Завершённость:** один UI flow работает без vendor-specific command building.

**Приёмка:** полный E2E на каждом driver stub и каждом применимом profile.

## Этап 14 — минимальная сквозная проверка

**Сделать:** поднять два независимых compose через Caddy; provision все profiles;
прогнать happy paths и базовый restart/reconnect.

**Завершённость:** весь функционал реализован, известные дефекты зарегистрированы;
не требуется ещё полный fault campaign.

**Приёмка:** three-profile stub E2E PASS; BlueCash и edge команды маршрутизируются
правильно; API/DB/journal evidence сохранено.

## Этап 15 — полный regression и bugfix

**Сделать:** unit/race/static/contracts/UI/security/RLS/compose; fault injection
на каждой I/O/journal boundary; duplicate/reorder/expiry; storage pressure;
72h/7d accelerated soak. Исправлять дефекты и повторять полный цикл до clean run.

**Приёмка:** два последовательных `full-regression` на неизменённом commit;
machine report без waived P0.

## Этап 16 — physical HIL и решение MVP1_GO

**Сделать:** BlueCash, DP-150, BluePad, combined DP+BluePad и Daisy USB matrices;
MiniPOS REST/MQTT и BLE paths; чек/terminal/KLEN evidence; network/power/reboot.

**Приёмка:** формула `MVP1_GO` из `index.md`; подписанный manifest с commit,
images/APK/firmware hashes, device serial/FMIN/firmware и evidence links. Любой
неустранённый UNKNOWN/RECOVERY_REQUIRED либо external vendor ambiguity означает
NO-GO.
