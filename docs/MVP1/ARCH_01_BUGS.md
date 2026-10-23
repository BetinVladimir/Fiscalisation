# ARCH_01 — Очевидные баги

Формат: **ID · Sev · Status · Место · Проблема · Исправление (шаги) · Тест (красный→зелёный) · Приёмка (автоматическая) · DoD**.
Общий DoD (CDOD) — в [ARCH_00_INDEX.md](ARCH_00_INDEX.md). Status: `CONFIRMED` = подтверждено чтением кода; `TO VERIFY` = сначала написать тест.

| ID | Sev | Компонент | Кратко | Status |
|---|---|---|---|---|
| BUG-001 | P0 | fiscal-backend | `receiptMoneyCents` неверно парсит знак: `-0.50` → +50 | CONFIRMED |
| BUG-002 | P1 | fiscal-backend | `validMoney` пропускает `+`/`-` | CONFIRMED |
| BUG-003 | P0 | fiscal-backend | `Simulated:true` захардкожен для реальных операций | CONFIRMED |
| BUG-004 | P1 | fiscal-backend | `FiscalOperation`: «висящая» EXECUTING-операция и не тот driver | CONFIRMED |
| BUG-005 | P0 | fiscal-backend | Синхронизация часов ФУ по MQTT — фиктивна; UTC-дата вместо Europe/Sofia | CONFIRMED |
| BUG-006 | P1 | fiscal-backend | Сага чека: тупиковые состояния, in-memory, не используется | CONFIRMED |
| BUG-008 | P1 | fiscal-backend | `OriginalDocumentAt` = `UpdatedAt` операции, а не время фискального документа | CONFIRMED |
| BUG-009 | P0 | fiscal-backend | Конфиг: по умолчанию dev + stubs + CORS `*`; проверки только при `APP_ENV=="prod"` | CONFIRMED |
| BUG-010 | P1 | fiscal-backend | `/readyz` == `/livez`, зависимости не проверяются | CONFIRMED |
| BUG-011 | P1 | fiscal-backend | HMAC-токен принимается рядом с OIDC; нет iss/aud/nbf/max TTL | CONFIRMED |
| BUG-012 | P1 | fiscal-backend | 5xx «отравляет» Idempotency-Key | TO VERIFY |
| BUG-020 | P0 | BlueCash | Чек не отменяется после ошибки между cmd 48 и cmd 56 | CONFIRMED |
| BUG-021 | P0 | BlueCash | Карта сторнируется при UNKNOWN фискальном результате | CONFIRMED |
| BUG-022 | P0 | BlueCash | Валидация строк после списания по карте и открытия чека | CONFIRMED |
| BUG-023 | P1 | BlueCash/backend/POS | Три разных политики округления; дробное количество → отказ | CONFIRMED |
| BUG-024 | P1 | BlueCash | Повтор `SALE_FINALIZE` после сбоя → `SALE_ALREADY_EXISTS` вместо UNKNOWN | CONFIRMED |
| BUG-025 | P1 | BlueCash | `COMPENSATED`/`RECOVERY_REQUIRED` не считаются терминальными при повторе | CONFIRMED |
| BUG-026 | P0 | BlueCash | Выдуманный `fiscal_reference="DATECS-<op>"` | CONFIRMED |
| BUG-027 | P1 | BlueCash | `ok()` принимает пустой ответ ФУ как успех | CONFIRMED |
| BUG-028 | P0 | BlueCash | UNKNOWN определяется по 4 строкам сообщения; таймаут/IOException → FAILED | CONFIRMED |
| BUG-029 | P0 | BlueCash | Нет таймаута чтения, SYN, NAK-retransmit (GAP-011) | CONFIRMED |
| BUG-030 | P1 | BlueCash | `acknowledge` не сверяет hash с журналом | CONFIRMED |
| BUG-031 | P2 | BlueCash | `printerTest` оставляет нефискальный чек открытым при сбое | CONFIRMED |
| BUG-032 | P0 | BlueCash | Рантайм живёт в Activity; нет foreground service; пароль оператора в EditText | CONFIRMED |
| BUG-040 | P1 | S3 | «Устройство не готово» после резервирования → `RECOVERY_REQUIRED` без I/O | CONFIRMED |
| BUG-041 | P0 | S3 | `now()==0` при несинхронизированных часах не блокирует BLE/HTTP-продажи | CONFIRMED |
| BUG-042 | P1 | S3 | Только CASH/CARD из 8 типов оплаты | CONFIRMED |
| BUG-043 | P1 | S3 | `decode` не сверяет сумму строк с суммой оплат | CONFIRMED |
| BUG-044 | P2 | S3 | На `PayloadConflict`/ошибку decode клиент не получает ответа | CONFIRMED |
| BUG-050 | P0 | POS-app/gateway | `reconcile`: `:reconcile` vs `/reconcile` | CONFIRMED |
| BUG-051 | P0 | POS-app | Один `FISCAL` base URL для `/public/v1` (fiscal-backend) и `/api/v1` (шлюз) | CONFIRMED |
| BUG-052 | P1 | POS-app | Idempotency-Key для reconcile константный | CONFIRMED |
| BUG-053 | P0 | POS-app | `unp` подменяется `fiscalReference`; чек печатается с `fiscalRef=operationId` и пустым UNP | CONFIRMED |
| BUG-054 | P1 | POS-app | `await r.json()` до `r.ok`; общий таймаут 4 с для всех вызовов | CONFIRMED |
| BUG-055 | P0 | pos-gateway | Любой чек финализируется как единственный CASH, сумма по умолчанию `0.00` | CONFIRMED |
| BUG-056 | P1 | pos-gateway | НДС проекции чека `/1.2`; сумма из несуществующего поля | CONFIRMED |
| BUG-057 | P1 | pos-gateway | `Idempotency-Key` игнорируется; пустой `clientOperationId` → новый UUID | CONFIRMED |
| BUG-058 | P1 | pos-gateway | Проверка `AUTHORIZED_TEST` — fail-open при ошибке БД | CONFIRMED |
| BUG-059 | P1 | pos-gateway | Потребитель результатов: `Nack(requeue=true)` без DLX; подписан на командные ключи | CONFIRMED |
| BUG-060 | P0 | POS | `pos-app/credentials.json` отслеживается git | CONFIRMED |
| BUG-061 | P1 | POS-app | Сумма строки через `Number()*Number()` (float) | CONFIRMED |
| BUG-062 | P1 | backend/BlueCash | Несогласованные шаблоны УНП/ИН ФУ между компонентами | CONFIRMED |

> Снят после перепроверки: гипотеза «`edge_sync_v2.go` не проверяет налоговую политику» — **неверна** (проверка есть в `edge_sync_v2.go:163`).

---

## fiscal-backend

### BUG-001 · P0 · `receiptMoneyCents` неверно обрабатывает знак
- **Место:** `fiscal-backend/internal/domain/receipt_saga.go:96-111`; вызывается из `ValidateReceiptPlan` (`:66`) и `finalizeSaleForTenant` (`service.go:977`, сравнение `expected`).
- **Проблема:** `ParseInt("-0")=0`, затем `whole*100+fraction` → `"-0.50"` даёт `+50`; `"-1.50"` даёт `-50` (должно быть `-150`); `"1.-5"` даёт `95`. Проверка `cents <= 0` пропускает `-0.50`. Сумма оплат `[-0.50, 10.00]` совпадёт с итогом `10.50`, и отрицательный платёж уйдёт на устройство.
- **Исправление:**
  1. Открыть `receipt_saga.go`, функция `receiptMoneyCents`.
  2. Заменить тело на вызов `parseFixed(v, 2)` из `service.go:1501` **плюс** предварительную проверку регулярным выражением `^[0-9]+\.[0-9]{2}$` (определить `var moneyPattern = regexp.MustCompile(...)` в этом же файле).
  3. При несовпадении вернуть `errors.New("invalid money")`.
  4. Убедиться, что `ValidateReceiptPlan` и `finalizeSaleForTenant` используют обновлённую функцию (других вызовов нет: `grep -n receiptMoneyCents -r fiscal-backend`).
