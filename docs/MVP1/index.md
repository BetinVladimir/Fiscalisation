# MVP1 — канонические требования physical MVP

Статус: обязательный scope реализации и приёмки.  
Дата фиксации: 2026-08-13.

Этот документ является точкой принятия решений по составу MVP1. При конфликте
со старыми формулировками в каталоге применяется этот scope; конфликтующие
документы должны быть приведены к нему до реализации.

## 1. Цель MVP1

MVP1 — полностью работающая система настройки кассового места, фискализации и
приёма платежей из MiniPOS на следующих физических устройствах:

| ID профиля | Фискальное устройство | Платёжный терминал | Исполняющий adapter | Физический канал |
|---|---|---|---|---|
| `DATECS_BLUECASH50_EMBEDDED` | Datecs BlueCash-50 | встроенный BlueCash pinpad | `SmartDevices/bluecash-app` | встроенные Android vendor API/локальные interfaces |
| `DATECS_DP150_BLUEPAD50` | Datecs DP-150 MX | Datecs BluePad-50 Plus | `IoT/firmware/edge-agent-s3` | ФУ: COM/RS-232 через UART level converter; terminal: BLE |
| `DAISY_COMPACT_S01` | Daisy Compact S 01 | отсутствует либо отдельный поддержанный payment route вне MVP | `IoT/firmware/edge-agent-s3` | USB host, native Daisy protocol |

Все три профиля обязательны. `edge-agent-s3` не является альтернативой
BlueCash: оба adapter runtime входят в один MVP.

## 2. Обязательные пользовательские процессы

### 2.1 Настройка в BeeFiscalApp

В `/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp` сотрудник
активного tenant должен:

1. выбрать торговую точку и кассовое место;
2. выбрать fiscal vendor/model;
3. выбрать adapter (`BLUECASH_ANDROID` или `EDGE_AGENT_S3`);
4. выбрать канал связи с ФУ;
5. при необходимости добавить отдельный payment terminal, его vendor/model и
   канал;
6. увидеть только совместимые комбинации из versioned capability matrix;
7. активировать/привязать adapter device;
8. выполнить physical discovery/probe и сверить serial/FMIN/firmware;
9. сохранить versioned binding и выполнить тестовую диагностику;
10. отключить, перепривязать или заблокировать устройство с audit trail.

Tenant берётся из access token BeeFiscalApp и не выбирается в форме.

### 2.2 Составной edge binding

Для одного кассового места разрешено и требуется поддержать:

```text
one EDGE_AGENT_S3 adapter
  ├── exactly one fiscal endpoint
  └── zero or one payment endpoint
```

Fiscal и payment endpoints могут иметь разных vendor/model/transport, но имеют
общие `tenant_id`, `location_id`, `register_id`, `edge_device_id` и согласованную
`binding_generation`. Для MVP обязательна комбинация DP-150 MX/COM +
BluePad-50 Plus/BLE. Daisy Compact S 01/USB работает без обязательного terminal.

Нельзя моделировать составной edge как два конкурирующих adapter route. Backend
резервирует одну immutable route snapshot, внутри которой находятся fiscal и
optional payment endpoints.

### 2.3 Продажа в MiniPOS

`/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos` работает только через
public Fiscal OpenAPI либо backend-issued direct BLE session. Обязательный путь:

```text
login/operator session
→ open shift/readiness/time verification
→ first line creates sale + UNP
→ lines/discounts/taxes
→ ordered CASH/CARD payments
→ one SALE_FINALIZE receipt session
→ physical card authorization when CARD exists
→ one fiscal receipt
→ signed device journal/sync
→ authoritative result + webhook
→ Z report and shift close
```

Процесс должен работать для каждого из трёх profiles. Потеря конечного ФУ
блокирует продажу. Потеря cloud разрешает direct BLE только при действующей
authority и живом пути MiniPOS→adapter→ФУ; события синхронизируются по MQTT после
восстановления.

### 2.4 Обязательный dual-route и автоматическое переключение

Каждый профиль MVP обязан поддерживать два логических маршрута POS:

```text
PRIMARY:
MiniPOS → public REST → fiscal-backend → MQTT → adapter → physical devices
                                         └→ WebHook → MiniPOS backend

FALLBACK:
MiniPOS → open BLE GATT (MVP exception) → тот же adapter → physical devices
                                              └→ local journal → MQTT sync
                                                                  → backend/WebHook
```

