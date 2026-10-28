# ARCH_13 — Статический анализ: minipos, AdminApp, BeeFiscalApp, erp-connector, public-api-core (POS-срез)

Дата: 2026-10-22 · Метод: чтение исходников (без запуска тестов), D-11 (DECISIONS.md) · Код не менялся.

## 1. Охват и честные границы
| Компонент | Что прочитано полностью | Что только просмотрено | Не читалось |
|---|---|---|---|
| `minipos/beeminipos-backend` (8 128 строк Go, 37 тест-файлов) | `internal/fiscalintegration/*` (client, operation, execution_authorization, payment_reconciliation); вебхук и dispatch в `internal/api` | `domain/service.go` (grep по UNKNOWN), сгенерированные контракты | остальной `domain/service.go` (2 341 стр.), `persistence/*`, `emailauth`, `mqttclient` |
| `minipos/BeeMiniPOS` (RN, 116 тест-файлов) | нативный модуль `beeloy-execution-crypto` (Kotlin+Swift) | метрики (any/console/storage) | `App.tsx` (2 092 стр.), UI |
| `minipos/miniposweb` | — | метрики | всё остальное |
| `AdminApp` (3 264 стр., 0 юнит-тестов) | `App.tsx` 1-175, `usePlatformOidc.ts` | `package.json` | UI-компоненты |
| `BeeFiscalApp` (17 714 стр., 87 тест-файлов) | `src/emailOtpAuth.ts`, `src/smartDeviceBle.native.ts` | `App.tsx` (grep по API/Idempotency) | экраны, e2e-спеки |
| `POS/erp-connector` (1 378 стр., 1 тест-файл) | `internal/orchestration/*`, `internal/consumer/runtime.go`, `transport/queue_defs.go` | `applier/pg.go` (не читался) | `cmd/erp/main.go` |
| `BeeloyBackend/public-api-core` (197 112 стр. — ERP целиком) | `internal/adminerp/networkmessaging/fiscal_results.go` | список файлов `fiscal_orchestration.go`, `fiscal_admin_facade.go` | всё, кроме POS/fiscal-среза |

Вывод по охвату: это **выборочный** анализ интеграционных границ с Fiscal, а не аудит каждого репозитория. Непрочитанное перечислено — под каждое задаётся ссылка в §4.

## 2. Что в порядке (проверено)
- Нативный модуль подписи: ключ неэкспортируемый, tombstone отзыва сохраняется **до** удаления ключа (`ExecutionKeyStore.kt:revoke`), проверка смены публичного ключа перед подписью, лимиты размера сообщения, сериализация операций.
- Вебхуки MiniPOS: HMAC-подпись с окном времени, сверка заголовков `BeeFiscal-Event-Id`/`Source-System-Id` с телом.
- Выпуск авторизации исполнения: сверка tenant/app_instance/ключа клиента с запросом.
- Файлы секретов (`AdminApp/store/*.jks`, `credentials*.json`, `BeeFiscalApp/credentials.json`, `BeeMiniPOS/credentials.json`) присутствуют на диске, **не отслеживаются git** (`.gitignore:56,59,62`). Содержимое в документы не копировалось. Рекомендация: пароли хранилищ ключей считать скомпрометированными, если рабочий каталог передавался третьим лицам или попадал в логи (ротация — задача владельца, D-7).

