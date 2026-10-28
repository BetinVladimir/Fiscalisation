# Daisy Smart S — протоколы от вендора (разбор) 

Источники (в `docs/vendor/`, sha256 в `manifest.json`):
- `Protocol_Smart_Pro_v2_2025.pdf` — «Протокол за комуникация между фискални устройства и персонален компютър» v2.2025 (97 стр.), JSON. Применим к Smart Pro / Smart / Smart plus / Smart S (таблица лимитов, §21).
- `Protocol_Daisy_Smart_Payment_v1_2026.pdf` — «Протокол за комуникация между Payment Bridge и ПК» v1.2026 (10 стр.), JSON; банки `dsk`, `viva`.
Транспорт, который указал владелец (2026-10-23): **Android-приложение Smart S, MQTT и BLE как у BC-50; к самому устройству — HTTP через localhost**. В PDF **нет**: URL/порта/пути, заголовков, авторизации, способа упаковки JSON в HTTP. Всё это — вопросы вендору (§5), в конфигурации хранится `baseUrl` (по умолчанию не задан).

## 1. Фискальный протокол (Smart Pro v2.2025)
- Все сообщения — JSON; ответ `{"ok":bool,"data":{…},"messages":[{type,text,code}],"seq":n}`.
- Продажа/сторно/анулиране/корекция/фактура — **одна атомарная команда** `PrintFiscalReceipt` (нет отдельных open/sale/pay/close и нет отмены открытого чека).
- `data`: `UNP` (строка, формат `OO000000-0000-0000000` в примере), `refNum` (обязателен), `opCode`, `opPassword`, `items[]` (`type`: `plu` | `department` | `item`(имя+`vatGroup` кириллица А–З) | `comment`), `payments[]` (`amount`, `payTypeCode`, `type:"payment"`), необязательные `discountType`(`percent`|`amount`)+`discountValue`.
- Сторно (Н-18): `isStorno:true`, `reason` 0 операторская ошибка / 1 возврат / 2 уменьшение основы, `docNumLink`, `fmNumLink`, `dateTimeLink` (`dd-mm-yyyy hh:mm:ss`), `invoiceLink`.
- Оплаты по умолчанию: 0 «В БРОЙ», 1 «С ЧЕК», 2 «Кредит», 3 «USD», 4 «EURO»; список и `payTypeNap` — команда `GetPayments`. Данные таблицы **программируются в устройстве** → читать при старте, не хардкодить.
- Прочее: `OperatorLogin`, `PrintDailyReport{reset:true=Z}` (ответ `reportNumber`, `salesVatRows`, `stornosVatRows`), `GetStatus` (флаги: `clockSynchronized`, `paperEnd`, `ejFull`, `fmFull`, `deviceIsFiscalized`, `lastErrorCode`), `SetCashSumsInOut`, `GetPayments`, `GetVatGroups`, `GetOperators`, `PrintEj`, `PrintReceiptDuplicate{bonNumber}`.
- Лимиты символов (Smart S): артикул 20, пользователь 24, платёж 14, департамент 20 → ошибка 2101. Коды ошибок из примеров: 1000 (JSON), 1007, 1008 (оператор/пароль), 2101, 10000, 10100 (нет наличных).

### Критичные пробелы протокола (блокируют требования MVP)
| # | Факт | Последствие |
|---|---|---|
| F-1 | Успешный ответ продажи — только `{"ok":true,"seq":n}`: **номера документа, номера ФП, даты/времени ФУ нет** | `fiscal_reference` и доказательство документа (Н-18 сторно требует `docNumLink`/`fmNumLink`/`dateTimeLink`) получить нечем; нужна команда чтения последнего/конкретного документа (вопрос Q-S2) |
| F-2 | Нет команды чтения документа/ЕКЛ в машинном виде (`PrintEj`, `PrintReceiptDuplicate` только печатают, ответ `{"ok":true}`) | Разрешение `UNKNOWN` по УНП невозможно штатно; ответ UNKNOWN держать в `UNKNOWN`, не ретраить слепо (Q-S3: дедупликация по `UNP`/`refNum`?) |
| F-3 | Суммы — JSON-числа `Float(7,2)`, количество `Float(8,3)`; в таблице `payments.amount` тип указан Integer, в примерах дробные | Формировать число строго из целых центов (без бинарной погрешности), единый форматтер; уточнить Q-S4 |
| F-4 | В примерах опечатки: ключ `" items"` с пробелом, `"sec"` вместо `"seq"`, `"discountValue "`, `opPassword` то строка, то число | Использовать строгую схему `fixtures/smart_protocol_examples.json`, пароль как строка; Q-S5 |
| F-5 | Нет признака валюты ФУ (BGN/EUR); `GetPayments.currencyRate` EURO=1.95583 | EUR-гейт (D-10/VG-09) по `GetConstants`? — формат ответа не описан (Q-S6) |
| F-6 | `UNP` в примере — шаблон `OO000000-0000-0000000`; формат Daisy из Н-18 `DY000600-OP01-0000001` | Подтвердить, что ФУ принимает внешний УНП и не меняет (Q-S7) |