- **Тест:** создать `fiscal-backend/internal/domain/receipt_money_test.go`, табличный `TestReceiptMoneyCentsRejectsSignedAndMalformed`: входы `"-0.50"`, `"-1.50"`, `"+1.00"`, `"1.-5"`, `"1.5"`, `"01.00"`(допустить или нет — зафиксировать решение), `""`, `"1e2.00"` → ошибка; `"0.01"`→1, `"12.34"`→1234. Второй тест `TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly`: payments `-0.50`+`10.00`, total `10.50` → ошибка.
- **Приёмка (автоматическая):**
  - `cd fiscal-backend && go test ./internal/domain -run 'TestReceiptMoneyCents|TestValidateReceiptPlanRejectsNegative' -count=1` → `ok`, до правки — `FAIL`.
  - `grep -n 'ParseInt(parts\[0\]' fiscal-backend/internal/domain/receipt_saga.go` → пусто.
- **DoD:** CDOD + оба теста в CI (`make test`), нет других реализаций парсинга денег в пакете (`grep -rn 'Split(.*"\.")' fiscal-backend/internal/domain` показывает только `parseFixed`).

### BUG-002 · P1 · `validMoney` пропускает знак
- **Место:** `service.go:1489-1499`.
- **Проблема:** `ParseInt(strings.ReplaceAll(m.Amount,".",""))` принимает `"-5.00"`, `"+5.00"`. Позже `lineTotalCents` отсекает `p<0`, но `+5.00` нормализуется по-разному в хэшах/идемпотентности и `validMoney` используется в `edge_sync*.go`, `delegated_sale_prepare.go`.
- **Исправление:** (1) добавить `moneyPattern` (см. BUG-001, вынести в общий файл `money.go` пакета `domain`); (2) в `validMoney` — `m.Currency=="EUR" && moneyPattern.MatchString(m.Amount)`; (3) выделить длину ≤ 12 цифр до точки (защита от переполнения).
- **Тест:** `service_money_test.go`: `TestValidMoney` — валидные `0.00`,`10.99`; невалидные `-1.00`,`+1.00`,`1.0`,`1,00`,`１.００`(полноширинные цифры), `1234567890123.00`.
- **Приёмка:** `go test ./internal/domain -run TestValidMoney -count=1` → ok; `go test ./... -count=1` без новых падений.
- **DoD:** CDOD; `grep -rn "func validMoney\|func receiptMoneyCents" fiscal-backend` → по одному определению.

### BUG-003 · P0 · `Simulated:true` захардкожен
- **Место:** `service.go:261, 357, 1021, 1129, 1324`; `exports.go:105, 151`. Правильное значение только в `edge_sync.go:351` (`false`).
- **Проблема:** операции на реальном ФУ маркируются симулированными → искажение API-ответов (`simulated`), webhook, экспортов, аудита НАП; нельзя отличить HIL-прогон от симулятора.
- **Исправление:**
  1. Добавить в интерфейс `Driver` (файл `service.go`, рядом с `Simulator`) метод `IsSimulated() bool` **или** (меньше правок) функцию `driverIsSimulated(d Driver) bool { _, ok := d.(*Simulator); return ok }`.
  2. В пяти местах `Operation{...}` заменить `Simulated: true` на `Simulated: driverIsSimulated(driver)` (где `driver` — результат `s.boundDriver(...)`; в `CancelSale`/`exports` — `s.driver`).
  3. Для операций, завершённых по MQTT/edge-sync, оставить логику `edge_sync.go:351`.
- **Тест:** `TestOperationSimulatedFlagFollowsDriver`: сервис с `Simulator` → `op.Simulated==true`; сервис с фейковым non-Simulator `Driver` (stub, реализующий интерфейс) → `false`. Покрыть `FinalizeSaleForTenant`, `PayForTenant`, `ReverseForTenant`, `FiscalOperation`.
- **Приёмка:** `go test ./internal/domain -run TestOperationSimulatedFlagFollowsDriver -count=1` → ok. `grep -n 'Simulated: true' fiscal-backend/internal/domain/service.go` → пусто.
- **DoD:** CDOD; ответы OpenAPI не изменили схему (`make contract-test`).

### BUG-004 · P1 · `FiscalOperation`: осиротевшая операция и неверный driver
- **Место:** `service.go:306-375`.
- **Проблема:** (a) `reserveFiscalOperation` сохраняет операцию `EXECUTING` (`PutOperation`), а при ошибке `Prepare`/`PutResource` (`:317-322`) функция возвращает ошибку, операция остаётся `EXECUTING` навсегда (X/Z/CASH_IN зависают в списке). (b) `executeReservedFiscalOperation` вызывает `s.driver.Execute`, а не драйвер, найденный `boundDriver` для конкретного регистра.
- **Исправление:** (1) при ошибке после `PutOperation` перевести операцию в `FAILED` (`ErrorCode:"COMMAND_PREPARE_FAILED"`) через `s.repo.CommitOperationEvent`; (2) передавать выбранный `driver` в `executeReservedFiscalOperation(op, register, tenant, driver)`.
- **Тест:** `TestFiscalOperationPrepareFailureMarksOperationFailed` (stub durableQueuedDriver, `Prepare` возвращает ошибку → `repo.Operation(id).State=="FAILED"`); `TestFiscalOperationUsesBoundDriver` (два регистра, два stub-драйвера, проверка какой вызван).
- **Приёмка:** `go test ./internal/domain -run 'TestFiscalOperation' -count=1` ok.
- **DoD:** CDOD; список `GET /operations?state=EXECUTING` после сбоя Prepare пуст (интеграционный тест в `integration/`).

### BUG-005 · P0 · Фиктивная синхронизация часов ФУ
- **Место:** `fiscal-backend/internal/mqttclient/client.go:94-104` (`DeviceTime` возвращает `time.Now().UTC()`, `SetDeviceTime` не отправляет команд); `service.go:450-491` (`SyncWorkstationClock`, `HasDailyClockSync`).
- **Проблема:** (a) при MQTT-маршруте дрейф всегда 0, `Verified=true` — требование Приложения 29 п.5 (синхронизация времени ФУ при запуске / раз в рабочий день) не выполняется фактически; комментарий в коде признаёт это. (b) `SyncWorkstationClock` использует `s.driver`, а не драйвер регистра. (c) `business_date` и `HasDailyClockSync` считают по **UTC**; рабочий день в БГ — `Europe/Sofia` (UTC+2/+3): с 00:00 до 02:00/03:00 местного времени проверка «сегодняшней» синхронизации даёт ложный результат. (d) `HasDailyClockSync` перебирает все ресурсы типа на каждый платёж.
- **Исправление (шаги):**
  1. Добавить команду `DEVICE_TIME` (уже в `allowed` списка `reserveFiscalOperation`) как источник истины: `SyncWorkstationClock` ставит в очередь команду `DEVICE_TIME` (read) и ждёт результат с `device_time` и `drift_seconds` в `ProjectedResult`; при `|drift|>30 c` — вторая команда установки времени.
  2. `business_date` считать как `now.In(sofia).Format("2006-01-02")` (локация загружается один раз при инициализации; `time/tzdata` уже импортирован в `reversal_policy.go:6`).
  3. Хранить последний успешный sync в ресурсе с ключом `workstation|business_date` (O(1) чтение), а не перебирать все события.
  4. Пока шаг 1 не готов: `Bridge.DeviceTime` должен возвращать ошибку `DEVICE_CLOCK_NOT_MEASURED`, а readiness — `NOT_READY`, чтобы режим не выглядел «зелёным» (fail-closed).
