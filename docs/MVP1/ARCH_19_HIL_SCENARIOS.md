# ARCH_19 — Сценарии S1…S8 по путям (HIL и эмулятор)

Каждый сценарий исполняется дважды: **EMU** (эмулятор из ARCH_20, обязателен в CI) и **HIL** (реальное ФУ, строка в `docs/vendor/hil_results.json`, проверяет `scripts/check-hil-matrix.py` SCR-40). Статус `SKIP` запрещён. Формат ключа результата: `<path>.<client>.<transport>.S<n>`.
Общие числа (одинаковы везде): товар «Тест» 1,000 × 12,34 € ; скидка 0,34 → итого **1200¢**; строка 2: 0,500 × 3,33 = 1,665 → **167¢** (half-up), итого с обеими строками **1367¢**; НДС группа Б 20%.
Общий evidence на сценарий: (e1) строка журнала СУПТО с `unp`, `fiscal_no`, `status`; (e2) сырой обмен hex (`raw_io.log`); (e3) фото/скан чека для HIL; (e4) JSON `result` в `hil_results.json`.
Общая инъекция сбоев: `FAIL_AT=<cmd>`, `DROP_AFTER=<cmd>`, `DELAY_MS=<n>` (ARCH_20); на HIL — физически (выдернуть кабель, выключить ФУ, закрыть бумагу).

## Сценарии (смысл одинаков для всех путей; различия — таблицами ниже)
| S | Шаги | Ожидание (числа) | Сбой | Evidence |
|---|---|---|---|---|
| S1 наличные | продажа 2 строки, оплата CASH 1367¢ | `COMMITTED`, `fiscal_no` растёт на 1, сумма чека 1367¢, сдача 0 | нет | e1–e3 |
| S2 карта | продажа 1200¢, оплата CARD | карта `approved` ровно 1 раз, затем фискализация `COMMITTED`; `purchaseCalls==1` | отказ карты → чек не печатается, `FAILED_CLEAN`, `fiscalCalls==0` | e1,e2 |
| S3 смешанная | 1367¢: CASH 500¢ + CARD 867¢ | порядок: карта первой; слоты 0 и 1 в кадре cmd 53; сумма слотов = 1367 | карта ок, печать падает → `FISCAL_PENDING`, возврата карты НЕТ | e1,e2 |
| S4 сторно | сторно чека S1 (D-12 storno-first) | сторно-документ `COMMITTED` с ссылкой на оригинал; возврат карты только после него | сбой сторно → возврат карты не вызывается (`refundCalls==0`) | e1,e2 |
| S5 UNKNOWN | обрыв после отправки cmd 56 | статус `UNKNOWN` (не FAILED/NOT_SENT); `resolve()` находит документ по УНП → `COMMITTED` или `NOT_COMMITTED`→отмена | `DROP_AFTER=56` | e1,e2 |
| S6 Z-отчёт | X затем Z | Z только при закрытом чеке; повтор Z → `ALREADY_DONE`, номер Z +1 один раз | отказ при открытом чеке `OPEN_DOCUMENT` | e1,e3 |
| S7 повтор ключа | тот же `idempotency_key` дважды | второй вызов = тот же результат, ФУ не вызывается (`fiscalCalls==1`) | перезапуск приложения между вызовами | e1,e2 |
| S8 часы/валюта | часы ФУ +6 мин; валюта ФУ ≠ операции | +6 мин → предупреждение, продажа разрешена; +3 ч или валюта ≠ → `DEVICE_GATE`, `purchaseCalls==0` | — | e1,e4 |

## Путь P-A: BlueCash-50 (Android, прямое, Datecs v2.11.4) — ключ `bc50.pos|minipos.direct`
| S | Специфика | Ожидаемый кадр/значение | Сбой |
|---|---|---|---|
| S1 | cmd 48(`1\t1\t24\tI\t`) → 49 → 53(слот0=1367) → 56 | ответ 49 `0\t<n>\t`, 56 → `0\t<doc>\t...` | — |
| S2 | слот1 (CARD) через BC-50 внутренний терминал | при `CARD_DECLINED` печати нет | `FAIL_AT=53` |
| S3 | слоты 0+1 в одном 53 | суммы 500+867 | — |
| S4 | cmd 48 с типом сторно | оригинальный номер в данных | `FAIL_AT=49` |
| S5 | USB/BT обрыв после 56 | resolve 74/76 | `DROP_AFTER=56` |
| S6 | cmd 69(X)/69(Z) | счётчик Z +1 | — |
| S7 | как общий | `fiscalCalls==1` | рестарт |
| S8 | cmd 62 часы; параметр валюты | +6 мин WARN | — |
Статус пути: `EXCLUDED_FROM_PROD` (MVP_GATES) — сценарии выполняются на стенде, не в проде.

