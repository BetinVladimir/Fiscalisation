# ARCH_20 — Спецификация эмуляторов (тестовые двойники)

Назначение: исполнять сценарии ARCH_19 и тесты ARCH_18 без железа. Код эмуляторов — тестовый, лежит в `tests/emu/` соответствующего модуля (не в продуктовом коде). Все эмуляторы детерминированы (без случайности, время через инъецируемые часы).

## Общий интерфейс инъекции сбоев (`FaultPlan`)
JSON: `{"FAIL_AT":[{"cmd":53,"status":"ERR","code":"0x0004"}],"DROP_AFTER":[56],"DELAY_MS":{"56":3000},"CORRUPT_BCC":[49],"CLOCK_SKEW_S":360,"CURRENCY_MODE":0|2|3}`.
- `FAIL_AT` — вместо исполнения вернуть кадр-ошибку; состояние ФУ не меняется.
- `DROP_AFTER` — команда ИСПОЛНЯЕТСЯ (состояние меняется), но ответ не отправляется (имитация UNKNOWN).
- `DELAY_MS` — ответ после задержки; при задержке > таймаута клиента повтор с тем же SEQ.
- Каждый эмулятор ведёт `calls[]` (cmd, seq, data_hex) и счётчики `purchaseCalls`, `fiscalCalls`, `refundCalls`, `padCalls`, `ioCalls` — на них строятся утверждения ARCH_18/19.

## E-1 Эмулятор ФУ Datecs (`FakeDatecsDevice`)
- Вход/выход: поток байт (UART-имитатор), либо TCP-порт для S3-прошивки на хосте.
- Парсинг кадра: `01 LEN SEQ CMD DATA 05 BCC 03`; проверка BCC и `LEN = len(DATA)+0x2A`; при ошибке ответ NAK 0x15.
- Состояние: `open_doc{unp,items[],paid}`, `doc_counter`, `z_counter`, `clock`, `currency_mode`, `seq_cache{seq→answer}`.
- Команды MVP: 48 (открыть), 49 (продажа), 53 (оплата, слоты по `payment_slots_bc50.json`), 56 (закрыть), 60 (отмена), 62 (часы), 69 (X/Z), 74/76 (статус/IsOpen), 103 (последний документ), параметр УНП.
- Дубль SEQ → кэшированный ответ, состояние не меняется (TST-VENDOR-06).
- Ответ = `01 LEN SEQ CMD DATA 04 STAT(8) 05 BCC 03`; STAT формируется из состояния (нет бумаги, ФП полна — флажки в FaultPlan).
- Golden-режим: если запрос совпадает с кадром из `datecs_golden_frames.json`, ответ берётся из парного ответа файла.
- Приёмка: прогон `datecs_golden_frames.json` 116/116 запросов → ответы совпадают с парными (где пара есть); тесты TST-VENDOR-01/06/07 зелёные.

## E-2 Эмулятор Daisy (`FakeDaisyDevice`, Compact S 01)
- USB CDC имитируется потоком байт; кадр `01 LEN(+0x20) SEQ CMD DATA 05 BCC 03`, ответ `…04 STATUS(6) 05 BCC 03`.
- Команды: 48 (с УНП формата `DY000694-OP01-0000018`), 74, 76, 119 и сторно/Z по PM Daisy.
- SYN 0x16 каждые 60 мс при `DELAY_MS` (ответ ещё готовится); клиент обязан продлевать ожидание.
- Налоговые буквы только кириллица; латинская → NAK (TAX_GROUP_LATIN).
- Оплата: только наличные; любой другой тип → ошибка эмулятора `UNSUPPORTED` (подтверждает D-14).
- Приёмка: 15 BCC-верных кадров из `daisy_golden_frames.json` проходят, 1 негативный кадр отвергается.

