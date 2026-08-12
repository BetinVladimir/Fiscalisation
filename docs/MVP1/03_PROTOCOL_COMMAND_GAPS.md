# Недостающие команды fiscal и card protocol

## 1. Текущее физическое покрытие BlueCash

Фактически wired в `BlueCashCommandProcessor` только:

- fiscal sale: Datecs `48`, `49`, `53`, `56`;
- fiscal reversal: Datecs `43`, `49`, `53`, `56`;
- card purchase: pinpad command family `61`, subcommand `1`;
- card reversal: pinpad command family `61`, subcommand `7`;
- предварительная terminal настройка `64/2` выполняется best-effort.

## 2. P0 для закрытия MVP1

| Capability | Что отсутствует | Требуемая реализация |
|---|---|---|
| Readiness | реальный fiscal status probe в processor | Datecs status command/parser, open receipt/error/FM state, serial/FMIN verification |
| Fiscal cancel before close | нет physical cancel path | vendor command cancel receipt; использовать только до неоднозначного close |
| X report | backend API есть, Android command отсутствует | Datecs report command с X payload и реальным reference/artifact |
| Z report | backend/MiniPOS shift close есть, Android command отсутствует | Datecs daily Z command; idempotent lookup после ambiguity |
| Cash in/out | backend body игнорирует amount/operator; Android отсутствует | исправить REST parsing, MQTT command и Datecs cash command |
| Split tender | несколько отдельных physical receipts | единый `SALE_FINALIZE` и durable session с ordered `payments[]`, одна последовательность `48/49*/53*/56` |
| Durable card evidence | RRN/auth только memory map | SQLite payment transaction до fiscal close и до sync ACK |
| Card recovery | нет lookup после reboot/timeout | terminal status/transaction lookup по STAN/RRN; без повторной purchase |
| Card reversal after restart | original map теряется | читать durable amount/RRN/auth/STAN и выполнять void/reversal |
| Atomic outcome | нет общей session/compensation state machine | journal каждого шага; `COMMITTED` либо доказанный `COMPENSATED`, иначе `RECOVERY_REQUIRED` |
| Fiscal rollback | нет cancel/storno coordinator | cancel незакрытого receipt; после close — связанное storno; затем card reversal |
| Outcome model | мало terminal fields | approved/declined/unknown, response code, RRN, auth, STAN, terminal ID, timestamps |
| Physical reachability | manager ON не подтверждает transaction readiness | fiscal/pinpad ping/status и раздельные hop metrics |

Номер конкретной Datecs команды для status/report/cash должен браться из
поставленной vendor-документации и закрепляться golden frame test; кодогенератор
не должен угадывать номер команды.

## 3. P1 для полного фискального API после sales MVP

- KLEN report/read/export;
- fiscal-memory report by period/range;
- operator report;
- department report;
- PLU report;
- чтение даты/времени ФУ и установка времени;
- чтение serial/FMIN/firmware/tax groups/payment mappings;
- current receipt/document lookup для reconciliation;
- last document number/status;
- invoice receipt, если будет включён продуктовый scope;
- drawer/service functions только если разрешены policy.

Backend уже принимает типы `KLEN`, `FISCAL_MEMORY`, `OPERATOR`, `DEPARTMENT`,
`PLU`, но MQTT envelope и BlueCash processor их не выполняют. До реализации API
не должен возвращать фиктивный successful artifact.

## 4. P1/P2 card terminal capabilities

Для базового MVP обязательны purchase, durable lookup/recovery и reversal.
Позже можно добавить:

- explicit cancel active transaction;
- end-of-day/acquirer close;
- terminal report by STAN;
- last transaction lookup;
- reprint merchant/customer slip;
- network/host diagnostics;
- key/config/version status;
- cashback, tip, preauthorization/completion только отдельным scope.

## 5. Ошибки текущих контрактов

### Cash movement теряет данные

Canonical `CashMovement` требует `type`, `amount`, `operator_id`, но handler
декодирует только `type`. Amount/operator не доходят до driver. Исправление:

```text
REST CashMovement → DeviceCommand CASH_IN|CASH_OUT
payload {amount EUR, operator identity}
→ Datecs cash command
→ signed physical result
```

### Reports обходят durable MQTT queue

`CreateReport/FiscalOperation` вызывают синхронный `Driver.Execute`. MQTT bridge
возвращает `MQTT_ASYNC_DRIVER_REQUIRES_QUEUE`. Все physical commands должны
использовать один durable dispatcher.

### Split payment несовместим с BlueCash processor

Backend вызывает payment endpoint по каждой части, но каждая команда содержит
полный sale и только текущий payment. Исправление должно быть контрактным, а не
эвристикой Android: backend резервирует один finalize operation после получения
всего ordered tender plan. BlueCash создаёт одну durable receipt session и
исполняет множество последовательных vendor commands. Каждый шаг имеет durable
request/result/certainty; при ошибке coordinator компенсирует уже выполненные
card/fiscal side effects в обратном порядке.

### Card reversal не durable

`BoricaPinpadCodec.originals` — process memory. После restart отсутствуют RRN и
authorization code. Эти данные нельзя восстанавливать из POS payload; они
должны поступать из защищённого локального journal физической операции.

## 6. Tests per new command

Для каждой команды обязательны:

- builder/parser golden request/response;
- vendor error/status matrix;
- timeout before send, during send, after possible effect;
- duplicate `operation_id`;
- reboot recovery;
- MQTT and BLE semantic equivalence;
- signed sync and backend materialization;
- physical HIL evidence.
