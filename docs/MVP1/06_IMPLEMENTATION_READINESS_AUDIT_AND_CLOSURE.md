# Аудит готовности реализации и план закрытия MVP1

Статус: обязательная спецификация закрытия разрывов; повторно подтверждена по
HEAD `963d4e3` без повышения незакрытых P0.

Дата повторного среза кода: 2026-08-13.
Область проверки: `docs/MVP1`, `docs/SUPTO/index.md`, contracts, MiniPOS,
`fiscal-backend`, BlueCash Android, BeeFiscalApp, БД, MQTT и compose.

## 1. Вердикт

Описанные ранее доработки задают правильную архитектуру, но **недостаточны для
признания текущей реализации рабочим MVP**. Они не покрывали несколько
межмодульных несовместимостей, которые проявляются только при сопоставлении
фактического wire-format и physical path.

MVP1 считается рабочим только когда:

1. все строки `MVP1-P0-*` ниже имеют статус `PASS` с привязанным test/evidence;
2. один чек с ordered split tender проходит как одна durable receipt session;
3. оба маршрута REST/MQTT и direct BLE дают эквивалентный физический результат;
4. timeout/reboot не приводит к повторному списанию или повторному чеку;
5. конечное ФУ, а не только broker/app, подтверждает readiness и свой FMIN;
6. два compose запускаются раздельно и проходят E2E через Caddy;
7. реальный BlueCash-50 проходит HIL matrix.

Это controlled non-production MVP. Он не означает `BG-014 PASS`, декларационную
готовность СУПТО или допуск в production. Текущий machine trace показывает
`PASS=3`, `PARTIAL=19`, `NOT_APPLICABLE=2`; verifier правильно сохраняет
production block.

Термин `base functional software MVP PASS` в корневом
`MVP_COMPLETION_AUDIT.md` относится только к simulator/STUB profile и не является
синонимом рабочего physical MVP, определённого здесь. Для physical MVP действует
только критерий этого каталога и решение документа 07.

## 2. Что уже реализовано и пригодно как основа

| Контур | Подтверждено в коде | Оценка |
|---|---|---|
| MiniPOS sale opening | первый товар вызывает `clock-sync`, readiness, workstation session и `/sales:open-with-line`; серверная projection отображает УНП | основа есть |
| Backend compliance model | UNP, readiness lease, session/operator, sale lifecycle, audit/export и trace tests существуют | software semantics в основном есть |
| MQTT command transport | signed QoS1 command outbox, expiry, fencing token, sync batches/ACK | основа есть, маршрутизация и readiness неполны |
| BlueCash fiscal path | Datecs 48/49/53/56 и storno 43 реализованы | только базовая продажа/storno |
| BlueCash card path | purchase и reverse wire commands присутствуют | recovery недолговечен, bridge имеет stub |
| Direct BLE | ticket, X25519 handshake, encrypted framed GATT command channel, local SQLite sale store | aggregate payment/session неполны |
| Local journal | SQLite append-only chain, MQTT batch/ACK, purge acknowledged records старше 3 месяцев | формат подписи несовместим с backend |
| Deployment | отдельные fiscalisation/minipos compose и Caddy overlays для dev/prod | E2E не доказывает physical MQTT route |

## 3. Обязательные P0-разрывы

### MVP1-P0-001 — единый формат device ECDSA signature

**Факт.** Android `AndroidKeystoreSigner` использует `SHA256withECDSA` и возвращает
ASN.1 DER. Backend `verifyDeviceSignature` принимает fixed-width P-256
IEEE-P1363 `r || s`. Сейчас hardware-signed event/batch будет отвергнут.
Дополнительно обе стороны фактически подписывают SHA-256 digest алгоритмом,
который хеширует вход ещё раз. Это допустимо только если явно закреплено, но
создаёт ненужный и плохо переносимый double-hash contract.

**Требование.** Закрепить один wire profile:

```text
algorithm = ECDSA-P256-SHA256
canonical_bytes -> SHA-256 -> raw ECDSA NONEwithECDSA
signature = base64url(no-pad, r[32] || s[32])
kid = base64url(SHA-256(canonical public JWK))
wire = kid + ":" + signature
```

