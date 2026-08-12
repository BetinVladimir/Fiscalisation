# BlueCash command traceability

## 1. Cloud route: MiniPOS → REST → MQTT → BlueCash

### Продажа и cash/card payment

| Этап | Фактический вызов/объект | Реализация | Статус |
|---|---|---|---|
| MiniPOS order | `POST /public/v1/minipos/orders/{id}/checkout[-batch]` | MiniPOS handler/service | Есть |
| MiniPOS → Fiscal | `POST /sales`, затем `/sales/{id}/lines`, затем `/sales/{id}/payments` | `minipos/.../domain/service.go` | Есть; legacy multi-call |
| Fiscal reserve | `ReserveSalePaymentCommand` | fiscal domain/repository | Есть |
| MQTT envelope | `command_type=FISCAL_SALE`, topic `tenants/{tenant}/devices/{device}/commands` | `mqttclient.Bridge.Prepare` | Есть |
| BlueCash validation | tenant/device/fencing/expiry/HMAC/payload hash | `BlueCashMqttRuntime.handleCommand` | Есть |
| Card acquire | pinpad `64/2` optional, `61/3`, `61/1 subcommand 1`, event `14` | `BoricaPinpadCodec.purchase` | Есть |
| Fiscal open | Datecs command `48` | `BlueCashCommandProcessor.sale` | Есть |
| Fiscal lines | Datecs command `49` per line | same | Есть |
| Fiscal totals | Datecs command `53` per payment | same | Есть |
| Fiscal close | Datecs command `56` | same | Есть |
| Local evidence | signed `ACCEPTED/EXECUTING/FISCALIZED|FAILED|UNKNOWN` | Android journal | Есть |
| Sync | `.../sync/batches/{batch}` QoS 1 | BlueCash MQTT runtime | Есть |
| Backend ACK | `.../sync/acks/{batch}` | backend processor | Есть |
| WebHook | `fiscal.operation.updated` | backend outbox | Есть |

### Reversal

| Этап | Команда | Статус |
|---|---|---|
| MiniPOS | `POST /minipos/orders/{id}/reversals` | Есть |
| Fiscal | `POST /sales/{sale_id}/reversals` | Есть |
| MQTT | `command_type=REVERSAL` | Есть |
| Card | pinpad `61/3`, затем `61/1 subcommand 7`, event `14` | Есть, но original data только RAM |
| Fiscal storno open | Datecs `43` | Есть; требует vendor/HIL подтверждения BC-50 |
| Lines/total/close | `49* → 53* → 56` | Есть |
| Sync/WebHook | `REVERSED` → backend materialization | Есть |

### Cloud-route разрывы

1. `command_type` разрешает только `FISCAL_SALE` и `REVERSAL`.
2. X/Z/KLEN/FM/cash movement не queue-ятся MQTT bridge.
3. Split tender создаёт несколько payment REST calls и несколько MQTT команд;
   BlueCash каждые `FISCAL_SALE` завершает `48→...→56`. Требуется один
   `SALE_FINALIZE` с полным ordered payments и одной durable receipt session.
4. Partial payment state backend несовместим с физическим закрытием чека.
5. MQTT command использует HMAC transport authority, а BLE — ticket + AEAD;
   semantic envelope не унифицирован.
6. Result содержит мало card evidence: нет durable RRN/auth/STAN/terminal ID.

## 2. Direct route: MiniPOS → BLE → BlueCash → MQTT sync

### REST authority preparation

1. MiniPOS создаёт X25519 keypair.
2. Вызывает `POST /registers/{register_id}/ble-sessions`.
3. Backend возвращает ticket с tenant/location/register/edge/FU/operator,
   client public key, scopes, fencing token и expiry.
4. MiniPOS проверяет все outer binding fields и signed ticket.

### GATT/crypto

```text
HELLO(ticket, client nonce/public key)
→ CHALLENGE(edge nonce/public key)
→ AUTH_PROOF
→ READY
→ deterministic CBOR ComplianceIntent in AES-GCM frame
→ encrypted result notification
```

BlueCash реализует X25519, HKDF-SHA-256, directional AES-GCM, counter/replay,
chunking и GATT characteristics.

### BLE business actions

| `ComplianceIntent.action` | Локальное действие | Physical I/O | Статус |
|---|---|---|---|
| `OPEN_WITH_LINE` | создаёт local sale/version | Нет | Есть |
| `ADD_LINE` | добавляет line | Нет | Есть |
| `CHANGE_LINE` | заменяет line | Нет | Есть |
| `CANCEL_LINE` | удаляет line | Нет | Есть |
| `CANCEL_SALE` | закрывает local aggregate | Нет | Есть |
| `PAYMENT` | резервирует УНП и вызывает processor.sale | card purchase + `48/49/53/56` | Есть |
| `REVERSE` | вызывает processor.reverse | card reverse + `43/49/53/56` | Есть |

### BLE → MQTT backend sync

Physical result попадает в тот же Android journal. После MQTT connect runtime
строит signed contiguous batch, backend materializes operation/sale/WebHook и
возвращает signed business ACK. BLE response POS не заменяет backend sync.

### Direct-route разрывы

1. MiniPOS checkout UI должен действительно выбирать BLE при cloud loss; наличие
   crypto modules само по себе не доказывает end-to-end switch.
2. BLE local aggregate поддерживает только один `SalePayment`, поэтому настоящий
   split tender и общий compensation coordinator не поддерживаются.
3. Нет BLE actions для X/Z/cash movements/reports.
4. Card original evidence не переживает restart.
5. До получения MQTT ACK local BLE sale должен отображаться как pending sync;
   необходимо явное UI/backend reconciliation состояние.
6. Offline УНП range должен быть provisioned, fenced и неистощён; exhaustion
   обязан блокировать `OPEN_WITH_LINE/PAYMENT` по принятой state-machine policy.

## 3. Required command/result correlation

Для обоих маршрутов сохраняются неизменными:

```text
client_intent_id / idempotency key
server sale_id
operation_id
payment_id(s)
UNP
route_id + binding_version
device_id + fiscal memory number
card transaction reference (если CARD)
fiscal document reference
journal sequence + event hash
backend ACK ID
WebHook event ID
```

Ни MQTT reconnect, ни BLE fallback не создают новый `operation_id` для уже
зарезервированной операции.

## 4. Требуемая последовательность одной session

```text
SESSION RESERVED
→ CARD authorization 1..N
→ Datecs OPEN 48
→ Datecs LINE 49 × N
→ Datecs PAYMENT 53 × N в порядке tender plan
→ Datecs CLOSE 56
→ SESSION COMMITTED
→ signed journal/sync/WebHook
```

При отказе выполняется обратная компенсация:

```text
cancel/open-receipt recovery или storno, если чек уже закрыт
→ reverse approved card operations в обратном порядке
→ COMPENSATED
```

Если результат команды неизвестен, выполняется lookup. До доказанного результата
session остаётся `RECOVERY_REQUIRED`, а повтор purchase/close запрещён.
