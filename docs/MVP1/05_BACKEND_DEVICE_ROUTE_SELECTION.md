# Fiscal backend: выбор маршрута устройства

## 1. Текущий дефект

В `cmd/fiscal-backend/main.go` driver выбирается глобально при старте:

```text
EMQX configured → один mqttBridge для всех касс
иначе → один simulator для всех касс
```

Это не учитывает vendor/model/adapter конкретного active register binding.
`activeFiscalDeviceSnapshot` фиксирует device metadata, но dispatch использует
глобальный `s.driver`. Поэтому невозможно корректно сосуществовать BlueCash,
ESP32 и другими smart-device routes, а reports/cash movements обходят MQTT queue.

## 2. Целевая модель данных

Добавить immutable/versioned `device_route`:

```text
route_id
tenant_id
location_id
register_id
role = FISCAL_DEVICE | PAYMENT_TERMINAL
adapter_kind = BLUECASH_ANDROID | EDGE_AGENT_S3 | DAISY_SMART_STUB
device_id
vendor/model/hardware_revision
transport = MQTT_DEVICE | LOCAL_EMBEDDED
command_topic / sync topic namespace
capability set
payment code mapping
binding_version
active_from / active_to / status
```

Register хранит один active adapter route. Route содержит exactly one fiscal
endpoint и optional payment endpoint. У BlueCash оба endpoints embedded в одном
Android adapter; у edge-agent-s3 обязательна поддержка разных типов/каналов в
одном binding, например DP-150 MX/COM + BluePad-50 Plus/BLE. Нельзя выбирать
route по client-supplied vendor или по последнему online устройству.

## 3. Route resolver

Ввести интерфейс:

```go
type DeviceRouteResolver interface {
    Resolve(ctx context.Context, tenantID, registerID string,
            commandType DeviceCommandType) (ResolvedRoute, error)
}
```

Resolver обязан:

1. загрузить register строго в tenant scope;
2. проверить ACTIVE interval и binding version;
3. получить ровно один fiscal route;
4. для CARD получить active payment route;
5. проверить совместимость fiscal/payment routes;
6. проверить capability требуемой команды;
7. вернуть immutable snapshot;
8. при ambiguity/missing/stale binding вернуть BLOCK, не fallback simulator.

## 4. Dispatcher

```go
type DeviceCommandDispatcher interface {
    Prepare(route ResolvedRoute, command DeviceCommandEnvelopeV2) (Outbox, error)
    Publish(outbox Outbox) error
    Probe(route ResolvedRoute) Connectivity
}
```

Для `BLUECASH_ANDROID` и `EDGE_AGENT_S3` dispatcher может использовать один MQTT
transport, но разные route/device IDs, topics и capability sets. Vendor protocol
выбирается на устройстве из подписанной configuration, не backend switch-case.

## 5. Алгоритм необратимой операции

```text
REST request
→ tenant/operator/client_operation_id UUID/digest validation
→ resolve route
→ final FU/payment readiness
→ atomic insert-or-get operation by tenant + client_operation_id
→ canonical DeviceCommandEnvelopeV2
→ atomic commit(operation EXECUTING + route snapshot + command outbox)
→ publish QoS1
→ return 202 EXECUTING
→ signed device sync
→ verify/materialize atomically
→ WebHook
```

Publication failure не меняет operation на FAILED и не создаёт новую команду.
Outbox republish использует те же bytes и operation ID до expiry. После expiry —
`UNKNOWN/RECONCILIATION_REQUIRED`, если факт side effect нельзя исключить.

## 6. Direct BLE interaction

Backend не меняет physical route уже отправленной cloud command. MiniPOS может
автоматически сменить transport на BLE по документу 09, но только для того же
resolved route и binding version. При uncertain cloud send BLE сначала делает
`OPERATION_LOOKUP`; MiniPOS передаёт тот же operation/intent identity. Device
dedupe и receipt journal являются общими для MQTT/BLE.

Стабильный `client_operation_id` создаётся и durable сохраняется MiniPOS до
transport send. Если REST недоступен
до server operation reservation, offline Edge authority резервирует operation и
при необходимости regulatory identifier из fenced range, затем materializes его
через sync. Нельзя параллельно исполнять cloud и local authority для одной sale
intent; receipt может продолжаться между transports только последовательными
steps с общей fencing state.

Backend не выдаёт новый operation ID при переходе на BLE. Если внутренняя модель
имеет отдельный server operation PK, таблица содержит immutable unique
`(tenant_id, client_operation_id)`, а outbox сохраняет exact client UUID в bytes.

## 7. Маршрут card payment

Для `CARD/AUTO_IF_AVAILABLE`:

1. Resolver требует active payment terminal.
2. Если payment terminal встроен в BlueCash, fiscal и payment route имеют один
   adapter/device ID.
3. Если ESP32 управляет отдельным BluePad, routes могут иметь разные device
   identities, но общий edge route и совместимую binding generation.
4. Command содержит surrogate receipt/session ID, terminal policy и immutable
   ordered tender plan.
5. Edge последовательно выполняет acquiring всех CARD частей и durable сохраняет
   результаты каждой до открытия фискального чека.
6. После одобрения card частей Edge выполняет одну последовательность команд ФУ
   и один close receipt.
7. Ошибка запускает compensation coordinator: cancel/storno ФУ согласно commit
   point и reverse approved card операций в обратном порядке.
8. Decline оставляет sale open только после доказанной компенсации. Unknown
   блокирует повтор и register до reconciliation.

## 8. Команды, которые должны идти через dispatcher

```text
SALE_FINALIZE
SALE_REVERSE
REPORT_X
REPORT_Z
CASH_IN
CASH_OUT
KLEN
FISCAL_MEMORY
OPERATOR_REPORT
DEPARTMENT_REPORT
PLU_REPORT
DEVICE_STATUS
DEVICE_TIME_GET/SET
OPERATION_LOOKUP
```

Для MVP1 обязательны первые шесть, status и lookup. Остальные включаются по
declared capability, но не могут ложно завершаться через simulator в production.

## 9. Routing tests

- два tenant с одинаковым register ID не пересекаются;
- BlueCash register получает BlueCash topic;
- ESP32 register получает ESP32 topic;
- stale binding version блокируется;
- inactive/duplicate fiscal route блокируется;
- CARD без terminal route блокируется;
- CASH не требует payment route;
- report/cash movement используют durable queue;
- route не меняется после reservation;
- rebind отзывает старые BLE tickets и future MQTT commands;
- simulator невозможен при non-DEV route;
- sync от другого device/topic/tenant отклоняется.