Если Android provider не предоставляет `NONEwithECDSA`, DER из
`SHA256withECDSA(canonical_bytes)` преобразуется в P1363; digest не передаётся в
hashing signature API повторно. Один helper обязан использоваться для event и
batch. Activation proof может иметь отдельный profile, но он должен быть явно
назван и покрыт cross-language vectors.

**Acceptance:** 20 shared golden vectors Go/Kotlin; отрицательные DER,
high/oversized R/S, wrong kid, mutated payload; реальный Android Keystore batch
принят backend и ACK проверен устройством.

### MVP1-P0-002 — end-device readiness вместо broker readiness

**Факт.** `mqttclient.Bridge.Probe()` подтверждает только `client.IsConnected()`.
Service использует этот результат для readiness/clock decisions. Он не доказывает
доступность BlueCash app, fiscal socket, ФУ, paper/FM state, FMIN или pinpad.

**Требование.** Ввести signed request/reply `DEVICE_PROBE` через тот же durable
dispatcher. Ответ содержит отдельно:

```yaml
device_app: {reachable, app_version, observed_at}
fiscal: {reachable, ready, fmin, serial, firmware, status_bits, open_document}
payment_terminal: {configured, reachable, terminal_id, version, status}
binding_version: integer
probe_nonce: string
```

Readiness lease создаётся только по свежему физическому ответу с совпадающими
tenant/register/device/binding/FMIN. Broker connectivity остаётся отдельной
метрикой. Перед card sale terminal readiness обязателен; cash sale не должна
блокироваться только из-за отсутствия optional pinpad.

**Acceptance:** выключение ФУ при живых MQTT и BlueCash блокирует open/payment;
cloud outage при живом BLE→BlueCash→ФУ допускает локальный маршрут; неверный
FMIN блокирует кассовое место.

### MVP1-P0-003 — общий durable `SALE_FINALIZE` coordinator

**Факт.** MQTT принимает только `FISCAL_SALE|REVERSAL`; backend формирует одну
команду на один payment; BlueCash закрывает отдельный чек на команду. BLE хранит
единственный `payment`. Это нарушает требование одного чека со split tender.

**Требование.** Реализовать immutable aggregate:

```yaml
receipt_session_id: uuid
client_sale_surrogate_id: uuid
operation_id: uuid
unp: BG_UNP_V1
items: ordered immutable array
payments: ordered array
expected_total: EUR money
operator_binding: {subject, operator_code, fiscal_operator_number}
```

Каждый физический шаг journal-before-I/O и имеет `step_id`, attempt, request hash,
result, certainty и vendor reference. State machine и compensation rules из
документа 01 обязательны. Повторный `operation_id` продолжает session, а не
создаёт чек/списание.

**Acceptance:** cash; card; cash+card; две card legs; decline; timeout до/после
approval; reboot после каждого шага; fiscal failure после card approval;
compensation failure → `RECOVERY_REQUIRED`. Ровно один fiscal receipt и не более
одного успешного side effect на payment leg.

### MVP1-P0-004 — durable payment journal и terminal reconciliation

**Факт.** `BoricaPinpadCodec.originals` хранит amount/RRN/auth только в памяти.
После process death reversal невозможен. Не реализован надёжный lookup outcome
после неоднозначного timeout.

**Требование.** До card I/O сохранять payment transaction с operation/payment
IDs, amount, terminal ID, STAN allocation, request hash и `PREPARED`. После
ответа атомарно сохранять response code, RRN, auth code, STAN, timestamps,
approved/declined/unknown и raw evidence hash. Реализовать vendor lookup/status;
при `UNKNOWN` запрещена повторная purchase до reconciliation. Reverse использует
durable original fields. Удаление — только после backend ACK и 3 месяцев.

**Acceptance:** force-stop/reboot после send и после approval, затем автоматическое
восстановление без duplicate charge; reverse после reboot проходит.

### MVP1-P0-005 — полная семантика line/payment payload

**Факт.** MQTT parser BlueCash переносит name/tax/price/quantity, но принудительно
ставит `department=0` и теряет discount/unit. Поэтому server totals могут
отличаться от физического чека.

