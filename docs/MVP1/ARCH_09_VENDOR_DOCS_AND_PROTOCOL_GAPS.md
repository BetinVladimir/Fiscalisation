# ARCH_09 — Вендорная документация и сверка протоколов с реализацией

Дата: 2026-10-22. Метод: прочитан полный текст вендорных PDF (извлечён `pdftotext`), параграфы сопоставлены с кодом. Код не менялся.
Формат карточек как в ARCH_01: **ID · Sev · Спека · Место · Проблема · Исправление (шаги) · Тесты · Приёмка · DoD**. CDOD — в [ARCH_00_INDEX.md](ARCH_00_INDEX.md).

## 1. Инвентаризация вендорной документации

| Код | Документ | Где лежит | Версия/дата | SHA256 (первые 16) | Для чего |
|---|---|---|---|---|---|
| V-01 | Datecs «Communication protocol v2 / Programmer's manual», `PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4` (136 стр.) | `BeeloyBackend/docs/Fiscal/beefiscal_IoT/doc/Datecs/PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4 (7).pdf`; [официальный URL](https://www.datecs.bg/en/downloads/pdf?id=PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4.pdf) | v2.11.4, добавлен 18.11.2022 | `001beab7889dc803` | BlueCash-50 (BC-50), DP-25, FP-700 и др.: все команды ФУ |
| V-02 | Daisy `93PC_FD_CProtocol_ver_2_0-4_2026_EN` (68 стр.) + BG-версия `22PC_…_BG` | `…/doc/daisy/` | v2.0-4, 2026 | `9436ab7a9786a3df` / `970ae4b4cc578912` | Daisy ФУ |
| V-03 | Tremol `protocol_description_BG_2507141400` (124 стр.) | `…/doc/Termol/` | BG 2507141400 | `977eca19fa616d3b` | Tremol ФУ |
| V-04 | Datecs `BluePad-50-Plus` datasheet (1 стр.) | `…/doc/Datecs/BluePad-50-Plus.pdf` | — | `8d3641cea1f835aa` | пинпад, BLE 4.2 |
| V-05 | Eltrade protocol 1.3.2 (EN/BG), Fibank ECR Wiseasy 2.0.1 | `…/doc/eltrade/`, `…/doc/Fibank/` | — | `c5c73d17356a7cc5`, `cc9cd96c7e608109` | вне MVP (см. D-1) |
| V-06 | Datecs Android SDK/DEMO (Fiscal, PinpadSDK) | `BeeloyBackend/docs/Fiscal/Android/DatecsBC/` | — | не посчитан | демо/SDK BlueCash |
| V-07 | Datecs Pay Pinpad Commands v3 (2025) `DatecspayLowLevel.pdf` | в локальном SDK `examples/datecs/BC-50/datecspay-java-sdk_2025-02-28_v9.0.0Src/DOC` (по `03_DEVICE_PROTOCOLS.md:69`) | v3, 2025 | не найден в смонтированных каталогах | платёжный протокол |

Все PDF находятся **вне** репозитория `Fiscalisation` (в `BeeloyBackend`). Это разрыв воспроизводимости: агент в `Fiscalisation` их не видит (см. VG-19).

