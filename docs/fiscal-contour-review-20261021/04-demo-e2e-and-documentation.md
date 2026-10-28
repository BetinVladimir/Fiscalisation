# Demo E2E фискальных контуров и обновление документации

Дата: 2026-10-21. План, не отчёт о пройденных сценариях.

## 1. Среда

Все API/БД/broker/storage/source business/accounting работают на demo. WEB, Android и iOS — клиенты demo. Локально только клиентские симуляторы устройств и модели/AI workers. Существующие tests/e2e/full-fiscal и compose-e2e — dev/CI интеграция, а не доказательство выполнения на demo; не запускать их локальные backend/БД в рамках этой цели.

Использовать отдельные QA actor/tenant/branch/register/device и тестовые товарные данные. При обнаружении production routing/credentials или возможности реального банковского списания остановить только соответствующий monetary шаг и уточнить конфигурацию; не получать зелёный результат реальной оплатой. Поддержанный sandbox/stateful device peer маркировать SIMULATED_DEVICE, не REAL_DEVICE.

## 2. Очерёдность бизнесов

1. Рестораны: исходный заказ → приготовление/доставка → подтверждённая оплата → допустимая Fiscal sale/receipt связь → склад и бухгалтерия.
2. Фитнес: абонемент/услуга → продажа/оплата → fiscal result → посещение/возврат → учёт.
3. Beauty: запись → выполнение услуги → продажа/оплата → fiscal result → отмена/коррекция/расходники → учёт.
4. Retail, autoservice, property/custom и другие фактически поддержанные бизнесы: каждый источник сопоставить с применимым fiscal profile. Неприменимость доказать контрактом/продуктовым решением; нельзя придумывать чек для некоммерческого действия.

Для каждой цепочки: before snapshot, точные operation/source IDs, действия через UI, authoritative after snapshot, readback после перезагрузки, ошибки/recovery и отсутствие дублей.

## 3. Минимальная обязательная матрица каждого применимого бизнеса

| ID | Сценарий | WEB | Android | iOS | Итоговый критерий |
| --- | --- | --- | --- | --- | --- |
| DEM-01 | Enrollment/provisioning и правильный register | NOT RUN | NOT RUN | NOT RUN | Applied binding и actor scope подтверждены |
| DEM-02 | Cash sale | NOT RUN | NOT RUN | NOT RUN | Одна operation, один подлинный/sandbox-marked result, source связь |
| DEM-03 | Card Approved | NOT RUN | NOT RUN | NOT RUN | Один tender и fiscal result, суммы/reference совпали |
| DEM-04 | Card Declined/UNKNOWN | NOT RUN | NOT RUN | NOT RUN | Нет второго debit, верная certainty и recovery |
| DEM-05 | Split cash/card | NOT RUN | NOT RUN | NOT RUN | Per-tender сохранность, один итоговый чек |
| DEM-06 | Потеря ответа после commit | NOT RUN | NOT RUN | NOT RUN | Readback существующей операции, нет дубля |
| DEM-07 | Kill/reopen приложения | NOT RUN | NOT RUN | NOT RUN | Сохранённая identity и UI восстановления |
| DEM-08 | S3/device restart в критических точках | NOT RUN | NOT RUN | NOT RUN | Journal recovery и original references |
| DEM-09 | Storno/reversal и compensation | NOT RUN | NOT RUN | NOT RUN | Верная original-document связь, неизвестность не скрыта |
| DEM-10 | Shift/X/Z/cash movement | NOT RUN | NOT RUN | NOT RUN | Разные права и side effects, reload/reconcile |
| DEM-11 | Cloud/BLE/offline и возврат связи | NOT RUN | NOT RUN | NOT RUN | Нет дублирующего route, replay/expiry/revoke корректны |
| DEM-12 | Электронный чек | NOT RUN | NOT RUN | NOT RUN | Original artifact/hash/recipient delivery; gap не исключён |
| DEM-13 | Webhook/outbox/duplicate/out-of-order | NOT RUN | NOT RUN | NOT RUN | Один верный durable result, tenant isolation |
| DEM-14 | Склад/бухгалтерия исходного бизнеса | NOT RUN | NOT RUN | NOT RUN | Totals/tax/currency/postings сверены, не только Paid |
| DEM-15 | Роли/tenant/ошибки и визуал | NOT RUN | NOT RUN | NOT RUN | Чужой доступ запрещён; без rawJSON и опасного «повторить оплату» |

Серверную проверку, общую для платформ, можно переиспользовать при идентичной версии и scope, но UI/native transport каждого клиента проверяются отдельно. WEB BLE неподдержанного браузера — явный capability отказ, не незаметный cloud fallback после возможной отправки.

## 4. Артефакты и обновление документов

Каждый запуск сохраняет sanitized result.json: scenario ID, business, platform, environment, source/app/firmware/device profile versions, from/to states, expected/actual, certainty, timestamp, evidence paths и unresolved gaps. Не сохранять JWT/OTP/private keys/PAN/PIN/CVV, полный raw финансовый журнал или необезличенные чеки.

После каждого результата обновить:

1. Локальную карточку сценария и актуальную матрицу без перезаписи исторических runs.
2. Подробный GAP/T/UI register и тематический protocol/recovery документ.
3. README и IMPLEMENTATION_STATUS/MVP_GATES, если вывод меняет общий readiness.
4. Runtime/OpenAPI/AsyncAPI/generated/lock при изменении контракта — согласованным diff, не одним новым SHA.
5. Пользовательскую инструкцию бизнеса после закрытия цепочки; ограничения и роль сотрудника должны совпадать с фактом.
6. Общий deferred backlog `BeeloyBackend/docs/gaps/` для небыстрых исправлений; быстрый fix остаётся в активном E2E-журнале с rerun.

Статусы: SOURCE_PRESENT → проверка → VERIFIED_PASS/VERIFIED_FAIL. REPORTED_PASS не повышать без проверки текущего source и среды. Любое новое изменение source может инвалидировать предыдущую отметку — явно указать повторный gate.

## 5. Условия закрытия

Все применимые ячейки имеют актуальный PASS; исключения обоснованы и не выкидывают исходные требования. Не остаётся незакрытых обязательных contract/security/recovery/электронных/accounting разрывов. Работоспособность backend, simulator, demo, native artifacts и HIL указана раздельно. Внешняя нормативная/банковская приёмка не объявляется выполненной по software test; производственная публикация и проверки магазинов в текущую задачу не входят.