**Требование.** Canonical OpenAPI schema обязана включать line ID, name,
quantity(3), unit price EUR(2), tax group, discount type/value, department, unit;
payment ID/type/amount/order. Backend → MQTT → Kotlin mapping без defaulting,
кроме явно versioned profile defaults. Перед close вычислить expected total и
сверить с ФУ/terminal totals; mismatch → cancel/recovery, но не COMMITTED.

**Acceptance:** shared golden receipts для A–H, quantity, amount/percent discount,
rounding, split payments; байты Datecs и backend materialized totals совпадают.

### MVP1-P0-006 — production vendor bridge в BlueCash

**Факт.** `DatecsAndroidPorts.DefaultDatecsPinpadCodec` и
`VendorDatecsPaymentPort` содержат `*_NOT_INSTALLED` stubs. Наличие низкоуровневого
codec само по себе не доказывает, что runtime использует его на устройстве.

**Требование.** Composition root release build обязан выбирать реальный fiscal
и pinpad adapters, проверять permissions/socket lifecycle и запрещать stubs.
Build-time/static gate должен падать при stub binding вне debug. Daisy SMART
остаётся отдельным заявленным stub и не может участвовать в BlueCash MVP evidence.

**Acceptance:** release APK static gate + instrumentation test подтверждают
фактические classes; HIL purchase/reverse и fiscal sale/storno проходят.

### MVP1-P0-007 — полный durable dispatcher для всех физических команд

**Факт.** MQTT `Prepare` строит только sale/reversal. Reports и cash movement
идут через synchronous `Driver.Execute`, который для MQTT возвращает
`MQTT_ASYNC_DRIVER_REQUIRES_QUEUE`; cash handler теряет amount/operator.

**Требование.** `DEVICE_PROBE`, `CLOCK_GET/SET`, `SALE_FINALIZE`, `REVERSAL`,
`RECEIPT_CANCEL`, `REPORT_X`, `REPORT_Z`, `CASH_IN`, `CASH_OUT` проходят через
один outbox/dispatcher/status API. REST command не сообщает success до signed
physical result. Cash payload не может терять amount/operator.

**Acceptance:** contract + integration test для каждой команды, reconnect,
expiry, duplicate QoS1 и signed result; X/Z/cash verified на HIL.

### MVP1-P0-008 — route resolver и запрет silent fallback

**Факт.** backend выбирает один глобальный MQTT driver при старте либо simulator.
Нет per-register resolution по активному binding/capabilities. Это не обеспечивает
выбор правильного BlueCash/ESP32 route.

**Требование.** Реализовать документ 05: authoritative binding, role/capability,
route health, fencing, route freeze после первого physical send. Simulator
разрешён только explicit test profile. Неоднозначный/stale binding → BLOCK.

**Acceptance:** два кассовых места с разными adapters получают команды только на
свой device topic; rebind инвалидирует старый fencing token; после send route не
переключается автоматически.

### MVP1-P0-009 — operator/FMIN/time identity end-to-end

**Факт.** BlueCash runtime получает fiscal operator number/password/till из
локального UI/config, а MQTT `operator_id` не определяет эти значения. FMIN также
не подтверждается physical read в текущем probe. Backend clock sync не означает
установку/проверку часов ФУ.

**Требование.** Activation/binding хранит versioned mapping backend operator code
→ vendor operator number и защищённый credential reference; raw password не
передаётся в command/journal. Device сверяет actual FMIN/serial с binding.
`CLOCK_GET/SET/VERIFY` выполняется на ФУ минимум ежедневно и на startup policy.

**Acceptance:** два оператора создают правильные УНП и vendor operator fields;
wrong credential/FMIN блокирует; изменение времени создаёт before/after/source
event и повторно считывается с ФУ.

### MVP1-P0-010 — MQTT security/configuration и broker как часть stack

**Факт.** compose принимает внешний `EMQX_BROKER`, но не поднимает broker.
Backend Paho config устанавливает username/password, но не показывает TLS trust
configuration; Android использует issued certificate/socket factory. Два compose
сами по себе не создают воспроизводимый end-to-end MQTT environment.

**Требование.** Для dev/e2e добавить отдельный broker service/profile с TLS,
per-device ACL и healthcheck либо формально описать обязательный external
dependency и provisioning script. Backend должен валидировать `ssl://`/TLS CA в
non-dev. ACL: device читает только свой commands/acks и пишет только свой
activation/sync; backend имеет scoped service principal. Retained commands
запрещены.

