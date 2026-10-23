# ARCH_17 · Пути MVP (P-A, P-S, P-B, P-C, P-D, P-E): текущее состояние, разрывы, факты по Daisy, HIL-матрица

Основание: `MVP_GOAL.md`. Метод — статический (код не запускался). Факты ниже проверены по исходникам и вендорным PDF; всё прочее помечено **ПРОВЕРИТЬ**.

## 1. Что есть в коде сейчас (проверено)

| Путь | Что реализовано | Чего нет / риск |
|---|---|---|
| P-A BlueCash-50 | Android `SmartDevices/bluecash-app`: `BlueCashEngine`, три транспорта (MQTT `BlueCashMqtt.kt`, BLE GATT, локальный HTTP) на одном процессоре; Datecs v2.11.4 | BUG-020…032, BUG-071/072, VG-01…23 (ARCH_01/09); пинпад встроен — отдельный путь BluePay не нужен |
| P-S Daisy Smart S | Только `SmartDevices/daisy-smart-app/.../DaisySmartStub.kt`: симулятор (сценарии success/card_decline/timeout…), `require(!prod)` и `BuildConfig.STUB_ADAPTER && DEBUG`; тест `DaisySmartStubTest`; модуль `:shared` содержит только интерфейс `Driver.kt` (`FiscalDriver`, `Command`, `Result`) | **Реальной интеграции нет.** Нет vendor SDK/API/лицензии (DOC-V4, GAP-033); нет MQTT/BLE/HTTP-ввода, журнала, процессора и жизненного цикла — всё это сейчас только в `bluecash-app` и не вынесено в общий модуль; возможности терминала/ФУ у Smart S **ПРОВЕРИТЬ по SDK** |
| P-B Datecs COM | Профиль `EdgeProfile::DatecsDp150BluePad50` (`provisioned_binding.h:8`), `uart_runtime`, `profile_executor.cpp` (фискализация, доказательство документа через журнал EJ cmd 125/123, Z-отчёт, CASH_IN/OUT), C++ адаптер `IoT/protocol-abstraction/src/DatecsFiscalAdapter.cpp` | DP-150 MX электрический интерфейс — `DOCUMENTED_NOT_APPROVED` (`MVP_GATES.md`); C++ кодек: GAP-010 (LEN на 4) — **ПРОВЕРИТЬ**, статус в реестре OPEN; паритет с исправлениями VG-01…VG-07 для C++ не описан |
| P-C BluePay | В том же профиле: `bluepad_ble_central.cpp` (226 строк), `DatecsPayTerminalAdapter.cpp`, `payment_reconciliation.cpp` | **Профиль объединяет DP-150 и BluePad-50**: BluePay без Datecs-ФУ профилем не задаётся. Низкоуровневый протокол `DatecspayLowLevel.pdf` v3 не в репозитории (DOC-V3); GAP-013/014 (vendor wake/flow, correlation) |
| P-D Daisy Compact S 01 (USB) | Профиль `EdgeProfile::DaisyCompactS01` (`provisioned_binding.h:8`); `usb_cdc_runtime.cpp` (VID/PID из привязки: `usb_vid`, `usb_pid`); `DaisyFiscalAdapter.cpp`; Daisy-ветка `profile_executor.cpp` (диагностика cmd 90, последний документ cmd 113, доказательство документа, подитог 51, оплата, закрытие, Z-отчёт) | Ветка «не Datecs ⇒ Daisy» безусловна; GAP-009 (runtime codec ≠ протокол) и GAP-024 в реестре OPEN — **ПРОВЕРИТЬ**; `DOCUMENTED_NOT_APPROVED`: питание VBUS и USB host, прошивка с евро; у Compact S 01 **нет** встроенного терминала и нельзя подключить внешний — карта по этому пути невозможна |
| Транспорт MQTT/BLE на S3 | `mqtt_runtime.cpp`, `ble_runtime.cpp`, провижининг BFPE | GAP-001…008 в реестре OPEN; GAP-001 по коду backend, по-видимому, уже исправлен (`client.go:237` публикует `tenants/{t}/devices/{id}/commands`, S3/BlueCash подписываются так же) — **ПРОВЕРИТЬ** на S3 |
| Клиент MiniPOS | `minipos/miniposweb/src/fiscal/browserGatt.ts`, `webBleRoute.ts`; `BeeMiniPOS/src/webBle.ts`; REST к `beeminipos-backend` | находки MP-01…07, MC-01 (ARCH_13) |
| Клиент POS | Expo `POS/pos-app`: только REST → `pos-gateway` → fiscal-backend → MQTT; **BLE в приложении нет** (поиск по `src` и `package.json`) | BLE для POS — решение D-13 |

