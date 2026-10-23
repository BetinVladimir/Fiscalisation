# ARCH_18 · Логика 60 тестов каталога ARCH_03 (Given / When / Then с числами)

Назначение: слабая модель пишет тест **по этому описанию**, не придумывая ожиданий. Формат: **G** (дано) · **W** (действие) · **T** (проверки — каждая отдельный assert) · **R** (красный до исправления: что именно падает). Деньги в центах `¢`. Источник правды для требований — карточка задачи в ARCH_01/02/09; здесь — логика проверки. Общие фикстуры: `fixtures/rounding_golden.json`, `fixtures/storno_matrix.json`, `fixtures/payment_slots_bc50.json`. Все тесты исполняются без реальных устройств; эмуляторы — ARCH_20.

Части: **1** Go (TST-GO), контрактные (CT), интеграционные (INT) · **2** Kotlin (KT), S3 (S3), POS (POS) · **3** вендорные (VENDOR).

---
## Часть 1 · Backend Go, контракты, интеграция

### TST-GO-01 `TestReceiptMoneyCentsRejectsSignedAndMalformed` (BUG-001)
G: функция `receiptMoneyCents(string)(int64,error)`. W: таблица входов. T: ошибка для `"-0.50"`, `"-1.50"`, `"+1.00"`, `"1.-5"`, `"1.5"`, `""`, `"1e2.00"`, `" 1.00"`, `"1.000"`; успех `"0.00"→0`, `"0.01"→1`, `"12.34"→1234`, `"99999.99"→9999999`. Доп.: план с платежами `-0.50`+`10.00` и ExpectedTotal `10.50` → `ValidateReceiptPlan` возвращает ошибку `invalid payment amount`. R: на `"-0.50"` сейчас нет ошибки.

### TST-GO-02 `TestValidMoney` (BUG-002)
G: `validMoney(string)bool`. W: таблица. T: true для `"0.00"`, `"0.01"`, `"99999.99"`; false для `"-0.01"`, `"100000.00"` (> предел цены), `"1,00"`, `"01.00"` (по решению D-4: фиксируется в тесте одним из значений), `".50"`, `"1."`; граница `"99999.99"` проходит, `"100000.00"` — нет. R: падает на отрицательных/выше предела.

### TST-GO-03 `TestOperationSimulatedFlagFollowsDriver` (BUG-003)
G: два драйвера — `Simulator` и фейк «реальный» (`simulated=false`). W: финализация продажи 1200¢ каждым. T: операция симулятора: `Simulated=true`, у реального — `false`; в ответе API и в событии outbox флаг совпадает; при `AppEnv=prod` симулятор отвергается конфигурацией. R: флаг сейчас константа.

### TST-GO-04 `TestFiscalOperationPrepareFailureMarksOperationFailed` / `…UsesBoundDriver` (BUG-004)
G: драйвер, у которого `Prepare` возвращает ошибку; в регистре привязан драйвер B, по умолчанию A. W: `FiscalOperation`. T: (1) ошибка Prepare → операция `FAILED` с `error_code`, UNP не потрачен повторно, outbox не содержит команды; (2) вызваны методы драйвера B, у A счётчик вызовов = 0. R: операция остаётся `PENDING`/вызван A.

### TST-GO-05 `TestHasDailyClockSyncUsesSofiaBusinessDate` / `TestSyncDriftOver30sTriggersSet` (BUG-005, VG-21)
G: тестовые часы. W: синхронизация в 2026-10-24 23:59:30 Europe/Sofia, проверка в 00:00:30 следующего дня. T: `HasDailyClockSync` в 23:59:30 = true; в 00:00:30 (новая бизнес-дата 25.10) = false; дрейф 31 с у драйвера с `SetTime` → вызван set; дрейф 29 с → не вызван; для BC-50 (нет cmd 61): дрейф 3 ч → `CLOCK_DRIFT_BLOCK`, 6 мин → предупреждение. R: сравнение по UTC-дате.