**Acceptance:** unauthorized cross-tenant subscribe/publish denied; plaintext
prod URI rejected; compose health waits for broker; reconnect QoS1 suite PASS.

### MVP1-P0-011 — sync materialization для saga и recovery

**Факт.** backend materializer понимает ограниченные terminal events
`FISCALIZED/REVERSED/FAILED/UNKNOWN`; детальные payment/fiscal steps и
compensation не являются authoritative receipt session.

**Требование.** Расширить event schema и materializer для step events, payment
transactions, receipt session state, certainty и compensation. ACK выдаётся
только после одной DB transaction: chain verify + events + projections + outbox.
Повтор batch возвращает тот же ACK; gap/fork/quarantine доступны оператору.

**Acceptance:** replay batch, ACK loss, duplicated batch, gap, fork, out-of-order,
backend restart; projection всегда одна и воспроизводима replay из events.

### MVP1-P0-012 — OpenAPI/AsyncAPI, DDL и traceability lock

**Требование.** До кодогенерации добавить/обновить:

- public OpenAPI: finalize aggregate, operation status/reconcile, readiness,
  clock, reports, cash, BLE session и webhooks;
- device AsyncAPI либо versioned JSON Schemas: MQTT command/event/ACK topics;
- BLE CBOR schemas с теми же semantic DTO;
- DDL/data dictionary: receipt sessions, physical steps, payment transactions,
  route bindings/leases, readiness observations, sync quarantine;
- trace registry `MVP1-P0-* → API → DB → implementation → test → HIL evidence`.

Generated contracts должны проверяться drift tests. Нельзя считать README
исполняемым контрактом вместо схемы.

**Acceptance:** 100% runtime endpoints/messages находятся в contracts; examples
валидируются; migration проходит на empty и upgraded DB; trace verifier не
разрешает `PASS` без существующего теста/evidence.

### MVP1-P0-013 — Platform device API зарегистрирован в runtime

**Факт повторного аудита.** `make contract-test` на commit `963d4e3` падает:
OpenAPI содержит восемь операций platform device registry, для которых verifier
не находит router registration:

```text
POST /platform/v1/manufacturing/devices:register
GET  /platform/v1/devices
GET  /platform/v1/devices/{device_id}
POST /platform/v1/devices/{device_id}:assign-tenant
POST /platform/v1/devices/{device_id}:unassign-tenant
POST /platform/v1/devices/{device_id}:suspend
POST /platform/v1/devices/{device_id}:resume
POST /platform/v1/devices/{device_id}:retire
```

**Требование.** Либо реализовать и зарегистрировать операции в отдельном
Admin/Platform API с точным documented base path и AdminApp client, либо удалить
их из текущего runtime contract и явно перенести в последующую версию. Для MVP с
централизованным управлением и tenant assignment рекомендуется реализация, а не
удаление. Public tenant Fiscal API не должен случайно публиковать platform-admin
surface. Нужны отдельные issuer/audience/scopes, rate limits и audit events.

**Acceptance:** `verify_contract_surface` PASS; generated client вызывает все
восемь endpoints; manufacturing identity proof проверяется; lifecycle и
assign/unassign имеют CAS/idempotency/audit; tenant actor не получает platform
access; AdminApp integration test PASS.

### MVP1-P0-014 — automatic REST/WebHook ↔ BLE failover

**Требование нового scope.** Каждый кассовый profile обязан автоматически
переключать POS с primary REST route на direct BLE при подтверждённой потере
cloud и возвращать новые intents на REST после устойчивого восстановления,
синхронизации и reconciliation. Открытая sale/receipt session продолжается между
transports с теми же IDs и общей device-side idempotency.

Добавить `HEAD /connectivity/ping`: Caddy→in-memory backend liveness без DB,
MQTT, registry, audit и business middleware. Статический ответ одного Caddy не
достаточен. Ping не заменяет physical readiness.

Полный алгоритм, thresholds, authority, uncertain-send lookup и 23 acceptance
сценариев заданы в документе 09.

**Acceptance:** cross-transport duplicate создаёт ровно один physical effect;
receipt начинается REST и заканчивается BLE, а также наоборот после sync; один
потерянный ping не вызывает switch; восстановление не запускает параллельный
step; 1000-client ping test не создаёт DB/MQTT/business calls.