## 2. Daisy: факты из вендорных материалов (PDF `93PC_FD_CProtocol_ver_2_0-4_2026_EN`, `daisy/info.md`)

| Параметр | Значение |
|---|---|
| Физика (Compact S 01) | USB CDC/ACM, **VID 0483 / PID 5740**, аппаратных требований к скорости/формату UART нет; ESP32-S3 должен работать USB host и **подавать VBUS 5 В** |
| Платежи | встроенного терминала нет, внешний подключить нельзя → карта на Daisy невозможна; возврат средств — вне устройства |
| Кадр | `01 LEN SEQ CMD DATA 05 BCC 03`; ответ `01 LEN SEQ CMD DATA 04 STATUS(6) 05 BCC 03`; LEN 1 байт (+0x20; при >224 — 0xFF); DATA 0–200; BCC 4 ASCII; тот же SEQ+CMD повторно — команда не исполняется, повторяется последний ответ |
| Открытие чека | **cmd 48**: `OperatorNum,Password,UNP[…]`; пароль ≤6 цифр; УНП `DY000600-OP01-0000001` (2 лат. буквы + 6 цифр — 4 символа оператора — 7 цифр); ответ `AllReceipt,FiscReceipt` |
| Сторно/возврат | cmd 48 вариант `R` (Reason 0 возврат/рекламация, 1 ошибка оператора, 2 уменьшение основы; DocLinkNum, DocLinkDT `DD-MM-YY HH:mm[:SS]`, номер ФП) и `C` (кредитное известие, InvLink) |
| Печать без бумаги | cmd 55 — только для прошивки в режиме online-магазина |
| Восстановление | cmd **76**: `Open,Items,Amount[,Tender,Remainder]`; cmd 103 информация о чеке; cmd 113 последний документ; cmd 90 диагностика |
| Текст/налоги | CP1251; налоговые группы кириллические (VG-17) |
| Евро | в протоколе есть запись о датах перехода параметра валюты (код 201/C9h) — **ПРОВЕРИТЬ** способ чтения/гейта; применимость прошивки — DOC-V4/DOC-V5 |

## 2a. Daisy Smart S: что нужно по образцу BlueCash
Требования те же, что к P-A: тип приёма команд (MQTT/BLE/HTTP), один процессор с блокировкой (BUG-072), журнал с ACK по hash (BUG-030), foreground-сервис (BUG-032), UNKNOWN≠FAILED (BUG-028), предвалидация до побочных эффектов (BUG-022), закрытие/восстановление открытого чека, реальные фискальные ссылки. Отличается только слой vendor SDK. Поэтому целесообразно **вынести общий рантайм из `bluecash-app` в общий модуль** и подключить к двум приложениям.

## 2b. Smart S: протоколы получены (2026-10-23)
Фискальный JSON-протокол и Payment Bridge — см. `docs/vendor/DAISY_SMART_PROTOCOLS.md`. SDK не требуется. Новые задачи: DOC-V4B (вопросы вендору), SMS-08 (HTTP-клиент), SMS-09 (мост); SMS-03/04/05/07 переписаны под JSON/HTTP. Критичные пробелы F-1…F-6 и Q-S*/Q-P* описаны там же.

## 2c. BluePay: протокол получен (2026-10-23)
Формат кадра `3E CMD 00 LH LL DATA XOR`, транзакции, события и прокси сокета — `docs/vendor/DATECSPAY_PROTOCOL.md`. Критично: ридеру без собственного интернета нужен прокси сокета банка на хосте — на ESP32-S3 его нет (новая задача BP-04); разрешение UNKNOWN — BP-05; вопросы вендору — BP-06. BLE UUID в PDF отсутствуют (взяты из Android SDK-примера).

## 3. Разрывы по путям (новые задачи; все внесены в `execution/status.json`)