### TST-GO-06 `TestValidateRejectsUnknownAppEnv` / `TestLoadDefaultsAreSafe` (BUG-009)
G: окружение с `APP_ENV` ∈ {`prod`,`dev`,`test`,`Prod`,`production`,``,`staging`}. W: `config.Load`+`Validate`. T: `prod` — применяются prod-проверки (HTTPS, секреты не дефолтные, симулятор запрещён); неизвестные значения (`Prod`, `production`, `staging`) → ошибка, а не молчаливые dev-настройки; пусто → безопасные значения по умолчанию (тест-режим выключен, CORS не `*`). R: сейчас prod-проверки только при строгом равенстве `"prod"`.

### TST-GO-07 `TestReadyzFailsWhenRepositoryUnavailable` (BUG-010)
G: репозиторий-фейк с `Ping()` → ошибка. W: GET `/readyz` и `/livez`. T: `/readyz` = 503 с кодом `REPOSITORY_UNAVAILABLE`; `/livez` = 200 (процесс жив); после восстановления `/readyz` = 200 без перезапуска. R: `/readyz` всегда 200.

### TST-GO-08 `TestHMACTokenCannotCarryAdminRoleInProd` (BUG-011)
G: HMAC-токен с ролью `ADMIN`; `AppEnv=prod` и `dev`. W: запрос на админ-эндпоинт. T: prod → 401/403 `HMAC_ADMIN_FORBIDDEN`; dev → 200; токен `CASHIER` в prod допустим только для кассовых путей. R: роль принимается из подписанного токена без ограничения среды.

### TST-GO-09 `TestIdempotencyKeyReusableAfterPreSideEffect503` (BUG-012)
G: ключ K1; первый запрос падает 503 **до** побочных эффектов (БД недоступна на чтении). W: повтор с K1 после восстановления. T: второй запрос исполняется (не возвращает кэшированный 503), создан ровно один чек; если 5xx случился **после** побочного эффекта (`PutAmbiguousReplay`) — повтор возвращает сохранённый результат/`UNKNOWN`, новый документ не создаётся. R: 5xx «отравляет» ключ. (Статус карточки: TO VERIFY — тест сначала подтверждает дефект.)

### TST-GO-10 `TestReversalPolicy_<Case>` (TG-009, NORM-04/10/16)
G: `fixtures/storno_matrix.json` (7 кейсов) + политика сроков/ролей из DECISIONS §3 (`reversal.max_age_days=30`, роль supervisor). W: для каждого кейса последовательность запросов сторно. T: ответы совпадают с `expect`; сумма сторно ≤ остатка; повторное сторно полностью возвращённого → `ALREADY_FULLY_REVERSED`; чек старше 30 дней → `REVERSAL_POLICY_AGE`; оператор без роли → `REVERSAL_POLICY_ROLE`; сторно сохраняет ссылку: исходный УНП + дата документа. R: нет остатка/ссылки.

### TST-GO-11 `TestConcurrentFinalizeSameKeyProducesOneOperation` и др. (TG-001)
G: `RepositoryContract` запускается на Memory и Postgres. W: 50 горутин финализируют с одним ключом; затем рестарт репозитория. T: ровно одна операция; один УНП; после рестарта состояние идентично; `generation` строго возрастает; конфликт `ErrConcurrentState` обрабатывается повтором без потери записи; пустое хранилище → пустое состояние без ошибки. R: гонка/потеря.

### TST-GO-12 `TestUNPAllocatorExhaustion` / `TestUNPFormatRegex` (TG-002)
G: аллокатор с серией `DT000600-OP01-`. W: 9 999 999 выделений (в тесте — подмена верхней границы на 5). T: номера `…-0000001…-0000005` без дыр; шестое → `UNP_EXHAUSTED`; формат `^[A-Z]{2}\d{6}-[A-Z0-9]{4}-\d{7}$`; 50 параллельных → 50 уникальных; номер после рестарта продолжается без повторов. R: исчерпание не обработано.

