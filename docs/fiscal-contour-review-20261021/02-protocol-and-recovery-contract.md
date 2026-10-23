# Фискальные контуры: обязательный протокол и восстановление

Дата: 2026-10-21. Статус: требования к проверке/доработке. Не новая версия wire API и не разрешение внешних денежных операций.

## 1. Источники истины

- [Единый протокол внешнего POS](../EXTERNAL_POS_INTEGRATION_PROTOCOL.md).
- [Public OpenAPI](../../../BeeloyBackend/docs/Fiscal/api/openapi-public-v1.yaml).
- [Runtime OpenAPI](../../contracts/openapi-runtime-v1.yaml).
- [Corrections overlay](../../contracts/openapi-corrections-v1.yaml).
- [AsyncAPI](../../../BeeloyBackend/docs/Fiscal/events/asyncapi-device-v1.yaml).
- [BLE GATT v1](../../../BeeloyBackend/docs/Fiscal/ble/ble-gatt-v1.md).
- [Local adapter](../../contracts/openapi-local-adapter-v1.yaml).
- [Vendor/model sources](../remediation/2026-10-17/03_DEVICE_PROTOCOLS.md).

Не создавать параллельную «упрощённую» финансовую схему для REST/BLE/MQTT. Изменение требует reviewed diff, версионирования/overlay по правилам проекта, generated клиентов и контрактных тестов. Текущий runtime lock расходится с файлами — см. FISC-DOC-001.

## 2. Границы полномочий

MiniPOS — отдельный продукт с собственной моделью заказа/смены/товаров. Его backend обращается к Fiscal через публичный контракт. UI не получает доступ к внутренней БД Fiscal и MQTT устройств. Нельзя копировать private Fiscal код в MiniPOS или восстанавливать удалённый Go edge-agent.

| Участок | Авторитетный результат | Чего недостаточно |
| --- | --- | --- |
| POS → Fiscal API | Принятая операция с проверенными owner/tenant/identity | Нажатие кнопки и любойHTTP2xx |
| Fiscal → S3 / SmartDevice | Подписанный intent с согласованным binding/policy | Клиентский JSON или выбранный UI route |
| Payment terminal | Подтверждённый банковский результат с исходными references | Timeout, локальный optimistic success |
| Fiscal device | Настоящее документное доказательство конкретного ФУ | Придуманная EDGE/RECOVERED ссылка |
| Device → sync → Fiscal | Durable verified result и ACK точного batch | Доставка MQTT packet без серверного commit |
| Fiscal → MiniPOS | Проверенная operation projection / подписанный webhook | Статус другого tenant/sale/operation |
| Fiscal → Accounting | Сверенные idempotent postings в demo | Только Paid или напечатанный чек |

## 3. REST и webhook

- Строго использовать публичный base `/public/v1` и версию, указанную reviewed контрактом; URL без credentials/query/fragment. Device/admin/runtime пути не подменяют публичную границу MiniPOS.
- Business mutation получает устойчивый operation/idempotency identity **до отправки**. Имя заголовка, UUID-профиль, сроки и конфликт payload брать из соответствующего endpoint, не угадывать.
- Одинаковая команда с тем же payload возвращает исходный результат; другой payload под тем же ключом — конфликт, без второго debit/receipt.
- `If-Match` / expected version, где требуется, проверяются до записи. Нельзя игнорировать конфликт и повторить под новым ID.
- Accepted/queued — не конечное финансовое подтверждение. Сверять operation, sale, tenant, register и client_operation_id в authoritative readback.
- Webhook проверяется по фактической raw-body подписи и timestamp/event binding. Inbox dedup переживает перезапуск. Неверная/просроченная/чужая подпись не меняет заказ.
- HTTP404 не доказывает NOT_SENT по другому маршруту. После возможной отправки нельзя переключаться CLOUD↔BLE и выдавать новую операцию без reconciliation.

## 4. Binding, readiness и полномочия

До monetary I/O должны быть подтверждены tenant/location/register/operator/workstation, device identity и нужная generation composite binding. Черновик или pending-apply не равны установленной конфигурации.

Execution authorization и административное reconciliation authorization — разные полномочия. Sale permission не даёт права объявить неизвестную банковскую операцию оплаченной. Проверять подпись, purpose, scope, version/generation, TTL и одноразовость/sequence по контракту.

Trusted time — отдельный verified источник полномочий; NTP/wall-clock и произвольное локальное время не заменяют подписанный time vertical. После suspend/reboot/отзыва/потери непрерывности монотонных часов authority инвалидируется по актуальному runtime policy. Не ослаблять gate ради прохождения demo.