## E-3 Эмуляторы Smart S: `FakeSmartFiscalServer` и `FakeBridge`
- HTTP-сервер на localhost (в тесте — in-process), принимает JSON Smart Pro v2.2025; ответы из `fixtures/smart_protocol_examples.json`. Команды: `PrintFiscalReceipt`, `PrintDailyReport`, `GetStatus`, `GetPayments`, `OperatorLogin`.
- Режимы: `OK` (`{"ok":true,"seq":n}` — как у реального ФУ, без номера документа), `OK_WITH_DOC` (гипотетический ответ после Q-S2), `ERROR(code)`, `DROP_AFTER_BODY` (запрос принят, ответа нет), `DUP_UNP_REJECT` (гипотеза Q-S3).
- Валидирует вход строго: ключ `items` без пробела, `seq` (не `sec`), суммы как числа с ≤2 знаками, `vatGroup` кириллица, лимиты 20/24/14 → код 2101.
- `FakeBridge`: `payment_request` → записывает операцию; `check_status` → `SUCCESS|REJECTED|NOT_FOUND|FAILED` по сценарию; повтор `reference_id` не создаёт второго списания (`realCharges==1`); `currency != EUR` → `FAILED`.
- Приёмка: ARCH_19 P-S S1…S8 зелёные; все запросы эмулятору сериализуются без `double`-хвостов (`13.67`, не `13.670000001`).

## E-4 Эмулятор BluePad (`FakeBluePad`) + `FakeBankHost`
- Говорит реальными кадрами `3E CMD 00 LH LL DATA XOR` (по `datecspay_golden_frames.json`), ответ `3E 00 ST LH LL …`; XOR/длина проверяются.
- Команды: `PING`, `TRANSACTION START` (типы 1, 7, 10, 14), `GET RECEIPT TAGS`, `TRANSACTION END`, `GET PINPAD STATUS` (R/C/0), `GET REPORT TAGS BY STAN`, `GET RTC`, `40/01 RECEIVE DATA`, `40/02 EVENT CONFIRM`, `40/03 GET MAX MTU`.
- События: `0F/01 SOCKET OPEN`, `0F/03 SEND DATA`, `0F/02 SOCKET CLOSE`, `0E/01 TRANSACTION COMPLETE` (`DF05`,`DF06`,`DF07`,`DF08`,`81`,`9F41`), `0E/02`, `0E/03`.
- Режимы: `APPROVE`, `DECLINE(DF09)`, `ERROR(DF06)`, `DROP_EVENT_0E01` (операция проведена, события нет), `BLE_DISCONNECT`, `SOCKET_FAIL`.
- `FakeBankHost` — TCP-сервер: принимает SEND DATA, отвечает блобом N байт (в т.ч. 3000 → три RECEIVE DATA при MTU 1024).
- Идемпотентность: повторный `TRANSACTION START` с теми же параметрами при незавершённой транзакции → `errBusy (38)`/`errNoPermit (7)`; `realCharges==1`.
- Приёмка: ARCH_19 P-C S2…S5, S7 зелёные.

## E-5 MQTT-брокер (`FakeBroker`)
- In-process брокер: topic-ACL, QoS0/1, retained, LWT, keepalive, очередь сообщений при отключённом клиенте (persist до 100).
- FaultPlan: `BROKER_DOWN_S`, `DUP_DELIVERY`, `MAX_PAYLOAD`.
- Утверждения: доставка ≤ 2000 мс виртуального времени; дубль `msg_id` исполняется один раз (дедупликация на S3-стороне проверяется отдельно).
- Допустима замена на `mosquitto` в Docker для интеграционных тестов, с тем же сценарным набором.

## E-6 BLE-периферия (`FakePeripheral`)
- GATT-сервис с write/notify характеристиками; параметр `MTU` (23 или 247) и `DROP_FRAGMENT_N`.
- Фрагментация: payload режется по `MTU-3`; сборка на приёмнике по длине-префиксу; потерянный фрагмент → таймаут и `UNKNOWN`, а не частичная команда.
- Приёмка: S8 транспорта (MTU 23/247) и потеря фрагмента зелёные.

## Размещение и запуск
| Эмулятор | Каталог (тестовый код) | Команда |
|---|---|---|
| E-1/E-2 | `IoT/firmware/edge-agent-s3/test/host/` + `Fiscalisation/**/test/emu/` | описывается в карточке задачи EMU-xx |
| E-3/E-4 | тесты Android-модуля (`src/test`) | gradle test |
| E-5/E-6 | `tests/emu/` | `pytest tests/emu` или gradle |
Задачи создания эмуляторов: EMU-01…EMU-06 (по одной на E-1…E-6), зависимость — ARCH_18 части 3 и fixtures.