### TST-GO-13 `TestPaymentTypeCapabilityMatrix` (BUG-042)
G: профили устройств: BC50_DEFAULT (`payment_slots_bc50.json`), Daisy Compact (только наличные), Smart S (по SDK). W: финализация с каждым типом оплаты CASH/CARD/CHEQUE/VOUCHER/COUPON/NHIF/DEFERRED/INTERNAL. T: допустимые идут к драйверу; недопустимые → `PAYMENT_TYPE_UNSUPPORTED` **до** любых побочных эффектов (карта не списана, УНП не потрачен). R: тип не проверяется по возможностям.

### TST-GO-14 `TestMoneyRoundingGolden` (BUG-023)
G: `fixtures/rounding_golden.json` (34 вектора). W: `lineTotalCents`/`discountedLineTotalCents`. T: каждый вектор совпадает; печать `ROUND OK 34`; добавление вектора с ошибочным ожиданием делает тест красным (mutation-проверка).

### TST-CT-01 Golden округления: один JSON — три потребителя
G: `contracts/money-rounding-golden-v1.json` = копия фикстуры. W: Go, Kotlin (`MoneyRoundingGoldenTest`), TS (`decimal.golden.test.ts`) читают файл. T: во всех трёх 34/34; sha256 файла записан в `scripts/check-rounding-golden.sh`; расхождение копий → exit 1.

### TST-CT-02 OpenAPI ↔ handler'ы
G: `contracts/openapi-*.yaml`. W: для каждой операции — валидный и невалидный запрос. T: ответ валиден по схеме; неизвестное поле тела → 400; коды ошибок из каталога; число операций в runtime = числу в lock (**сейчас FAIL — FISC-DOC-001**).

### TST-CT-03 MQTT-конверты backend ↔ S3 ↔ BlueCash
G: `contracts/mqtt-vectors/*.json` (≥10: команда finalize, reverse, ack, result COMMITTED/NOT_COMMITTED/UNKNOWN, неверная подпись, просрочено). W: каждый вектор разбирается тремя реализациями. T: одинаковый разбор или одинаковый код отказа; топик `tenants/{t}/devices/{d}/commands` совпадает у публикующего и подписчика.

### TST-CT-04 `FiscalIntentPayload`
G: `fixtures/intent-valid-*.json`, `intent-invalid-*.json` (≥12: нет платежей, Σплатежей≠итога, неизвестный тип, qty=0, дробные центы, валюта USD, нет ключа). W: валидация в TS и Go. T: valid → оба принимают; invalid → оба отвергают **с одним кодом**.

### TST-CT-05 Подписи HMAC/CBOR на трёх реализациях
G: `contracts/crypto-vectors-v1.json` (DER и P1363, MC-01). W: проверка в Go, Kotlin, C++. T: валидные подписи проходят; перепутанный формат DER↔P1363 → отказ без паники; искажённый 1 байт → отказ.

### TST-INT-01 Рестарт backend между `PutOperation` и ответом
G: стенд compose; fault-hook после commit. W: убить процесс; клиент повторяет с тем же ключом. T: один чек, один УНП; ответ повтора = исходному.
### TST-INT-02 Отключение PostgreSQL на 10 с
W: во время финализации. T: 503 + `Retry-After: 5`; после восстановления повтор с тем же ключом успешен; дубликата документа нет.
### TST-INT-03 Потеря MQTT между publish и результатом
T: операция `EXECUTING` → reconciler по таймеру (60 с, настраиваемо) → терминальное состояние через запрос статуса устройства (cmd 76/74 либо журнал); карта не сторнируется до определённости.
### TST-INT-04 Дубликат MQTT-результата
W: один результат доставлен 3 раза. T: одно событие в outbox, одна запись в аудите, состояние не меняется повторно.
### TST-INT-05 Неверная HMAC результата
T: сообщение отклонено; `mqtt_bad_signature_total` +1; состояние операции не изменилось.
### TST-INT-06 RabbitMQ недоступен 30 с
G: POS outbox. W: 5 продаж за время простоя. T: после восстановления доставка в порядке 1…5; на эмуляторе ФУ ровно 5 документов; нет дублей.
### TST-INT-07 50 финализаций с одним ключом
T: один `FISCALIZED`, 49 ответов-повторов с тем же телом; один УНП.