## 5. S3 transport и протокол устройства

Основной код: `IoT/firmware/edge-agent-s3/idf`, ESP-IDF5.5.6 по текущему build lock. Ключи — NVS S3; внешний ATECC608 не добавляется.

- HTTP local-adapter, BLE и MQTT обязаны приводить к общей проверенной бизнес-команде; границы auth/framing различаются по соответствующим contracts.
- Канал клиента и канал к платёжному терминалу — отдельные sessions/lifecycle. UUID, MTU, framing/CBOR/crypto брать из reviewed GATT, не придумывать по datasheet терминала.
- Проверить sequence, total/max length, digest, canonical serialization, replay и таймауты сборки до исполнения. GATT write callback не означает выполненную фискальную операцию.
- UART/RS-232, USB CDC host и BLE GATT — разные физические интерфейсы. UART TTL не подключается прямо к RS-232; конкретные пины/питание/firmware фиксируются в HIL runbook.
- Vendor/channel facade обязан отвергать неподдержанную пару до wire write.
- DatecsPay payment и Datecs fiscal — разные протоколы. Модель BluePad и datasheet не разрешают угадывать payment TLV/UUID/банковские команды.
- Нельзя переносить Camera Evidence v1 на фискальный S3: камера и фискализация имеют разные contracts и purpose ключей.

## 6. Деньги и состояния

Денежные расчёты — точная decimal/minor arithmetic без float. Проверять overflow, rounding, discounts/tax и сумму tender до любого устройства. Currency/tax group/payment code задаются профилем, а не guessed default. Несовпадение валюты ресторанного источника и Fiscal нельзя «исправить» переименованием currency или неявной конвертацией.

Базовая семантика для тестов, а не новая wire enum:

- NOT_SENT: есть доказательство, что внешняя финансовая команда не отправлена.
- UNKNOWN: возможная отправка/результат не доказан; нужен readback/reconcile.
- COMMITTED: есть настоящий проверенный конечный результат соответствующего устройства и durable commit.
- COMPENSATED: доказана компенсация исходной операции, а не просто попытка reversal.

Точные enums брать из contracts/runtime. UI не должен сводить UNKNOWN к обычному «не удалось, оплатите ещё раз».

## 7. Card + fiscal и split

1. Проверить readiness, authority и точный план чека.
2. Устойчиво записать operation/reservation/per-tender checkpoints до monetary send.
3. Для карточного tender получить настоящий результат purchase с исходной identity.
4. При UNKNOWN остановить новые debits и перейти к query/reconciliation.
5. Только при подтверждённой оплате выполнить соответствующий fiscal plan.
6. Fiscal UNKNOWN не означает необходимость нового чека или немедленного reversal; сначала установить факт печати.
7. При доказанном неисполнении fiscal после подтверждённого purchase — выполнить предусмотренную компенсацию с оригинальной payment reference.
8. Компенсация тоже может быть UNKNOWN; сохранять это состояние до доказательства.
9. Для split хранить состояние каждого tender; нельзя повторять уже Approved часть после сбоя следующей.
10. При restart продолжить из durable journal, а не из пустого UI state.

Наличие простого `ReceiptSaga` helper не доказывает весь runtime: обязательно проверять actual intent processor → storage → profile executor → real vendor bytes.

## 8. Receipt, reversal и отчёты

- Fiscal reference должна связывать sale/operation/device/документ и проверяемое содержимое результата.
- Reversal/storno сохраняет исходные requisites; не генерировать случайную reference при отсутствии доказательства.
- X, Z, служебное внесение/вывод, reprint и возврат — разные команды с разными side effects и правами.
- Unknown после Z/close не разрешает повторный отчёт по новому ID без проверки device state.
- Электронный фискальный чек — оригинальный device-issued artifact с identity/hash/retention/access/delivery lifecycle. PDF-резюме UI и email-счёт не являются доказательством такого чека.
- Извлечение/отправка электронного чека после сбоя не должно создавать вторую продажу.

## 9. Ограничения приёмки

Unit mocks подтверждают ветви кода. Stateful protocol emulator подтверждает обработку bytes на software boundary. Demo подтверждает удалённые интеграции. Только HIL подтверждает конкретные аппараты/firmware/физический transport; только отдельные разрешённые sandbox/acquirer проверки подтверждают банковскую сторону. Ни один из этих уровней не заменяет внешний нормативный gate.
