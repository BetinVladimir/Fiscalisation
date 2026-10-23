# Fiscalisation: критерии unit-тестов

Дата: 2026-10-21. Это требования, не результаты запуска. Привязать к существующим GAP/T/UI из основного remediation-плана; не переименовывать его закрытые/открытые строки.

## Правила

Каждая строка — обязательный отдельный case или явно перечисленный parameterized набор. Тестировать production функции через fake-порты, не копию алгоритма. Фиксировать вход, state до/после, число side effects, ожидаемый результат, test name/source SHA и PASS/FAIL/NOT RUN. Fake-clock/RNG/FS/transport/SQL driver допустимы без локального backend или БД. Клиентский симулятор не заменяет demo.

Для driver acceptance подменять только физическую границу и проверять реальные protocol bytes независимым stateful peer; generic purchase=true/fiscalize=true mocks недостаточны. Криптографические test keys — синтетические. PAN/PIN/CVV и реальные secrets запрещены.

## Identity, tenant, roles и session

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| AUTH-01 | Нет/повреждённый/expired bearer | Отказ до business mutation и внешнего I/O |
| AUTH-02 | Actor другого tenant/location/register | Отказ; нет чтения или записи чужой операции |
| AUTH-03 | Sale permission вместо ADMIN reconcile | Reconciliation запрещена |
| AUTH-04 | Logout/account switch во время ответа | Поздний ответ не меняет новую сессию или её UI |
| AUTH-05 | Чужая operation/sale identity вHTTP200 | Проекция отклонена, success не отображается |
| AUTH-06 | Отозванный operator/workstation | Новые monetary commands не допускаются |
| AUTH-07 | Параллельные владельцы одной команды | Только допустимый owner; late callback fenced |
| AUTH-08 | Неверный purpose/algorithm/key_id/signature | Отказ без fallback и расхода финансовых ресурсов |

## REST/AsyncAPI и сериализация

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| CON-01 | Каждый endpoint request/response | Соответствует effective reviewed OpenAPI, включая headers/parameters |
| CON-02 | Неизвестные поля/enum/body shape | Отказ по контракту; нет частичного исполнения |
| CON-03 | Idempotency-Key отсутствует/невалиден | Отказ в endpoint где обязателен |
| CON-04 | Stale If-Match/expected_version | Conflict без новой версии/устройства |
| CON-05 | Decimal/minor encoding | Стабильное точное представление без float |
| CON-06 | Canonical CBOR/JSON known vectors | Совпадает с независимым oracle byte-for-byte |
| CON-07 | Oversized/deep/duplicate-key input | Ограниченный отказ до dispatch |
| CON-08 | Недокументированный success/redirect | Не превращается в финансовый success |

## Денежный план

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| MONEY-01 | Cash/card/split валидные планы | Суммы tender точно равны ожидаемому итогу |
| MONEY-02 | Ноль/отрицательная/overflow сумма | Отказ по семантике endpoint до I/O |
| MONEY-03 | Fractional scale/rounding boundary | Явное правило currency, нет потери minor units |
| MONEY-04 | Discount/tax/items multiplication | Итоги проверены независимыми fixtures, нет overflow |
| MONEY-05 | Currency mismatch источника и Fiscal | Отказ/явно согласованный conversion flow; не переименование currency |
| MONEY-06 | Tax group вне1..8/unsupported profile | Отказ до vendor write |
| MONEY-07 | Card paymentCode не задан | Не угадывается код; actionable error |
| MONEY-08 | Изменение items после prepare | Запрет или новый допустимый lifecycle, старый intent не изменён |

## Идемпотентность и durable-before-device

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| IDEM-01 | Одинаковый command+payload повторён | Исходный результат, один внешний side effect |
| IDEM-02 | Одинаковый command+другой payload | Payload conflict, нет device I/O |
| IDEM-03 | Новая команда после UNKNOWN | Не разрешает повторный debit/receipt без reconcile |
| IDEM-04 | Рестарт после reservation до send | Устойчивая identity/UNP, безопасное продолжение |
| IDEM-05 | Рестарт после possible-send | UNKNOWN/recovery, не NOT_SENT по предположению |
| IDEM-06 | Persistence reserve/commit uncertain | Fail closed, не fallback на память |
| IDEM-07 | Два concurrent send одного operation | Один admitted executor и durable dedup |
| IDEM-08 | Reused ID другого tenant | Tenant fence и отдельная identity, без leakage |

