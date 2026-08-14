# BlueCash direct BLE — требования закрытия MVP1

Статус: `P0 OPEN`  
Дата фиксации: 2026-08-14

## 1. Причина

Сопоставление единого POS-протокола с кодом выявило два несовместимых runtime
разрыва:

1. MiniPOS принимает только `BleSession.security_mode=OPEN_MVP` и передаёт
   нешифрованные digest-bound frames `BFF1`. `BlueCashTransactionGattServer`
   всегда создаёт `BlueCashBleServerHandshake` и требует legacy
   ticket/X25519/HKDF/AES-GCM channel. Клиент и устройство не могут начать одну
   transaction session.
2. MiniPOS отправляет один aggregate `action=SALE_FINALIZE` с immutable
   `items[]`, ordered `payments[]`, `client_operation_id` и
   `receipt_session_id`. `BlueCashComplianceIntentExecutor` принимает legacy
   `PAYMENT`, хранит только одну оплату и при преобразовании строки теряет
   optional `discount`.

Канонический контракт находится в
[`../EXTERNAL_POS_INTEGRATION_PROTOCOL.md`](../EXTERNAL_POS_INTEGRATION_PROTOCOL.md)
и `contracts/openapi-runtime-v1.yaml`. Исправление кода не должно возвращать POS
к legacy wire format.

## 2. P0-BC-BLE-001 — совместимый `OPEN_MVP` GATT

### Изменяемые пути

- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashTransactionGattServer.kt`;
- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashBleCommandChannel.kt`;
- при необходимости новые `BlueCashOpenMvpFrameAssembler.kt` и
  `BlueCashOpenMvpCommandChannel.kt`;
- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/MainActivity.kt`;
- тесты в `SmartDevices/bluecash-app/src/test/...` и Android instrumentation.

### Требуемое поведение

1. Для binding/session с `security_mode=OPEN_MVP` не запускать
   `HELLO/CHALLENGE/AUTH_PROOF`, X25519, HKDF или AES-GCM.
2. Использовать UUID сервиса/command/event из backend route package; они должны
   совпадать с MiniPOS `BleSessionPackage`.
3. Command и event characteristic передают `BFF1` frames, совместимые с
   `minipos/BeeMiniPOS/src/bleFrames.ts` и ESP-IDF `ble_runtime.cpp`:

```text
magic "BFF1"              4 bytes
message UUID             16 bytes
total length              2 bytes, big-endian
offset                    2 bytes, big-endian
final flag                1 byte
SHA-256 whole payload    32 bytes
body                      remaining bytes
```

4. Максимальный payload — 8192 bytes. Проверять bounds, последовательный offset,
   неизменные UUID/total/digest, final length и SHA-256 до CBOR decode.
5. На ошибке очищать только assembly текущего message/connection; physical I/O
   не выполнять.
6. Результат кодировать canonical CBOR, фреймировать тем же `BFF1`, публиковать
   notify с исходным message UUID.
7. Разные connections не используют общий mutable assembler. Disconnect очищает
   connection state.
8. `X25519_AES_GCM` оставить отдельным production profile, но он не должен быть
   выбран backend/MiniPOS для controlled MVP.

### Acceptance

- TypeScript→Kotlin golden frames в обе стороны совпадают byte-for-byte;
- 1/2/N chunks, MTU 64/185/517;
- invalid magic/UUID/length/offset/final/digest и interleaved connection fail
  before executor;
- MiniPOS native и Web BLE clients получают `ComplianceIntentResult`;
- Android process restart/disconnect не оставляет READY/partial frame state;
- legacy encrypted frame не принимается в `OPEN_MVP` characteristic.

## 3. P0-BC-BLE-002 — aggregate `SALE_FINALIZE` и скидка

### Изменяемые пути

- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashComplianceIntent.kt`;
- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt`;
- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt`;
- `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsPayloads.kt` либо фактический payload builder;
- соответствующие JVM/instrumentation/cross-channel tests.

### Валидация intent

До journal/I/O проверить:

- `action == SALE_FINALIZE`;
- UUIDv4 `intent_id == client_operation_id`;
- UUIDv4 `receipt_session_id` и каждого `payment_id`;
- tenant/register/edge/binding generation равны active persisted binding;
- `server_sale_id`, `client_sale_surrogate_id`, УНП и operator присутствуют;
- `items[]` непустой, порядок сохраняется;
- `payments[]` содержит 1..10 элементов CASH/CARD, порядок сохраняется;
- EUR и canonical decimal scale;
- canonical payload SHA-256;
- authoritative `sum(net line totals) == sum(payments)`.

`discount` необязателен и является абсолютной EUR-суммой строки:

```text
gross = round_half_up(quantity × unit_price)
net   = gross - discount
0 <= discount <= gross
```

Отсутствие означает zero. Скидку нельзя потерять при mapping в `FiscalLine` или
Datecs command 49; driver получает discount semantics ровно один раз.

### Durable saga

1. Атомарно зарезервировать operation, receipt session, immutable items,
   ordered payments и payload digest **до** device I/O.
2. Same UUID + same digest возвращает сохранённый result; другой digest —
   `IDEMPOTENCY_PAYLOAD_CONFLICT`.
3. CARD payments выполнять последовательно, сохраняя PREPARED/APPROVED и
   terminal reference/RRN/auth до следующего side effect.
4. После всех approved CARD открыть один fiscal receipt, передать все items со
   скидками, затем все ordered tenders и закрыть один раз.
5. Decline до fiscal open возвращает доказанный failure без чека.
6. Ошибка после approved card запускает reverse в обратном порядке. Только
   доказанный полный откат даёт `COMPENSATED`; неопределённость —
   `FISCAL_RESULT_UNKNOWN/RECOVERY_REQUIRED`.
7. Restart восстанавливает canonical receipt/payment plan и выполняет
   lookup/reconciliation, а не повтор purchase/receipt.
8. Terminal result записывается до BLE notify; затем тот же journal
   синхронизируется MQTT и материализует один backend operation/WebHook.

Legacy `PAYMENT` можно оставить для внутренних migration tests, но новый
MiniPOS/public protocol его не вызывает и acceptance на нём не строится.

### Acceptance

- cash, card и ordered split проходят через реальный production executor mock;
- optional discount отсутствует/zero/nonzero и даёт одинаковый total через REST,
  MQTT и BLE;
- negative/over-gross/currency/rounding/overflow отклоняются до journal/device;
- duplicate BLE, MQTT-after-BLE и lost result notification дают один fiscal/card
  side effect;
- decline, timeout-before-send, timeout-after-send, fiscal open/line/tender/close
  faults и reverse fault имеют deterministic certainty;
- reboot на каждой journal boundary восстанавливает тот же plan/reference;
- sync ACK loss не повторяет операцию и не удаляет unacknowledged evidence;
- минимум 3 месяца retention применяется только к backend-acknowledged rows.

## 4. Обязательный интеграционный сценарий

```text
MiniPOS создаёт sale через REST
→ добавляет discounted line
→ cloud ping теряется
→ автоматически подключается OPEN_MVP BLE к BlueCash
→ отправляет тот же durable SALE_FINALIZE
→ BlueCash выполняет CARD + один fiscal receipt
→ BLE result теряется либо доставляется
→ BlueCash MQTT sync materializes operation
→ Fiscal WebHook доставляется MiniPOS backend
→ polling/WebHook показывают один authoritative completed sale
```

Повторить для CASH, CARD, split, decline, unknown/recovery и compensation.
Отдельно проверить REST timeout после backend outbox: BLE lookup/dedupe не
допускает второй physical execution.

## 5. Definition of done

Оба P0 закрыты только если:

1. `make contract-test`, BlueCash JVM tests, MiniPOS TypeScript tests и Android
   instrumentation PASS;
2. добавлен cross-language `BFF1 + canonical CBOR` golden-vector suite;
3. добавлен REST→BLE→MQTT→WebHook stub E2E с discount и split tender;
4. в executable BlueCash MVP path нет обязательного handshake и legacy-only
   `PAYMENT` dispatch;
5. machine traces и единый POS-протокол ссылаются на новые test/evidence paths;
6. повторный аудит не находит иных software gaps.

После software PASS остаётся физический BlueCash-50 fiscal/card/BLE HIL. HIL не
может использоваться для формального закрытия описанных software incompatibilities.