---
## Часть 2 · BlueCash (Kotlin), S3 (C++ host), POS

Общее для KT: фейки `FakeDatecsPort` (записывает последовательность команд, умеет `failAt(cmd)`, `eofAfter(cmd)`, `rejectAt(cmd, code)`), `FakeCardPort` (счётчики `purchaseCalls`, `reverseCalls`), `InMemoryJournal`. Товар для всех: «Кафе» 12.00 (1200¢, группа Б, qty 1.000), платёж CASH 12.00 если не сказано иное. Последовательность команд успешной продажи: `[48, 49, 53, 56]` (+51 если подитог включён).

### TST-KT-01 открытый чек закрывается при сбое (BUG-020)
G: порт; `rejectAt(49, -111)`. W: `sale()`. T: последовательность = `[48, 49✗, CANCEL]`; журнал: `RECEIPT_CANCELLED`; результат `FAILED(code=-111)`; следующая `sale()` проходит (чек не заблокирован). Вариант 2: `rejectAt(53, -112)` → `[48,49,53✗,CANCEL]`. Вариант 3: отмена тоже падает → журнал `RECEIPT_CANCEL_FAILED`, результат `RECOVERY_REQUIRED`.

### TST-KT-02 карта не сторнируется при UNKNOWN (BUG-021)
G: карта CARD 12.00 APPROVE; `eofAfter(56)`. W: `sale()`. T: `reverseCalls==0`; результат `FISCAL_RESULT_UNKNOWN`/`RECOVERY_REQUIRED(card_state=APPROVED)`. Вариант B: `rejectAt(56, -111)` (определённый отказ) → `reverseCalls==1`, журнал `CARD_COMPENSATED`.

### TST-KT-03 предвалидация (BUG-022)
G: строки: имя 73 символа; затем qty `"1e3"`; затем qty `120.000`. T: имя 73 → `FAILED PAYLOAD_INVALID:name`, `purchaseCalls==0`, команд к ФУ 0, журнал без `PAYMENT_PREPARED`; `"1e3"` → отказ ввода; `120.000` × 1.00 → принято (если лимит протокола допускает — сверка PDF; иначе фиксируем отказ с кодом); цена×кол-во 100000.00×100.000 > 9999999.99 → `PAYLOAD_INVALID:amount`.

### TST-KT-04 `MoneyRoundingGoldenTest` (BUG-023) — 34/34 вектора совпадают; строка `ROUND OK 34`.

### TST-KT-05 повтор после падения (BUG-024)
G: store содержит `PAYMENT_PENDING`, журнал `ACCEPTED+EXECUTING`. W: повторный `execute` той же команды. T: ответ `FISCAL_RESULT_UNKNOWN` (не `SALE_ALREADY_EXISTS`); `unp.next` не вызывался повторно; команд к ФУ нет (сначала запрос статуса).

### TST-KT-06 повтор компенсированной продажи (BUG-025)
G: журнал `CARD_COMPENSATED`. W: повтор. T: ответ `COMPENSATED` (терминальный), ни карты, ни ФУ не вызваны. То же для `RECOVERY_REQUIRED` — возвращается он, а не новое исполнение.

### TST-KT-07 нет номера документа → UNKNOWN (BUG-026)
G: ответ cmd 56 с пустым полем номера. T: результат `FISCAL_RESULT_UNKNOWN` (не `FISCALIZED`); при корректном ответе `SlipNumber=470` → `fiscal_reference="470"`, строка `DATECS-<op>` нигде не появляется (grep).