- **Тест:** `TestClockSyncRejectsHostClockAsDeviceClock` (Bridge без реального ответа → ошибка); `TestHasDailyClockSyncUsesSofiaBusinessDate` (фиксированное время 2026-10-21T22:30Z = 23.10 01:30 EEST → дата `2026-10-22`... записать ожидание по Sofia); `TestSyncDriftOver30sTriggersSet`.
- **Приёмка:** `go test ./internal/domain ./internal/mqttclient -run 'ClockSync|DailyClock' -count=1` ok. HIL: тест `HIL-CLK-01` из ARCH_03 (сдвинуть часы ФУ на +2 мин → sync выставил → дрейф ≤30 с).
- **DoD:** CDOD + запись в `docs/SUPTO/…` статуса п.5: `SOFTWARE_VERIFIED_HIL_PENDING`.

### BUG-006 · P1 · Сага чека
- **Место:** `receipt_saga.go:143-150`; нигде не вызывается (`grep -rn ReserveReceiptSaga fiscal-backend --include=*.go | grep -v _test` → только определения).
- **Проблема:** (a) `ReceiptRecoveryRequired` не имеет исходящих переходов — состояние тупиковое, выход только ручной; (b) из `CardApproved`, `FiscalOpen`, `LinesRegistering`, `PaymentsRegistering` нет перехода в `RecoveryRequired` (сбой после неопределённого результата); (c) только in-memory, не персистентна; (d) мёртвый код создаёт ложное впечатление защиты.
- **Исправление:** принять решение (ADR): либо удалить (рекомендуется для MVP, т.к. логика сидит в BlueCash `BlueCashCommandProcessor` и S3 executor), либо довести: таблица переходов + Postgres-хранилище + использование в `finalizeSaleForTenant`. Для удаления: убрать файл и тесты, добавить ADR `docs/decisions/ADR-SAGA-001.md`.
- **Тест:** при сохранении — `TestReceiptSagaTransitionsTable` (каждое состояние имеет ≥1 исходящий переход, кроме `COMMITTED`/`COMPENSATED`; `RECOVERY_REQUIRED → COMMITTED|COMPENSATED|RECOVERY_REQUIRED`).
- **Приёмка:** `go test ./internal/domain -run ReceiptSaga -count=1` или, при удалении, `go build ./... && go vet ./...` ok и `grep -rn 'ReceiptSaga' fiscal-backend` пусто.
- **DoD:** CDOD + ADR.

### BUG-008 · P1 · Время исходного документа для сторно
- **Место:** `service.go:261` (`OriginalDocumentAt: original.UpdatedAt`).
- **Проблема:** `UpdatedAt` меняется при reconcile/повторных коммитах; Datecs cmd 43 требует точные дату-время исходного документа (`DatecsStornoDocument.documentDateTime`, формат `dd-mm-yy hh:mm:ss`, `DatecsProtocol.kt:132-135`). Несовпадение → отказ ФУ или некорректное сторно.
- **Исправление:** сохранять в `Operation` поле `FiscalDocumentAt` из результата ФУ при `FISCALIZED` (источник: ответ cmd 56/ФП-дата) и использовать его; если нет — отказ `ORIGINAL_DOCUMENT_TIME_UNAVAILABLE`, а не подстановка.
- **Тест:** `TestReverseUsesFiscalDocumentTimestamp`: операция с `UpdatedAt` на час позже `FiscalDocumentAt` → в команде сторно дата из `FiscalDocumentAt`.
- **Приёмка:** `go test ./internal/domain -run TestReverseUsesFiscalDocumentTimestamp -count=1` ok; контракт OpenAPI — аддитивное поле, `make contract-test`.
- **DoD:** CDOD + миграция БД (если новое поле в типизированной таблице) в `database/fiscal/`.

### BUG-009 · P0 · Небезопасные значения по умолчанию
- **Место:** `config/config.go:27,35,28,38-100`.
- **Проблема:** `APP_ENV` по умолчанию `dev`; `ALLOW_STUB_ADAPTERS` по умолчанию `true`; `CORS_ALLOWED_ORIGINS` по умолчанию `*`; **все** prod-проверки в `Validate()` включены только при `AppEnv == "prod"`. `APP_ENV=production|PROD|staging` отключает их. Забытая переменная → backend с симулятором и открытым CORS.
- **Исправление:** (1) разрешить только `dev|test|staging|prod`; иное — `log.Fatal`; (2) `ALLOW_STUB_ADAPTERS` по умолчанию `false`, в `dev` включать явным значением в `.env.example`; (3) для `staging` применять те же проверки, что для `prod`, кроме явного списка исключений; (4) при `EMQXBroker != ""` и `AllowStubAdapters` — ошибка.
- **Тест:** `config_test.go`: `TestValidateRejectsUnknownAppEnv`, `TestValidateProdChecksApplyToStaging`, `TestLoadDefaultsAreSafe` (пустое окружение → `AllowStubAdapters=false`).
- **Приёмка:** `go test ./internal/config -count=1` ok; `grep -n '"true"' fiscal-backend/internal/config/config.go` не содержит дефолта для stub.
- **DoD:** CDOD; обновлены `.env.example`, `compose.fiscalisation.dev.yaml` (явно `ALLOW_STUB_ADAPTERS=true`), `README`.

### BUG-010 · P1 · readiness
- **Место:** `fiscal-backend/internal/api/handler.go:146-148` — `/livez`, `/readyz`, `/healthz` → один `h.live`.
- **Исправление:** `/readyz` проверяет: доступность БД (ping с таймаутом 1 с), состояние `repositoryAvailability`, подключение MQTT-моста (если настроен), RabbitMQ. Возвращает 503 + JSON со списком компонентов.
- **Тест:** `TestReadyzFailsWhenRepositoryUnavailable`, `TestReadyzOkWhenAllUp`, `TestLivezAlwaysOk`.
- **Приёмка:** `go test ./internal/api -run Readyz -count=1` ok; `compose` healthcheck в `compose.fiscalisation.yaml` указывает на `/readyz`.
- **DoD:** CDOD.

### BUG-011 · P1 · Токены
- **Место:** `auth/auth.go:46-60` (fallback на HMAC, если OIDC не прошёл), `auth.go:Parse/validateClaims`.
- **Проблема:** в prod при настроенном OIDC любой, кто знает `AUTH_HMAC_KEY`, выпускает токен с ролью `ADMIN`; нет проверки `iss`, `aud`, `nbf`, максимального TTL, нет `kid`-ротации ключа HMAC.
- **Исправление:** (1) в prod HMAC допустим только для scope `minipos.*`/устройств и только с `iss==Config.LocalTokenIssuer`; роли `ADMIN`,`PLATFORM_*` из HMAC-токена запрещены; (2) `exp - iat ≤ 24h`; (3) проверка `nbf`.
- **Тест:** `TestHMACTokenCannotCarryAdminRoleInProd`, `TestTokenWithTTLOverLimitRejected`.
- **Приёмка:** `go test ./internal/auth -count=1` ok.
- **DoD:** CDOD + запись в `docs/` о выдаче токенов.

### BUG-012 · P1 · Идемпотентность и 5xx (TO VERIFY)
- **Место:** `fiscal-backend/internal/api/handler.go:530-600`, `fiscal-backend/internal/domain/repository.go:627` (`PutAmbiguousReplay`).
- **Гипотеза:** ответ ≥500 до побочных эффектов (например, 503 `repositoryAvailability`) сохраняется как `Pending` — повтор с тем же ключом всегда получает 5xx / `IDEMPOTENCY_IN_PROGRESS` (запись не освобождается). Клиент вынужден генерировать новый ключ, что нарушает принцип «повтор безопасен».
- **Шаги:** (1) написать тест `TestIdempotencyKeyReusableAfterPreSideEffect503`; (2) если красный — различать «ничего не выполнено» (освободить claim `DeleteReplayClaim`) и «исход неизвестен» (оставить Pending); (3) добавить заголовок `Retry-After`.
- **Приёмка:** тест зелёный; `go test ./internal/api -run Idempotency -count=1`.
- **DoD:** CDOD.