| ID | Sev | Путь | Суть | Критерий |
|---|---|---|---|---|
| DSY-01 | P0 | P-D | Golden-кадры Daisy 2.0-4 (≥15: cmd 48 обычный/`R`/`C`, 49, 51, 53 по платёжным кодам, 56, 60/отмена, 76, 90, 113, 103; NAK/SYN; SEQ; LEN>224; экранирование) в `fixtures/daisy_golden_frames.json` + host-тест C++ | `IoT/protocol-abstraction/tests` зелёный |
| DSY-02 | P0 | P-D | GAP-009/GAP-024 reverify и исправление `DaisyCodec::decode/encode`, `serial_exchange` (дедлайн, SYN, NAK, SEQ) на общих векторах | host-тесты DSY-01 |
| DSY-03 | P0 | P-D | Развязка в `profile_executor.cpp`: выбор Daisy по профилю, а не «всё не-Datecs»; тест профилей (`Unconfigured` → отказ, не Daisy) | `run_native_saga.sh` |
| DSY-04 | P0 | P-D | Рестарт/обрыв: восстановление по cmd 76/103/113 вместо повторной фискализации; UNKNOWN≠NOT_SENT; доказательство документа по УНП | кейсы `profile_executor_native.cpp` |
| DSY-05 | P0 | P-D | Сторно Н-18 (cmd 48 `R`, FM-номер, дата-время исходного) — исходные данные из доказательства чека | `storno_matrix.json` для Daisy |
| DSY-06 | P1 | P-D | USB: зонд PRB-07 на реальном Compact S 01 (VID/PID, интерфейс, VBUS, переподключение, питание) — `DOCUMENTED_NOT_APPROVED` → результат в `probe_results.json` | probe-файл |
| DSY-07 | P1 | P-D | Маппинг платежей Daisy: только наличные/доступные типы; CARD → `PAYMENT_TYPE_UNSUPPORTED` на этом пути (нет терминала) — отдельная таблица `fixtures/payment_slots_daisy.json` | тест маппинга |
| DSY-08 | P1 | P-D | EUR-гейт: чтение параметра валюты, блокировка продаж при несоответствии (аналог VG-09) — после уточнения команды по DOC-V4 | тест `CurrencyParamGate` |
| SMS-01 | P0 | P-S | Получить SDK/API/лицензию и описание поддерживаемых прошивок Smart S (DOC-V4); положить в `docs/vendor/` с sha256. Без этого — `BLOCKED_SPEC`, остальные SMS-* не начинать | manifest sha256 |
| SMS-02 | P0 | P-S | Вынести общий Android-рантайм (процессор, журнал, MQTT/BLE/HTTP-ввод, сервис, секреты) из `bluecash-app` в общий модуль `:shared-runtime`; `bluecash-app` не меняет поведение (характеристические тесты до выноса) | `./gradlew :bluecash-app:testProdDebugUnitTest` зелёный до и после |
| SMS-03 | P0 | P-S | Порт `SmartFiscalPort` поверх vendor SDK; замена `DaisySmartStub` (стаб остаётся для debug-тестов); типизированные ошибки NotSent/Rejected/Unknown | тесты порта на фейке SDK |
| SMS-04 | P0 | P-S | Фискальные операции: открытие чека с УНП, строки, оплата, закрытие, сторно Н-18, X/Z, отмена открытого чека; реальные `fiscal_reference` | тесты по `storno_matrix.json` |
| SMS-05 | P0 | P-S | Платёжный терминал Smart S (если есть по SDK): покупка/возврат, UNKNOWN, компенсация только при подтверждённом NOT_COMMITTED (BUG-021/071) | тесты порядка вызовов |
| SMS-06 | P1 | P-S | Гейт PROD: стаб запрещён в PROD (оставить), реальный адаптер — за флагом `LAB` до evidence; тест запрета | `DaisySmartGateTest` |
| SMS-07 | P1 | P-S | Платёжные типы и слоты Smart S — `fixtures/payment_slots_smart.json` по SDK | тест маппинга |
| S3D-01 | P0 | P-B | Паритет исправлений VG-01/02/03/04/06/07 и BUG-063 (CP1251) для C++-пути (`FrameCodec.cpp`, `DatecsFiscalAdapter.cpp`, `serial_exchange.cpp`, `profile_executor.cpp`): для каждого — тест на тех же golden-кадрах, что в Kotlin | `fixtures/datecs_golden_frames.json` общий для Kotlin и C++ |
| S3D-02 | P0 | P-B | GAP-010 (LEN +0x2A) — перепроверить на C++ и Kotlin; общий вектор «пустой TX = `30 30 32 3A`» | тест vectors |
| S3D-03 | P1 | P-B | Платёжные слоты и валюта на DP-150: VG-23/VG-09 для C++ (`daisy_card_code`/Datecs PaidMode из `FiscalExecutionConfig`) | `fixtures/payment_slots_*.json` |
| BP-01 | P0 | P-C | Получить `DatecspayLowLevel.pdf` v3 (DOC-V3), положить в `docs/vendor/`; без него P-C = `BLOCKED_SPEC` | manifest sha256 |
| BP-02 | P0 | P-C | GAP-013/014: vendor BLE wake/flow, correlation, типы ссылок — тесты `bluepad_host` на векторах из PDF | `tests/bluepad_host` зелёный |
| BP-03 | P1 | P-C | Развязка профиля: BluePay-пинпад как отдельная привязка, а не неделимая часть `DatecsDp150BluePad50` — **решение D-15** (иначе P-C тестируется только вместе с P-B) | ADR |
| MQ-01 | P0 | P-A…D | Реальный publisher backend → ingress BlueCash и S3: одна операция доставляется один раз; чужие tenant/device отвергаются (GAP-001 reverify) | `client_test.go`, `run_native_mqtt.sh`, HIL |
| MQ-02 | P0 | P-A…D | BLE: стандартный аутентифицированный операционный канал (план 70 P10) обязателен до HIL; MTU/framing (GAP-005…008) | `run_native_ble.sh` |
| MQ-03 | P1 | P-A…D | Единый контракт результата и UNKNOWN для всех путей (`DeviceResult`: `COMMITTED/NOT_COMMITTED/UNKNOWN` + evidence) | контрактный тест |
| CL-01 | P1 | POS | D-13: BLE у POS (Expo) — нет; DEFAULT: POS → MQTT через backend, BLE только MiniPOS | решение владельца |
| CL-02 | P0 | MiniPOS | Маршрутизация MiniPOS → MQTT или BLE по доступности, без повторной отправки при UNKNOWN (MP-01…07) | e2e `TST-E2E-19` |
| HILMX-01 | P0 | все | Матрица HIL §5 + `scripts/check-hil-matrix.py` (SCR-40) | скрипт exit 0 |
| NAP-P-01 | P0 | P-E | Приложение 29 / Н-18 трассировка для каждого профиля (BlueCash, Datecs-S3, Daisy-S3): УНП, журнал, сторно, часы, Z/X, экспорты, e-чек | `check-nap-baseline.sh` + по профилю |