### 1.1. Что найдено в сети (проверено 2026-10-22) и чего не хватает
- Страницы [BlueCash-50](https://datecs.bg/en/products/322) / [316](https://datecs.bg/en/products/316): публичные документы — PM Communication Protocol v2.11.4 (18.11.2022), «Пълно ръководство за BlueCash-50» (BG, 27.10.2022), [UM_BC50MX-BUL_BlueCashPos.pdf](https://datecs.bg/en/downloads/pdf?id=UM_BC50MX-BUL_BlueCashPos.pdf) (BG, v1.2, ноябрь 2025). **SDK, драйверов, firmware и сертификатов на странице нет.**
- Более старый общий протокол: [PM_CommunicationProtocol_v2.11_eng.pdf](https://www.datecs.bg/en/downloads/pdf?id=PM_CommunicationProtocol_v2.11_eng.pdf) (v2.11.1, 2021) — заменять им v2.11.4 нельзя.
- Руководство BlueCashPos v1.2 (ноябрь 2025) описывает только меню «Конвертиране на цени в евро» (с 1.01.2026), настройки пинпада и ручной Z-отчёт. **Параметр валюты, курс, блокировка при потере связи с НАП, кодовая страница в нём не описаны** — вендор рекомендует обращаться на sales@datecs.bg.
- Протокол v2.11.4 (2022) **не содержит** производственного параметра валюты 0–3 из Прил. 1 т.16–18 Н-18 (введён ДВ 42/2025). В нём есть только `CurrNameLocal=ЛВ`, `CurrNameForeign=EUR`, `ExchangeRate=195583`, платёж PaidMode 6 «чужая валюта» и cmd 115 («только для BC-50»). Значит **нужна письменная позиция вендора** по EUR-прошивке (VG-09).

### 1.2. Документы, которые нужно получить (BLOCKED_SPEC, владелец — Анастасия/вендор)
| ID | Что получить | У кого | Блокирует |
|---|---|---|---|
| DOC-V1 | Релиз-ноты/версия прошивки BC-50 с поддержкой параметра валюты (Прил.1 т.16–18) и **допълнение към свидетелството за одобрен тип** от БИМ (§71(4) ПЗР Н-18) | Datecs / БИМ | VG-09, HIL |
| DOC-V2 (**частично закрыт**, см. DECISIONS §4; осталась HIL-проба PRB-01) | Описание чтения/записи параметров cmd 255 для BC-50: список имён (`UNP`, `NapBlockDateTime`, `DFR_needed`, параметр валюты) — в v2.11.4 они есть для семейства, но применимость к BC-50 подтверждена не вся | Datecs | VG-04, VG-10 |
| DOC-V3 | **Получено 2026-10-23** (`docs/vendor/`, разбор `DATECSPAY_PROTOCOL.md`; открыто Q-B1…Q-B3) — `DatecspayLowLevel.pdf` v3 (2025) в репозиторий + манифест | локальный SDK | VG-16 |
| DOC-V4 | **Получено 2026-10-23:** JSON-протоколы Smart (фискальный v2.2025, Payment Bridge v1.2026) в `docs/vendor/`; разбор и вопросы — `docs/vendor/DAISY_SMART_PROTOCOLS.md`. Нативного SDK нет и не нужен; осталось DOC-V4B (14 вопросов) | Daisy | VG-17 |
| DOC-V5 | Свидетельства одобренного типа (номер, дата) для моделей/прошивок HIL | Datecs/Daisy/Tremol | GATE-01 |
| DOC-V6 | Эквайринг: сертификация терминала у банка, merchant credentials | банк | HIL карты |

## 2. Что проверено и совпало со спецификацией (не менять)
- Кадр Datecs: `01 LEN(4) SEQ CMD(4) DATA 05 BCC(4) 03`, ответ с `04 STAT(8)`; длина = `len(DATA)+0x2A`; BCC — сумма байт после PRE до PST включительно. Независимо пересчитано на 10 примерах спецификации (cmd 33, 48 ×2, 53, 56, 60, 62, 74, 76, 103) и 49 — **все совпали** (`vendor/datecs_golden_vectors.json`). Реализация `DatecsFrameCodec.encode` на cmd 33 тестами уже подтверждена (`DatecsCodecVendorHost.kt`).
- Параметры cmd 48 по синтаксису №2 (`OpCode, OpPwd, NSale, TillNmb, Invoice`), cmd 43 (11 полей, порядок совпадает), cmd 49 (8 полей), cmd 53 `PaidMode, Amount`, налоговые группы A–H → `1..8`.
- Лимиты кадра: запрос ≤ 496 B данных, ответ ≤ 480 B — проверяются.
- C++ `common-modules/datecs/DatecsProtocol.cpp` **уже** обрабатывает SYN/NAK, таймаут 500 мс и ретрансмит с тем же SEQ; у него есть `rtcNotSynced()`, `syntaxError()`. Расхождения только в BlueCash/Kotlin (см. ниже).

## 3. Реестр разрывов «спецификация ↔ реализация»

Сводка (Sev: P0 — блокирует любую продажу на реальном ФУ; P1 — до HIL; P2 — до пилота):

| ID | Sev | Кратко | Связано |
|---|---|---|---|
| VG-01 | P0 | Текст на ФУ отправляется в UTF-8, а ФУ ждёт windows-1251: кириллица в названии товара искажается | новый |
| VG-02 | P0 | Статус-байты (STAT) не читаются; пустой ответ = успех; коды ошибок не классифицируются | расширяет BUG-027 |
| VG-03 | P0 | SEQ-политика: повтор того же SEQ возвращает кэш ответа и **не выполняет команду** — после рестарта приложения (SEQ снова 0x20) возможна «тихая» потеря команды | расширяет BUG-029 |
| VG-04 | P0 | Нет чтения состояния ФУ (cmd 74/76/103, параметр `UNP`) — UNKNOWN не разрешается, висящий чек не снимается | расширяет BUG-020/028 |
| VG-05 | P1 | Отображение типов оплаты на PaidMode неверно (CHEQUE→«ДЕБ.КАРТА» и т.д.); ответ cmd 53 `D`/`R` игнорируется | новый |
| VG-06 | P1 | Валидаторы уже диапазонов спецификации (сумма ≤ 99 999.99, кол-во ≤ 99.999); NSale в `open()` не проверяется по формату | новый |
| VG-07 | P1 | Не собирается доказательство чека (слип, номер документа, FM, время, UNP) → невозможны сторно (cmd 43) и печать | расширяет BUG-026/053 |
| VG-08 | P1 | Сторно: причина 0..2, инвойс-сторно, зависимость от VG-07 | связано NAP-01 |
| VG-09 | P1 | EUR: параметр валюты ФУ не читается и не проверяется; нет позиции вендора по прошивке | ARCH_10 |
| VG-10 | P1 | Параметры блокировки/связи с НАП (`NapBlockDateTime`, `DFR_needed`, ошибки −110210/−110225/−110226) не входят в readiness | связано NAP |
| VG-11 | P1 | Часы: на BC-50 cmd 61 «не используется»; нужна иная схема сверки (cmd 62 + бит 0.2) | уточняет BUG-005 |
| VG-12 | P1 | Z/X-отчёт (cmd 69) не реализован в движке BlueCash; ответ Z не сверяется с backend | новый |
| VG-13 | P2 | Параметр `Unit` (синтаксис №2) отправляется пустым, хотя «не пустая строка» | HIL |
| VG-14 | P2 | Решение D-8: платёж картой через пинпад самого ФУ (cmd 53 Type=1 / cmd 55) vs внешний Datecs Pay + сага | архитектура |
| VG-15 | P2 | cmd 70 (служебное внесение/изъятие) не используется — нужно для наличности при сторно | NAP |
| VG-16 | P1 | Datecs Pay / BluePad-50(+) — разные транспорты (BT3.0 SPP vs BLE 4.2); спецификация payment-TLV не в репозитории | BLOCKED_SPEC |
| VG-17 | P1 | Daisy: CP1251, кириллические налоговые буквы А–З, SYN каждые 100 мс; Android-интеграция — заглушка | проверить |
| VG-18 | P2 | Tremol BG2507141400: адаптер есть, сверка с PDF не проведена | вне MVP при D-1 |
| VG-19 | P1 | Вендорные PDF вне репозитория; нет манифеста и проверки хешей | процесс |
| VG-20 | P0 | Нет golden-векторов из спецификации в тестах BlueCash/S3 (кроме cmd 33) | тесты |
| VG-21 | P0 | **cmd 61 (установка даты/времени) на BC-50 не используется** (PM §4.22, «The command is not used on BC-50»): задача TG-003/BUG-005 не может «выставить» часы ФУ; допустимо только чтение (cmd 62) и измерение дрейфа | новый |
| VG-22 | P1 | Список команд, не применимых к BC-50: 33, 35, 47, 61, 63, 80, 106 («not used on … BC-50»); 115 — **только** BC-50. Клиент/эмулятор не должен их вызывать | новый |
| VG-23 | P0 | Маппинг PaidMode не соответствует PM §4.15/[*13]: код `BlueCashEngine.kt:397-406` CHEQUE→2 (в спеке 2 = дебитная карта), VOUCHER/COUPON→3, NHIF→4, DEFERRED/INTERNAL→5 (слоты 3–5 имеют заводские имена ЧЕК/ВАУЧЕР/КУПОН) | расширяет BUG-042, VG-05 |

---

### VG-01 · P0 · Кодировка текста: windows-1251
- **Спека:** Datecs PM §4.12 (стр. 11) — бинарный пример `cmd 49`: имя «Топено сирене» передано байтами `D2 EE EF E5 ED EE 20 F1 E8 F0 E5 ED E5` (windows-1251). Daisy §1: «All text data … according to code page CP1251» (`93PC_…EN.pdf`, раздел 1). UTF-8 дал бы `D0 A2 D0 BE D0 BF …` (проверено расчётом).
- **Место:** `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt` — `DatecsPayloads.bytes()` (`toByteArray(Charsets.UTF_8)`); `BlueCashEngine.kt:200` (`printerTest` строка), `BlueCashEngine.kt:377-383` (`ok()` декодирует ответ в UTF-8); `DatecsResponse.data` при чтении `fiscal_reference`. В `IoT/` и `protocol-abstraction/` перекодирования в CP1251 нет (`grep -rIn '1251' IoT` пуст).
- **Проблема:** названия товаров на кириллице (обязательно для БГ-рынка, Н-18 чл. 26) уйдут как UTF-8-последовательности и распечатаются «кракозябрами» или будут отвергнуты; длины в байтах тоже считаются неверно (лимит 72 символа ≠ 72 байта UTF-8).
- **Исправление (шаги):**
  1. `DatecsProtocol.kt`: ввести `object DatecsText { val cs = Charset.forName("windows-1251"); fun encode(s: String): ByteArray; fun decode(b: ByteArray): String }`. `encode` — через `CharsetEncoder` с `CodingErrorAction.REPORT` (символ без отображения → `IllegalArgumentException("DATECS_TEXT_UNMAPPABLE")`, не `?`).
  2. `bytes(...)` собирать как конкатенацию полей: текстовые поля — `DatecsText.encode`, числовые — ASCII; разделитель `\t`. Лимит длины считать по байтам после кодирования.
  3. `ok()`/разбор ответов: `DatecsText.decode` вместо UTF-8.
  4. Политика символов вне CP1251 (эмодзи, типографские кавычки «“ ”»): нормализовать заранее (список замен) либо отказывать; зафиксировать в ADR (`docs/decisions/ADR-FISCAL-TEXT-001.md`).
  5. Аналогично в C++: `DatecsPrinter::registerSale`/`DatecsFiscalAdapter` — UTF-8→CP1251 на границе (таблица 128 символов, без `iconv`); для Daisy то же + кириллические налоговые буквы.
- **Тесты:** `DatecsPayloadsTest.encodesCyrillicNameAsWindows1251` (вектор `cmd49_sale_cp1251` из `vendor/datecs_golden_vectors.json`: полный кадр совпадает побайтно); `rejectsUnmappableEmoji`; `counts72BytesNotChars`; `decodesCp1251Response`; C++: `datecs_cp1251_test.cpp` (те же байты).
- **Приёмка:** `python3 SmartDevices/bluecash-app/tests/host/run-datecs-codec.py` → `OK` с новым кейсом; `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsPayloads*'` зелёный; HIL `HIL-BC-03` (печатный бон содержит «Топено сирене» читаемо, фото в evidence).
- **DoD:** CDOD; ADR принят; `grep -rn "Charsets.UTF_8" SmartDevices/bluecash-app/src/main/kotlin/**/Datecs*.kt BlueCashEngine.kt` не находит использований для ФУ-текста.

### VG-02 · P0 · Статус-байты, пустой ответ, классификация ошибок
- **Спека:** §3.4: «при синтаксической ошибке … возвращается кадр с нулевой длиной поля данных и установленным битом статуса»; §6 (стр. 119): `0.0` Syntax error, `0.1` Command code invalid, `1.0` Overflow, `1.1` Command not permitted, `2.0` End of paper, `0.6` Cover open, `0.4` printer failure, `4.0/4.4/4.6` ошибки/переполнение/повреждение ФП, `0.5` общая ошибка, `0.2` часы не синхронизированы, `2.2` EJ full, `2.1` почти нет бумаги, `2.3` открыт фискальный чек, `2.5` открыт нефискальный. §7 (стр. 121+): справочник кодов ошибок.
- **Место:** `BlueCashEngine.kt:377-383` `ok()`: `text.startsWith("0\t") || text == "0" || text.isEmpty()`; `DatecsResponse.status` разбирается, но не используется (grep `\.status` в `BlueCashEngine.kt` пуст).
- **Проблема:** (1) пустые данные считаются успехом, а по спецификации это именно признак отказа (синтаксис/неизвестная команда) → команда не выполнена, а движок идёт дальше (например, `49` не принят, но `56` закрывает чек на меньшую сумму); (2) бит `0.5/1.1/2.0/4.x` не останавливает продажу; (3) любые отрицательные коды (−111015 «чек открыт», −110210 «связь с НАП заблокирована», −104000 «нужен Z») сворачиваются в строку кода без категории (retry/нужен Z/блок НАП/ошибка данных).
- **Исправление (шаги):**
  1. Новый `DatecsStatus` (8 байт → набор флагов + `fatal: Boolean`, `needsAttention`) и `DatecsErrorCatalog` (`code → {name, category: SYNTAX|STATE|PAPER|FM|NAP|PINPAD|DATA, retryable, unknownAfter}`), данные из §7 (файл `DatecsErrorCatalog.kt`, сгенерировать из PDF скриптом, список кодов не писать вручную).
  2. `DatecsResponse.requireOk(expectedFields: Int)`: данные не пустые, первое поле — целое `ErrorCode`; `ErrorCode != 0` → `DatecsRejected(code, category)`; число полей ответа проверять для каждой команды (карта `command → ожидаемое число`).
  3. STAT: `fatal` при `0.0, 0.1, 0.5, 1.0, 1.1, 2.0, 4.0, 4.6, 4.4, 0.6`; `warning` при `2.1, 2.4, 4.3`; `0.2` → `CLOCK_NOT_SYNCED` (блок продаж, см. VG-11).
  4. Категория `NAP` и `NEED_Z` пробрасывать в `readiness`/ответ POS как структурный код (не строка).
- **Тесты:** `OkRejectsEmptyData`, `OkRejectsSyntaxErrorStatusBit`, `OkParsesNegativeErrorCodeCategory` (−111015, −110210, −104000, −100114), `StatusDecodesAllBytesFromSpecExamples` (ответ cmd 74 из спецификации: `80 80 80 80 86 9A 80 80`), `UnknownFieldCountRejected`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsStatus*' --tests '*DatecsErrorCatalog*'`; проверка полноты каталога: скрипт `scripts/check-datecs-error-catalog.sh` сравнивает число кодов с `pdftotext`-выгрузкой раздела 7 (допуск 0).
- **DoD:** CDOD; закрывает BUG-027.

### VG-03 · P0 · Политика SEQ и повтор команд
- **Спека:** §3.4: «Если ФУ получает сообщение с тем же `<SEQ>`, что и последнее, оно **не выполняет операцию** и повторяет последний ответ»; диапазон `20h…FFh`. §3.3: таймаут хоста 500 мс; повтор с тем же SEQ и той же командой; NAK `15h`; SYN `16h` каждые 60 мс.
- **Место:** `DatecsProtocol.kt:177-197`: `private var sequence = 0x20` в экземпляре `DatecsSocketProtocol`; `require(sequence in 0x20..0x7f)`; новый экземпляр на подключение (проверить время жизни — см. `BlueCashEngine`/`BlueCashRuntime*`).
- **Проблема:** после перезапуска приложения/новом TCP-подключении SEQ снова `0x20`. Если последняя принятая ФУ команда имела SEQ `0x20`, **первая новая команда с тем же SEQ вернёт кэшированный ответ предыдущей и не выполнится**. Если команда та же (напр. две продажи подряд `49`), корреляция (`sequence`+`command`) пройдёт — строка чека потеряется молча. Для денежных операций это тихая потеря строки/оплаты.
- **Исправление (шаги):**
  1. Хранить `lastSeq` в журнале (`AndroidJournal`) вместе с `command_id`; при старте брать `next = lastSeq+1` (по кругу `0x20..0xFF`), а если неизвестно — начинать со случайного значения и первой командой отправлять read-only (`cmd 74`), чтобы «сжечь» совпадение.
  2. Разрешить диапазон `0x20..0xFF` (сейчас `≤0x7f`), `require` исправить.
  3. Повтор после таймаута — тем же кадром (BUG-029), затем при исчерпании попыток **не** считать FAILED: перейти в `UNKNOWN` + `DatecsRecovery` (VG-04).
  4. Проверка: ответ с чужим SEQ/командой — отбросить и продолжать читать до дедлайна (а не падать).
- **Тесты:** `SequenceSurvivesRestart`, `SameSeqReplayDoesNotCreateDuplicateLine` (эмулятор ФУ с кэшем последнего ответа), `SeqWrapsAt0xFF`, `RetryUsesIdenticalFrame` (общий с BUG-029), `TimeoutBecomesUnknownNotFailed`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsSequence*'`; HIL `HIL-BC-02`.
- **DoD:** CDOD; закрывает риск в `02_GAP_REGISTER.md:145` («каждый вызов стартует seq 0x20») для BlueCash.

### VG-04 · P0 · Чтение состояния ФУ и восстановление
- **Спека:** §4.34 cmd 76 (стр. 43) — `IsOpen (0 закрыт, 1 продажа, 2–4 сторно, 5 нефискальный, 6–9 инвойс), Number, Items, Amount, Payed`; §4.33 cmd 74 syntax #2 `Option=0` — статус текущего чека; §4.52 cmd 103 — `fStorno, Inv, InvNumb, суммы НДС`; §4.21 cmd 60 — отмена фискального чека; §4.73.1 cmd 255 чтение параметра по имени (список параметров стр. 112–118): **`UNP` — «последний напечатанный УНП (21 символ)», `StornoUNP`, `nLastFiscalDoc`, `nFBonDailyCount`, `DFR_needed`, `FMnumber`, `Fiscalized`**.
- **Место:** `BlueCashEngine.kt` использует только 38/39/42/48/49/53/56 (grep `execute(` ). Команды 60/74/76/103/255/62/64 не вызываются нигде в Kotlin.
- **Проблема:** (1) нет способа понять, закрыт ли чек, если связь оборвалась на `56` → `UNKNOWN` не разрешается автоматически; (2) после сбоя между `48` и `56` остаётся открытый чек (BUG-020), следующий `48` получает −111015/−111047; (3) нет «якоря» до `48`: номер последнего слипа и последний УНП не запоминаются.
- **Исправление (шаги):** `DatecsRecovery` (новый файл `DatecsRecovery.kt`, ≤ 150 строк):
  1. `snapshot()` → `cmd 76` + `cmd 255 UNP` + `cmd 255 nLastFiscalDoc`; вызывать **перед `48`** и записывать в журнал `PRE_OPEN{number, unp, lastDoc}`.
  2. `resolve(expected)` после UNKNOWN: `cmd 76`; `IsOpen=0 ∧ Number>pre.number ∧ UNP==expected.unp` → `COMMITTED`; `IsOpen=0 ∧ UNP==pre.unp` → `NOT_COMMITTED` (безопасно повторять/компенсировать); `IsOpen≠0` → чек наш: если оплаты не завершены — `cmd 60`, иначе дозавершить `56`; иначе `UNKNOWN` остаётся (ручное вмешательство).
  3. На старте и перед `48` при `IsOpen≠0` без активной операции — `cmd 60` + запись в журнал (`ORPHAN_RECEIPT_CANCELED`).
  4. Карту не сторнировать, пока `resolve` не вернул `NOT_COMMITTED` (BUG-021).
- **Тесты:** эмулятор ФУ с таблицей состояний; `ResolvesCommittedWhenUnpMatches`, `ResolvesNotCommittedWhenUnpUnchanged`, `CancelsOrphanOpenReceipt`, `DoesNotReverseCardWhileUnknown`, `StartupSnapshotRecorded`. Векторы запросов cmd 60/76/103 — из `vendor/datecs_golden_vectors.json`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsRecovery*'`; HIL `HIL-BC-04` (физический разрыв кабеля/BT на `56`).
- **DoD:** CDOD; BUG-020/021/028 ссылаются на VG-04; применимость `UNP` через cmd 255 на BC-50 подтверждена на стенде (иначе DOC-V2).

### VG-05 · P1 · Типы оплаты и ответ cmd 53
- **Спека:** §4.15 (стр. 15): `PaidMode 0 нал., 1 кредитная карта, 2 дебитная карта, 3/4/5 «other pay#3/#4/#5», 6 чужая валюта`; имена по умолчанию (§5, [*31], стр. 118): `1-КРЕДИТ, 2-ДЕБ.КАРТА, 3-ЧЕК, 4-ВАУЧЕР, 5-КУПОН, 6-EUR`. Ответ: `ErrorCode, Status ('D' остаток к доплате / 'R' сдача), Amount`. Н-18: «резерв 1 …», «резерв 2 — отложено плащане / вътрешно потребление» (`/tmp` текст: консолидированный Н-18, чл. 26б, чл. 118(8) ЗДДС).
- **Место:** `BlueCashEngine.kt:397-406` `datecsPaymentMode`: `CASH→0, CARD→1, CHEQUE→2, VOUCHER/COUPON→3, NHIF→4, DEFERRED/INTERNAL_CONSUMPTION→5`; вызовы `fiscal.execute(53, payment(...))` на `:265, :357` без чтения ответа.
- **Проблема:** `CHEQUE→2` — это **дебитная карта** по спецификации; `VOUCHER→3` печатается как «ЧЕК»; `NHIF→4` как «ВАУЧЕР»; `DEFERRED` и `INTERNAL_CONSUMPTION` делят один слот 5 («КУПОН») и нормативно различаются («резерв 2» — разные случаи). Статус `D` (недоплата) не проверяется → `56` на недоплаченном чеке даст отказ уже после списания по карте.
- **Исправление (шаги):**
  1. Таблица `PaymentSlotProfile` на модель/прошивку (из D-9): тип → слот → печатное имя; профиль хранится в конфиге активации, а не в коде.
  2. При активации читать/программировать названия слотов (cmd 255, имена платежей) и **сверять чтением**; расхождение → устройство `NOT_READY`.
  3. Для типов без подтверждённого слота — `UNSUPPORTED_PAYMENT_TYPE` до любого списания (после VG-06 валидации на входе).
  4. Разбор ответа 53: сумма `D` > 0 при последнем платеже → ошибка плана; `R` для CARD → ошибка (сдача с карты недопустима); сверка суммы платежей = сумме строк **до** `48`.
- **Тесты:** `PaymentModeMatchesSpecDefaults`, `UnsupportedTypeRejectedBeforeCardCharge`, `PartialPaymentStatusDRejected`, `ChangeOnCardRejected`; фикстуры ответов из §4.15 (`R 5.09`).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*PaymentSlot*'`; HIL-матрица из `docs/SUPTO/HIL_DATECS_DAISY_AUDIT_2026-09-11.md` (CHEQUE/VOUCHER/NHIF/DEFERRED = BLOCKED до профиля).
- **DoD:** CDOD; D-9 решён; закрывает пересечение с BUG-042 (S3, те же типы).

### VG-06 · P1 · Диапазоны и форматы полей
- **Спека:** §4.12: `Price 0.01…9999999.99`, `Quantity 0.01…999999.999`, `DiscountValue 0.01…9999999.99` (сумма) или `0.01…100.00` ([*28], процент), `PluName ≤ 72`, `Department 0…99`; §4.11/4.6: `NSale = "LLDDDDDD-CCCC-DDDDDDD"` (L=A–Z, C=0-9A-Za-z, D=0-9); `OpCode 1…30`, `OpPwd` (по умолчанию «0000» на BC-50, [*1]), `TillNmb 1…99999`.
- **Место:** `DatecsProtocol.kt` — `money = Regex("[0-9]{1,5}\\.[0-9]{2}")`, `quantity = Regex("[0-9]{1,2}\\.[0-9]{3}")`, `open(): unp.length in 8..64`, `password {1,8}`.
- **Проблема:** (1) легитимные позиции отвергаются (напр. 100.000 кг, сумма ≥ 100 000.00) — продажа падает **после** списания по карте (BUG-022); (2) `open()` пропускает любой УНП длиной 8–64 — неверный УНП уйдёт на ФУ; в `stornoOpen` формат уже строгий (`[A-Z]{2}[0-9]{6}-[A-Za-z0-9]{4}-[0-9]{7}`); (3) пароль оператора: PM §5 [*1] — пароль по умолчанию для BC-50 «0000» (4 цифры), т.е. «ровно 8 цифр» **неверно**; допустимая длина `{1,8}` оставляется, спец. ограничения нет (проверить PRB-текст на стенде); (4) [*4] «Max value of Price × Quantity is 9999999.99» — отдельная проверка произведения до отправки (в дополнение к диапазонам по полям).
- **Исправление:** вынести валидаторы в `DatecsLimits` (константы из спеки + источник), `open()` — тот же regex НСале, что и `stornoOpen`; специальные форматы УНП `XX000000-0000-0000000`/`ОО…` из ARCH_08 (NAP-04) проходят этот regex (проверить: кириллические «ОО» в `L[A-Z]` не проходят → вопрос к вендору, NORM-xx); для процентной скидки отдельный диапазон; валидировать план целиком **до** любого внешнего I/O (BUG-022).
- **Тесты:** `AcceptsMaxPriceAndQuantity`, `RejectsOverMax`, `OpenRequiresSpecUnpFormat`, `PercentDiscountMax100`, `RejectsPriceTimesQuantityOver9999999_99` ([*4]), свойство: любые значения в диапазоне спеки кодируются и декодируются симметрично.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsLimits*'`.
- **DoD:** CDOD.

### VG-07 · P1 · Доказательство фискального документа
- **Спека:** cmd 56 → `SlipNumber`; cmd 62 → дата/время (`DD-MM-YY hh:mm:ss DST`); параметры cmd 255 `FMnumber`, `nLastFiscalDoc`, `UNP` (стр. 115–116); cmd 43 требует `DocNum`, `DateTime`, `FMNumber`, `NSale` оригинала.
- **Место:** `BlueCashEngine.kt:266-270`: `closed.data … split('\t').filter { isNotBlank }.lastOrNull() ?: "DATECS-"+operationId` — последний токен ответа cmd 56 (для `0\t269\t` это слип «269»; если слипа нет — вернётся код ошибки «0»).
- **Проблема:** backend и POS получают только одну строку; нужных для сторно `DateTime` и `FMNumber`, а также `nLastFiscalDoc` нет → сторно (cmd 43) невозможно построить честно (BUG-008, BUG-053/026).
- **Исправление:** после успешного `56` (в том же критическом участке журнала) прочитать `cmd 62`, `cmd 255 FMnumber|nLastFiscalDoc|UNP`; сформировать `FiscalEvidence{slip, fiscalDocNumber, fmNumber, fiscalDateTime(+DST, Europe/Sofia), unp}`; сверить `UNP == command.unp`; журнал `FISCALIZED` хранит структуру; контракт backend/OpenAPI — поля `fiscal_document_number`, `fm_number`, `fiscal_datetime`, `unp` (после reviewed diff; contract-lock).
- **Тесты:** `EvidenceBuiltFromSpecExamples`, `UnpMismatchIsUnknown`, `MissingEvidenceDoesNotBecomeFiscalized`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*FiscalEvidence*'`; `make contract-lock` (если меняется контракт).
- **DoD:** CDOD; BUG-026 закрыт ссылкой.

### VG-08 · P1 · Сторно (cmd 43)
- **Спека:** §4.6 (стр. 7): `Storno 0 ошибка оператора / 1 возврат / 2 уменьшение налоговой основы`; `DocNum 1…9999999`, `DateTime` оригинала (≥ 01-01-2000), `FMNumber`, `Invoice 'I'`, `ToInvoice`, `Reason ≤ 64`, `NSale` — обязателен, если оригинал печатал ПК-ПО; ответ — `SlipNumber`. cmd 76 `IsOpen 2–4` — открыт сторно-чек, `7–9` — кредитное известие.
- **Место:** `DatecsProtocol.kt:137-170` `stornoOpen` (соответствует), `BlueCashEngine.kt:318-358` `reverse`.
- **Проблема:** корректный payload, но источник `DocNum/DateTime/FMNumber` — VG-07 (сейчас нет); платёж в сторно: «Payment in storno can not have change» (−111094); при возврате наличными/на карту — NAP-02/03 (наличность и возврат через ФУ).
- **Исправление:** зависит от VG-07; добавить маппинг reason (POS-причина → 0/1/2) через справочник; для инвойс-сторно — отдельная ветка; сторно-платёж: сверка суммы, запрет сдачи; после `43` — те же `49/53/56` с проверками VG-02.
- **Тесты:** `StornoOpenMatchesSpecExampleVector` (пример из §4.6: `21\t21\t9876\t0\t428\t24-04-19 08:36:27\t02636571\t\t\t\tDT636497-0021-0010001\t`), `StornoReasonMapping`, `StornoPaymentNoChange`, `StornoRecoveryByCmd76IsOpen3`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Storno*'`; HIL `HIL-BC-06`.
- **DoD:** CDOD; связка с NAP-01…03.

### VG-09 · P1 · EUR и параметр валюты ФУ
- **Спека/норма:** Н-18 Прил. 1 т.16–18 (ДВ 42/2025): производственный параметр `0` неактивен / `1` основная лев, доп. евро / `2` основная евро, доп. лев / `3` только евро; смена — только последовательно и однократно; перед сменой автоматический Z и служебный бон. §71(1),(4) ПЗР: привести ФУ/СУПТО в соответствие; прошивка — через БИМ, допълнение към свидетелството. Datecs PM v2.11.4 (2022): валютные поля `CurrNameLocal/CurrNameForeign/ExchangeRate`, `PaidMode 6`, cmd 115 (только BC-50) — **параметра 0–3 в документе нет**.
- **Место:** в BlueCash нет чтения валюты (`grep -i currency` пуст в `SmartDevices/bluecash-app/src/main`); `moneyMinor` и `DatecsPayloads` не знают валюту; backend экспортирует в EUR (`bgEuroAdoption`).
- **Проблема:** суммы отправляются на ФУ «как есть». Если ФУ ещё в режиме «основная лев» (параметр 1) — бон будет в лв, а карта списана в EUR (`card.purchaseEur`). Это прямое расхождение денег и фискального документа.
- **Исправление:** см. ARCH_10 NAP-EUR-01…05: чтение параметра валюты и курса при активации (cmd 255, имя из DOC-V2), `READY` только при значении `3` (или `2` по D-10), запись `currency_mode` в binding, проверка в `reserve`/до `48`; тест «ФУ в режиме лев → отказ до списания по карте».
- **Тесты:** `RefusesSaleWhenDeviceCurrencyNotEuro`, `ReadsCurrencyParameterAtActivation`, `CardChargedOnlyAfterCurrencyVerified`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DeviceCurrency*'`; HIL `HIL-BC-EUR-01`: фото бона с «EUR» и параметр на экране/через чтение.
- **DoD:** CDOD; DOC-V1 получен или задача `BLOCKED_SPEC`.

### VG-10 · P1 · Связь с НАП и блокировки ФУ в readiness
- **Спека:** параметры `NapBlockDateTime` (дата, после которой ФУ блокируется), `DateLastSucceededSent`, `EcrUnsentWarning` (предупреждение за N часов), `NapRegistered`, `DeregOnServer`, `Fiscalized`, `DFR_needed` (нужен Z); ошибки `−110210 ERR_NAP_BLOCKED_72H`, `−110225 BLOCKED_MAX_SELLERRORS (неотправленные документы)`, `−110226 BLOCKED_24H`, `−110217 MAX_ZERRORS`, `−111024 END_OF_24_HOUR_PERIOD`, `−104000 NEED_Z_REPORT`, `−111098 WAIT_UNTIL_MIDNIGHT`; cmd 68 `ReportsLeft`; статус `4.3` «меньше 60 Z-отчётов в ФП».
- **Место:** readiness в backend строится по `HasDailyClockSync` и binding (см. BUG-005/010), не по состоянию ФУ.
- **Проблема:** продажа стартует на ФУ, который через минуты заблокируется или уже требует Z; сбой обнаруживается после списания карты.
- **Исправление:** `DatecsReadiness.read()` → объект `{fiscalized, napRegistered, napBlockAt, unsentWarning, zNeeded, reportsLeft, rtcSynced, paperOk, receiptOpen, currencyMode}` (cmd 255 ×N + 74 + 68); правила: `napBlockAt - now < N ч` → предупреждение в POS; `zNeeded ∧ day>24h` → блок до Z; публикуется в `device readiness` и учитывается в `reserve` (backend).
- **Тесты:** таблица `ReadinessMatrix` (по строке на правило), `BlockedByNapIsStructuredError`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsReadiness*'`; backend `go test ./internal/domain -run Readiness -count=1`.
- **DoD:** CDOD; поля добавлены в OpenAPI через reviewed diff.

### VG-11 · P1 · Часы ФУ (BC-50)
- **Спека:** §4.22 cmd 61: «**The command is not used on BC-50**»; §4.23 cmd 62 читает дату/время; статус `0.2` «RTC not synchronized»; формат `DD-MM-YY hh:mm:ss DST`; ошибки `−100508 ERR_RTC_ERROR`, `−102019 DATETIME_EARLIER_THAN_PREV_Z`, `−111077 WRONG_DATE_FM`.
- **Место:** `fiscal-backend/internal/mqttclient/client.go:94-104` (BUG-005); Kotlin — нет чтения.
- **Проблема:** запланированный в BUG-005 «DEVICE_TIME + выставление времени» не работает на BC-50 (cmd 61 недоступна). Источник времени для BC-50 — сеть/НАП; нужна только **проверка** через cmd 62.
- **Исправление:** `DatecsClock.read()` (cmd 62, парсинг `DST` → смещение Europe/Sofia), расхождение с доверенным временем > порога (по Н-18 допуск часов — не более 2 ч, чл. 25, ал. 12, PDF стр. 27; порог readiness из NORM) → `NOT_READY`; для моделей, где cmd 61 доступна (DP-25 и др.), — установка с пост-проверкой; отражено в профиле модели.
- **Тесты:** `ParsesDstSuffix`, `DriftAboveThresholdBlocksSale`, `Bc50NeverSendsCmd61`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsClock*'`; BUG-005 тест `TestClockSyncRejectsHostClockAsDeviceClock` остаётся.
- **DoD:** CDOD.

### VG-12 · P1 · X/Z-отчёты (cmd 69)
- **Спека:** §4.29.1 cmd 69 `ReportType X|Z`, ответ `nRep, TotA…TotH, …`; параметр `DFR_needed`; авто-Z при смене параметра валюты; §4.57 cmd 110 доп. суммы по платежам.
- **Место:** `DatecsPayloads.report(z)` есть; в `BlueCashEngine` нет сценария/ответа Z; backend `exports.go` строит 18.1–18.9 по своим данным.
- **Проблема:** нет закрытия дня на реальном ФУ из системы, нет сверки Z ↔ backend, повтор Z после UNKNOWN не определён (идемпотентность Z: второй Z вернёт `ERR_Z_REPORT_CLOSED −104006`, что нужно трактовать как «уже выполнен»).
- **Исправление:** команда `DAILY_REPORT{Z}` с журналом (`EXECUTING→DONE|UNKNOWN`), разрешение UNKNOWN через `nZreport` (cmd 255) до/после; сохранение `nRep`, итогов; сверка с backend-суммами → `Z_MISMATCH` инцидент.
- **Тесты:** `ZRepeatAfterUnknownIsIdempotent`, `ZTotalsStoredAndReconciled`, `XDoesNotChangeCounters`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DailyReport*'`; HIL `HIL-BC-05`.
- **DoD:** CDOD.

### VG-13 · P2 · Параметр `Unit`
- **Спека:** §4.12 синтаксис №2: `Unit` — «Not empty string, ≤ 6 символов» (пример «бр.» в CP1251: `E1 F0 2E`).
- **Место:** `DatecsPayloads.line` — `v.unit` по умолчанию `""`.
- **Проблема:** пустое значение при 8 полях может быть истолковано как синтаксис №2 с пустой единицей (отказ) либо как синтаксис №1 — поведение модели не описано.
- **Исправление:** всегда отправлять синтаксис №2 с `Unit="бр."` по умолчанию (через VG-01), либо №1 с 7 полями — выбрать по профилю модели после HIL; зафиксировать в профиле.
- **Тесты:** `LineUsesDefaultUnit`; HIL: оба варианта на реальном BC-50, результат в evidence.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsPayloads*'`; HIL-BC-03.
- **DoD:** CDOD.

### VG-14 · P2 · Решение D-8: пинпад ФУ или внешний терминал
- **Спека:** §4.15.2 cmd 53 `PaidMode=2, Type=1|12` — оплата через пинпад, подключённый к ФУ; специальный ответ `−111560` («пинпад успешен, а ФУ — нет») с `Sum`, `CardNum`; `−111555`; §4.17 cmd 55 команды пинпада; руководство BlueCashPos v1.2: настройки пинпада.
- **Место:** текущая архитектура: Datecs Pay SDK (`card.purchaseEur`) → затем `53` как «карта» (`BlueCashEngine.kt:249` `purchaseEur`, `:259-266`, `:346-357`) + сага компенсации.
- **Проблема/вариант:** встроенный путь снимает класс ошибок «карта списана, чек не напечатан» на уровне ФУ (само устройство возвращает −111560), но привязывает платёж к конкретным прошивкам/банку и меняет HIL-сценарии. Нужно сознательное решение.
- **Действие:** решение владельца **D-8** (в MASTER_PLAN); до решения оставить текущий путь, VG-04 обязателен в любом случае; если выбран встроенный — отдельные карточки (в статусе `BLOCKED_SPEC` до DOC-V6).
- **Приёмка (автоматическая):** `grep -n 'D-8' docs/MVP1/MASTER_PLAN.md MVP_GATES.md` содержит строку с принятым решением (`DECIDED: internal-terminal|device-pinpad`); для `device-pinpad` — тесты `Cmd53PinpadErrorMinus111560IsUnknown` и `PinpadSuccessFiscalFailureRoutesToRecovery`.
- **DoD:** CDOD; решение записано в ADR (`docs/decisions/ADR-FISCAL-CARD-PATH-001.md`).

### VG-15 · P2 · cmd 70 — служебное внесение/изъятие
- **Спека:** §4.30 (стр. 38): `Type 0/1` внесение/изъятие (и 2/3 в валюте), в ответе остатки.
- **Применение:** сторно с возвратом наличных при недостатке наличности в кассе (NAP-02), закрытие смены. Сейчас `DatecsPayloads.cash()` есть, в движке не используется.
- **Исправление:** сценарий `CASH_IN_OUT` + журнал + проверка остатка перед сторно-наличными.
- **Тест:** `StornoCashRequiresAvailableBalance`, `CashOutUnknownResolvedByCmd70Totals`.
- **Приёмка:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*CashInOut*' --no-daemon` exit 0.
- **DoD:** CDOD; связка с NAP-02.

### VG-16 · P1 · Datecs Pay / BluePad-50 (Plus)
- **Спека:** BluePad-50 Plus datasheet: BLE 4.2; BluePad-50 (старый) — BT 3.0 SPP/iAP (`03_DEVICE_PROTOCOLS.md:61`); низкоуровневый протокол — `DatecspayLowLevel.pdf` v3 (2025), не в смонтированных каталогах (DOC-V3).
- **Место:** `IoT/common-modules/datecspay/*`, `DatecsPayBleProfile.h`, `DatecsPayTerminalAdapter.cpp`.
- **Проблема:** профиль BLE (UUID/TLV) нельзя угадывать; без DOC-V3 любые тесты — на самодельных эмуляторах.
- **Действие:** DOC-V3 в манифест (VG-19); сверка констант `DatecsPayBleProfile.h` с документом (таблица «константа ↔ раздел PDF»); до сверки статус `BLOCKED_SPEC`; HIL карты — только с merchant credentials (DOC-V6).
- **Приёмка:** `scripts/check-vendor-docs.sh` (VG-19) + файл `docs/vendor/datecspay-constants-trace.md` со строкой на каждую константу.

### VG-17 · P1 · Daisy
- **Спека:** `93PC_…EN.pdf` §1: **CP1251**, налоговые группы — **кириллические** А,Б,В,Г,Д,Е,Ж,З (`C0…C7`); §2: ответ ≤ 100 мс, таймаут ПК ≥ 500 мс, повтор с тем же SEQ; SYN каждые 100 мс (у Datecs 60 мс); кадр STX/ETX (`STX LEN SEQ CMD DATA Postamble1 BCC ETX`).
- **Место:** `IoT/protocol-abstraction/src/DaisyFiscalAdapter.cpp`, `FrameCodec.cpp`; `SmartDevices/daisy-smart-app/.../DaisySmartStub.kt` — **заглушка**, Android-интеграции нет.
- **Проблема:** ловушка «латинская A ≠ кириллическая А»; CP1251 (см. VG-01); без SDK (DOC-V4) нативная интеграция Daisy Smart невозможна.
- **Действие:** тесты `DaisyTaxGroupUsesCyrillicCp1251`, `DaisySynEvery100ms`; golden-векторы Daisy из PDF по тому же методу (таблица в ARCH_03 TST-VENDOR-04); решение D-1 — входит ли Daisy в MVP.
- **Приёмка:** `make iot-protocol-test`.

### VG-18 · P2 · Tremol
- Адаптер есть (`TremolFiscalAdapter.cpp`, STX 02h/терминатор 0Ah, XOR-контрольная сумма по `03_DEVICE_PROTOCOLS.md:79`); сверка с V-03 не выполнялась. Вне MVP при ответе D-1 «только Datecs BC-50». Действие: golden-векторы (метод как VG-20) — после решения D-1.
- **Приёмка:** `bash IoT/protocol-abstraction/run-tests.sh` содержит и проходит `TremolGoldenVectors`; до D-1 статус `BLOCKED_SPEC`.
- **DoD:** CDOD.

### VG-19 · P1 · Управление вендорной документацией
- **Проблема:** PDF вне репозитория, нет манифеста и проверки хешей; агент исправлений не сможет открыть спецификацию.
- **Исправление (шаги):**
  1. Создать `Fiscalisation/docs/vendor/README.md` и `docs/vendor/manifest.json` (имя, версия, URL, дата получения, SHA256, лицензия/ограничения, кто и что покрывает; значения из таблицы §1).
  2. Положить PDF в `docs/vendor/` (протокол Datecs допускает свободное использование для написания драйверов — §2 PDF; для остальных проверить лицензию, иначе хранить в приватном хранилище и оставить только манифест).
  3. Текстовые выгрузки (`pdftotext -layout`) — в `docs/vendor/text/` для поиска агентом.
  4. `scripts/check-vendor-docs.sh`: пересчёт SHA256 по манифесту, ненулевой код при расхождении; включить в `make plan-check`.
- **Приёмка:** `bash scripts/check-vendor-docs.sh` → exit 0; `jq 'length' docs/vendor/manifest.json` ≥ 7.
- **DoD:** CDOD.

### VG-20 · P0 · Golden-векторы из спецификации
- **Основание:** `vendor/datecs_golden_vectors.json` (создан этим анализом): 11 запросов и 2 ответа из примеров спецификации, независимо воспроизведены энкодером (`match_encoder: true` для всех).
- **Место:** `SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt` содержит один вектор (cmd 33); `IoT/common-modules/datecs/tests/DatecsProtocolTest.cpp`, `IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp` — проверить покрытие.
- **Исправление:** добавить в оба хост-теста все векторы из JSON (cmd 48 ×2, 49 (CP1251!), 53, 56, 60, 62, 74, 76, 103, ответы 56/76 с проверкой BCC и разбором STAT).
- **Тесты:** все 11 запросов и 2 ответа из JSON воспроизводятся побайтно в Kotlin и C++.
- **Приёмка:** `python3 SmartDevices/bluecash-app/tests/host/run-datecs-codec.py` и `bash IoT/common-modules/datecs/run-tests.sh` — все новые кейсы зелёные; проверка полноты: `jq 'keys|length' docs/MVP1/vendor/datecs_golden_vectors.json` = числу кейсов в тестах (скрипт `scripts/check-golden-vectors.sh`).
- **DoD:** CDOD.

### VG-21 · P0 · Часы ФУ BC-50 нельзя установить командой 61
- **Спека:** PM §4.22 cmd 61 «The command is not used on BC-50»; cmd 62 (чтение) поддерживается; Н-18 чл.25 ал.12 — расхождение часов ≤ 2 ч.
- **Проблема:** TG-003 предполагает синхронизацию с записью. На BC-50 запись времени вендором не описана → «синхронизация» возможна только как **контроль дрейфа**: прочитать cmd 62, сравнить с серверным временем, при |Δ| > порога блокировать продажи (`CLOCK_DRIFT_BLOCK`) и требовать сервисное вмешательство/синхронизацию средствами ФУ (сеть НАП).
- **Исправление:** TG-003 переформулировать: шаг «установка» заменить на «чтение + сравнение + блокировка»; порог `CLOCK_DRIFT_MAX = 2h` (Н-18) и предупреждающий порог 5 мин; запись `CLOCK_DRIFT_OBSERVED`.
- **Тесты:** `ClockDriftOver2hBlocksSales`, `ClockDrift5minWarnsOnly`, `BlueCashNeverSendsCmd61` (перехват кадров эмулятора: cmd ≠ 61).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*ClockDrift*'`; `go test ./internal/domain -run ClockDrift -count=1`.
- **DoD:** CDOD; HIL-BC-02: фактическое поведение cmd 61 на BC-50 записать в `vendor/probe_results.json` (PRB-05).

### VG-22 · P1 · Неприменимые к BC-50 команды
- **Спека:** PM §4.1 (33), §4.2 (35), §4.10 (47), §4.22 (61), §4.24 (63), §4.35 (80), §4.54 (106) — «not used … BC-50»; §4.60 (115) — only BC-50.
- **Исправление:** константа `BC50_UNSUPPORTED_CMDS = {33,35,47,61,63,80,106}` в `DatecsLimits`; эмулятор BC-50 отвечает ошибкой; клиентский код при формировании запроса проверяет профиль модели.
- **Тест:** `Bc50EmulatorRejectsUnsupportedCommands`, `EngineNeverEmitsUnsupportedCommands` (прогон всех сценариев sale/reverse/z-report/clock с перехватом кадров).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Bc50Profile*'`.
- **DoD:** CDOD.

### VG-23 · P0 · Маппинг PaidMode по спецификации
- **Место:** `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt:397-406`.
- **Исправление:** профиль `BC50_DEFAULT` из DECISIONS D-9: CASH→0, CARD→1, CHEQUE→3, VOUCHER→4, COUPON→5; NHIF/DEFERRED/INTERNAL_CONSUMPTION → ошибка `PAYMENT_TYPE_UNSUPPORTED` **до** списания по карте (валидация плана до внешнего I/O, BUG-022). Функцию сделать `paymentSlot(profile, type): Int?`.
- **Тесты:** `PaymentSlotMatchesVendorTable` (таблица из PM [*13]), `ChequeIsNotDebitCardSlot`, `UnsupportedTypeRejectedBeforeCardCharge`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*PaymentSlot*'`.
- **DoD:** CDOD.

## 4. Расширение декомпозиции для слабой модели
Каждый VG разбивается на подзадачи `ID.n` (≤ 3 файлов, один инвариант) — перечень в `execution/status.json` (поле `subtasks`). Правило: подзадача заканчивается красным→зелёным тестом и одной командой приёмки; без подзадач с «изучить» и «разобраться».

## 5. Чего эта сверка не покрывает (честно)
- Реальные ответы устройства (формат полей cmd 255 для имён параметров на BC-50) — только HIL / DOC-V2.
- Протоколы Eltrade/Fibank не сверялись (вне MVP).
- Datecs Pay payment-TLV и банковские коды — DOC-V3/V6.