---

## BlueCash (Kotlin) — `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/`

### BUG-020 · P0 · Незакрытый фискальный чек
- **Место:** `BlueCashEngine.kt:217-300` (`sale`), `:305-375` (`reverse`).
- **Проблема:** после `execute(48)` (открытие чека) любое исключение (отказ строки cmd 49, платежа cmd 53, транспорт) уходит в `catch`, где **нет** команды отмены чека. Чек остаётся открытым на ФУ; следующая продажа получит ошибку «чек уже открыт», касса заблокирована до ручного вмешательства.
- **Исправление (шаги):**
  1. В `DatecsPayloads` добавить `cancelReceipt()` (команда отмены/аннулирования чека по протоколу v2.11.4 — **номер команды взять из PDF вендора и записать в константу с комментарием-ссылкой на раздел**; не угадывать).
  2. В `sale()`/`reverse()`/`printerTest()` ввести флаг `receiptOpened` (ставится после успешного cmd 48/43/38).
  3. В `catch`, если `receiptOpened && !unknown`, вызвать `runCatching { fiscal.execute(CANCEL) }` и записать в журнал `RECEIPT_CANCELLED` или `RECEIPT_CANCEL_FAILED`.
  4. Если `unknown` (см. BUG-028) — **не** отменять вслепую: сначала `lookup` состояния ФУ (cmd статуса) → решение о закрытии/отмене.
- **Тест:** `BlueCashEngineTest.kt` (уже существует, 223 строки): `saleCancelsOpenReceiptWhenLineRejected`, `saleCancelsOpenReceiptWhenPaymentRejected`, `saleDoesNotBlindCancelOnUnknown`. Фейковый `DatecsFiscalPort` записывает последовательность команд.
- **Приёмка:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest*'` → BUILD SUCCESSFUL; проверка в тесте: последовательность команд для отказа строки = `[48,49(fail),<CANCEL>]`.
- **DoD:** CDOD; HIL-тест `HIL-BC-05` (ARCH_03) выполнен или помечен ожидающим; GAP-011/BLOCKER Datecs зафиксированы ссылкой.

### BUG-021 · P0 · Сторно карты при неопределённом фискальном результате
- **Место:** `BlueCashEngine.kt:274-300` (`catch`: если `approvedCardPayments.isNotEmpty()` → компенсация независимо от `unknown`).
- **Проблема:** при `DATECS_EOF`/таймауте чек мог быть закрыт и фискализирован; карта сторнируется → клиент вернул деньги, а фискальный чек существует (расхождение «фискальный документ без оплаты»; нарушение собственного принципа «UNKNOWN ≠ NOT_SENT» из `70_FULL_PRE_HIL…`).
- **Исправление:** если `unknown` → **не компенсировать**; записать `RECOVERY_REQUIRED` с `card_state=APPROVED`; компенсировать только после определения результата ФУ lookup'ом (состояние `NOT_FISCALIZED` подтверждено номером документа/счётчиком ФП).
- **Тест:** `saleDoesNotReverseCardWhenFiscalResultUnknown` (fiscal.execute(56) бросает `DATECS_EOF`; `card.reverse` **не** вызван; результат `RECOVERY_REQUIRED`/`UNKNOWN`); `saleReversesCardWhenFiscalDefinitelyRejected` (ошибка `-111…` от ФУ → reverse вызван один раз).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest.sale*'` ok; в тесте `verify(card, never()).reverse(...)`-аналог счётчик=0.
- **DoD:** CDOD + раздел «Восстановление после UNKNOWN» в `docs/BLUECASH_POS_INTEGRATION.md` (процедура оператора).

### BUG-022 · P0 · Валидация после побочных эффектов
- **Место:** `DatecsProtocol.kt:87-88,106-130` (`money`, `quantity`, `line`); вызовы в `BlueCashEngine.kt` после `PAYMENT_APPROVED`.
- **Проблема:** `DatecsPayloads.line` бросает `IllegalArgumentException` (имя >72 символов, цена >99999.99, количество ≥100.000, налоговая группа вне A–H, отрицательные значения) **после** списания по карте и открытия чека. Количество ≥ 100 (например, 120 шт.) — обычный розничный кейс — всегда отказ.
- **Исправление:** (1) перед `journal.append(ACCEPTED)` собрать **все** payload'ы (`open`, `line`×N, `payment`×M) и провалидировать; ошибка → `FAILED` с `error_code=PAYLOAD_INVALID:<поле>` без I/O; (2) вынести лимиты в константы по документации модели ФУ; расширить `quantity` до допустимого протоколом диапазона (проверить в PDF; не угадывать); (3) `BlueCashComplianceIntent.line()` — валидировать `quantity` регэкспом `^[0-9]+(\.[0-9]{1,3})?$` до `BigDecimal(...)` (иначе `"1e3"` проходит).
- **Тест:** `salePrevalidatesAllPayloadsBeforeCardPurchase` (имя из 73 символов → `card.purchaseEur` не вызван, журнал без `PAYMENT_PREPARED`); `quantityScientificNotationRejected`; `quantity120Accepted` (если протокол допускает).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest*' --tests '*BlueCashComplianceIntentTest*'` ok.
- **DoD:** CDOD.

### BUG-023 · P1 · Единая политика округления
- **Место:** BlueCash `BlueCashComplianceIntent.kt:415-417` (`net()` — произведение без округления, строгое `compareTo==0` с платежом `:394,229-231`); backend `service.go:1518-1528` (`(p*q+500)/1000`, half-up); POS `LocalReceiptOperations.ts` (`roundHalfEven(String(Number(unitPrice)*Number(qty)),2)`), `domain/calculation/CalculationEngine.ts`.
- **Проблема:** цена 1.99 × 0.333 = 0.66267: backend 0.66; BlueCash сравнивает платёж 0.66 с 0.66267 → `PAYMENT_TOTAL_MISMATCH`; POS half-even может дать 0.66 или 0.67 на границах (x.xx5). Разные суммы между POS, backend и ФУ.
- **Исправление:** (1) ADR «Money rounding»: строка = `round_half_up(price_cents × qty_milli / 1000)`, скидка вычитается после округления (как в backend); (2) BlueCash `net()` и POS использовать целочисленную арифметику (`BigDecimal.setScale(2, RoundingMode.HALF_UP)`, в TS — `BigInt`/decimal-библиотека); (3) общий golden-файл `contracts/money-rounding-golden-v1.json` (≥40 векторов) и по тесту в каждом компоненте.
- **Тест:** `MoneyRoundingGoldenTest` (Go), `MoneyRoundingGoldenTest` (Kotlin), `moneyRounding.golden.test.ts` (POS) — читают один и тот же JSON.
- **Приёмка:** три команды зелёные; в golden есть векторы `1.99×0.333`, `0.01×0.500`, `0.01×0.499`, `99999.99×99.999`.
- **DoD:** CDOD + ADR + ссылка в `docs/EXTERNAL_POS_INTEGRATION_PROTOCOL.md`.