### TST-KT-08 пустой ответ ≠ успех (BUG-027)
G: порт возвращает `""` на cmd 49 и на cmd 56. T: оба — ошибка/UNKNOWN по фазе (до flush — NotSent; после — Unknown); статус-байты разбираются по таблице PM §6 (каждый бит — отдельный подтест).

### TST-KT-09 любая ошибка транспорта после записи = UNKNOWN (BUG-028)
G: параметры: `IOException`, `SocketTimeoutException`, `DATECS_LENGTH`, `DATECS_SEPARATOR`, `DATECS_FRAME_TOO_LARGE`, `DATECS_HEX`, EOF. T: после записи → `Unknown`; до записи → `NotSent`; разобранный ответ с кодом ≠0 → `Rejected(code)`.

### TST-KT-10 протокол (BUG-029)
`synBytesDoNotCorruptFrame`: перед ответом 5 байт `0x16` → кадр разобран. `nakTriggersByteIdenticalRetransmit`: на первый ответ `0x15` → повтор того же кадра, SEQ не изменился, максимум 3, четвёртый NAK → Unknown. `silentDeviceTimesOutWithUnknown`: молчание 30 с (виртуальное время) → Unknown, сокет закрыт. `concurrentCallersSerialized`: два потока одновременно — кадры не перемешаны, порядок SEQ последовательный. `lateOldSequenceIgnored`. SEQ после 0x7f → 0x20.

### TST-KT-11 ACK журнала (BUG-030)
G: события seq 1…5. W: `acknowledge(5, hashОтДругогоСобытия)`. T: `JournalAckMismatch`, чекпоинт остаётся 0, `purgeBefore` ничего не удаляет; верный hash → чекпоинт 5; разрыв цепочки `prev_hash` на seq 3 → отказ.

### TST-KT-12 нефискальный документ закрывается (BUG-031)
G: `printerTest`: `[38, 42✗]`. T: после сбоя вызван cmd 39; результат FAILED; повторный тест проходит.

### TST-KT-13 жизненный цикл (BUG-032)
T: уничтожение Activity не останавливает MQTT/BLE/HTTP (счётчик `stop()==0`); сервис перезапускается после kill (`START_STICKY`); `BootReceiver` стартует сервис только для активированного устройства; пароль оператора не пишется в SharedPreferences открытым текстом (grep по файлу prefs в тесте).

### TST-KT-14 `journalChainBreakDetected`
G: 10 событий. W: подменить payload события 4. T: `verifyChain()` возвращает ошибку на seq 4 (и только с него); неизменённая цепочка — OK.

### TST-KT-15 `localHttpRejectsUnsignedAndReplayedRequests`
G: локальный HTTP. W: (а) без подписи; (б) с неверной подписью; (в) валидный запрос дважды; (г) устаревший timestamp (>5 мин). T: (а)(б) 401; (в) первый 200, второй — тот же результат без повторного исполнения (`execute` один раз); (г) 401 `EXPIRED`.

### TST-S3-01 резервирование и UNP (BUG-040)
G: `executor_.ready()==false`. W: intent SALE_FINALIZE. T: `reserve_command` не вызван; UNP-счётчик не изменился; ответ `REJECTED_BEFORE_IO`; повтор с тем же `operation_id` после готовности использует тот же УНП.
### TST-S3-02 нет доверенного времени (BUG-041)
T: intent при `time_anchor` недействителен → `TRUSTED_TIME_REQUIRED`, ни резерва, ни I/O; после якоря — принимается.
### TST-S3-03 неподдерживаемый тип оплаты (BUG-042)
T: тип `NHIF` на профиле Daisy → отказ **до** резерва/I/O.
### TST-S3-04 сумма не сходится (BUG-043)
G: строки Σ=1200¢, платежи Σ=1199¢. T: отказ до резерва; код `PAYMENT_TOTAL_MISMATCH`.
### TST-S3-05 конфликт payload (BUG-044)
G: тот же `operation_id`, другой payload. T: ответ `PAYLOAD_CONFLICT` отправлен (раньше — тишина); исходная операция не изменена.
### TST-S3-06 отключение питания на каждой границе записи
G: durable storage с инжекцией «сбой после N-й записи», N=1…K (все границы между reserve, execute, outcome, outbox). W: для каждого N рестарт. T: состояние — одно из {не начато, `RECOVERY_REQUIRED`, терминальное}; никогда «исполнено дважды»; УНП не повторяется.