## Путь P-B: Datecs через ESP32-S3 + COM (профиль `DatecsDp150BluePad50`) — ключ `dtx.pos|minipos.mqtt|ble`
| S | Специфика | Ожидание | Сбой |
|---|---|---|---|
| S1 | MQTT команда `fiscal/<dev>/cmd` → S3 → UART 115200 8N1 | кадры = `datecs_golden_frames.json` (побайтно), ack на `…/evt` | — |
| S2 | карта через BluePad (см. P-C) затем печать | порядок «карта → печать» | отказ карты |
| S3 | слоты 0+1 | как P-A | — |
| S4 | storno-first | как общий | `FAIL_AT=60` |
| S5 | отключение UART после 56 | S3 отвечает `UNKNOWN`, backend не ретраит слепо; resolve 76 | `DROP_AFTER=56` |
| S6 | Z | как общий | — |
| S7 | повтор MQTT QoS1 дубль | S3 дедуплицирует по `msg_id`; SEQ не меняется для повтора, ФУ вернул кэш | дубль доставки |
| S8 | cmd 62 | +6 мин WARN | — |

## Путь P-C: BluePay через ESP32-S3 + BLE (`bluepad_ble_central.cpp`) — ключ `bluepay.pos|minipos.ble`
Статус: спецификация получена (DOC-V3). Реализация блокирована BP-04 (прокси сокета банка на S3). HIL — только с merchant credentials (DOC-V6). Кадры — `fixtures/datecspay_golden_frames.json`.
| S | Специфика | Ожидание | Сбой |
|---|---|---|---|
| S1 | наличные: BluePad не используется | `padCalls==0` | — |
| S2 | `PING` → `TRANSACTION START` покупка 867¢ (`81 04 00 00 03 63`) → сокет-обмен (OPEN/SEND/RECEIVE/CLOSE с EVENT CONFIRM) → `0E/01` `DF05=0` → `GET RECEIPT TAGS` → `TRANSACTION END 00 01` | `approved`, сохранены RRN, AuthID, STAN; ровно 1 покупка | `DF05=1` отказ → `FAILED_CLEAN`, `fiscalCalls==0` |
| S3 | CASH 500¢ + карта 867¢ | карта первой; фискальный чек после `0E/01` | карта ок, фискализация упала → `FISCAL_PENDING`, void не вызывается |
| S4 | сторно: `TRANSACTION START` тип 7: `81`, `DF01` RRN, `DF02` AuthID исходной операции | void только после фискального сторно (D-12); операция в памяти ридера → без карты | сбой сторно → `refundCalls==0` |
| S5 | BLE-разрыв после `TRANSACTION START` | `UNKNOWN`; `0x1A` (R/C/0) затем `0x24` по STAN → `COMMITTED`/`NOT_COMMITTED`; STAN неизвестен → `UNKNOWN` вручную; новая покупка при флаге R/C заблокирована | `DROP_AFTER=0E/01` |
| S6 | конец дня: тип `0x0A` | отдельно от фискального Z | — |
| S7 | тот же `idempotency_key` | вторая покупка не создаётся (`purchaseCalls==1`) | рестарт |
| S8 | валюта ридера EUR (`5F2A 09 78`), RTC (`GET RTC 0x07`) в допуске | иначе `DEVICE_GATE`, `purchaseCalls==0` | Q-B3 открыт → `spec_pending` |

## Путь P-D: Daisy Compact S 01 через ESP32-S3 + USB CDC (VID 0483/PID 5740) — ключ `dsyc.pos|minipos.mqtt|ble`
Статус: `DOCUMENTED_NOT_APPROVED`. Платёж по D-14 только наличные.
| S | Специфика | Ожидание | Сбой |
|---|---|---|---|
| S1 | кадры из `daisy_golden_frames.json`: cmd 48 с УНП `DY000694-OP01-0000018`, налог кириллица | побайтно | — |
| S2 | карта | `PAYMENT_TYPE_UNSUPPORTED` без I/O (`ioCalls==0`) | — |
| S3 | смешанная | то же отказ | — |
| S4 | сторно через cmd для сторно | как общий | `FAIL_AT` |
| S5 | USB отключён после 56-аналога | resolve 74/76; SYN 0x16 продлевает ожидание | `DROP_AFTER` |
| S6 | Z | как общий | — |
| S7 | как общий | — | рестарт |
| S8 | часы | как общий | — |