### MVP1-P0-015 — MiniPOS-generated UUID каждой операции

**Требование.** Каждая изменяющая операция внутри чека получает собственный
UUIDv4 `client_operation_id` в MiniPOS до первого send. UUID durable сохраняется
с intent и проходит без замены через REST/Idempotency-Key, backend/outbox, MQTT,
BLE, device journal, sync, lookup и WebHook.

Backend и device атомарно сохраняют UUID вместе с operation type, sale/session
binding и canonical payload digest. Повтор того же UUID/digest возвращает прежнее
состояние; другое содержимое под тем же UUID блокируется как
`IDEMPOTENCY_PAYLOAD_CONFLICT`. Новый сознательный кассовый intent всегда имеет
новый UUID.

**Acceptance:** concurrent REST duplicates, REST→BLE, BLE→MQTT late delivery,
restart MiniPOS/backend/adapter, payload conflict и retention tests доказывают
один logical и physical effect.

## 4. Минимальный порядок реализации

1. P0-012/P0-013/P0-014/P0-015 schemas, operation identity, router/ping boundary,
   DDL и trace skeleton.
2. P0-001 crypto wire compatibility.
3. P0-006 real runtime vendor composition.
4. P0-008 route resolver и P0-010 broker/ACL.
5. P0-002 readiness + P0-009 identity/time.
6. P0-005 lossless fiscal payload.
7. P0-004 durable payment journal/reconciliation.
8. P0-003 aggregate coordinator/compensation.
9. P0-007 reports/cash/cancel through dispatcher.
10. P0-011 sync materialization/replay.
11. automated regression, затем physical HIL/fault campaign и bugfix cycle.

Нельзя начинать HIL acceptance до закрытия 001, 005 и 006: иначе тестируется
не тот runtime либо backend гарантированно отвергает evidence.

## 5. Acceptance matrix рабочего MVP

| Gate | Минимальный результат |
|---|---|
| Contracts | OpenAPI/AsyncAPI/BLE schemas validate; generated drift = 0 |
| DB | fresh/upgrade migrations; RLS tenant isolation; crash-atomic saga writes |
| Crypto | Kotlin↔Go golden vectors и Android Keystore integration PASS |
| Online | MiniPOS→Caddy→backend→MQTT→BlueCash→ФУ/pinpad→sync→webhook PASS |
| BLE fallback | backend ticket→MiniPOS BLE→BlueCash→ФУ/pinpad; later MQTT sync PASS |
| Device loss | живой broker + отключённое ФУ блокирует sale/payment |
| Cloud loss | живой BLE/FU выполняет локально, сохраняет journal и синхронизирует |
| Payment safety | timeout/reboot matrix доказывает no duplicate charge |
| Fiscal safety | no duplicate receipt; cancel before close; storno after close |
| Split tender | один чек, ordered legs, exact EUR total |
| Operations | X, Z, cash in/out, shift close return real signed evidence |
| Retention | только ACKed records старше 3 месяцев удаляются; unacked сохраняются |
| Routing | correct device per register; fencing/rebind/no fallback verified |
| Deployment | оба dev/prod compose config-valid; dev E2E через два Caddy entrypoints |
| HIL | реальный BlueCash-50, vendor firmware/version recorded, evidence signed |

## 6. Что допустимо оставить после MVP1

При controlled MVP можно отложить production secure boot/flash encryption/eFuse,
массовый provisioning/OTA, Web Bluetooth, несколько active devices одной роли,
invoice, tips/cashback/preauth, полный KLEN/FM/PLU/department reporting и Daisy
SMART real driver. Нельзя откладывать transport security, real fiscal readiness,
один-чек semantics, durable card recovery, signed journal compatibility, X/Z и
cash movement, если заявляется управление сменами.

## 7. Definition of Done

Фраза «MVP реализован» допустима только при наличии machine-readable release
manifest, содержащего commit SHA, contract hashes, APK/backend image digests,
selected BlueCash model/serial/FMIN/firmware, результаты всех gates и ссылки на
HIL evidence. Любой незакрытый `MVP1-P0-*` означает `MVP_NOT_READY`.