## 4. Решения владельца (новые)
(D-16 снят: две Daisy-интеграции подтверждены владельцем — Smart S и Compact S 01.)
| ID | Вопрос | DEFAULT |
|---|---|---|
| D-13 | BLE для POS (Expo)? | нет: POS → MQTT через backend; BLE — MiniPOS |
| D-14 | Какие типы оплаты поддерживает путь Daisy | только наличные и типы, имеющиеся в протоколе; карта — нет (нет терминала) |
| D-15 | Разделять ли BluePad как отдельную привязку | да, отдельная привязка (BP-03) |

## 5. HIL-матрица (строка = обязательный прогон)
Сценарии S1 наличные · S2 карта · S3 смешанная · S4 сторно · S5 обрыв (UNKNOWN→resolve) · S6 Z-отчёт · S7 повтор с тем же ключом · S8 часы/валюта-гейт.

| Путь | Клиент | Транспорт | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 |
|---|---|---|---|---|---|---|---|---|---|---|
| P-A BlueCash | POS | MQTT | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-A BlueCash | MiniPOS | MQTT | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-A BlueCash | MiniPOS | BLE | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-B Datecs COM | POS | MQTT | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-B Datecs COM | MiniPOS | MQTT | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-B Datecs COM | MiniPOS | BLE | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-C BluePay (+Datecs) | POS | MQTT | — | ✓ | ✓ | ✓ | ✓ | — | ✓ | — |
| P-C BluePay (+Datecs) | MiniPOS | MQTT | — | ✓ | ✓ | ✓ | ✓ | — | ✓ | — |
| P-C BluePay (+Datecs) | MiniPOS | BLE | — | ✓ | ✓ | ✓ | ✓ | — | ✓ | — |
| P-S Daisy Smart S | POS | MQTT | ✓ | по SDK | по SDK | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-S Daisy Smart S | MiniPOS | MQTT | ✓ | по SDK | по SDK | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-S Daisy Smart S | MiniPOS | BLE | ✓ | по SDK | по SDK | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-D Daisy USB | POS | MQTT | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-D Daisy USB | MiniPOS | MQTT | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| P-D Daisy USB | MiniPOS | BLE | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ |

«—» = сценарий неприменим к пути (нет пинпада / нет ФУ в данной привязке); «✓» = обязателен. Если решение D-13 изменит набор клиентов, строки POS/BLE добавляются. Итого обязательных ячеек считает `check-hil-matrix.py`.

## 6. Что пересмотрено в других документах (по этому файлу)
- `DECISIONS.md`: D-1, D-2, D-11 помечены «пересмотрено по MVP_GOAL»; добавлены D-13…D-15.
- `ARCH_09`: Eltrade/Fibank остаются вне MVP; Daisy (Smart S и Compact S 01) — в MVP (VG-17 поднят до P0).
- `ARCH_07` R-05 (линия отсечения): карточки P05–P11, P17, P19, P23–P27, P32 — **до** первого HIL.
- `ARCH_11` §1: критерии E1–E10 относятся к пути P-A; добавлены E11–E14 (P-B Datecs COM, P-C BluePay, P-D Daisy USB, транспорты).