### BUG-024 · P1 · Повтор агрегата после сбоя
- **Место:** `BlueCashComplianceIntent.kt:243-251` (`require(current == null)`).
- **Проблема:** если процесс упал после `store.put(PAYMENT_PENDING)` (запись `processed[id]` с `result=null`) до физического результата, повтор той же команды доходит до `aggregate()` и падает `SALE_ALREADY_EXISTS` вместо повторного вызова `processor.sale`, который вернул бы `UNKNOWN/RECOVERY_REQUIRED` из журнала. Клиент получает бизнес-ошибку, а не безопасный UNKNOWN.
- **Исправление:** в начале `aggregate`: если `current != null && current.processed[id]?.hash == hash && result == null` → перейти к `processor.sale(...)` с сохранённым UNP (не выделять новый).
- **Тест:** `aggregateReplayAfterCrashReturnsRecoveryRequired` (симуляция: store содержит PENDING; journal содержит ACCEPTED+EXECUTING; повторный `execute` → `FISCAL_RESULT_UNKNOWN`, `unp.next` не вызван повторно).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashComplianceIntentTest*'` ok.
- **DoD:** CDOD.

### BUG-025 · P1 · Терминальные состояния при повторе
- **Место:** `BlueCashEngine.kt:157,231,(reverse ~320)`: набор `FISCALIZED/FAILED/UNKNOWN` не включает `COMPENSATED`, `RECOVERY_REQUIRED`, `REVERSED` (в sale).
- **Исправление:** вынести `private val TERMINAL = setOf(...)` и использовать во всех четырёх методах; `MainActivity.operationResult` (`:~290`) уже учитывает расширенный набор — привести к одной константе.
- **Тест:** `replayOfCompensatedSaleReturnsCompensated`, `replayOfRecoveryRequiredReturnsRecoveryRequired`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest*'` ok.
- **DoD:** CDOD.

### BUG-026 · P0 · Выдуманный фискальный референс
- **Место:** `BlueCashEngine.kt:269,361` (`?: "DATECS-" + operationId`); потребитель — `service.go:~250` (`strconv.ParseInt(original.FiscalReference)`, требует число ≤ 9999999).
- **Проблема:** если разбор ответа cmd 56 ничего не нашёл, в систему уходит «фискальный номер», которого нет. Сторно такой продажи невозможно; в отчётах НАП — фиктивный номер.
- **Исправление:** разобрать ответ cmd 56 по структуре из PDF (поле «номер документа»/FM-номер); при невозможности разбора — результат `UNKNOWN` с `error_code=FISCAL_REFERENCE_UNPARSEABLE` и принудительная сверка (cmd статуса последнего документа).
- **Тест:** `closeReceiptWithoutParsableDocumentNumberIsUnknown`; `closeReceiptParsesDocumentNumberFromVendorFixture` (фикстура из `tests/host/DatecsCodecVendorHost.kt`).
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest*'` ok; `grep -n '"DATECS-' BlueCashEngine.kt` пусто.
- **DoD:** CDOD.

### BUG-027 · P1 · `ok()` принимает пустой ответ
- **Место:** `BlueCashEngine.kt:377-384` (`text.isEmpty()` → успех).
- **Исправление:** пустой `data` = `DATECS_EMPTY_RESPONSE` (UNKNOWN). Если какая-то команда по протоколу возвращает пустой data при успехе, перечислить такие команды явным `setOf` и подтвердить ссылкой на PDF.
- **Тест:** `emptyDeviceResponseIsNotSuccess`.
- **Приёмка:** gradle-тест ok.
- **DoD:** CDOD.

### BUG-028 · P0 · Классификация UNKNOWN
- **Место:** `BlueCashEngine.kt:170,206,274,366`: `e.message in setOf("DATECS_EOF","DATECS_BAD_FRAME","DATECS_BCC","DATECS_CORRELATION")`.
- **Проблема:** сравнение по тексту. `IOException`/`SocketTimeoutException`/`DATECS_LENGTH`/`DATECS_SEPARATOR`/`DATECS_FRAME_TOO_LARGE`/`DATECS_HEX` после **отправки** команды трактуются как `FAILED` (определённый отказ) — хотя результат неизвестен.
- **Исправление:** ввести `sealed class DatecsFailure { class NotSent; class Rejected(code); class Unknown }` в `DatecsProtocol.kt`; транспорт бросает типизированные исключения; всё, что произошло после `output.flush()` и без разобранного ответа с кодом ошибки, = `Unknown`. `Rejected` только при корректно разобранном ответе с ненулевым кодом.
- **Тест:** параметризованный `everyTransportErrorAfterWriteIsUnknown` (список исключений), `parsedNonZeroErrorCodeIsRejected`.
- **Приёмка:** gradle-тест ok; `grep -n 'e.message in setOf' *.kt` пусто.
- **DoD:** CDOD.

### BUG-029 · P0 · Протокол Datecs: таймаут, SYN, NAK
- **Место:** `DatecsProtocol.kt:177-197` (`DatecsSocketProtocol.execute`); существующий `GAP-011` (`docs/remediation/2026-10-17/02_GAP_REGISTER.md:127`) — **подтверждён в коде**.
- **Проблема:** `input.read()` блокирующий без дедлайна внутри `@Synchronized` — зависание ФУ блокирует весь процессор (MQTT, BLE, HTTP). Байты `SYN(0x16)` «устройство занято» попадают в буфер кадра → `DATECS_BAD_FRAME` → UNKNOWN у занятой, но исправной кассы. `NAK(0x15)` не вызывает повторной отправки с тем же SEQ. Тайм-аут <500 мс на повтор по документации (см. `03_DEVICE_PROTOCOLS.md:24`).
- **Исправление (шаги, мелкие):**
  1. Обернуть чтение в `Future`/`withTimeout` через `ExecutorService`; общий дедлайн на команду из таблицы (X/Z — до 120 с; чек — 30 с; остальное 5 с).
  2. В цикле чтения: `0x16` → сбросить таймер ожидания, продолжить; `0x15` → повторить отправку **байт-в-байт** тот же кадр (SEQ не менять), максимум 3 раза; иначе `Unknown`.
  3. Игнорировать мусор до `SOH`; ограничить размер кадра.
  4. При дедлайне — закрыть сокет, бросить `DatecsFailure.Unknown`.
  5. `sequence` увеличивать только после успешного ответа; при resync — по правилу из PDF.
- **Тест:** `DatecsProtocolTest.kt` (есть): `synBytesDoNotCorruptFrame`, `nakTriggersByteIdenticalRetransmit`, `silentDeviceTimesOutWithUnknown`, `lateOldSequenceIgnored`, `concurrentCallersSerialized`. Peer — потоковый эмулятор с инжекцией задержек.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsProtocolTest*'` ok; время выполнения набора < 30 с; HIL `HIL-BC-02`.
- **DoD:** CDOD + закрыть GAP-011 ссылкой на SHA.

### BUG-030 · P1 · ACK журнала без сверки hash
- **Место:** `AndroidJournal.kt:157-195` (`acknowledge`); аналог `SignedMemoryJournal.acknowledge` (`BlueCashEngine.kt:~120`).
- **Проблема:** проверяется только формат `hash` (64 hex) и наличие `journal_seq`; не проверяется, что `hash == event_hash` строки `throughSequence`. Подложенный/ошибочный ACK продвигает чекпоинт и разрешает `purgeBefore` удалить неподтверждённые события.
- **Исправление:** `SELECT event_hash FROM journal_event WHERE journal_seq=?` и `require(stored == hash)`; плюс проверка цепочки `prev_hash` до `throughSequence` (разовая) при первом ACK после рестарта.
- **Тест:** `acknowledgeWithWrongHashRejected`, `acknowledgeDoesNotAdvanceCheckpointOnMismatch`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Journal*'` (создать `AndroidJournalTest` с Robolectric или вынести SQL-слой в JVM-тестируемый класс).
- **DoD:** CDOD.

### BUG-031 · P2 · Нефискальный чек при сбое
- **Место:** `BlueCashEngine.kt:181-215` (`printerTest`: 38 → 42 → 39). Сбой на 42 оставляет документ открытым.
- **Исправление:** в `catch` — `runCatching { fiscal.execute(39) }`. **Тест:** `printerTestClosesNonFiscalReceiptOnFailure`. **Приёмка:** gradle-тест ok.