## Путь P-S: Daisy Smart S (Android, прямое; к ФУ — JSON/HTTP localhost) — ключ `dsys.pos|minipos.direct`
Статус: `LAB` до ответов DOC-V4B (нет номера документа в ответе, F-1). Сценарии идут против `FakeSmartFiscalServer` и `FakeBridge` (ARCH_20 E-3), данные — `fixtures/smart_protocol_examples.json`.
| S | Специфика | Ожидание | Сбой |
|---|---|---|---|
| S1 | один `PrintFiscalReceipt`, оплата `payTypeCode=0`, 13.67 | `httpCalls==1`; тело = fixture `sale_cash`; PROD без номера документа → `UNKNOWN`, LAB → `COMMITTED`+`NO_DOC_NUMBER` | `ok:false 1008` → `Rejected`, `fiscalCalls` не растёт повтором |
| S2 | `payment_request` (dsk, EUR 8.67) → `SUCCESS` → фискализация | `purchaseCalls==1`, затем `fiscalCalls==1` | `REJECTED`/`FAILED` → `fiscalCalls==0` |
| S3 | CASH 5.00 + карта 8.67 | карта первой; в чеке две оплаты, сумма 13.67 | карта ок, `PrintFiscalReceipt` упал → `FISCAL_PENDING`, `refundCalls==0` |
| S4 | сторно `isStorno:true reason:1` + `dateTimeLink dd-mm-yyyy hh:mm:ss` | поля из доказательства оригинала; возврат карты (ручной, Q-P4) только после сторно | нет доказательства → `NO_ORIGINAL_PROOF`, `httpCalls==0` |
| S5 | обрыв HTTP после записи тела продажи | `UNKNOWN`; повтор НЕ отправляется; для карты — `check_status`: `NOT_FOUND`→`NOT_COMMITTED`, `SUCCESS`→`COMMITTED` | `DROP_AFTER=PrintFiscalReceipt` |
| S6 | `PrintDailyReport{reset:true}` | `reportNumber` +1; повтор → `ALREADY_DONE` | чек не закрыт/«Need daily Z report» |
| S7 | тот же `idempotency_key` | `httpCalls` не растёт, результат тот же | рестарт приложения между вызовами |
| S8 | `GetStatus`: `clockSynchronized=false`/`paperEnd`/`ejFull`/`fmFull` | продажа блокирована `DEVICE_GATE`, `httpCalls(sale)==0`; валюта карты только `EUR` | `currency:"BGN"` в моста → отказ до сети |

## Транспорт MQTT / BLE — ключ `transport.<mqtt|ble>.S<n>` (без ФУ, fake-устройство)
| S | Шаги | Ожидание | Сбой |
|---|---|---|---|
| S1 | cmd→ack доставка QoS1 | ack ≤ 2000 мс; `msg_id` совпадает | — |
| S2 | ответ с payload ≤ лимита | целостность crc32 | payload лимит+1 → отказ `TOO_LARGE` |
| S3 | параллельно 2 устройства | сообщения не смешиваются (topic per `dev`) | — |
| S4 | ACL | чужой topic → отказ брокера | — |
| S5 | разрыв брокера на 30 с | команды очередью не теряются, повтор по `msg_id` не исполняется дважды | `DROP` |
| S6 | LWT | `offline` публикуется ≤ keepalive×1.5 | kill клиента |
| S7 | дубль `msg_id` | второй игнорируется, ack повторён | дубль |
| S8 | BLE MTU 23/247 | фрагментация/сборка, итог байт в байт; потеря фрагмента → таймаут и `UNKNOWN`, не частичная команда | потеря фрагмента |

**POS по BLE (D-13, 2026-10-23):** для каждого из пяти путей P-A, P-B, P-C, P-D, P-S сценарии S1…S8 повторяются с клиентом POS и транспортом BLE (ключи `<path>.pos.ble.S<n>`, +40; задача POS-BLE-03); транспортные сценарии `transport.ble` выполняются и для POS.

Итого: 6 путей × 8 = **48 сценариев**; каждый — отдельная запись в `hil_results.json` и тест эмулятора (`tests/hil_emu/test_<path>_S<n>`). Критерий приёмки документа: `python3 tools/check-doc-refs.py --scenarios` находит 48 ключей.
