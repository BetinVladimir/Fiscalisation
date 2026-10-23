# Декомпозиция до функций: pos

## BUG-050
Клиент POS: URL reconcile. `Adapters.ts` класс `HttpFiscalApiClient`, метод `reconcileOperation(id)` (стр. 97-104).

1. **Тест `reconcileOperation` (mock fetch): URL заканчивается `/reconcile`, метод POST, нет `:reconcile`**
   - файлы: `POS/pos-app/src/infrastructure/runtime/HttpFiscalApiClient.test.ts`
   - проверка: красный
   - приёмка: `cd POS/pos-app && npx jest src/infrastructure/runtime -t reconcile`
2. **Заменить путь на `/api/v1/fiscal/operations/${encodeURIComponent(id)}/reconcile`**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`
   - проверка: зелёный + grep
   - приёмка: `cd POS/pos-app && npx jest src/infrastructure/runtime -t reconcile && ! grep -n ":reconcile" src/infrastructure/runtime/Adapters.ts`
3. **E2E `transport.spec.ts`: POST `/reconcile` не 404/405**
   - файлы: `POS/tests/e2e/fiscal-integration/transport.spec.ts`
   - проверка: зелёный в CI с FISCAL_URL/POS_GATEWAY_URL
   - приёмка: `cd POS && npx playwright test tests/e2e/fiscal-integration/transport.spec.ts`

## BUG-051
Два базовых URL. Методы `/public/v1/*` (`openSaleWithLine`:40, `addSaleLine`:47, `cancelSaleLine`:72, `openShift`:105, `closeShift`:148) → fiscal-backend; `/api/v1/*` (`submitOperation`:81, `getOperationStatus`:92, `reconcileOperation`:97) → шлюз.

1. **Тест-таблица `HttpFiscalApiClient.routing.test.ts`: метод → ожидаемый базовый URL (8 строк) — красный**
   - файлы: `POS/pos-app/src/infrastructure/runtime/HttpFiscalApiClient.routing.test.ts`
   - проверка: красный
   - приёмка: `cd POS/pos-app && npx jest src/infrastructure/runtime -t routing`
2. **Конструктор `HttpFiscalApiClient(fiscalBase, gatewayBase)`; приватный `url(kind:"fiscal"|"gateway", path)`; заменить все вызовы**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`
   - проверка: зелёный
   - приёмка: `cd POS/pos-app && npx jest src/infrastructure/runtime -t routing`
3. **`di/container.impl.ts:70,358`: читать `EXPO_PUBLIC_FISCAL_API_URL` и `EXPO_PUBLIC_GATEWAY_URL`; валидатор `assertBaseUrls(env)` (оба заданы; в prod только https)**
   - файлы: `POS/pos-app/src/infrastructure/di/container.impl.ts`, `POS/pos-app/src/config/baseUrls.ts`
   - проверка: тест assertBaseUrls
   - приёмка: `cd POS/pos-app && npx jest baseUrls`
4. **Обновить `.env.example` и `pos-app/README.md`**
   - файлы: `POS/pos-app/.env.example`, `POS/pos-app/README.md`
   - проверка: grep
   - приёмка: `grep -q EXPO_PUBLIC_GATEWAY_URL POS/pos-app/.env.example`

## BUG-052
Ключ идемпотентности reconcile (`Adapters.ts:101`).

1. **Таблица SQLite `fiscal_attempt(operation_id, attempt_no, key, created_at)` + репозиторий `nextAttempt(operationId)` (миграция)**
   - файлы: `POS/pos-app/src/infrastructure/db/`
   - проверка: тест
   - приёмка: `cd POS/pos-app && npx jest fiscalAttempt`
2. **`reconcileOperation`: ключ = `${id}-reconcile-${attemptNo}`, хранится до получения ответа**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`
   - проверка: тест reconcileUsesNewKeyPerAttempt
   - приёмка: `cd POS/pos-app && npx jest src/infrastructure/runtime -t reconcileUsesNewKeyPerAttempt`

## BUG-053
УНП и фискальный референс — разные сущности. Места: `Adapters.ts:230` (`normalizeStatus`), `:713`, `LocalReceiptOperations.ts:10`, `LocalSuptoJournal.ts`.

1. **Тип `FiscalStatusResponse` получает раздельные `unp`, `fiscalNumber`, `fiscalReference` (без fallback друг на друга)**
   - файлы: `POS/pos-app/src/infrastructure/fiscal/FiscalApiClient.ts`
   - проверка: tsc
   - приёмка: `cd POS/pos-app && npx tsc --noEmit`
2. **`normalizeStatus` (Adapters.ts:222-270): убрать `unp ?? fiscalReference`; тест `normalizeStatus.test.ts` (3 поля независимы)**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`, `POS/pos-app/src/infrastructure/runtime/normalizeStatus.test.ts`
   - проверка: зелёный
   - приёмка: `cd POS/pos-app && npx jest normalizeStatus`
3. **`LocalReceiptOperations.ts:10`: `fiscalRef = result.fiscalReference`, `unp = result.unp` (без `?? ""`); пустой УНП → `deliver()` не печатает, статус `NEEDS_ATTENTION`**
   - файлы: `POS/pos-app/src/application/workflows/LocalReceiptOperations.ts`
   - проверка: тест
   - приёмка: `cd POS/pos-app && npx jest LocalReceiptOperations`
4. **`LocalSuptoJournal.ts` и миграция БД: колонки `unp`,`fiscal_number`,`fiscal_reference` раздельно (каталог миграций найти `grep -rn fiscal_ref POS --include=*.sql -l`)**
   - файлы: `POS/pos-app/src/infrastructure/supto/LocalSuptoJournal.ts`
   - проверка: тест
   - приёмка: `cd POS/pos-app && npx jest LocalSuptoJournal`
5. **Защита: `grep -rn "fiscalRef: result.operationId" POS/pos-app/src` пусто**
   - файлы: `POS/pos-app/src`
   - проверка: grep
   - приёмка: `! grep -rn "fiscalRef: result.operationId" POS/pos-app/src`

## BUG-054
Разбор ответа и таймауты (`Adapters.ts:255-266`, приватный `request`).

1. **`request()`: сначала `r.ok`, затем `text=await r.text(); body=text?safeParse(text):null`; не-JSON → typed `HttpError(status, rawSnippet)`**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`
   - проверка: тест request.nonJsonError
   - приёмка: `cd POS/pos-app && npx jest request.nonJsonError`
2. **Таймауты через `AbortController`: чтение 4 с, запись 15 с (константы `TIMEOUT_READ_MS`, `TIMEOUT_WRITE_MS`)**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`
   - проверка: тест
   - приёмка: `cd POS/pos-app && npx jest request.timeout`
3. **Таймаут записи → исключение `FiscalUnknownError(operationId)`; вызывающий обязан `getOperationStatus` по тому же `operationId`, а не повторять запись с новым ключом**
   - файлы: `POS/pos-app/src/infrastructure/runtime/Adapters.ts`, `POS/pos-app/src/application/workflows/LocalReceiptOperations.ts`
   - проверка: тест timeoutOnWriteMarksUnknown
   - приёмка: `cd POS/pos-app && npx jest request.timeoutOnWriteMarksUnknown`

## BUG-055
Шлюз финализирует всё как CASH: `drain/idle.go:96-108` внутри `(*outboxDrainer).drain` (52-136).

1. **Тест `finalizeCommandCarriesAllPaymentsAndTypes` (CARD+CASH, суммы 7.00/3.00) в `idle_test.go` — красный**
   - файлы: `POS/pos-gateway/internal/drain/idle_test.go`
   - проверка: красный
   - приёмка: `cd POS/pos-gateway && go test ./internal/drain -run Finalize -count=1`
2. **Вынести `buildFinalizeCommand(intent map[string]any) (cmd, error)` из `drain` — чистая функция**
   - файлы: `POS/pos-gateway/internal/drain/idle.go`, `POS/pos-gateway/internal/drain/finalize_cmd.go`
   - проверка: build
   - приёмка: `cd POS/pos-gateway && go build ./...`
3. **В `buildFinalizeCommand`: платежи из `intent["payments"]` как есть (валидация: тип из списка, `^\d+\.\d{2}$`, сумма = `expected_total`); `payment_id` из платежа; `terminal_policy` по типу; нет `amount` → ошибка**
   - файлы: `POS/pos-gateway/internal/drain/finalize_cmd.go`
   - проверка: тесты
   - приёмка: `cd POS/pos-gateway && go test ./internal/drain -run Finalize -count=1`
4. **`FiscalHandlers.Submit` (fiscal_handler.go:61): при отсутствии payments/expected_total → 422 на приёме; тест `finalizeRejectedWhenAmountMissing`**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler.go`
   - проверка: тест
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run Submit -count=1`
5. **E2E TST-E2E-02 (card-sale) — спецификация в ARCH_15**
   - файлы: `docs/MVP1/ARCH_15_TEST_SPECS_WITH_LOGIC.md`
   - проверка: файл есть
   - приёмка: `test -f docs/MVP1/ARCH_15_TEST_SPECS_WITH_LOGIC.md`

## BUG-056
Проекция чека в шлюзе: `fiscal_handler.go:128-137` (в `Submit`).

1. **Тесты `Submit_projectsTotalsFromPayload`, `Submit_doesNotAssume20PercentVAT` (9%, 0%) — красные**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler_test.go`
   - проверка: красные
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run Submit -count=1`
2. **Заменить `round($5/1.2,4)` на суммы/разбивку НДС из `payload.totals`/`payload.lines`; `monetary.Amount` из payload.totals.gross; `lines` не пустой**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler.go`
   - проверка: зелёные
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run Submit -count=1`

## BUG-057
Idempotency-Key (`fiscal_handler.go:61-90`, строка 83).

1. **Если `q.ClientOperationID==""` → взять из заголовка `Idempotency-Key`; оба заданы и различаются → 422**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler.go`
   - проверка: тест
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run Submit_usesIdempotencyKeyAsOperationID -count=1`
2. **Тест `Submit_usesIdempotencyKeyAsOperationID` + `Submit_conflictingKeysReturn422`**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler_test.go`
   - проверка: зелёные
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run Submit -count=1`

## BUG-058
Fail-open (`fiscal_handler.go:68`).

1. **Тест `Submit_failsClosedOnPolicyLookupError` (pool возвращает не-ErrNoRows ошибку → 503)**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler_test.go`
   - проверка: красный
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run failsClosed -count=1`
2. **`errors.Is(err, pgx.ErrNoRows)` → политика по умолчанию; иная ошибка → 503 `POLICY_LOOKUP_FAILED`**
   - файлы: `POS/pos-gateway/internal/handler/fiscal_handler.go`
   - проверка: зелёный
   - приёмка: `cd POS/pos-gateway && go test ./internal/handler -run failsClosed -count=1`

## BUG-059
Потребитель результатов `fiscal/result_consumer.go`: `StartResultConsumer`:20, `apply`:73, `mapState`:247.

1. **Тест `invalidEnvelopeGoesToDLQNotRequeue` — красный**
   - файлы: `POS/pos-gateway/internal/fiscal/result_consumer_test.go`
   - проверка: красный
   - приёмка: `cd POS/pos-gateway && go test ./internal/fiscal -run invalidEnvelope -count=1`
2. **Объявить DLX `beeloy.dlx` и очередь `pos-gateway.fiscal-results.dlq`; основная очередь с `x-dead-letter-exchange`**
   - файлы: `POS/pos-gateway/internal/fiscal/result_consumer.go`
   - проверка: тест топологии
   - приёмка: `bash scripts/check-rabbit-topology.sh`
3. **Убрать командные биндинги (`fiscal.sale.finalize.v1`, `fiscal.sale.reverse.v1`, `fiscal.shift.*.v1`) либо ADR с обоснованием**
   - файлы: `POS/pos-gateway/internal/fiscal/result_consumer.go`
   - проверка: тест
   - приёмка: `cd POS/pos-gateway && go test ./internal/fiscal -run Bindings -count=1`
4. **`Nack(false,true)` заменить: невалидный конверт → `Nack(false,false)` + метрика; временная ошибка БД → requeue с задержкой (повторная публикация в delay-очередь)**
   - файлы: `POS/pos-gateway/internal/fiscal/result_consumer.go`
   - проверка: тесты
   - приёмка: `cd POS/pos-gateway && go test ./internal/fiscal -run Consumer -count=1`

## POS-03
Платежи и суммы сквозные = BUG-055 + BUG-056 + E2E TST-E2E-02/03/04.

1. **Выполнить подзадачи BUG-055 и BUG-056 (ссылка)**
   - файлы: 
   - проверка: см. BUG-055/056
   - приёмка: `true`
2. **E2E TST-E2E-02/03/04 реализованы по ARCH_15 с фикстурой `fixtures/e2e/split_payment.json`**
   - файлы: `POS/tests/e2e/fiscal-integration/`
   - проверка: e2e
   - приёмка: `cd POS && npx playwright test tests/e2e/fiscal-integration`

## POS-04
УНП и фискальный референс = BUG-053 + модель данных.

1. **Выполнить подзадачи BUG-053 (ссылка)**
   - файлы: 
   - проверка: см. BUG-053
   - приёмка: `true`
2. **Миграция БД POS: `unp`, `fiscal_number`, `fiscal_reference` в таблице чеков; backfill существующих `fiscal_ref` → `fiscal_reference`**
   - файлы: `POS/pos-app/src/infrastructure/db/`
   - проверка: jest LocalSuptoJournal
   - приёмка: `cd POS/pos-app && npx jest LocalSuptoJournal`