### BUG-032 · P0 · Жизненный цикл Android-рантайма
- **Место:** `MainActivity.kt:174-300, 325-335`; `AndroidManifest.xml` (нет `FOREGROUND_SERVICE`, `RECEIVE_BOOT_COMPLETED`, `WAKE_LOCK`, `<service>`).
- **Проблема:** MQTT, BLE GATT, локальный HTTP (порт 8088) запускаются из Activity и останавливаются в `onDestroy` (смена конфигурации/выгрузка из памяти → касса «выпала» из сети посреди смены). После перезагрузки автозапуска нет. Пароль оператора ФУ вводится в обычный `EditText` (не `inputType=textPassword`), не сохраняется (после рестарта рантайм не поднимется без ручного ввода). Локальный HTTP — без TLS.
- **Исправление (шаги):** (1) `BlueCashRuntimeService : Service` (foreground, тип `connectedDevice`/`dataSync`, уведомление); (2) перенести `startManagedRuntime` в сервис; Activity — только UI активации; (3) `BootReceiver`; (4) пароль оператора — `EncryptedSharedPreferences`/Keystore-обёртка, ввод `textPassword`; (5) локальный HTTP — TLS с device-сертификатом или bind только на разрешённый интерфейс + документированный риск.
- **Тест:** Robolectric `RuntimeServiceLifecycleTest` (сервис стартует/рестартует, при уничтожении Activity MQTT не останавливается); инструментальный `BootReceiverTest`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*RuntimeService*'`; `aapt dump permissions` релизного APK содержит `FOREGROUND_SERVICE`, `RECEIVE_BOOT_COMPLETED`.
- **DoD:** CDOD + HIL `HIL-BC-07` (перезагрузка кассы → через ≤2 мин снова READY без касаний).

---

## S3 прошивка (`IoT/firmware/edge-agent-s3/idf/main/`)

### BUG-040 · P1 · Резервирование перед проверкой готовности
- **Место:** `intent_processor.cpp:420-445`. После `reserve_command` (UNP израсходован) при `!executor_.ready()` операция записывается как `RECOVERY_REQUIRED` без обращения к устройству.
- **Проблема:** повтор с тем же `operation_id` вернёт этот результат вечно (`ReservationStatus::Duplicate`); с новым — расходует ещё один UNP → дыры в нумерации, хотя денег/чека не было.
- **Исправление:** проверять `executor_.ready()` **до** `reserve_command`; если резервирование уже есть и I/O не было — состояние `REJECTED_BEFORE_IO` с возможностью повторить тем же `operation_id` и тем же UNP (по ADR-SUPTO-004).
- **Тест:** host-тест в существующем `idf/tests/receipt_saga_native.cpp` (запуск `idf/tests/run_native_saga.sh`): `device_unavailable_does_not_consume_unp`, `retry_after_unavailable_reuses_operation`.
- **Приёмка:** `make iot-test` → exit 0; тест присутствует в выводе.
- **DoD:** CDOD (C++).

### BUG-041 · P0 · Время `0`
- **Место:** `intent_processor.cpp:77-80` (`now()` возвращает 0, если часы раньше 2024-01-01), `:400-470` (`at=now()` без проверки для BLE/HTTP).
- **Проблема:** продажа по BLE/HTTP при несинхронизированных часах получит временную метку 0 в журнале и в фискальном сообщении; для MQTT есть `command_valid_now`, для локальных каналов — нет. Нарушает п.5 Приложения 29 (достоверное время).
- **Исправление:** в `execute`: `if (at == 0 || !time_anchor_trusted()) return ESP_ERR_INVALID_STATE` с кодом результата `TIME_NOT_TRUSTED` для денежных операций (SALE_*, CASH_*, REPORT_Z). Использовать `device_time_anchor.cpp`.
- **Тест:** `time_anchor_crypto_host` + новый `intent_requires_trusted_time` (host).
- **Приёмка:** `make iot-test` exit 0; тест `intent_requires_trusted_time` PASS.
- **DoD:** CDOD + HIL `HIL-S3-03` (отключить сеть/NTP → продажа отклонена с кодом).

### BUG-042 · P1 · Типы оплат
- **Место:** `intent_processor.cpp:388-392`. Разрешены только `CASH` и `CARD`; backend (`supportedPaymentType`, `receipt_saga.go:~90`), BlueCash (`datecsPaymentMode`) и scope MVP (`contracts/supto-feature-scope.json`) — 8 типов.
- **Исправление:** либо расширить S3 (CHEQUE, VOUCHER, DEFERRED, NHIF, INTERNAL_CONSUMPTION, COUPON с маппингом на режимы ФУ по профилю), либо запретить остальные **на backend** по возможностям профиля устройства (`capabilities`) и вернуть `PAYMENT_TYPE_UNSUPPORTED_BY_DEVICE` до отправки команды.
- **Тест:** контрактный `payment_types_capability_matrix_test` (Go) — для каждой пары (профиль, тип) ожидаемое решение; host-тест S3.
- **Приёмка:** `go test ./internal/domain -run PaymentTypeCapability -count=1` и `make iot-test`.
- **DoD:** CDOD + матрица в `docs/`.

### BUG-043 · P1 · Сверка строк и оплат
- **Место:** `intent_processor.cpp:396-398` (`return total > 0`).
- **Исправление:** вычислять сумму строк (с той же политикой округления, BUG-023) и требовать равенство сумме оплат (или `>=` при наличии наличных с «ресто» после GAPX-004).
- **Тест:** host `sale_total_mismatch_rejected_before_reservation`.
- **Приёмка:** `make iot-test`.

### BUG-044 · P2 · Молчаливые отказы
- **Место:** `intent_processor.cpp:431-432, 508-512` — на `PayloadConflict` и ошибке decode `result_sink_` не вызывается.
- **Исправление:** для `PayloadConflict` отправлять результат `IDEMPOTENCY_CONFLICT` по каналу приёма; для ошибок decode — `COMMAND_REJECTED` с кодом (без персистентной записи).
- **Тест:** host `payload_conflict_produces_reply`. **Приёмка:** `make iot-test`.

---

## POS ↔ шлюз (см. также [ARCH_05](ARCH_05_POS_INTEGRATION.md))

### BUG-050 · P0 · reconcile URL
- **Место:** `POS/pos-app/src/infrastructure/runtime/Adapters.ts:99` (`/api/v1/fiscal/operations/${id}:reconcile`) vs `POS/pos-gateway/cmd/gateway/main.go:227` (`POST …/operations/{operationId}/reconcile`).
- **Исправление:** в клиенте — `/api/v1/fiscal/operations/${encodeURIComponent(id)}/reconcile`; ключ идемпотентности см. BUG-052.
- **Тест:** `pos-app`: `HttpFiscalApiClient.test.ts` — `reconcileOperation` вызывает URL с `/reconcile` (mock fetch). Контрактный тест `tests/e2e/fiscal-integration/transport.spec.ts` расширить: POST `/reconcile` → не 404/405.
- **Приёмка:** `cd POS/pos-app && npx jest src/infrastructure/runtime -t reconcile` ok; `grep -n ':reconcile' src/infrastructure/runtime/Adapters.ts` пусто.
- **DoD:** CDOD (TS).