## 2. Платёжный мост (Payment Bridge v1.2026)
- Отдельный JSON-сервер, «предварительно установлен и конфигурирован на соответствующем порту» (порт не указан). Банки: `dsk`, `viva`.
- `payment_request`: `{"operation":"payment_request","bank":"dsk|viva","payload":{"reference_id","currency":"EUR","amount":42.50,"tip":0.00,"description"}}`.
- `check_status`: `{"operation":"check_status","bank":…,"payload":{"reference_id"}}` → `status` ∈ `SUCCESS | REJECTED | NOT_FOUND | FAILED`, `reference_id`, `transaction_details{amount,rrn,currency,card_type,masked_pan,stan,verification_method,authorisation_code,aid,order_code}`, `receipt[]{text1,align,text2}`, `error_code`, `message`, `timestamp` (UTC ISO-8601).
- **Плюс:** `check_status` по `reference_id` даёт штатное разрешение `UNKNOWN` для карты (`NOT_FOUND` = не проводилась; `SUCCESS` = проведена).
- **Нет:** команд возврата/отмены/сверки итогов; описания ответа `payment_request` (в PDF: «`status` Array»); таймаутов; списка кодов `error_code`; формата подписи/авторизации.
- Расхождение: поле `currency` описано как «EUR», в примере `"BGN"`. В MVP отправлять только `EUR` (Q-P3).
- Вопросы вендору: Q-P1 порт/путь/авторизация, Q-P2 ответ `payment_request` (синхронный до завершения или быстрый ACK + опрос?), Q-P3 валюта, Q-P4 возврат (refund/void) — есть ли команда; без неё D-12 для карты Smart S = «возврат вручную на терминале», Q-P5 идемпотентность `reference_id` (повтор запроса с тем же id), Q-P6 коды `error_code`.

## 3. Отображение на модель Bee
| Bee | Smart S |
|---|---|
| `open→sale→pay→close` | один `PrintFiscalReceipt` (атомарно); окна «открыт чек» нет → `FAILED_CLEAN` возможен только если запрос не ушёл или получен `ok:false` |
| `DeviceResult.COMMITTED` | `ok:true` **и** (после ответа на Q-S2) прочитан номер документа; пока Q-S2 открыт: `COMMITTED_NO_REFERENCE` недопустим в PROD — результат остаётся `UNKNOWN`/ручное подтверждение |
| `NOT_COMMITTED` | `ok:false` с кодом ФУ (до печати) |
| `UNKNOWN` | таймаут/обрыв HTTP после отправки |
| карта | Payment Bridge: `payment_request` → при UNKNOWN `check_status` → затем фискализация |
| платёжные слоты | `payTypeCode` из `GetPayments` (0 CASH; карта в ФУ как «Кредит»(2)/«С ЧЕК»(1) — уточнить Q-S8: как фискализируется оплата картой через мост) |

## 4. Где что в коде (ожидаемо)
`SmartDevices/daisy-smart-app` (заменить `DaisySmartStub`), общий рантайм `:shared-runtime` (SMS-02), клиенты: `SmartFiscalHttpClient` (SMS-08), `PaymentBridgeClient` (SMS-09).

## 5. Вопросы вендору (DOC-V4b) — единый список
Q-S1 URL/порт/путь/заголовки/авторизация фискального HTTP; Q-S2 как получить номер документа, номер ФП и время ФУ после `PrintFiscalReceipt`; Q-S3 дедупликация повторного `UNP`/`refNum`; Q-S4 тип `amount` в `payments`; Q-S5 корректные имена полей (items/seq), тип `opPassword`; Q-S6 чтение валюты ФУ; Q-S7 внешний УНП; Q-S8 оплата картой в ФУ при использовании моста; Q-P1…Q-P6 (выше).
