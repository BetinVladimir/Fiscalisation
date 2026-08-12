# MVP1: требования и инструкции для кодогенерации

## 1. Граница компонентов

| Компонент | Путь | Ответственность MVP1 |
|---|---|---|
| MiniPOS UI/backend | `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos` | POS intent, собственная БД, REST/WebHook, BLE client |
| Fiscal backend | `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend` | authoritative sale, УНП, route selection, durable commands, sync/materialization |
| BlueCash adapter | `/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app` | MQTT/BLE transport, durable journal, Datecs fiscal/pinpad I/O |
| ESP32 mandatory adapter | `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3` | DP-150 MX/COM + BluePad-50 Plus/BLE composite route и Daisy Compact S 01/USB route |

MiniPOS не обращается к MQTT, vendor SDK или внутренней БД Fiscal. Direct BLE
разрешён только по REST-issued session ticket и передаёт `ComplianceIntent`, а
не raw Datecs command.

## 2. Обязательные MVP-инварианты

1. Tenant берётся только из проверенной identity; client body не выбирает его.
2. Register имеет ровно один active fiscal endpoint и не более одного active
   payment endpoint. Для edge-agent-s3 они могут находиться внутри одного
   composite adapter binding и использовать разные vendor/transport.
3. Snapshot маршрута фиксируется при резервировании необратимой операции:
   `device_id`, adapter kind, vendor/model, binding version, route generation.
4. Потеря cloud допускает BLE fallback. Потеря конечного ФУ блокирует продажу.
5. `CARD` при активном terminal сначала выполняет acquiring; fiscal payment
   печатается только после `APPROVED`.
6. Card decline/timeout не должен закрыть fiscal receipt как успешно оплаченный.
7. Команда и её canonical digest записываются durable до vendor I/O.
8. MiniPOS генерирует отдельный UUIDv4 `client_operation_id` для каждой
   изменяющей операции до первого send. Повтор через REST, MQTT или BLE сохраняет
   UUID и даёт максимум один физический side effect.
9. Ambiguous result переходит в `UNKNOWN`; автоматический повтор запрещён.
10. Результат подписывается устройством, хранится локально и выгружается через
    MQTT до business ACK backend.
11. PUBACK не является business ACK и не разрешает удаление journal.
12. Неподтверждённые записи не удаляются. Подтверждённые хранятся минимум 90 дней.
13. Split tender передаётся одной агрегированной business-командой и исполняется
    как последовательность физических команд в одной durable receipt session;
    нельзя печатать отдельный чек на каждую часть оплаты.
14. RRN, authorization code, STAN и terminal outcome сохраняются durable до
    изменения authoritative sale state.
15. REST и BLE должны материализовать одинаковые sale/operation/WebHook projections.
16. REST/WebHook primary и direct BLE fallback обязательны для каждого MVP
    profile; переключение выполняется автоматически по cloud connectivity state.
17. Одна sale/receipt session может продолжаться через другой transport без
    изменения identifiers, route snapshot и без повторного physical side effect.
18. Восстановление ping возвращает новые intents на REST только после hysteresis,
    device journal sync и reconciliation authoritative projection.
19. Backend не заменяет и не регенерирует `client_operation_id`; внутренний
    `operation_id` может быть равен ему. Если нужен отдельный server ID, оба
    значения хранятся неизменно, а lookup/dedupe работает по client ID.
20. HTTP `Idempotency-Key` должен быть равен `client_operation_id` либо
    однозначно и проверяемо ссылаться на него; два независимых idempotency keys
    одной операции запрещены.

### 2.1 Client operation identity

Каждая mutation содержит UUID `client_operation_id`, operation type,
sale/session binding и `canonical_payload_sha256`. UUID создаётся безопасным
генератором MiniPOS и durable сохраняется в его локальной БД вместе с intent до
сетевого вызова.

Backend выполняет atomic insert-or-get по `(tenant_id, client_operation_id)`.
Adapter выполняет atomic reserve-or-get по
`(binding_generation, client_operation_id)`. В обеих точках сравниваются type,
sale/session binding и canonical digest.

- retry, app restart, MQTT QoS1 redelivery и BLE fallback используют тот же UUID;
- каждая payment leg имеет свой `payment_id`, а весь `SALE_FINALIZE` — отдельный
  `client_operation_id`;
- physical steps finalize имеют отдельные durable `step_id`, но не становятся
  новыми POS operations;