### BUG-051 · P0 · Один base URL для двух сервисов
- **Место:** `Adapters.ts:38-110` (`openSaleWithLine`, `addSaleLine`, `openShift`… → `/public/v1/*` fiscal-backend; `submitOperation`, `getOperationStatus`, `cashOperation` → `/api/v1/fiscal/*` шлюз); `POS/pos-app/src/infrastructure/di/container.impl.ts:70,358` (`FISCAL = EXPO_PUBLIC_FISCAL_URL ?? HTTP`).
- **Проблема:** один `base` не может быть корректным для обоих; в E2E (`tests/e2e/fiscal-integration/*.spec.ts`) используются раздельные `FISCAL_URL` и `POS_GATEWAY_URL`. Приложение работает только если обратный прокси объединяет оба префикса — это нигде не зафиксировано.
- **Исправление:** `HttpFiscalApiClient(fiscalBase, gatewayBase)`; методы `/public/v1/*` → `fiscalBase`; `/api/v1/*` → `gatewayBase`; конфиг `EXPO_PUBLIC_FISCAL_API_URL` и `EXPO_PUBLIC_GATEWAY_URL` валидируются при старте (https в prod).
- **Тест:** `HttpFiscalApiClient.routing.test.ts` — таблица «метод → базовый URL».
- **Приёмка:** `npx jest src/infrastructure/runtime -t routing` ok.
- **DoD:** CDOD + обновление `pos-app/README.md`, `.env.example`.

### BUG-052 · P1 · Ключ идемпотентности reconcile
- **Место:** `Adapters.ts:101` (`${id}-reconcile`).
- **Проблема:** backend сохраняет ответ по ключу; повторный reconcile после UNKNOWN вернёт **устаревший** кэшированный ответ.
- **Исправление:** `${id}-reconcile-${attemptNumber}` (счётчик в SQLite `fiscal_attempt`), либо UUID на попытку, хранимый до получения ответа.
- **Тест:** `reconcileUsesNewKeyPerAttempt`. **Приёмка:** jest ok.

### BUG-053 · P0 · UNP и фискальный референс
- **Место:** `Adapters.ts:230` (`unp: result.unp ?? result.fiscalReference ?? result.fiscal_reference`), `Adapters.ts:713` (`fiscalRef: result.unp ?? doc.fiscalRef`), `POS/pos-app/src/application/workflows/LocalReceiptOperations.ts:10` (`fiscalRef: result.operationId`, `unp: result.unp ?? ''`).
- **Проблема:** УНП и номер фискального документа — разные сущности. Подстановка искажает печатаемую информацию; чек без УНП (`''`) не должен выдаваться как фискальный (Приложение 29: УНП обязателен).
- **Исправление:** `FiscalStatusResponse` получает раздельные `unp`, `fiscalReference`, `fiscalNumber`; `deliver()` при пустом `unp` → не печатать как фискальный, поставить документ в `NEEDS_ATTENTION`; `receipt.fiscalRef = fiscalReference`.
- **Тест:** `normalizeStatus.test.ts`, `LocalReceiptOperations.test.ts`: пустой UNP → печать не вызывается, статус `NEEDS_ATTENTION`.
- **Приёмка:** jest ok.

### BUG-054 · P1 · Разбор ответа и таймаут
- **Место:** `Adapters.ts:255-266`.
- **Исправление:** `const text = await r.text(); const body = text ? JSON.parse(text) : null` с защитой; сначала `r.ok`/статус; таймауты по классам вызова (чтение 4 с; запись, меняющая состояние — 15 с; после таймаута записи статус **UNKNOWN** и обязательный `getOperationStatus` по тому же `operationId`, а не повтор записи с новым ключом).
- **Тест:** `request.nonJsonError.test.ts`, `request.timeoutOnWriteMarksUnknown.test.ts`.
- **Приёмка:** jest ok.

### BUG-055 · P0 · Шлюз финализирует всё как CASH
- **Место:** `POS/pos-gateway/internal/drain/idle.go:96-108`.
- **Проблема:** команда `fiscal.sale.finalize.v1` формируется с единственным платежом `type:"CASH"`, `amount` из `intent["amount"]` (по умолчанию `"0.00"`), `terminal_policy:"NONE"`, `payment_id=event.ID`. Реальные платежи POS (CARD, смешанные) теряются; `0.00` отклоняется валидацией (`cents<=0`). Любая карточная продажа будет проведена как наличная либо отклонена.
- **Исправление:** (1) `FiscalIntentPayload` (POS) уже несёт `payments`; шлюз передаёт их без изменений после валидации (типы из списка, суммы `^\d+\.\d{2}$`, сумма = ожидаемому итогу); (2) `payment_id` брать из платежа POS; (3) `terminal_policy` по типу; (4) `expected_total` из payload; отсутствие → 422 на приёме (`Submit`), а не `0.00` в outbox.
- **Тест:** Go `drain/idle_test.go`: `finalizeCommandCarriesAllPaymentsAndTypes`, `finalizeRejectedWhenAmountMissing`; E2E `TST-E2E-12` (ARCH_03).
- **Приёмка:** `cd POS/pos-gateway && go test ./internal/drain -run Finalize -count=1` ok; E2E сценарий card-sale в `tests/e2e/fiscal-integration` зелёный.
- **DoD:** CDOD (Go, POS).

### BUG-056 · P1 · Проекция чека в шлюзе
- **Место:** `fiscal_handler.go:128-137`.
- **Проблема:** `round($5/1.2,4)` — НДС 20% для всех (в БГ также 9% и 0%); `monetary.Amount` берётся из `payload.amount`, которого в `FiscalIntentPayload` нет → `0.00`; `lines` пустой.
- **Исправление:** суммы и разбивку НДС брать из `payload.totals`/`payload.lines` (POS уже считает по `taxSnapshot`); проекцию не вычислять по эвристике.
- **Тест:** `Submit_projectsTotalsFromPayload`, `Submit_doesNotAssume20PercentVAT` (Go + PG testcontainers или sqlmock).
- **Приёмка:** `go test ./internal/handler -run Submit -count=1` ok.

### BUG-057 · P1 · Idempotency-Key игнорируется
- **Место:** `fiscal_handler.go:61-90` (требуется заголовок, но `q.ClientOperationID` при пустом значении = `uuid.NewString()`, `:83`).
- **Исправление:** если `clientOperationId` пуст → взять из `Idempotency-Key`; если оба заданы и различаются → 422.
- **Тест:** `Submit_usesIdempotencyKeyAsOperationID`. **Приёмка:** `go test ./internal/handler -run Submit -count=1`.

### BUG-058 · P1 · Fail-open
- **Место:** `fiscal_handler.go:68`: `err == nil && installationMode == "AUTHORIZED_TEST"`; при ошибке БД (≠ `ErrNoRows`) проверка пропускается и фискальная печать разрешается в режиме тестовой установки.
- **Исправление:** `errors.Is(err, pgx.ErrNoRows)` → политика по умолчанию; любая иная ошибка → 503.
- **Тест:** `Submit_failsClosedOnPolicyLookupError`. **Приёмка:** go test ok.

### BUG-059 · P1 · Потребитель результатов
- **Место:** `POS/pos-gateway/internal/fiscal/result_consumer.go:25-45` (обменники: результаты — `beeloy.events` с ключом `fiscal.operation.result.#` (стр. 35); командные ключи `fiscal.sale.finalize.v1`, `fiscal.sale.reverse.v1`, `fiscal.shift.*.v1` — на **другом** обменнике `beeloy.erp.events` (стр. 38–44)); `:63-67` — `Nack(false, true)` на любую ошибку `apply`.
- **Проблема:** (a) очередь `pos-gateway.fiscal-results.v1` подписана на ключи `fiscal.sale.finalize.v1`, `fiscal.sale.reverse.v1`, `fiscal.shift.*.v1` — это **команды**, а не результаты; при совпадении обменников они приходят без `state` → `invalid fiscal result envelope` → `Nack(requeue=true)`; (b) `x-delivery-limit:20` без DLX — после 20 попыток сообщение теряется молча; (c) «ядовитое» сообщение крутится без паузы.
- **Исправление:** убрать командные биндинги или обосновать (ADR); объявить DLX/DLQ `pos-gateway.fiscal-results.dlq`; для невалидных конвертов — `Nack(requeue=false)` + метрика; для временных ошибок БД — requeue с задержкой.
- **Тест:** `result_consumer_test.go`: `invalidEnvelopeGoesToDLQNotRequeue`; интеграция с RabbitMQ testcontainer.
- **Приёмка:** go test ok; в RabbitMQ management API очередь `…dlq` существует (скрипт `scripts/check-rabbit-topology.sh`).