## 3. Реестр находок
| ID | Приор. | Компонент | Суть |
|---|---|---|---|
| MP-01 | P0 | minipos/beeminipos-backend | Диспатч подготовленной фискальной операции: (1) любая ошибка → HTTP 409 с текстом ошибки (`problem(w,409,err |
| MP-02 | P1 | minipos/beeminipos-backend | Статус вебхука отображается так: `SUCCEEDED`/`SUPERSEDED` → как есть, **всё остальное → `FAILED`** |
| MP-03 | P0 | minipos/beeminipos-backend | Гонка «вебхук обгоняет запись связи»: `mutateResource` сначала вызывает Fiscal (`c |
| MP-04 | P1 | minipos/beeminipos-backend | Сначала меняется состояние у Fiscal (`PATCH /integration/v1/binding`), и только потом локальная проверка верси |
| MP-05 | P1 | minipos/beeminipos-backend | В БД хранится `encryption_key_id`, но расшифровка всегда идёт единственным ключом из конфигурации; при ротации |
| MP-06 | P1 | minipos/beeminipos-backend | Проверяются `tenant_id`, `app_instance_id`, публичный ключ клиента внутри payload, но не проверяются: `sha256` |
| MP-07 | P1 | minipos (контракты) | Контракты MiniPOS допускают: `tax_group` `^[A-D]$` для строк (ответы допускают A–H); `quantity ^[0-9]+\ |
| MC-01 | P1 | minipos/BeeMiniPOS/modules/beeloy-execution-crypto | В проекте два формата подписи ECDSA: DER (мобильные клиенты → backend) и P1363 r‖s (устройства/BLE/MQTT) |
| AA-01 | P0 | AdminApp | Глобальный `crypto |
| AA-02 | P1 | AdminApp + BeeFiscalApp | Ключ идемпотентности генерируется заново при каждом вызове |
| AA-03 | P1 | AdminApp + BeeFiscalApp | Фолбэк `http://localhost:8080` при отсутствии `EXPO_PUBLIC_*`: релизная сборка без переменной тихо пойдёт на l |
| AA-04 | P2 | AdminApp | Нет юнит-тестов (только `e2e/wcag-aa |
| BF-01 | P0 | BeeFiscalApp | **Заглушка:** `authenticate()` отправляет `AUTH_V2` и всегда бросает `EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE`; |
| BF-02 | P1 | BeeFiscalApp | (1) Любая ошибка `refresh` (в т |
| EC-01 | P0 | POS/erp-connector | Семантика «не более одного раза» вместо заявленной «at-least-once»: строка `erp_fiscal_command_inbox` вставляе |
| EC-02 | P0 | POS/erp-connector | Нарушен принцип UNKNOWN ≠ FAILED: (1) `awaitFiscalOperation` возвращает `UNKNOWN` вместе с ошибкой при таймаут |
| EC-03 | P1 | POS/erp-connector | Сегменты пути берутся из сообщения без `url |
| EC-04 | P1 | POS/erp-connector | Токен, выданный при `…verify`, сохраняется «по возможности»: ошибка шифрования/БД игнорируется, неполный ответ |
| EC-05 | P1 | POS/erp-connector | Начальная сумма в кассе при открытии смены жёстко 0 |
| EC-06 | P2 | POS/erp-connector | Две схемы хэширования payload: сырые байты (consumer) и re-marshal через `any` (orchestration, public-api-core |
| EC-07 | P2 | POS/erp-connector | Тестовая команда обходит проверку хэша и выполняется в боевом коде; в prod это поверхность для обхода целостно |
| PA-01 | P1 | BeeloyBackend/public-api-core | Любая ошибка `applyFiscalResult` (включая постоянные: `RESULT_PAYLOAD_HASH_MISMATCH`, `INVALID_RESULT_PAYLOAD` |
| PA-02 | P1 | BeeloyBackend/public-api-core | Хэш результата необязателен: сообщение без `payloadSha256` принимается без проверки целостности (erp-connector |
| PA-03 | P0 | BeeloyBackend/public-api-core | Словарь статусов результата не нормализован: в общем пути любой статус пишется в `company_fiscal_operations |

## 4. Карточки

### MP-01 · P0 · minipos/beeminipos-backend
- **Место:** `internal/api/execution_authorization.go:162-165` (handler dispatch), `internal/fiscalintegration/execution_authorization.go:81-94` (`DispatchExecutionAuthorization`)
- **Проблема:** Диспатч подготовленной фискальной операции: (1) любая ошибка → HTTP 409 с текстом ошибки (`problem(w,409,err.Error())`), хотя рядом есть `writeExecutionUpstreamError`, различающий 401/403/5xx/сеть; таймаут 15 с (`http.Client.Timeout`) превращается в «409 conflict» и утечку текста; (2) запрос `POST …/execution-authorizations:dispatch` уходит **без** `Idempotency-Key` (`c.call(..., "", nil, …)`); при таймауте результат диспатча неизвестен, повтор клиента может отправить операцию дважды, если сервер не дедуплицирует по `authorization_id`.
- **Исправление:** 1) В обработчике заменить `problem(w,409,err.Error())` на `writeExecutionUpstreamError(w, err)`. 2) В `DispatchExecutionAuthorization` передавать `idempotency = "dispatch-"+authorizationID` (детерминированно). 3) Для сетевой ошибки/таймаута (не `*HTTPStatusError`) возвращать типизированную `ErrDispatchOutcomeUnknown`; обработчик отвечает 202 с `state:"UNKNOWN"` и `allowed_actions:["READ"]` (клиент опрашивает `GetFiscalOperation`), а не 4xx/5xx. 4) Проверить в fiscal-backend, что `:dispatch` дедуплицирует по `authorization_id` (`internal/domain/delegated_dispatch.go`).
- **Тесты (логика проверки):**
  - TestDispatchTimeoutReturnsUnknownNot409: Given fake Fiscal, отвечающий через 20 с (клиент timeout 15 с); When POST /minipos/execution-authorizations:dispatch; Then HTTP 202, body.state=="UNKNOWN", body не содержит текста Go-ошибки, order.State остаётся FISCAL_PENDING.
  - TestDispatchSendsDeterministicIdempotencyKey: Given fake Fiscal, запоминающий заголовки; When dispatch вызван дважды для одного authorization_id; Then оба запроса имеют Idempotency-Key=="dispatch-<id>".
  - TestDispatch5xxMapsTo503: Fiscal отвечает 502 → 503 "Fiscal issuer unavailable"; TestDispatch403MapsTo403.
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/api ./internal/fiscalintegration -run "Dispatch" -count=1`
- **Подзадачи:**
  - `MP-01.1` Тесты TestDispatch* красные (fake Fiscal на httptest) — файлы: `minipos/beeminipos-backend/internal/api/execution_authorization_dispatch_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/api -run Dispatch -count=1`
  - `MP-01.2` `DispatchExecutionAuthorization`: Idempotency-Key + ErrDispatchOutcomeUnknown (net.Error/ctx deadline) — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/execution_authorization.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run Dispatch -count=1`
  - `MP-01.3` Обработчик: writeExecutionUpstreamError + 202 UNKNOWN — файлы: `minipos/beeminipos-backend/internal/api/execution_authorization.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/api -run Dispatch -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-02 · P1 · minipos/beeminipos-backend
- **Место:** `internal/fiscalintegration/client.go:359-390` (`ProcessWebhook`, строки 370-373)
- **Проблема:** Статус вебхука отображается так: `SUCCEEDED`/`SUPERSEDED` → как есть, **всё остальное → `FAILED`**. Промежуточные статусы (`ACCEPTED`, `IN_PROGRESS`, `RETRYING`), если Fiscal их шлёт, помечают ресурс как `FAILED` с `last_error_code=FISCAL_OPERATION_FAILED`, и `ReadyForFiscalOperations` остаётся false.
- **Исправление:** Явная таблица: `SUCCEEDED→SUCCEEDED`, `SUPERSEDED→SUPERSEDED`, `FAILED|REJECTED→FAILED`, `ACCEPTED|IN_PROGRESS|RETRYING→ACCEPTED` (без изменения), неизвестный → 400 `UNKNOWN_WEBHOOK_STATUS` (не молчаливый FAILED). Список статусов взять из контракта Fiscal (`rg "integration.command.updated"` в `fiscal-backend/contracts`).
- **Тесты (логика проверки):**
  - TestWebhookNonTerminalStatusKeepsAccepted: Given link sync_status=ACCEPTED; When webhook status=IN_PROGRESS; Then sync_status остаётся ACCEPTED, last_error_code IS NULL.
  - TestWebhookUnknownStatusRejected: When status="WAT"; Then ProcessWebhook возвращает ошибку, строка не меняется.
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run Webhook -count=1`
- **Подзадачи:**
  - `MP-02.1` Вынести `mapWebhookStatus(status string) (syncStatus string, terminal bool, err error)` + таблица-тест — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/webhook_status.go`, `minipos/beeminipos-backend/internal/fiscalintegration/webhook_status_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run MapWebhookStatus -count=1`
  - `MP-02.2` Использовать в `ProcessWebhook`; не обновлять строку при non-terminal — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/client.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run Webhook -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-03 · P0 · minipos/beeminipos-backend
- **Место:** `internal/fiscalintegration/client.go:295-319` (`mutateResource`) и `:359-390` (`ProcessWebhook`)
- **Проблема:** Гонка «вебхук обгоняет запись связи»: `mutateResource` сначала вызывает Fiscal (`c.call`), и только после ответа вставляет `company_fiscal_resource_links … sync_status='ACCEPTED'`. Если вебхук `SUCCEEDED` придёт между ответом Fiscal и INSERT, `UPDATE … WHERE last_operation_id=$6` затронет 0 строк, но `ProcessWebhook` всё равно фиксирует inbox и возвращает 204 (событие «съедено»). Затем INSERT перезапишет статус на `ACCEPTED` навсегда → `ReadyForFiscalOperations` вечно false, касса не может фискализировать.
- **Исправление:** 1) Проверять `RowsAffected()` в `ProcessWebhook`; при 0 — не писать inbox и вернуть `ErrLinkNotFoundYet` → обработчик отвечает 409/503 (Fiscal повторит доставку). 2) В `mutateResource` вставлять строку `PENDING_SEND` до вызова Fiscal, затем обновлять `ACCEPTED` только `where sync_status='PENDING_SEND'` (не понижать `SUCCEEDED`). 3) `ON CONFLICT … DO UPDATE … WHERE sync_status <> 'SUCCEEDED' OR source_version < excluded.source_version`.
- **Тесты (логика проверки):**
  - TestWebhookBeforeLinkIsRetriedNotLost: Given нет строки link; When приходит вебхук SUCCEEDED; Then ProcessWebhook → ErrLinkNotFoundYet, inbox пуст.
  - TestLinkNeverDowngradedFromSucceeded: Given link SUCCEEDED version 3; When mutateResource завершает запись с version 3; Then sync_status остаётся SUCCEEDED.
  - TestConcurrentWebhookAndMutate (-race, 100 итераций): в любом порядке итог — SUCCEEDED.
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run "Webhook|Link" -race -count=1`
- **Подзадачи:**
  - `MP-03.1` Тест гонки с реальным Postgres (scripts/dossier-minipos-postgres-integration.sh) — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/webhook_race_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run WebhookBeforeLink -count=1`
  - `MP-03.2` `ProcessWebhook`: RowsAffected==0 → ErrLinkNotFoundYet — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/client.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run WebhookBeforeLink -count=1`
  - `MP-03.3` `mutateResource`: PENDING_SEND до вызова, не понижать SUCCEEDED — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/client.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run Link -race -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-04 · P1 · minipos/beeminipos-backend
- **Место:** `internal/fiscalintegration/client.go:321-357` (`UpdateSourceCompanyID`)
- **Проблема:** Сначала меняется состояние у Fiscal (`PATCH /integration/v1/binding`), и только потом локальная проверка версии/транзакция. При `binding version conflict` или падении `tx.Commit` Fiscal уже содержит `next`, а локальная запись — старое: расхождение источников истины.
- **Исправление:** Порядок: (1) локальная транзакция `SELECT … FOR UPDATE where version=$expected` → если нет строки, вернуть конфликт **до** вызова Fiscal; (2) вызвать Fiscal с `Idempotency-Key`; (3) `UPDATE`+history и commit; при падении на шаге 3 — запись в `minipos.binding_repair_queue` для повторного применения.
- **Тесты (логика проверки):**
  - TestUpdateSourceCompanyIDVersionConflictDoesNotCallFiscal: fake Fiscal считает вызовы; Given expected=2 при фактической 3; Then ошибка конфликта, вызовов 0.
  - TestUpdateSourceCompanyIDCommitFailureQueuesRepair: Given Commit возвращает ошибку (sqlmock); Then repair-запись создана.
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run UpdateSourceCompanyID -count=1`
- **Подзадачи:**
  - `MP-04.1` Тесты (sqlmock/pg) красные — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/update_source_company_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run UpdateSourceCompanyID -count=1`
  - `MP-04.2` Переставить шаги: lock→Fiscal→commit; repair queue — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/client.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run UpdateSourceCompanyID -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-05 · P1 · minipos/beeminipos-backend
- **Место:** `internal/fiscalintegration/client.go:249` (запись `encryption_key_id`), `:70-76` (`New`: единственный `aead`), `:98-107` (`decrypt`)
- **Проблема:** В БД хранится `encryption_key_id`, но расшифровка всегда идёт единственным ключом из конфигурации; при ротации ключа (D-7) все сохранённые токены станут нечитаемыми (`cipher: message authentication failed`), касса потеряет доступ к Fiscal.
- **Исправление:** Конфигурация: набор ключей `keyID→aead` (`MINIPOS_FISCAL_KEYS="k1=<b64>,k2=<b64>"`, активный — `MINIPOS_FISCAL_ACTIVE_KEY`); `decrypt(keyID, blob)`; при чтении с неактивным ключом — перешифровать (re-wrap).
- **Тесты (логика проверки):**
  - TestDecryptUsesStoredKeyID: Given токен зашифрован k1, активный k2; Then tenantCredential возвращает исходный токен.
  - TestRewrapOnRead: после чтения encryption_key_id==k2.
  - TestUnknownKeyIDFailsClosed.
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run "Key|Rewrap" -count=1`
- **Подзадачи:**
  - `MP-05.1` Тесты ротации — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/key_rotation_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run Key -count=1`
  - `MP-05.2` Реализовать keyring и decrypt(keyID,…) — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/client.go`, `minipos/beeminipos-backend/internal/config/config.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration ./internal/config -run Key -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-06 · P1 · minipos/beeminipos-backend
- **Место:** `internal/fiscalintegration/execution_authorization.go:36-58` (`IssueExecutionAuthorization`)
- **Проблема:** Проверяются `tenant_id`, `app_instance_id`, публичный ключ клиента внутри payload, но не проверяются: `sha256` конверта относительно `payload`, срок действия (`expires_at` в payload) и подпись. Подмена ответа на уровне прокси не обнаружится (полагаемся только на TLS).
- **Исправление:** Добавить: `sha256(rawPayload)==envelope.SHA256`; `expires_at > now+5s` и `≤ source_expires_at`; `Algorithm` из белого списка; подпись проверять публичным ключом `key_id` либо явно задокументировать «доверие = TLS» в ADR.
- **Тесты (логика проверки):**
  - TestIssueRejectsShaMismatch
  - TestIssueRejectsExpiredAuthorization (часы через интерфейс Clock)
  - TestIssueRejectsUnknownAlgorithm
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run IssueExecutionAuthorization -count=1`
- **Подзадачи:**
  - `MP-06.1` Тесты — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/issue_authorization_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run IssueExecutionAuthorization -count=1`
  - `MP-06.2` Реализовать проверки sha/exp/alg — файлы: `minipos/beeminipos-backend/internal/fiscalintegration/execution_authorization.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/fiscalintegration -run IssueExecutionAuthorization -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MP-07 · P1 · minipos (контракты)
- **Место:** `internal/api/response_contracts_gen.go` (схемы lines/payments), `BeeMiniPOS/src` (ввод), fiscal `contracts/`
- **Проблема:** Контракты MiniPOS допускают: `tax_group` `^[A-D]$` для строк (ответы допускают A–H); `quantity ^[0-9]+\.[0-9]{3}$` и `amount ^-?[0-9]+\.[0-9]{2}$` без верхних границ и допускают **отрицательные** цены; ФУ Datecs: Price ≤ 9 999 999.99, Quantity ≤ 999 999.999, Price×Quantity ≤ 9 999 999.99 (PM [*4], VG-06). Нарушение обнаружится только на ФУ **после** списания по карте.
- **Исправление:** Добавить в схему `maxLength`/диапазон (через `scripts/generate_response_contracts.rb` + источник контрактов), серверную валидацию `ValidateLine(price,qty)` до checkout; отрицательные суммы — только для скидок/сторно (явный флаг). Согласовать множество `tax_group` с Fiscal (A–D vs A–H) — D-9/VG-05.
- **Тесты (логика проверки):**
  - TestRejectsPriceTimesQuantityOverLimit: 5000000.00 × 3.000 → 422 LINE_AMOUNT_LIMIT до создания операции.
  - TestRejectsNegativeUnitPrice.
  - TestTaxGroupSetMatchesFiscal: множества совпадают (сравнение с contracts/).
- **Приёмка:** `cd minipos/beeminipos-backend && go test ./internal/domain ./internal/api -run "LineLimit|TaxGroup|NegativePrice" -count=1`
- **Подзадачи:**
  - `MP-07.1` Тесты — файлы: `minipos/beeminipos-backend/internal/domain/line_limits_test.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/domain -run LineLimit -count=1`
  - `MP-07.2` `ValidateLine` в domain и вызов из create/add/replace line + checkout — файлы: `minipos/beeminipos-backend/internal/domain/service.go`, `minipos/beeminipos-backend/internal/domain/line_limits.go`; проверка: `cd minipos/beeminipos-backend && go test ./internal/domain -run LineLimit -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### MC-01 · P1 · minipos/BeeMiniPOS/modules/beeloy-execution-crypto
- **Место:** `android/.../ExecutionKeyStore.kt:33-43`, `ios/ExecutionKeyStore.swift:68-77`; верификатор `fiscal-backend/internal/domain/execution_authorization.go:293` (`ecdsa.VerifyASN1`); устройства: `ecdsa_p1363.go`
- **Проблема:** В проекте два формата подписи ECDSA: DER (мобильные клиенты → backend) и P1363 r‖s (устройства/BLE/MQTT). Нет сквозного теста «подпись из Android Keystore / iOS проверяется backend». Ключ создаётся без атрибуции (`setAttestationChallenge`) и без `setIsStrongBoxBacked` (iOS — без Secure Enclave).
- **Исправление:** Векторные тесты: фикстура `fixtures/execution_signature_vectors.json` (message, spki, signature_der), сгенерированная на устройстве/эмуляторе; backend-тест проверяет её; instrumented-тест `ExecutionKeyStoreTest` сверяет подпись JVM-верификатором. Атрибуция ключа — P2 (решение владельца).
- **Тесты (логика проверки):**
  - TestBackendAcceptsMobileDerSignatureVector: Given вектор из JSON; Then VerifyASN1 true; TestBackendRejectsP1363ForMobilePath: подпись r‖s (64 байта) для мобильного пути → false.
  - ExecutionKeyStoreTest.signatureVerifiesWithJvmVerifier: Signature.getInstance("SHA256withECDSA").initVerify(pub) true.
- **Приёмка:** `cd fiscal-backend && go test ./internal/domain -run "MobileDerSignature|P1363ForMobile" -count=1`
- **Подзадачи:**
  - `MC-01.1` Фикстура векторов (сгенерировать инструментированным тестом, закоммитить JSON) — файлы: `minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json`; проверка: `jq -e "length>=3" minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json`
  - `MC-01.2` Go-тест на фикстуре — файлы: `fiscal-backend/internal/domain/execution_signature_vectors_test.go`; проверка: `cd fiscal-backend && go test ./internal/domain -run MobileDerSignature -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### AA-01 · P0 · AdminApp
- **Место:** `App.tsx:131,147,153,164` (и ещё вызовы) — `crypto.randomUUID()`; `package.json` — нет `expo-crypto`
- **Проблема:** Глобальный `crypto.randomUUID()` не гарантирован в Hermes/React Native (в BeeFiscalApp и BeeMiniPOS используется `expo-crypto`). В нативной сборке AdminApp создание внешней системы / смена состояния устройства может падать `TypeError: crypto.randomUUID is not a function` (проверить на устройстве; на web работает).
- **Исправление:** Добавить `expo-crypto` (версия как в BeeFiscalApp `~15.0.9`), модуль `src/idempotency.ts` с `newKey(): string` (`Crypto.randomUUID()`), заменить все вызовы; правило: запрет глобального `crypto.randomUUID` в AdminApp.
- **Тесты (логика проверки):**
  - idempotency.test.ts: Given global.crypto = undefined (jest); When newKey(); Then строка UUID v4 (regexp), два вызова различны.
  - Статический тест: `rg "crypto\.randomUUID" AdminApp --glob "!node_modules" --glob "!src/idempotency.ts"` → пусто.
- **Приёмка:** `cd AdminApp && npx tsc --noEmit && ! rg -n "crypto\.randomUUID" App.tsx src`
- **Подзадачи:**
  - `AA-01.1` Добавить expo-crypto и src/idempotency.ts + тест — файлы: `AdminApp/package.json`, `AdminApp/src/idempotency.ts`, `AdminApp/src/idempotency.test.ts`; проверка: `cd AdminApp && npx jest src/idempotency.test.ts`
  - `AA-01.2` Заменить вызовы в App.tsx — файлы: `AdminApp/App.tsx`; проверка: `cd AdminApp && ! rg -n "crypto\.randomUUID" App.tsx`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### AA-02 · P1 · AdminApp + BeeFiscalApp
- **Место:** `AdminApp/App.tsx:131-164` (ключ на каждый клик); `BeeFiscalApp/App.tsx:75` (`${Date.now()}-${Math.random()}-admin`)
- **Проблема:** Ключ идемпотентности генерируется заново при каждом вызове. Если запрос ушёл, а ответ потерян, повторное нажатие создаст **второй** ключ → дубль команды (rotate-key дважды, повторная привязка). У BeeFiscalApp ключ строится из `Math.random` (не криптостойкий).
- **Исправление:** Ключ привязан к «намерению»: хранится в `useRef`, пока операция не завершится успехом или не изменится payload; при retry — тот же ключ. Общий модуль `idempotentRequest.ts` в обоих приложениях.
- **Тесты (логика проверки):**
  - idempotentRequest.test.ts: Given fetch mock: 1-й вызов reject(NetworkError), 2-й resolve; When run(intent) дважды подряд; Then заголовки Idempotency-Key одинаковы; после успеха следующий run → новый ключ; изменённый payload → новый ключ.
- **Приёмка:** `cd AdminApp && npx jest src/idempotentRequest.test.ts && cd ../BeeFiscalApp && npx jest __tests__/idempotent-request.test.ts`
- **Подзадачи:**
  - `AA-02.1` Модуль с тестом (чистая функция + ref-обёртка) — файлы: `AdminApp/src/idempotentRequest.ts`, `AdminApp/src/idempotentRequest.test.ts`; проверка: `cd AdminApp && npx jest src/idempotentRequest.test.ts`
  - `AA-02.2` Подключить в AdminApp/App.tsx (все POST/PATCH) — файлы: `AdminApp/App.tsx`; проверка: `cd AdminApp && npx tsc --noEmit`
  - `AA-02.3` Подключить в BeeFiscalApp/App.tsx:75 — файлы: `BeeFiscalApp/App.tsx`, `BeeFiscalApp/__tests__/idempotent-request.test.ts`; проверка: `cd BeeFiscalApp && npx jest __tests__/idempotent-request.test.ts`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### AA-03 · P1 · AdminApp + BeeFiscalApp
- **Место:** `AdminApp/App.tsx:36`, `BeeFiscalApp/src/emailOtpAuth.ts:12`, `BeeFiscalApp/App.tsx:48`
- **Проблема:** Фолбэк `http://localhost:8080` при отсутствии `EXPO_PUBLIC_*`: релизная сборка без переменной тихо пойдёт на localhost (в OIDC https проверяется, для API — нет). Нет таймаута `fetch` (AdminApp — 0 `AbortController`).
- **Исправление:** Функция `resolveApiBase(env, isProd)`: в prod требует `https://`, иначе ошибка при старте (экран «Не настроено»); общий `fetchWithTimeout(url, init, ms=15000)` с `AbortController`; ошибки показывать кодом, не сырым телом.
- **Тесты (логика проверки):**
  - apiBase.test.ts: isProd=true, env="" → throws API_BASE_REQUIRED; env="http://x" → throws API_BASE_NOT_HTTPS; env="https://x/" → "https://x".
  - fetchWithTimeout.test.ts: fake timers 15 с → reject AbortError.
- **Приёмка:** `cd AdminApp && npx jest src/apiBase.test.ts src/fetchWithTimeout.test.ts`
- **Подзадачи:**
  - `AA-03.1` apiBase.ts + тест — файлы: `AdminApp/src/apiBase.ts`, `AdminApp/src/apiBase.test.ts`; проверка: `cd AdminApp && npx jest src/apiBase.test.ts`
  - `AA-03.2` fetchWithTimeout.ts + тест, подключить в api() — файлы: `AdminApp/src/fetchWithTimeout.ts`, `AdminApp/src/fetchWithTimeout.test.ts`, `AdminApp/App.tsx`; проверка: `cd AdminApp && npx jest src/fetchWithTimeout.test.ts`
  - `AA-03.3` Те же правки для BeeFiscalApp emailOtpAuth.ts и App.tsx — файлы: `BeeFiscalApp/src/emailOtpAuth.ts`, `BeeFiscalApp/App.tsx`; проверка: `cd BeeFiscalApp && npx jest __tests__/fiscal-authorization-current.test.ts`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### AA-04 · P2 · AdminApp
- **Место:** `AdminApp/` целиком
- **Проблема:** Нет юнит-тестов (только `e2e/wcag-aa.mjs`); `App.tsx` — плотные однострочные функции (стр. 150-156), 16 `useState`; `useEffect` с неполным списком зависимостей (`load`); `bootstrap_token` остаётся в состоянии, пока не закрыт вручную; `logout` лишь очищает токен (нет выхода на сервере).
- **Исправление:** Тесты логики: `api()` (401 → сброс токена), `transition()` (передаёт `version`, `tenant_id` по условию), `createSystem()` (токен показывается один раз и очищается через 60 с/при logout). Рефакторинг в `src/screens/*` и `useReducer` — без изменения поведения.
- **Тесты (логика проверки):**
  - App.logic.test.tsx (React Testing Library, fetch mock): сценарии Given/When/Then в ARCH_15 §AA.
- **Приёмка:** `cd AdminApp && npx jest`
- **Подзадачи:**
  - `AA-04.1` Настроить jest + RTL в AdminApp (jest.config, setup) — файлы: `AdminApp/package.json`, `AdminApp/jest.config.cjs`; проверка: `cd AdminApp && npx jest --listTests`
  - `AA-04.2` Тесты api()/transition()/createSystem() — файлы: `AdminApp/__tests__/api.test.tsx`; проверка: `cd AdminApp && npx jest __tests__/api.test.tsx`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### BF-01 · P0 · BeeFiscalApp
- **Место:** `src/smartDeviceBle.native.ts:62-92` (`connectEdgeDeviceV2`, `authenticate` стр. 80)
- **Проблема:** **Заглушка:** `authenticate()` отправляет `AUTH_V2` и всегда бросает `EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE`; `authorized` никогда не `true`; `setWifi`/`bindStore` — пустые. Пока это так, BLE-провижининг S3-агента из приложения невозможен. Дополнительно: нет таймаута на `found.connect()`/`discoverAllServicesAndCharacteristics`, MTU не согласуется, `BleManager` создаётся при импорте модуля, `found: any`.
- **Исправление:** Реализовать по протоколу прошивки (IoT/…): (1) `authenticate`: получить challenge (notify), подписать capability, отправить ответ, дождаться `AUTH_OK` (таймаут 10 с); (2) `setWifi(ssid,pass)`: шифровать сессионным ключом, фрагментировать по MTU; (3) `bindStore(token)`; (4) таймауты 15 с; (5) ленивая инициализация `BleManager`. Протокол — ТОЛЬКО из документации прошивки; при отсутствии — `BLOCKED_SPEC`.
- **Тесты (логика проверки):**
  - smartDeviceBle.native.test.ts (jest, mock react-native-ble-plx): Given устройство отвечает AUTH_OK; When authenticate(); Then resolves и setWifi() пишет N фрагментов ≤ MTU-3; Given AUTH_FAIL → throws EDGE_AUTH_REJECTED; Given нет ответа 10 с → EDGE_AUTH_TIMEOUT; connect не завершился 15 с → EDGE_CONNECT_TIMEOUT и cancelDeviceConnection вызван.
  - HIL: BeeFiscalApp ↔ реальный S3 (HIL-BLE-01).
- **Приёмка:** `cd BeeFiscalApp && npx jest __tests__/smart-device-ble-native.test.ts`
- **Подзадачи:**
  - `BF-01.1` Прочитать протокол прошивки и зафиксировать таблицей «команда → payload → ответ» в docs/MVP1/specs/ble_provisioning.md — файлы: `docs/MVP1/specs/ble_provisioning.md`; проверка: `test -f docs/MVP1/specs/ble_provisioning.md`
  - `BF-01.2` Тесты с mock ble-plx (красные) — файлы: `BeeFiscalApp/__tests__/smart-device-ble-native.test.ts`; проверка: `cd BeeFiscalApp && npx jest __tests__/smart-device-ble-native.test.ts`
  - `BF-01.3` Реализовать authenticate/setWifi/bindStore с таймаутами — файлы: `BeeFiscalApp/src/smartDeviceBle.native.ts`; проверка: `cd BeeFiscalApp && npx jest __tests__/smart-device-ble-native.test.ts`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### BF-02 · P1 · BeeFiscalApp
- **Место:** `src/emailOtpAuth.ts:98-104` (`refresh`), `:106-120` (boot), `:123` (таймер)
- **Проблема:** (1) Любая ошибка `refresh` (в т.ч. отсутствие сети) очищает сессию и удаляет её из SecureStore → разлогин при кратковременном обрыве; то же при старте приложения. (2) Таймер `Math.max(1000, expiresAt-now-60000)`: при `expires_in ≤ 61 с` refresh запускается каждую секунду (шторм). (3) `call()` бросает `HTTP <status>` и теряет тело/код ошибки сервера.
- **Исправление:** (1) Различать ошибки: 401/400 `invalid_grant` → сброс; сеть/5xx → оставить сессию, повтор с backoff (1→2→4…60 с). (2) минимальный интервал 30 с; (3) `ApiError{status, code, retryable}` из `application/problem+json`.
- **Тесты (логика проверки):**
  - emailOtpAuth.refresh.test.tsx: Given refresh → network error; Then session сохранена, повтор через 1 с (fake timers); Given 401 → session null, SecureStore.deleteItemAsync вызван.
  - Given expires_in=30 → следующий refresh не раньше чем через 30 с.
- **Приёмка:** `cd BeeFiscalApp && npx jest __tests__/email-otp-refresh.test.tsx`
- **Подзадачи:**
  - `BF-02.1` Тесты refresh (красные) — файлы: `BeeFiscalApp/__tests__/email-otp-refresh.test.tsx`; проверка: `cd BeeFiscalApp && npx jest __tests__/email-otp-refresh.test.tsx`
  - `BF-02.2` ApiError + разветвление refresh/boot + backoff — файлы: `BeeFiscalApp/src/emailOtpAuth.ts`, `BeeFiscalApp/src/apiError.ts`; проверка: `cd BeeFiscalApp && npx jest __tests__/email-otp-refresh.test.tsx`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-01 · P0 · POS/erp-connector
- **Место:** `POS/erp-connector/internal/orchestration/service.go:86-135` (`Handle`), вставка inbox на строке 111
- **Проблема:** Семантика «не более одного раза» вместо заявленной «at-least-once»: строка `erp_fiscal_command_inbox` вставляется и **коммитится до** вызова Fiscal. Если процесс упал (или `publishResult` вернул ошибку) после коммита и до записи результата, при повторной доставке `RowsAffected()==0` → `return nil` → сообщение подтверждено, **команда потеряна навсегда** (продажа не финализирована, смена не открыта). Результат тоже может не опубликоваться.
- **Исправление:** Конечный автомат inbox: колонки `status` (`RECEIVED→EXECUTING→DONE`), `attempt`, `locked_until`, `result_published`. Алгоритм: `INSERT … ON CONFLICT DO NOTHING`; затем `SELECT status … FOR UPDATE`: `DONE` → переопубликовать сохранённый результат (если `result_published=false`) и ack; `RECEIVED/EXECUTING` и `locked_until<now()` → захватить и выполнить (идемпотентность гарантирует `Idempotency-Key` в Fiscal); публикация результата — через outbox-таблицу с отдельным релеем.
- **Тесты (логика проверки):**
  - TestCrashAfterInboxInsertIsRetried: Given fake Fiscal; процесс «падает» (panic в execute) после INSERT; When то же сообщение доставлено повторно; Then Fiscal вызван ровно 1 раз с тем же Idempotency-Key, результат опубликован, inbox.status=DONE.
  - TestRedeliveryAfterDoneRepublishesOnlyIfNotPublished.
  - TestConcurrentDuplicateDeliveriesExecuteOnce (-race, 10 горутин).
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run "Crash|Redelivery|ConcurrentDuplicate" -race -count=1`
- **Подзадачи:**
  - `EC-01.1` Миграция inbox: status/attempt/locked_until/result_published; outbox-таблица — файлы: `POS/erp-connector/migrations/NNN_inbox_state_machine.sql`; проверка: `test -f POS/erp-connector/migrations/NNN_inbox_state_machine.sql`
  - `EC-01.2` Тесты (red) с интерфейсом inboxRepo (in-memory fake + pg) — файлы: `POS/erp-connector/internal/orchestration/service_crash_test.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Crash -count=1`
  - `EC-01.3` Вынести `inboxRepo` interface (Claim/MarkDone/ListUnpublished) и реализовать pg — файлы: `POS/erp-connector/internal/orchestration/inbox.go`; проверка: `cd POS/erp-connector && go vet ./internal/orchestration`
  - `EC-01.4` Переписать Handle на claim→execute→persist→publish — файлы: `POS/erp-connector/internal/orchestration/service.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -race -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-02 · P0 · POS/erp-connector
- **Место:** `service.go:123-130` (`Handle`: `if callErr != nil { status = "FAILED" … retryable:true }`), `fiscal_operation.go:17,20-22`, `service.go:246-249` (`client.Do` → `"FAILED"`)
- **Проблема:** Нарушен принцип UNKNOWN ≠ FAILED: (1) `awaitFiscalOperation` возвращает `UNKNOWN` вместе с ошибкой при таймауте/сетевой ошибке GET, но `Handle` перезаписывает `status = "FAILED"` и публикует `retryable:true`; (2) таймаут самого `POST …:finalize` (15 с) → `"FAILED"`; (3) цикл ожидания не ограничен (только `ctx`). ERP/e-commerce трактует FAILED как «чека нет» → может отменить/возместить заказ при реально выданном чеке или повторить продажу.
- **Исправление:** Тип `Outcome{Status, Err}`: `SUCCEEDED`; `FAILED` (только авторитетный отказ: HTTP 4xx бизнес-ошибка, состояние FAILED/CANCELLED от Fiscal); `UNKNOWN` (таймаут, сеть, 5xx, UNKNOWN от Fiscal) → публикуется `RECONCILIATION_REQUIRED` без `retryable`; внутренний опрос ограничить `awaitMax=120 с`, затем фоновый reconciler (`GET /operations/{id}` по расписанию).
- **Тесты (логика проверки):**
  - TestFinalizeTimeoutPublishesReconciliationRequired: Given fake Fiscal, POST :finalize зависает; Then опубликован status=="RECONCILIATION_REQUIRED", error.retryable==false.
  - TestFiscalUnknownStatePublishesReconciliationRequired.
  - TestFiscal4xxPublishesFailed.
  - TestAwaitStopsAfterMax: fake clock +121 с → UNKNOWN.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run "Outcome|Timeout|UnknownState|AwaitStops" -count=1`
- **Подзадачи:**
  - `EC-02.1` Тесты (red) — файлы: `POS/erp-connector/internal/orchestration/outcome_test.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Outcome -count=1`
  - `EC-02.2` `classify(resp, err) Outcome` — чистая функция + таблица-тест — файлы: `POS/erp-connector/internal/orchestration/outcome.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Classify -count=1`
  - `EC-02.3` Handle/execute/awaitFiscalOperation используют Outcome — файлы: `POS/erp-connector/internal/orchestration/service.go`, `POS/erp-connector/internal/orchestration/fiscal_operation.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-03 · P1 · POS/erp-connector
- **Место:** `service.go:159-186` (пути из `c.AggregateID`, `p.ResourceType`, `p.SourceEntityID`, `p.DeviceID`)
- **Проблема:** Сегменты пути берутся из сообщения без `url.PathEscape` и валидации: `AggregateID="../../integration/v1/resources:sync"` изменит целевой URL (подмена endpoint с системным токеном). Ошибка разбора `p` для `fiscal.resource.*` игнорируется (`_ =`) → путь с пустыми сегментами.
- **Исправление:** Валидатор `safeSegment(s)`: `^[A-Za-z0-9._:-]{1,128}$` и не `..`; сборка пути через `url.JoinPath` + `PathEscape`; обязательная проверка ошибок разбора.
- **Тесты (логика проверки):**
  - TestRejectsPathTraversalInAggregateID: "../x" → status FAILED (авторитетно, до сети), вызовов Fiscal 0.
  - TestResourceSyncRequiresTypeAndId.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run "PathTraversal|ResourceSyncRequires" -count=1`
- **Подзадачи:**
  - `EC-03.1` Тесты — файлы: `POS/erp-connector/internal/orchestration/path_test.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Path -count=1`
  - `EC-03.2` safeSegment + buildPath(commandType, c) и использование в execute — файлы: `POS/erp-connector/internal/orchestration/path.go`, `POS/erp-connector/internal/orchestration/service.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Path -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-04 · P1 · POS/erp-connector
- **Место:** `service.go:259` (`_ = s.commitCredential`), `:337-352` (`commitCredential`: молча `return nil` при неполном ответе)
- **Проблема:** Токен, выданный при `…verify`, сохраняется «по возможности»: ошибка шифрования/БД игнорируется, неполный ответ → `nil`. Операция помечается SUCCEEDED, хотя учётные данные не сохранены — последующие `fiscal.resource.*` упадут «credential inactive».
- **Исправление:** Возвращать ошибку; для `verify`-команд считать операцию успешной только после успешного `commitCredential`; повторная доставка не должна повторно вызывать `verify` (ответ кэшируется в inbox.result).
- **Тесты (логика проверки):**
  - TestVerifyFailsWhenCredentialNotSaved: pool возвращает ошибку INSERT → status != SUCCEEDED.
  - TestVerifyIncompleteResponseIsFailure.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run Credential -count=1`
- **Подзадачи:**
  - `EC-04.1` Тесты — файлы: `POS/erp-connector/internal/orchestration/credential_test.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Credential -count=1`
  - `EC-04.2` commitCredential возвращает ошибку; execute проверяет — файлы: `POS/erp-connector/internal/orchestration/service.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run Credential -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-05 · P1 · POS/erp-connector
- **Место:** `service.go:172` (shift open: `"opening_cash": {"amount":"0.00"}`)
- **Проблема:** Начальная сумма в кассе при открытии смены жёстко 0.00 EUR, значение из payload (`openingCash`) игнорируется → расхождение с фактическим размененным остатком, влияет на X/Z-отчёты.
- **Исправление:** Брать `p.OpeningCash` (строка `^[0-9]+\.[0-9]{2}$`), по умолчанию `0.00` только если поле отсутствует; невалидное → FAILED.
- **Тесты (логика проверки):**
  - TestShiftOpenUsesOpeningCash: payload openingCash="50.00" → тело запроса к Fiscal содержит "50.00".
  - TestShiftOpenRejectsBadAmount.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run ShiftOpen -count=1`
- **Подзадачи:**
  - `EC-05.1` Тест + правка — файлы: `POS/erp-connector/internal/orchestration/shift_open_test.go`, `POS/erp-connector/internal/orchestration/service.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run ShiftOpen -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-06 · P2 · POS/erp-connector
- **Место:** `POS/erp-connector/internal/consumer/runtime.go:34` (`verifyPayloadHash`: sha256 сырых байт `RawMessage`), `service.go:96-104` (sha256 канонической перемаршализации)
- **Проблема:** Две схемы хэширования payload: сырые байты (consumer) и re-marshal через `any` (orchestration, public-api-core). Re-marshal превращает числа в `float64`: `1.10`→`1.1`, `12345678901234567890`→`1.2345678901234567e+19` — хэш отправителя не совпадёт; сырые байты зависят от пробелов/порядка полей.
- **Исправление:** Единая схема: RFC 8785 (JCS) с `json.Number` (`decoder.UseNumber()`); общая функция `internal/common/canonjson.go`; документ в `docs/`; кросс-языковые векторы (Go/Kotlin/TS).
- **Тесты (логика проверки):**
  - TestCanonicalHashPreservesDecimalPrecision: {"a":1.10} → хэш канонической формы «{"a":1.10}».
  - TestCanonicalHashOrderIndependent.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/common -run Canon -count=1`
- **Подзадачи:**
  - `EC-06.1` canonjson + векторы — файлы: `POS/erp-connector/internal/common/canonjson.go`, `POS/erp-connector/internal/common/canonjson_test.go`; проверка: `cd POS/erp-connector && go test ./internal/common -run Canon -count=1`
  - `EC-06.2` Подключить в consumer и orchestration — файлы: `POS/erp-connector/internal/consumer/runtime.go`, `POS/erp-connector/internal/orchestration/service.go`; проверка: `cd POS/erp-connector && go test ./... -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### EC-07 · P2 · POS/erp-connector
- **Место:** `service.go:98-100,148-151` (тип `E2E_PROBE`)
- **Проблема:** Тестовая команда обходит проверку хэша и выполняется в боевом коде; в prod это поверхность для обхода целостности.
- **Исправление:** Включать `E2E_PROBE` только при `APP_ENV!="prod"` (конфиг), иначе `unsupported command`.
- **Тесты (логика проверки):**
  - TestE2EProbeDisabledInProd.
- **Приёмка:** `cd POS/erp-connector && go test ./internal/orchestration -run E2EProbe -count=1`
- **Подзадачи:**
  - `EC-07.1` Флаг AllowProbe в Config + тест — файлы: `POS/erp-connector/internal/orchestration/service.go`, `POS/erp-connector/internal/orchestration/probe_test.go`; проверка: `cd POS/erp-connector && go test ./internal/orchestration -run E2EProbe -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### PA-01 · P1 · BeeloyBackend/public-api-core
- **Место:** `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results.go:51-57`
- **Проблема:** Любая ошибка `applyFiscalResult` (включая постоянные: `RESULT_PAYLOAD_HASH_MISMATCH`, `INVALID_RESULT_PAYLOAD`, `INVALID_OPERATION_ID`) → `Nack(false, true)` = немедленный requeue → бесконечный цикл «ядовитого» сообщения, нагрузка и блокировка очереди (без DLX/лимита).
- **Исправление:** Тип `*resultError` (постоянная) → `Nack(false, false)` в DLX `beeloy.dlx`; транзиентные (БД) → retry-очередь с TTL и счётчик `x-death`, после N=5 → DLX. Метрика `fiscal_result_dead_total`.
- **Тесты (логика проверки):**
  - TestPermanentErrorGoesToDLXNotRequeued: Given payload с неверным хэшем; Then Nack(requeue=false) вызван, повторной доставки нет.
  - TestTransientErrorRequeuedWithDelay.
- **Приёмка:** `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run "AckDecision|FiscalResult" -count=1`
- **Подзадачи:**
  - `PA-01.1` Вынести `ackDecision(err error, xdeath int) decision` + таблица-тест — файлы: `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/ack_decision.go`, `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/ack_decision_test.go`; проверка: `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run AckDecision -count=1`
  - `PA-01.2` Использовать в consumeCanonicalResults — файлы: `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results.go`; проверка: `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### PA-02 · P1 · BeeloyBackend/public-api-core
- **Место:** `fiscal_results.go:81` (`if e.PayloadSHA256 != "" && …`)
- **Проблема:** Хэш результата необязателен: сообщение без `payloadSha256` принимается без проверки целостности (erp-connector всегда отправляет хэш, другой издатель — нет).
- **Исправление:** Требовать непустой хэш для всех типов, кроме явно перечисленных в allowlist (с причиной).
- **Тесты (логика проверки):**
  - TestResultWithoutHashRejected (permanent error).
- **Приёмка:** `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run ResultWithoutHash -count=1`
- **Подзадачи:**
  - `PA-02.1` Тест + правка — файлы: `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results.go`, `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results_hash_test.go`; проверка: `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run ResultWithoutHash -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

### PA-03 · P0 · BeeloyBackend/public-api-core
- **Место:** `fiscal_results.go:91-98,118-130` (`status` → UPDATE; e-commerce допускает SUCCEEDED/FAILED/RECONCILIATION_REQUIRED/CANCELLED)
- **Проблема:** Словарь статусов результата не нормализован: в общем пути любой статус пишется в `company_fiscal_operations.status` как есть; пустой → `FAILED`. Вместе с EC-02 (erp-connector шлёт FAILED вместо UNKNOWN) заказ e-commerce окажется FAILED при возможном существующем чеке. Монотонность не доказана: поздний `FAILED` может перезаписать `SUCCEEDED` (фактическое WHERE в UPDATE усечено в просмотре — проверить).
- **Исправление:** Единый enum `OperationResultStatus{SUCCEEDED,FAILED,RECONCILIATION_REQUIRED,CANCELLED}`; неизвестный → permanent error; переходы только вперёд (`RECONCILIATION_REQUIRED→SUCCEEDED|FAILED`, `SUCCEEDED` терминален); SQL `WHERE status <> 'SUCCEEDED'`.
- **Тесты (логика проверки):**
  - TestSucceededIsTerminal: Given SUCCEEDED; When приходит FAILED; Then статус остаётся SUCCEEDED.
  - TestReconciliationRequiredCanBecomeSucceeded.
  - TestUnknownStatusRejected.
- **Приёмка:** `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run "Terminal|Reconciliation|UnknownStatus" -count=1`
- **Подзадачи:**
  - `PA-03.1` Тесты + enum — файлы: `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/result_status.go`, `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/result_status_test.go`; проверка: `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -run Terminal -count=1`
  - `PA-03.2` UPDATE с монотонным WHERE — файлы: `BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_results.go`; проверка: `cd BeeloyBackend/public-api-core && go test ./internal/adminerp/networkmessaging -count=1`
- **DoD:** CDOD; тесты красные до правки и зелёные после; `git diff --stat` не затрагивает файлы вне списка подзадач.

## 5. Непрочитанное → задачи на дочитывание (статически)
| Задача | Что прочитать | Что искать | Статус |
|---|---|---|---|
| SCOPE-02 | `minipos/beeminipos-backend/internal/domain/service.go` (оставшиеся ~2 000 строк), `persistence/postgres.go` | переходы состояний заказа, идемпотентность, UNKNOWN→COMPLETED/REVERSED, денежная арифметика (округление D-4) | DONE → MP-08 |
| SCOPE-03 | `minipos/BeeMiniPOS/App.tsx`, `src/offline*` | очередь оффлайн-заказов, повторная отправка, ключи идемпотентности (см. AA-02), хранение токенов (SecureStore vs AsyncStorage: 5 файлов с AsyncStorage) | DONE → BF-03 |
| SCOPE-04 | `POS/erp-connector/internal/applier/pg.go`, `cmd/erp/main.go` | порядок ревизий (`revision`), потеря обновлений, ack/nack политика | DONE → EC-08 |
| SCOPE-05 | `BeeloyBackend/public-api-core/internal/handlers/routing/fiscal_orchestration.go`, `fiscal_admin_facade.go` | валидация входа, авторизация tenant, идемпотентность фискальных команд | DONE → PA-04 |
| SCOPE-06 | `BeeFiscalApp/App.tsx` (1 216 стр.), `src/screens/*` | дублирование ключей идемпотентности, обработка 409/UNKNOWN в UI | DONE → BF-04 |

Каждая SCOPE-задача завершается дополнением к этому документу (новые карточки в том же формате) и командой `rg -c "^### " docs/MVP1/ARCH_13_*.md` — число карточек не меньше прежнего + найденного.

## 6. Дочитанные карточки (SCOPE-02..06)

### MP-08 · P2 · minipos/beeminipos-backend (SCOPE-02)
- **Место:** `minipos/beeminipos-backend/internal/domain/service.go:771-775` (`normalizeTaxRate`)
- **Проблема:** `normalizeTaxRate` использует `strconv.ParseFloat` для нормализации процентной ставки НДС, затем `strconv.FormatFloat` с форматом `'f', 2, 64`. Ставки вида 5.5 могут пройти через binary float64 и вернуться как «5.50», но ставки с тремя знаками (8.500, которая в Bulgarian VATEX = 8,5%) могут получить ошибку точности. Нарушение инварианта 1 для данных ФУ: ставка — не денежная сумма, но хранится и сравнивается как строка, поэтому float-путь рискует несоответствием при последующей строковой сверке с ФУ.
- **Исправление:** Принимать строку «8.50» / «20.00» как-есть (regex `^[0-9]{1,3}\.[0-9]{2}$`), не преобразовывать через float.
- **Тесты (логика проверки):**
  - TestNormalizeTaxRateNoFloatRoundTrip: Given "8.50" → normalizeTaxRate → "8.50" (не "8.499999" и не "8.5").
  - TestNormalizeTaxRateRejectsNonNumeric: Given "abc" → error.

### BF-03 · P1 · minipos/BeeMiniPOS (SCOPE-03)
- **Место:** `minipos/BeeMiniPOS/App.tsx:400-402` (`NativeExecutionAuthority`, `NativeExecutionManager`)
- **Проблема:** `NativeExecutionManager` и `NativeExecutionAuthority` передаётся `AsyncStorage` для персистенции состояния фискальных исполнений. `AsyncStorage` не шифрован; на устройстве с root/jailbreak его содержимое доступно сторонним процессам. Состояние транзакции (ключи идемпотентности, результаты выполнения) хранится без дополнительного шифрования. Для фискально-критичных данных требуется `expo-secure-store` (KeyChain/Keystore-backed).
- **Исправление:** Переписать `NativeExecutionAuthority` (или добавить адаптер) на `SecureStore.getItemAsync`/`setItemAsync` для ключей и критичных состояний исполнения; оставить `AsyncStorage` только для UI-состояния.
- **Тесты (логика проверки):**
  - TestExecutionAuthorityUsesSecureStorage: мок `AsyncStorage` не вызывается при чтении/записи ключей исполнения; мок `SecureStore` вызывается.

### EC-08 · P2 · POS/erp-connector (SCOPE-04)
- **Место:** `POS/erp-connector/internal/applier/pg.go:25` (`Apply`), `pg.go:28` (`ApplyMessage`)
- **Проблема:** `Apply(ctx, tenant, entity, id, revision, payload)` делегирует в `ApplyMessage` с `messageID=""`, отключая inbox-дедупликацию (ветка `if messageID != ""`). При повторной доставке события без `messageID` одно и то же событие может быть применено дважды, если ревизия и данные те же (UPDATE при одинаковом `source_revision` — WHERE не выполнится, RowsAffected=0, но switch-ветка для employee/customer/loyalty выполнится снова). Аудит-trail и downstream triggers могут получить дублирующие вызовы.
- **Исправление:** Все вызовы из consumer-кода должны передавать `messageID`; либо добавить отдельную дедупликацию по `(tenant, entity, id, revision)` в `Apply`.
- **Тесты (логика проверки):**
  - TestApplyWithoutMessageIDDeduplicated: вызов `Apply` дважды с одинаковыми аргументами не должен вызывать `applyEmployee` дважды.

### PA-04 · P1 · BeeloyBackend/public-api-core (SCOPE-05)
- **Место:** `BeeloyBackend/public-api-core/internal/handlers/routing/fiscal_orchestration.go:154-187` (`retryCanonicalOperation`)
- **Проблема:** Ретрай генерирует `messageID = uuid.New()` — новый UUID для каждого ретрая. Outbox-event вставляется с новым `messageID`, а downstream-потребитель дедуплицирует по `messageID`, а не по `operationId`. Если два ретрая достигнут downstream, операция будет обработана дважды. Кроме того, UPDATE статуса на ACCEPTED и INSERT в outbox выполняются в разных транзакциях: при падении между ними операция перейдёт в ACCEPTED без события в очереди (потеряна).
- **Исправление:** (1) Выполнять UPDATE + outbox INSERT в одной транзакции. (2) Использовать детерминированный `messageID = uuid.NewSHA1(namespace, []byte(operationId.String()+"retry:"+retryCount))` или хранить `messageID` ретрая в `company_fiscal_operations`.
- **Тесты (логика проверки):**
  - TestRetryOperationIsAtomic: если outbox INSERT падает, статус остаётся FAILED (не ACCEPTED).
  - TestRetryMessageIDIsDeterministic: два ретрая одной операции не создают два сообщения с разными messageID в outbox.

### BF-04 · P2 · BeeFiscalApp (SCOPE-06)
- **Место:** `BeeFiscalApp/App.tsx:173` (`bleAppInstance`)
- **Проблема:** `bleAppInstance` инициализируется как `` `beefiscal-admin-${Date.now()}` `` через `useState`. При каждом hot-reload или пересоздании компонента в dev-сборке генерируется новый `appInstance`, что может аннулировать уже выданные авторизации исполнения, привязанные к старому `appInstance`. В production React Native компонент пересоздаётся при deep-link navigation или Activity recreate. Параметр `appInstance` передаётся как часть запроса авторизации в `NativeExecutionManager`; смена `appInstance` при активной смене будет означать отказ на шаге проверки `app_instance_id`.
- **Исправление:** Хранить `appInstanceId` в `SecureStore` (генерировать один раз при первом запуске, сохранять навсегда); не использовать `Date.now()` как часть идентификатора.
- **Тесты (логика проверки):**
  - TestBleAppInstancePersistsAcrossRemount: при двух монтированиях компонента `bleAppInstance` одинаков (из постоянного хранилища).
  - TestBleAppInstanceNotDateBased: `bleAppInstance` не содержит временной метки.