- WebHook и operation lookup обязательно содержат client UUID;
- новый сознательный кассовый intent, даже с тем же payload, получает новый UUID;
- UUID не заменяет authorization и не кодирует tenant/register/operator;
- idempotency record хранится не меньше срока transaction journal.

## 3. Минимальный business command catalog

```text
SALE_OPEN_WITH_LINE
SALE_ADD_LINE
SALE_CHANGE_LINE
SALE_CANCEL_LINE
SALE_CANCEL
SALE_FINALIZE                  # весь чек и ordered payments
SALE_REVERSE
OPERATION_LOOKUP               # только чтение/reconciliation
DEVICE_READINESS
REPORT_X
REPORT_Z
CASH_IN
CASH_OUT
```

Для BlueCash `SALE_FINALIZE` является одной идемпотентной business-командой, но
не одним физическим вызовом. Внутри адаптера она раскрывается в журналируемую
последовательность terminal/fiscal команд под одним `receipt_session_id` и
`client_sale_surrogate_id`. Внешний POS не видит vendor command numbers.

## 3.1 Durable receipt transaction session

Каждая попытка завершить чек создаёт ровно одну session:

```json
{
  "receipt_session_id": "uuid",
  "client_sale_surrogate_id": "uuid",
  "client_operation_id": "uuid",
  "ordered_payments": [
    {"payment_id":"uuid","type":"CASH","amount":{"amount":"5.00","currency":"EUR"}},
    {"payment_id":"uuid","type":"CARD","amount":{"amount":"7.50","currency":"EUR"}}
  ]
}
```

Один и тот же surrogate/session/client operation ID используется при REST retry, MQTT
redelivery, BLE fallback, reboot recovery и sync. Изменение payload под тем же ID
возвращает `IDEMPOTENCY_PAYLOAD_CONFLICT`.

Минимальные durable состояния:

```text
RESERVED
→ CARD_AUTHORIZING
→ CARD_APPROVED
→ FISCAL_OPENING
→ FISCAL_OPEN
→ LINES_REGISTERING
→ PAYMENTS_REGISTERING
→ FISCAL_CLOSING
→ COMMITTED

любое состояние до COMMITTED
→ COMPENSATION_REQUIRED
→ CARD_REVERSING
→ COMPENSATED | RECOVERY_REQUIRED
```

Каждый переход и vendor request/result записывается и fsync-ится до перехода к
следующему физическому вызову. В journal сохраняются request digest, attempt,
terminal STAN/RRN/auth/reference, Datecs sequence/command, response/status и
certainty (`NOT_SENT`, `SENT_RESULT_KNOWN`, `SENT_RESULT_UNKNOWN`).

### Исполнение

1. Проверить ФУ и terminal, totals, ordered payments и отсутствие другой active
   receipt session на ФУ.
2. Durable сохранить `RESERVED` и полный immutable plan.
3. Для каждой `CARD` части последовательно выполнить terminal authorization.
   После каждого `APPROVED` durable сохранить RRN/auth/STAN до следующих I/O.
4. Если любая card часть отклонена, отменить/void уже одобренные card части в
   обратном порядке. Фискальный чек ещё не открывать.
5. После одобрения всех card частей открыть один фискальный чек: Datecs `48`.
6. Передать все позиции через `49` в исходном порядке.
7. Передать все виды оплаты через `53` в порядке `ordered_payments`.
8. Закрыть чек через `56`. Только подтверждённый успешный close является
   `COMMITTED`.
9. Подписать result, записать outbox и вернуть success.

### Откат и компенсация

У ФУ и независимого card terminal нет общей ACID-транзакции. Поэтому «откат
всего» означает доказанную компенсацию всех уже выполненных side effects:

- до открытия ФУ: reverse/void всех approved card операций;
- после открытия ФУ, но до подтверждённого close: выполнить разрешённую vendor
  cancel-receipt команду, затем reverse card операций;
- после успешного fiscal close: обычный cancel недопустим; выполнить нормативное
  fiscal storno и reverse card операций как отдельные, связанные compensation
  operations;
- при неизвестном результате любого шага сначала выполнить lookup/status, а не
  повтор команды.

`COMPENSATED` разрешён только когда подтверждены и fiscal compensation, и все
card reversals. Если состояние хотя бы одного устройства неоднозначно,
результат — `RECOVERY_REQUIRED`; продажа и новая session на кассе блокируются до
reconciliation. Нельзя возвращать `FAILED` так, будто side effects отсутствуют.

`KLEN` и `FISCAL_MEMORY` требуются как report/export capability перед заявлением
полного фискального охвата, но могут быть второй итерацией MVP1, если release
явно называется sales-only controlled MVP. Z обязателен для закрытия смены.