### TST-POS-01 reconcile URL (BUG-050): mock fetch; URL оканчивается `/reconcile`, метод POST, `:reconcile` отсутствует.
### TST-POS-02 routing (BUG-051): таблица 8 методов → базовый URL (fiscal `/public/v1/*`, gateway `/api/v1/*`); один базовый URL → ошибка конфигурации.
### TST-POS-03 `reconcileUsesNewKeyPerAttempt` (BUG-052): три вызова → ключи `…-reconcile-1/2/3`; operationId одинаков.
### TST-POS-04 пустой УНП (BUG-053): ответ `FISCALIZED, unp:""` → печать 0 вызовов, статус `NEEDS_ATTENTION`; `unp`, `fiscalNumber`, `fiscalReference` не подставляются друг вместо друга.
### TST-POS-05 таймаут записи (BUG-054): `fetch` не отвечает 15 с → `FiscalUnknownError(operationId)`; затем **один** `getOperationStatus` по тому же id, повторной записи нет.
### TST-POS-06 финализация шлюза (BUG-055): intent CARD 7.00 + CASH 5.00, `expected_total=12.00` → команда содержит два платежа с типами и `payment_id` из intent; без `amount`/`payments` → 422 на приёме.
### TST-POS-07 шлюз Submit (BUG-056/057/058): НДС 9% не пересчитывается как 20%; суммы из `payload.totals`; `Idempotency-Key` заменяет пустой `clientOperationId`, различающиеся → 422; ошибка БД при проверке политики (не `ErrNoRows`) → 503.
### TST-POS-08 потребитель результатов (BUG-059): невалидный конверт → `Nack(false,false)` в DLQ, requeue не происходит; валидный → применён; временная ошибка БД → отложенный requeue.
### TST-POS-09 golden округления TS: 34/34, на `Number`-арифметику запрет (lint-правило).

---
## Часть 3 · Вендорные тесты (TST-VENDOR)

**Эталонные данные извлечены из вендорных PDF автоматически и проверены по BCC** (воспроизводимо: `tools/extract_vendor_frames.py`, §tools):
- `fixtures/datecs_golden_frames.json` — **181 уникальный кадр** из Datecs PM v2.11.4 (116 запросов, 65 ответов; команды 33…123). Для **всех 116 запросов** `LEN = len(DATA) + 0x2A` (`"002:"` = `30 30 32 3A` для пустых данных, напр. cmd 56) — это закрывает сомнение GAP-010 по спецификации (код Kotlin `DatecsProtocol.kt:50` и C++ `FrameCodec.cpp` сверяются с этими кадрами; **ПРОВЕРИТЬ** результат на коде).
- `fixtures/daisy_golden_frames.json` — **16 кадров** из Daisy V.2.0-4 (cmd 48 с УНП `DY000694-OP01-0000018`, 74, 76, 119); 15 с подтверждённым BCC (один пример в PDF с расхождением BCC — оставлен с `bcc_verified:false` как негативный вектор).
Формулы (проверены на примере cmd 43 из PM): Datecs: `LEN = nibbles(len(DATA)+0x2A)` 4 байта (каждый nibble + 0x30); `BCC = nibbles(sum(байты от LEN до 0x05 включительно) & 0xFFFF)`. Daisy: `LEN = 0x20 + 3 + len(DATA) + 1`, BCC — та же сумма, 4 ASCII-nibble.