### BUG-060 · P0 · Секреты в git
- **Место:** `POS/pos-app/credentials.json` — `git ls-files` его показывает; содержит `android.keystore.keystorePath/keystorePassword/keyAlias/keyPassword`. В `POS/.gitignore` нет `credentials.json` (в Fiscalisation есть `**/credentials.json`).
- **Исправление:** (1) `git rm --cached pos-app/credentials.json`; (2) добавить `**/credentials.json` в `.gitignore`; (3) **сменить пароли keystore** (считать скомпрометированными; репозиторий — проверить историю `git log --all -- pos-app/credentials.json`); (4) вынести в EAS secrets; (5) `scripts/check-no-secrets.sh` в CI (gitleaks).
- **Приёмка (автоматическая):** `git ls-files | grep -E 'credentials\.json|\.jks$|\.keystore$'` → пусто; `gitleaks detect --no-git=false` → 0 findings (добавить job).
- **DoD:** CDOD + запись о ротации (без значений) в `docs/`.

### BUG-061 · P1 · Float в сумме строки POS
- **Место:** `LocalReceiptOperations.ts:10` (`String(Number(line.unitPrice)*Number(line.qty))`); `domain/calculation/CalculationEngine.ts`, `shared/utils/decimal.ts`.
- **Исправление:** только `decimal.ts` (целочисленная арифметика); запретить `Number(` для денег линтом (`no-restricted-syntax` для выражений `Number(*price*|*amount*)`).
- **Тест:** golden из BUG-023.
- **Приёмка:** `npx eslint src --rule` без нарушений; jest ok.

---

## Дополнение по результатам ARCH_08

### BUG-062 · P1 · Несогласованные шаблоны УНП и ИН ФУ
- **Место:** `fiscal-backend/internal/domain/regulatory_identifier.go` (`bgFMINPattern = ^[A-Za-z0-9]{8}$`, `bgUNPPattern`), `SmartDevices/.../DatecsProtocol.kt:101` (`unp.length in 8..64` при открытии чека) и `:152` (`originalUnp` — `[A-Z]{2}[0-9]{6}-[A-Za-z0-9]{4}-[0-9]{7}`).
- **Проблема:** backend принимает ИН ФУ из любых 8 буквенно-цифровых символов (в т.ч. строчных и «12345678»), а сторно в BlueCash требует 2 заглавные буквы + 6 цифр — продажа проходит, сторно позже отвергается. При открытии чека длина УНП допускает 8–64 символа, тогда как корректный УНП всегда 21 символ; повреждённое значение уходит на ФУ.
- **Исправление:** (1) решить формат ИН ФУ (источник — Приложение 29 п.9: «8-разряден»; фактический формат производителя подтвердить по ФУ — NORM/HIL); (2) единая константа/регулярное выражение в `contracts/` (golden-вектора допустимых/недопустимых УНП); (3) в `DatecsPayloads.open` проверять полным шаблоном (21 символ, разделители «-»); (4) та же проверка в S3 (`intent_processor.cpp`).
- **Тест:** `contracts/bg-unp-v1-golden.json` (существует) использовать во всех трёх компонентах: `TestUNPGoldenVectors` (Go), `unpGoldenVectors` (Kotlin), host-тест S3; негативные: строчные буквы, длина 20/22, `_` вместо `-`.
- **Приёмка:** `go test ./internal/domain -run UNPGolden -count=1`; `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Unp*'`; `make iot-test`.
- **DoD:** CDOD.

## Баги, найденные при сверке с вендорной спецификацией (2026-10-22)
Подробные карточки — в [ARCH_09](ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md) (тот же формат: место, шаги, тесты, приёмка, DoD). Здесь — реестр, чтобы приоритеты были в одном месте.

| ID | Sev | Компонент | Кратко | Карточка |
|---|---|---|---|---|
| BUG-063 | P0 | BlueCash/IoT/Daisy | Текст на ФУ отправляется в UTF-8, ФУ ожидает windows-1251 (кириллица искажается) | VG-01 |
| BUG-064 | P0 | BlueCash | Статус-байты не читаются; пустой ответ считается успехом (расширяет BUG-027); коды ошибок без классификации | VG-02 |
| BUG-065 | P0 | BlueCash | SEQ после рестарта снова `0x20`; ФУ повторяет кэш ответа и **не выполняет команду** (тихая потеря строки/оплаты) | VG-03 |
| BUG-066 | P0 | BlueCash | Нет чтения состояния ФУ (cmd 74/76/103, параметр `UNP`): UNKNOWN не разрешается, висящий чек не снимается (расширяет BUG-020/028) | VG-04 |
| BUG-067 | P1 | BlueCash | Типы оплаты → PaidMode неверны (CHEQUE→«ДЕБ.КАРТА» и др.); ответ cmd 53 (`D`/`R`) игнорируется | VG-05 |
| BUG-068 | P1 | BlueCash | Диапазоны уже спецификации (сумма ≤ 99 999.99, кол-во ≤ 99.999); `open()` не проверяет формат УНП | VG-06 |
| BUG-069 | P1 | BlueCash/backend | Нет фискального «доказательства» (слип, № документа, FM, время, УНП) → сторно невозможно | VG-07 |
| BUG-070 | P1 | BlueCash/backend | Не проверяется режим валюты ФУ (евро): возможно расхождение «карта в EUR — бон в лв» | VG-09 |

---

## Дополнение (раунд 3): найдены при статическом разборе BlueCash

| ID | Sev | Компонент | Суть | Статус |
|---|---|---|---|---|
| BUG-071 | P0 | BlueCash | `reverse()` возвращает деньги по карте ДО фискального сторно | CONFIRMED (статически) |
| BUG-072 | P0 | BlueCash | Нет взаимного исключения между MQTT/BLE/HTTP; `journal.find`+`append` — check-then-act | CONFIRMED (статически) |

### BUG-071 · P0 · Возврат по карте раньше фискального сторно
- **Место:** `BlueCashEngine.kt:305-376` (`reverse`).
- **Проблема:** если фискальное сторно затем падает или результат UNKNOWN, деньги уже возвращены, а сторно-документа нет; повтор видит состояние FAILED навсегда.
- **Исправление (D-12, DEFAULT: storno-first):** сначала фискальное сторно (cmd 48→…→56), только при подтверждённом успехе — `card.refund`; при Unknown — `RECOVERY_REQUIRED` без возврата; сбой возврата после успешного сторно — повторяемое `REFUND_PENDING`.
- **Тест:** `reverseCardRefundNeverBeforeFiscalStorno`, `reverseStornoUnknownDoesNotRefund`, `reverseRefundFailureAfterStornoIsRetryable` (`BlueCashEngineTest.kt`).
- **Приёмка:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashEngineTest*"`.
- **DoD:** CDOD; решение D-12 подтверждено владельцем (DECISIONS.md). Подзадачи — `execution/decomp/android.md`.

### BUG-072 · P0 · Гонка между транспортами BlueCash
- **Место:** `BlueCashCommandProcessor.sale/reverse/printerTest` (`BlueCashEngine.kt`); три транспорта используют один процессор без блокировки.
- **Проблема:** одинаковая команда, пришедшая по MQTT и HTTP одновременно, может исполниться дважды (двойной чек/двойное списание).
- **Исправление:** единый `ReentrantLock` на исполнение + повторный `journal.find` под локом.
- **Тест:** `concurrentDuplicateCommandsExecuteOnce`, `differentOperationsSerialized`.
- **Приёмка:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*BlueCashCommandProcessorTest*"`.
- **DoD:** CDOD.