## 4. Нормативный transport command

Не создавать отдельные несовместимые MQTT/BLE DTO. В OpenAPI определить один
`DeviceCommandEnvelopeV2`:

```json
{
  "version": 2,
  "client_operation_id": "uuid",
  "tenant_id": "uuid",
  "location_id": "uuid",
  "register_id": "uuid",
  "route_id": "uuid",
  "device_id": "uuid",
  "binding_version": 7,
  "command_type": "SALE_FINALIZE",
  "issued_at": "RFC3339Nano",
  "expires_at": "RFC3339Nano",
  "payload": {},
  "payload_sha256": "64 hex",
  "authorization": {},
  "signature": "transport-specific signed representation"
}
```

Wire field `operation_id` может временно дублировать `client_operation_id`, но
источник значения всегда MiniPOS; backend не создаёт новое значение при смене
transport.

MQTT serializes envelope as canonical JSON. BLE переносит ту же semantic model
в deterministic CBOR внутри AEAD. Golden-vector test обязан доказать одинаковый
`payload_sha256` и `client_operation_id`.

## 5. Порядок кодогенерации

### Этап 1 — contract freeze

1. Расширить OpenAPI schemas command types и results.
2. Добавить AsyncAPI topics для всех command types.
3. Добавить BLE mapping `ComplianceIntent.action ↔ DeviceCommandEnvelopeV2`.
4. Сгенерировать Go/TypeScript contracts.
5. Добавить golden JSON/CBOR vectors.
6. Сделать `client_operation_id` required UUID во всех mutation, lookup,
   WebHook, MQTT и BLE schemas.

Готово, когда `make contract-test` проходит и каждый command имеет request,
result, error catalog, idempotency и timeout semantics.

### Этап 2 — backend route resolver

1. Ввести `DeviceRouteResolver` и `DeviceCommandDispatcher`.
2. Удалить предположение об одном глобальном driver для всех касс.
3. Резервировать operation + immutable route snapshot + outbox атомарно.
4. Queue через выбранный adapter; publication failure оставляет durable command.
5. Распространить queue path на reports/cash movements, не только sale/reversal.

### Этап 3 — BlueCash unified processor

1. Расширить processor всем MVP catalog.
2. MQTT и BLE обязаны вызывать один processor.
3. Сохранять `ACCEPTED → EXECUTING → terminal/fiscal result` до I/O.
4. Сделать durable receipt session и card transaction tables.
5. Реализовать пошаговый executor и compensation coordinator.
6. Реализовать lookup/reconciliation без повторной покупки/печати.

### Этап 4 — sync/materialization

1. Sync batch содержит contiguous signed events.
2. Backend проверяет tenant/topic/device, signatures, hashes и cursor.
3. Материализация sale/operation/payment/artifact/WebHook атомарна.
4. ACK подписан независимым per-device key и двигает checkpoint только после
   успешного commit.

### Этап 5 — BLE equivalence

1. REST выдаёт session, связанную с tenant/location/register/edge/FU/operator.
2. MiniPOS проверяет ticket до BLE connect.
3. GATT доказывает possession X25519 key, затем AES-GCM frames.
4. BLE result сначала durable на устройстве, затем возвращается POS.
5. После cloud recovery те же events синхронизируются без второй операции.
6. Реализовать state machine и lightweight ping из документа 09.
7. Проверить cross-transport continuation одной sale и shared device dedupe.

### Этап 6 — regression/HIL

Обязательно проверить cash/card/split/reversal/X/Z, MQTT duplicate, BLE duplicate,
переключение MQTT→BLE, reboot после card approval, reboot после Datecs close,
потерю ACK, недоступный ФУ, недоступный pinpad и SD/DB failure.

## 6. Definition of done

- ни один MVP command не использует simulator/STUB на HIL-пути;
- все команды присутствуют в OpenAPI/AsyncAPI/BLE mapping;
- backend выбирает route по active register binding;
- BlueCash выполняет все обязательные commands;
- split tender исполняется одной durable session и создаёт один fiscal receipt;
- сбой любого шага заканчивается `COMMITTED`, доказанным `COMPENSATED` либо
  блокирующим `RECOVERY_REQUIRED`;
- card reversal работает после process restart;
- MQTT и BLE duplicate дают один physical effect;
- Z close возвращает реальный fiscal reference;
- неподтверждённые events переживают restart и синхронизируются;
- все HIL evidence связывает build hash, device ID и operation IDs.