## Payment saga и per-tender recovery

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| PAY-01 | Purchase Approved | Original reference/RRN/auth сохранены до следующего этапа |
| PAY-02 | Purchase Declined/NotSent | Не печатается оплаченный чек, нет лишнего reversal |
| PAY-03 | Purchase timeout/unknown | Нет второго purchase; query original identity |
| PAY-04 | Approved card + fiscal Committed | Один платёж, один подтверждённый чек |
| PAY-05 | Approved card + fiscal Unknown | Нет слепого reversal/повторной печати |
| PAY-06 | Approved card + proven fiscal rejection | Компенсация только исходной транзакции с правильной суммой |
| PAY-07 | Reversal Unknown | Не помечается COMPENSATED, дальнейший reconcile |
| PAY-08 | Split перваячасть approved, втораясбой | Первая не списывается повторно; checkpoints каждого tender |
| PAY-09 | Crash после каждого checkpoint | Recovery по persisted state, original references неизменны |
| PAY-10 | Reconcile с mismatched terminal/payment/amount | Отказ без перезаписи terminal evidence |

## Fiscal proof, storno и электронные чеки

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| REC-01 | Device close response validated | Reference связана с actual model/device/receipt/sale |
| REC-02 | Нет подлинного receipt proof | UNKNOWN; нет invented EDGE/RECOVERED номера |
| REC-03 | Readback чужого чека | Отказ; sale не становится fiscalized |
| REC-04 | Lost close ACK | Query original receipt без второго close по новомуID |
| REC-05 | Storno original references missing | Отказ до monetary I/O |
| REC-06 | Reversal replay | Одна операция возврата, оригинальная связь сохранена |
| REC-07 | Reprint | Не новая продажа/оплата |
| REC-08 | Electronic receipt unsupported | Явный unsupported до обещания электронного документа |
| REC-09 | Electronic artifact missing/corrupt/hash conflict | Нет фиктивной доставки; состояние сохранено для recovery |
| REC-10 | Electronic delivery retry/restart | Тот же artifact, tenant/recipient isolation, без повторной фискализации |

## S3 processor, storage, queue и ownership

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| FW-01 | Intent malformed/expired/wrong binding | Не попадает к драйверу |
| FW-02 | Journal reserve failure | Ноль device writes |
| FW-03 | SQLite/SD partial transaction/restart | Согласованная запись либо quarantine; нет потерянной authority |
| FW-04 | Queue full/duplicate dispatch | Ограниченный отказ без двойного executor |
| FW-05 | Callback после owner generation смены | Старый результат fenced |
| FW-06 | Outbox frozen batch replay | Те же bytes/hash/sequence до подтверждённого ACK |
| FW-07 | Sync partial/wrong ACK | Неподтверждённые события не удаляются |
| FW-08 | Storage power-loss по checkpoint | Сохранены dedup и per-tender certainty |
| FW-09 | Ошибка task/mutex/malloc | Безопасный stop, ресурс освобождён ровноодинраз |
| FW-10 | Долгий I/O | Не держит ненужные shared locks, deadline/yield ограничены |

## Provisioning, authority и время

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| TIME-01 | Pending/unapplied composite binding | Readiness false, monetary I/O запрещён |
| TIME-02 | Другая generation/rollback binding | Отказ; installed state не откатывается |
| TIME-03 | Provisioning package tampered/wrong device | Отказ без частичной установки secret |
| TIME-04 | NVS write/readback uncertain | Authority retired/quarantined, не объявлена INSTALLED |
| TIME-05 | Signed time purpose/signature/nonce неверны | Не создаёт trusted clock |
| TIME-06 | Time provider unavailable/unhealthy | Fail closed, без time.Now/SNTP fallback |
| TIME-07 | Reboot/suspend/backward monotonic | Authority invalidated по lifecycle |
| TIME-08 | Expired/revoked permit | Новая команда не допускается |
| TIME-09 | Late source health update/ABA | Epoch fence защищает последнюю publication |
| TIME-10 | Нет crypto entropy/key load failure | Нет нулевого/default рабочего ключа и секретов в логах |

## UART/USB/BLE/MQTT и vendor adapters

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| WIRE-01 | Unsupported vendor/channel | Отказ до открытия transport |
| WIRE-02 | Datecs/Daisy/Tremol valid frames | Проверяются настоящие bytes независимым parser/stateful peer |
| WIRE-03 | CRC/length/sequence повреждены | Нет ложного fiscal success; bounded resync |
| WIRE-04 | Short write/fragmented read | Правильные offsets и deadline, нет обрезки без ошибки |
| WIRE-05 | USB detach после possible send | UNKNOWN, не повтор под новым operation |
| WIRE-06 | BLE fragmented/reordered/duplicate frames | Сборка ограничена; replay и conflicts отклоняются |
| WIRE-07 | MTU минимальный/максимальный | Фрагменты соответствуют runtime negotiation, нет overflow |
| WIRE-08 | GATT write завершён, результата нет | Operation ещё pending/UNKNOWN, не Committed |
| WIRE-09 | MQTT duplicate/QoS reconnect | Durable dedup до device |
| WIRE-10 | MQTT foreign topic/device/publisher | Отказ до обработки; подписанный payload не заменяет transport ACL |

