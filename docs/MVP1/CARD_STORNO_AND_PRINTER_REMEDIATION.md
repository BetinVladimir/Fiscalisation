# Оплата картой, сторно и тест принтера — требования MVP1

Статус: `P0 OPEN`  
Дата аудита: 2026-08-14

## 1. Проверенный контур

Проверены executable paths:

- `IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp`;
- `IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp`;
- `IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp`;
- `SmartDevices/bluecash-app/.../BlueCashEngine.kt`;
- MQTT command processors обоих адаптеров.

Продажа `CARD + fiscal receipt` реализована частично. Оба адаптера выполняют
card authorization до закрытия фискального чека и имеют компенсацию при
доказанной фискальной ошибке. Однако следующие разрывы не позволяют считать
контур завершённым.

## 2. P0-CARD-001 — edge-agent: не агрегировать несколько CARD payments

Сейчас edge-agent суммирует все `CARD` tenders и выполняет один `purchase`,
после чего копирует один terminal reference/RRN/auth во все payment rows.
Необходимо:

1. исполнять ordered `payments[]` последовательно, один purchase на каждый
   `payment_id`;
2. резервировать каждый payment до I/O и сохранять его собственные terminal
   reference, RRN, auth code и certainty;
3. передавать в фискальный чек исходный порядок CASH/CARD tenders;
4. при ошибке компенсировать только approved card payments, в обратном порядке;
5. повтор UUID с тем же digest возвращает сохранённый результат, с другим —
   `IDEMPOTENCY_PAYLOAD_CONFLICT`;
6. recovery после restart продолжает lookup/reverse по сохранённой ссылке и не
   повторяет purchase.

## 3. P0-CARD-002 — edge-agent: card refund при `SALE_REVERSE`

Сейчас `SALE_REVERSE` попадает непосредственно в fiscal driver и печатает
сторно, но не выполняет refund/reversal через BluePad. Требуется durable saga:

1. загрузить immutable original receipt и его payments;
2. проверить, что каждый refund относится к исходному CARD payment и не
   превышает доступный остаток;
3. выполнить terminal reversal/refund по сохранённому terminal reference,
   отдельно для каждого payment;
4. затем выполнить фискальное сторно Datecs; при отказе ФУ компенсировать по
   поддерживаемой эквайером операции либо перевести операцию в
   `RECOVERY_REQUIRED`, не заявляя ложный success;
5. сохранять RRN/auth/reference результата возврата и связь
   `original_payment_id -> reversal_payment_id`;
6. повтор команды не создаёт второй возврат или второй сторно-чек.

Порядок card/fiscal side effects и допустимая компенсация должны быть сверены с
правилами эквайера в HIL. До этого software stub обязан покрывать success,
decline, timeout-before-send, timeout-after-send и unknown.

## 4. P0-CARD-003 — BlueCash: исправить идентификатор возврата

`BlueCashEngine.reverse()` сейчас вызывает `card.reverse(originalOperationId)`
для каждого CARD payment, тогда как purchase выполняется с `payment.id`. Для
нескольких оплат получается повтор одного неверного lookup. Требуется:

- восстанавливать исходные payment rows из journal;
- использовать собственный `payment_id` и сохранённый terminal reference каждой
  оплаты;
- исполнять возвраты в обратном порядке;
- хранить отдельные durable states `REFUND_PREPARED/APPROVED/DECLINED/UNKNOWN`;
- не печатать успешное сторно при недоказанном результате терминала;
- обеспечить restart recovery и cross-channel deduplication BLE/MQTT.

## 5. P0-PRINT-001 — каноническая команда тестовой печати

В runtime обоих адаптеров такой команды нет. Добавить `PRINTER_TEST` как
нефискальную сервисную операцию:

- REST: `POST /devices/{device_id}/tests/printer`, обязательны
  `Idempotency-Key` и `X-Api-Version`, ответ `202 Operation`;
- MQTT command: `command=PRINTER_TEST`, UUID операции, tenant/register/device,
  binding generation и timestamp; QoS 1;
- BLE: тот же canonical intent и UUID, без отдельной бизнес-семантики;
- драйвер печатает нефискальный документ: `BEELOY PRINTER TEST`, vendor/model,
  serial/FMIN, UTC time, adapter ID и config generation; затем корректно
  закрывает документ;
- команда не открывает fiscal receipt, не создаёт УНП, sale или payment и не
  изменяет денежные счётчики;
- результат содержит `SUCCESS|FAILED|UNKNOWN`, printer/paper status, error code
  и timestamps; он журналируется до ответа и доступен REST/UI;
- разрешение только `ADMIN`/`SERVICE_TECHNICIAN`, полный audit trail.

## 6. Acceptance

- cash, single card, multiple card и ordered split проходят через один чек;
- storno single/multiple card возвращает ровно исходные суммы и печатает один
  соответствующий сторно-чек;
- fault injection на каждой границе card/fiscal I/O и restart не создаёт дубль;
- одинаковая команда через REST→MQTT и direct BLE даёт один physical execution;
- Datecs BlueCash-50 и DP-150 + BluePad stubs проходят одинаковую contract suite;
- Daisy выполняет printer test, но fail-closed отклоняет CARD без configured
  payment endpoint;
- OpenAPI, MQTT schema, machine traces, backend, Android/ESP-IDF и BeeFiscalApp
  tests обновлены синхронно.