MiniPOS автоматически выбирает маршрут по state machine, описанной в
[`09_DUAL_ROUTE_FAILOVER_PROTOCOL.md`](09_DUAL_ROUTE_FAILOVER_PROTOCOL.md):

- пока lightweight server ping успешен, новые intents идут по REST;
- после подтверждённой потери ping REST circuit открывается и POS автоматически
  использует BLE при наличии server-issued route package и physical readiness;
- после устойчивого восстановления ping новые intents возвращаются на REST;
- открытый чек может начать работу по одному transport и продолжиться по другому;
- transport switch не меняет sale/receipt session/operation/step IDs, payload
  digests, УНП, binding generation или конечные устройства;
- неизвестный результат сначала reconciles/lookup, а не повторяется;
- восстановление cloud не переносит уже выполняющийся BLE physical step на REST;
  текущий шаг завершается/выясняется на устройстве, следующий intent может идти
  через REST после синхронизации authority/projection.

Ping endpoint не обращается к БД, MQTT, registry или fiscal device и не создаёт
business/audit записи. Он проверяет только доступность публичного cloud ingress и
минимальную готовность HTTP process; physical readiness является отдельным probe.

## 3. Функциональный минимум физических адаптеров

### 3.1 BlueCash-50 Android

- physical readiness, FMIN/serial/status/time;
- receipt open, items, discounts/taxes, ordered payments, close/cancel;
- cash and card sale, split tender;
- card approve/decline/timeout/lookup/reversal;
- fiscal storno с подтверждённым vendor path;
- X/Z, cash in/out;
- MQTT и direct BLE через один processor;
- durable receipt/payment journal и recovery после process/device restart.

### 3.2 DP-150 MX через edge-agent-s3

- конфигурируемый COM/UART port, baud/parity/framing и electrical profile;
- полный Datecs fiscal command path для обязательного catalog;
- physical readiness/FMIN/time/status;
- cancel/storno, X/Z, cash in/out и operation lookup;
- журнал до I/O, idempotency, recovery и MQTT sync.

### 3.3 BluePad-50 Plus через edge-agent-s3

- разрешённое BLE pairing/bonding и reconnect;
- terminal identity/capabilities/readiness;
- purchase, decline, timeout, lookup/reconcile и reversal;
- durable STAN/RRN/auth/terminal reference до fiscal close и backend ACK;
- совместная receipt saga с DP-150 MX и compensation.

### 3.4 Daisy Compact S 01 через edge-agent-s3

- ESP32-S3 USB host enumeration и stable device selection;
- native Daisy frame/command/response/status handling;
- readiness, serial/FMIN/time;
- sale/cancel/storno, X/Z, cash in/out;
- recovery после USB disconnect/reconnect и power loss.

## 4. Общие инварианты

1. Один active fiscal endpoint на кассовое место; payment endpoint optional.
2. CARD разрешён только при configured, ACTIVE и physically ready terminal.
3. CASH не блокируется отсутствующим optional terminal.
4. Один checkout создаёт один fiscal receipt, включая split tender.
5. Все side effects journal-before-I/O и idempotent по operation/payment/step ID.
6. Результат только `COMMITTED`, доказанный `COMPENSATED` либо блокирующий
   `RECOVERY_REQUIRED`; неоднозначность нельзя превращать в обычный failure.
7. Route snapshot не меняется после первого physical send.
8. MQTT и BLE вызывают одинаковую business state machine и дают одинаковые
   projections/webhooks.
9. Устройство подтверждает actual identity и readiness; broker connectivity не
   является готовностью ФУ.
10. Подтверждённые записи хранятся минимум 3 месяца; unacknowledged не удаляются.
11. Все public REST contracts описаны OpenAPI; MQTT — AsyncAPI/JSON Schema; BLE —
    versioned deterministic CBOR schema.
12. EUR является единственной валютой MVP1.
13. Все кассовые profiles поддерживают REST/WebHook primary route и direct BLE
    fallback, включая продолжение одной receipt session между transports.
14. Автоматический failover использует hysteresis/circuit breaker; одиночная
    потеря ping или один HTTP timeout не запускают параллельное исполнение.
15. Каждая изменяющая business-операция получает в MiniPOS отдельный UUIDv4
    `client_operation_id` до первой попытки отправки. Он неизменно проходит через
    REST, backend outbox, MQTT, BLE, device journal, sync и WebHook.
16. Идемпотентность определяется ключом
    `(tenant_id, client_operation_id, canonical_payload_sha256)`: повтор того же
    UUID и digest возвращает прежний результат без side effect; тот же UUID с
    другим digest даёт `IDEMPOTENCY_PAYLOAD_CONFLICT`.