## WEB, Android, iOS и сохранение результата

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| UI-01 | Двойное нажатие оплаты | Один сохранённый command; busy не единственная защита |
| UI-02 | Reload/kill приложения во время оплаты | Реальная operation восстановлена, не создаётся новая |
| UI-03 | Поздний401 старой сессии | Не разлогинивает нового owner |
| UI-04 | Unknown операция | Понятный статус и проверка, нет предложения оплатить повторно |
| UI-05 | Route cloud→BLE после possible send | Запрет обхода reconcile |
| UI-06 | Payment success без fiscal proof | Не показывается окончательно fiscalized |
| UI-07 | Order amount/operation mismatch | Не применяется чужой cached success |
| UI-08 | Ошибки SDK/HTTP | Читаемое сообщение без JSON/stack/secrets |
| UI-09 | Management read/write logout race | Старый callback не меняет новую company |
| UI-10 | Результат после повторного открытия | Данные перечитаны с authoritative source, stale cache помечен |

## Backend persistence, webhook, accounting

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| BE-01 | Repository unavailable/uncertain commit | Все затронутые read/write paths fail closed |
| BE-02 | Transaction rollback | Нет половины sale/payment/outbox состояния |
| BE-03 | Webhook неверная HMAC/raw body/time | Ноль projection mutations |
| BE-04 | Webhook duplicate/out of order | Одна inbox запись и корректное version policy |
| BE-05 | Webhook tenant/system/source mismatch | Отказ без чужой mutation |
| BE-06 | Restart dispatcher после publish доACK | Один idempotent consumer result |
| BE-07 | Accounting event повторён | Одна проводка по source identity |
| BE-08 | Accounting currency/totals конфликт | Не проводится молча, observable failure |
| BE-09 | Reversal после posted sale | Сохраняется исходная связь и корректная компенсирующая запись |
| BE-10 | Backup/reload snapshot owner/version | Нет возвращения в stale financial success |

## Смены, отчёты, exports и безопасность

| ID | Вход / ситуация | Критерий PASS |
| --- | --- | --- |
| REPORT-01 | X report повтор | Нет закрытия смены вместо X |
| REPORT-02 | Z/shift close uncertain | Readback/reconcile, без слепого повторного закрытия |
| REPORT-03 | Cash in/out разные permissions | Права и точные суммы проверены до I/O |
| REPORT-04 | Export tenant/date/filter boundary | Только разрешённые записи; нет cross-tenant leakage |
| REPORT-05 | Audit hash predecessor mismatch | Corruption видна, не silently repaired |
| REPORT-06 | Retention legal hold | Удержанные записи/артефакты не удаляются |
| REPORT-07 | Logging raw input/secret/PAN/PIN | Forbidden fields не попадают в logs/artifacts |
| REPORT-08 | Stub/dev device profile в release | Не активируется через permissive config |
| REPORT-09 | Generated contract drift | Gate FAIL, а не автообновление reviewed lock |
| REPORT-10 | Test suite skipped/zero tests | Не принимается как PASS полного набора |

## Привязка к командам и критерий завершения

В существующем Makefile найдены contract-test, boundary-test, iot-test, iot-protocol-test, iot-build-check, iot-build, iot-build-verify, native-harness-test, native-cloud-confirmation-test, smart-device-test и regression. Это entrypoints для mapping, не заявление, что они уже покрывают все строки. Не запускать весь make regression вслепую: часть целей требует SDK, физической инфраструктуры или поднимает integration resources.

Исполнитель должен составить manifest ID → actual production path → suite/test → exact command → result/source fingerprint. Не существующую suite обозначить NOT IMPLEMENTED; не считать одноимённый fixture доказательством runtime. До признания unit-фазы завершённой все применимые строки имеют PASS, все exception paths проверены, skipped/xfail отсутствуют либо явно оставляют критерий незакрытым.

Существующая S3 C++17 host-проверка не доказывает runtime C++23/ESP-IDF; native key harness не доказывает готовый APK/IPA. Hardware transport, PSP approval, настоящий fiscal receipt, электронная доставка и полный demo accounting остаются отдельными E2E/HIL gates. Открытые electronic receipt и trusted-time требования нельзя исключать ради зелёного набора.