### TST-VENDOR-01 кадры Datecs побайтно (Kotlin host)
G: `datecs_golden_frames.json`, `direction=request`. W: для каждого кадра разобрать `(seq, cmd, data)` и собрать обратно `DatecsFrameCodec.encode(seq,cmd,data)`. T: байты равны `frame_hex` (116/116); ответы `direction=answer` разбираются: `STAT`(8 байт после 0x04) не пуст, BCC верен (65/65); искажение любого одного байта кадра → `DATECS_BCC`/`DATECS_BAD_FRAME`, не исключение.
### TST-VENDOR-02 то же в C++
G/W/T как 01, для `FrameCodec.cpp::DatecsCodec` и `IoT/common-modules/datecs`; результаты Kotlin и C++ идентичны (сравнение хэшей вывода). Отдельный подтест: пустой TX → `LEN=30 30 32 3A`; кадр с `LEN` на 4 меньше → отказ.
### TST-VENDOR-03 кириллица
G: имя «Топено сирене» (13 символов) для cmd 49. W: сборка кадра. T: байты имени `D2 EE EF E5 ED EE 20 F1 E8 F0 E5 ED E5` (из PM §4.12); эмодзи `😀` → отказ `UNSUPPORTED_CHAR` до I/O; длина лимита считается в **байтах CP1251** (72), а не в символах: 72 кириллические буквы проходят, 73 — нет.
### TST-VENDOR-04 Daisy
G: `daisy_golden_frames.json`; налоговые буквы А–З. T: кадры cmd 48/74/76/119 кодируются побайтно; латинская `A` вместо кириллической `А` (C0) → отказ `TAX_GROUP_LATIN`; ответ SYN (0x16) продлевает ожидание, дедлайн сбрасывается.
### TST-VENDOR-05 статус и пустые ответы
G: 64 бита STAT (8 байт × 8). W: для каждого бита ответ с одним поднятым битом. T: биты, означающие ошибку/нет бумаги/ФП полна → соответствующий код и категория; «нормальные» → OK; пустой ответ (нет `04…05`) → не успех. Каждая категория ошибок §7 PM → одна запись каталога (ссылка TST-VENDOR-09).
### TST-VENDOR-06 дубль SEQ
G: `FakeDatecsDevice` кэширует последний ответ по SEQ. W: приложение перезапущено после отправки cmd 56; команда повторена с тем же SEQ. T: ФУ не исполняет повторно (счётчик документов не растёт, = 1), возвращает кэшированный ответ; приложение не теряет команду.
### TST-VENDOR-07 восстановление
G: обрыв после cmd 53 (оплата) на каждой границе: после 48, 49, 53, 56. W: `resolve()` через cmd 76 (`IsOpen`, `Number`), cmd 255/параметр `UNP`. T: после 48/49/53 → `IsOpen=1`, `NOT_COMMITTED` → cmd 60 (отмена) → `FAILED_CLEAN`; после 56 при `IsOpen=0` и документ с нашим `UNP` найден → `COMMITTED`, реальный номер; нет документа с УНП и `IsOpen=0` → `NOT_COMMITTED`; ответ недоступен → `UNKNOWN`.
### TST-VENDOR-08 валюта и часы
G: ФУ с параметром валюты 0 (лев) при EUR-операции. T: отказ `DEVICE_CURRENCY_MISMATCH` **до** `card.purchase` (`purchaseCalls==0`); режимы 2 и 3 допустимы (D-10); часы ФУ (cmd 62) расходятся на 6 мин → предупреждение, на 3 ч → блокировка.
### TST-VENDOR-09 каталог ошибок
G: раздел 7 PM. T: число кодов в каталоге = числу строк таблицы (скрипт SCR-xx); неизвестный код → категория `UNKNOWN_DEVICE_ERROR`, а не успех.
### TST-VENDOR-10 манифест
G: `docs/vendor/manifest.json`. T: SHA256 каждого файла совпадает; подмена файла → exit≠0.