17. `client_operation_id` уникален для каждой операции внутри чека: open с первой
    строкой, add/change/cancel line, sale cancel, finalize и reversal. Retry и
    смена REST↔BLE UUID не меняют; новый кассовый intent получает новый UUID.

### 4.1 Явное исключение безопасности BLE для MVP1

Для controlled MVP1 BLE GATT между MiniPOS и adapter **открыт и не требует
авторизации**. Ticket validation, X25519 handshake, HKDF и AES-GCM не являются
release gate `SOFTWARE_COMPLETE_HIL_PENDING` или hardware MVP1.

Открытый transport не отменяет обязательные business-инварианты:

- MiniPOS получает service/characteristic UUID и advertising identity из REST;
- adapter принимает versioned canonical `ComplianceIntent`, а не raw-команды ФУ;
- payload содержит tenant/location/register/edge/final-device/binding generation,
  `client_operation_id` и canonical payload digest;
- binding сверяется с локальной подписанной provisioned конфигурацией;
- journal-before-I/O и общий BLE/MQTT dedupe обязательны;
- mismatch binding, недоступность ФУ или ошибка journal блокируют исполнение.

Это осознанное непроизводственное ограничение. Open BLE запрещён для production.
Production gate требует signed short-lived ticket, revocation/expiry,
X25519/HKDF-SHA-256, directional AES-256-GCM и replay protection.

## 5. Требования к тестированию

Для каждого этапа обязательны:

- unit tests business/domain и protocol builders/parsers;
- shared golden request/response/frame vectors;
- driver-level stub/fake для каждого physical profile;
- backend↔MQTT↔adapter и MiniPOS↔BLE↔adapter integration tests;
- duplicate, timeout, disconnect, restart и journal recovery tests;
- full MiniPOS E2E для каждого profile на driver stub;
- финальный HIL на каждом реальном устройстве и обязательной комбинации.

Driver stub должен реализовывать те же contracts, status bits, errors и
certainty transitions, но evidence помечается `SIMULATED`. Stub PASS закрывает
software integration stage, но не закрывает physical MVP acceptance.

## 6. Критерий готовности

```text
MVP1_GO =
  all software P0 PASS
  AND BlueCash-50 fiscal+card HIL PASS
  AND DP-150 MX COM fiscal HIL PASS
  AND BluePad-50 Plus BLE card HIL PASS
  AND DP-150+BluePad combined checkout/compensation HIL PASS
  AND Daisy Compact S 01 USB fiscal HIL PASS
  AND BeeFiscalApp configure/probe/rebind E2E PASS
  AND MiniPOS cash/card/split/reversal/shift E2E PASS for applicable profiles
  AND no unresolved UNKNOWN or RECOVERY_REQUIRED test transaction
```

Production/legal approval остаётся отдельным последующим gate.

## 7. План и связанные документы

- [`08_PHYSICAL_MVP_IMPLEMENTATION_ROADMAP.md`](08_PHYSICAL_MVP_IMPLEMENTATION_ROADMAP.md)
  — последовательный план реализации и приёмки нового обязательного scope.
- [`09_DUAL_ROUTE_FAILOVER_PROTOCOL.md`](09_DUAL_ROUTE_FAILOVER_PROTOCOL.md) —
  ping, автоматическое REST↔BLE переключение и продолжение открытого чека.
- [`01_MVP_REQUIREMENTS_AND_CODEGEN.md`](01_MVP_REQUIREMENTS_AND_CODEGEN.md) —
  общие business contracts и receipt saga.
- [`02_BLUECASH_COMMAND_TRACEABILITY.md`](02_BLUECASH_COMMAND_TRACEABILITY.md) —
  трассировка BlueCash.
- [`04_EDGE_AGENT_S3_EQUIVALENT_TRACK.md`](04_EDGE_AGENT_S3_EQUIVALENT_TRACK.md) —
  обязательные DP-150/BluePad/Daisy edge profiles.
- [`05_BACKEND_DEVICE_ROUTE_SELECTION.md`](05_BACKEND_DEVICE_ROUTE_SELECTION.md) —
  composite route model.
- [`06_IMPLEMENTATION_READINESS_AUDIT_AND_CLOSURE.md`](06_IMPLEMENTATION_READINESS_AUDIT_AND_CLOSURE.md)
  и [`07_BLOCKERS_AND_MVP_DECISION.md`](07_BLOCKERS_AND_MVP_DECISION.md) — gaps и
  внешние evidence gates.
