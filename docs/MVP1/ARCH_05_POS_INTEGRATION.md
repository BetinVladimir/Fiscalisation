# ARCH_05 — Интеграция Beeloy POS ↔ Fiscalisation

Компоненты (`/Users/freelancer/Documents/Beeloy/POS`): `pos-app` (Expo/React Native), `pos-gateway` (Go: outbox → RabbitMQ, потребитель результатов), `erp-connector`, `public-api-core` (вне анализа). E2E: `POS/tests/e2e/fiscal-integration/*.spec.ts`. Документы: `POS/docs/05-fiscal-contract.md`, `POS/docs/88-pos-integration-rollout-checklist.md`.
Баги с кодом/строками и тестами — в [ARCH_01](ARCH_01_BUGS.md) (BUG-050…061). Здесь — карта потоков, несоответствия контрактов и задачи интеграции.
CDOD — [ARCH_00_INDEX.md](ARCH_00_INDEX.md). Пределы анализа: `erp-connector`, `public-api-core` и тесты `pos-gateway` не читались; утверждения по ним — TO VERIFY.

## 1. Фактические потоки (по коду)
```
pos-app ──(A) HTTP /public/v1/*  ───────────────▶ fiscal-backend   (open sale, add line, shift …)
pos-app ──(B) HTTP /api/v1/fiscal/* ─▶ pos-gateway ──outbox──▶ RabbitMQ (beeloy.pos.commands)
                                                         └─▶ fiscal-backend (fiscal.sale.finalize.v1)
fiscal-backend ──результат──▶ RabbitMQ (beeloy.events) ──▶ pos-gateway result_consumer ──▶ БД шлюза ──▶ pos-app (poll /operations/{id})
```
Два пути (A) и (B) используют **один** базовый URL в приложении (BUG-051).

## 2. Таблица несоответствий контракта
| Параметр | pos-app | pos-gateway | fiscal-backend | Проблема |
|---|---|---|---|---|
| reconcile | `/api/v1/fiscal/operations/{id}:reconcile` (`Adapters.ts:99`) | `POST …/{operationId}/reconcile` (`main.go:227`) | — | BUG-050 |
| Base URL | один `FISCAL` | — | `/public/v1` | BUG-051 |
| Платежи | массив в `FiscalIntentPayload` | один CASH, сумма `0.00` по умолчанию (`idle.go:96-108`) | типы CASH, CARD, … | BUG-055 |
| НДС | по `taxSnapshot` | `/1.2` (`fiscal_handler.go:128-137`) | ставки A–H | BUG-056 |
| Округление | half-even (Decimal) | — | half-up (`service.go:1518`) | BUG-023 |
| УНП | `unp ?? fiscalReference` (`Adapters.ts:230`) | — | `unp` отдельно | BUG-053 |
| Idempotency | `${id}-reconcile` | заголовок игнорируется (`:83`) | `Idempotency-Key` | BUG-052/057 |
| Таймаут | 4 с на всё | — | операции с ФУ — секунды | BUG-054 |
| Тестовый режим | — | `AUTHORIZED_TEST` fail-open (`:68`) | Annex 29 п.20 запрещает test-режим в PROD | BUG-058 |

## 3. Задачи интеграции
Формат как в ARCH_02. Все команды запускать из указанных каталогов.

### POS-01 · P0 · Единая схема `FiscalIntentPayload`
- **Файлы (создать):** `POS/docs/contracts/fiscal-intent.v1.json` (JSON Schema), `POS/docs/contracts/fiscal-result.v1.json`.
- **Шаги:** (1) выписать поля из `pos-app/src/infrastructure/fiscal/FiscalApiClient.ts` и из `pos-gateway/internal/handler/fiscal_handler.go` (структуры запроса) в схему; (2) добавить обязательные: `operationId`, `idempotencyKey`, `lines[]` (qty в тысячных, цена в центах-строкой, `taxGroup`), `payments[]` (`type`, `amount`, `paymentId`), `expectedTotal`, `currency=EUR`; (3) сгенерировать типы: TS (`json-schema-to-typescript`), Go (`go-jsonschema` или ручной валидатор); (4) обе стороны валидируют по схеме.
- **Тест:** `contract.fiscalIntent.test.ts` (pos-app) и `fiscal_intent_contract_test.go` (pos-gateway) на одних и тех же `fixtures/intent-valid-*.json`/`intent-invalid-*.json` (≥ 12 файлов: пустые платежи, сумма ≠ итогу, неизвестный тип, qty=0, дробные центы, валюта USD…).
- **Приёмка:** `cd POS/pos-app && npx jest contract.fiscalIntent` ok; `cd POS/pos-gateway && go test ./internal/handler -run FiscalIntentContract -count=1` ok; число fixtures одинаково (`ls fixtures | wc -l` проверяется скриптом).
- **DoD:** CDOD; ссылка в `05-fiscal-contract.md`.

### POS-02 · P0 · Маршрутизация и базовые URL
- Реализуется как BUG-050 + BUG-051. **Приёмка:** `npx jest -t routing` ok; E2E `transport.spec.ts` дополнен проверкой `POST /reconcile` ≠ 404/405 (`FISCAL_URL`, `POS_GATEWAY_URL` заданы в CI).

### POS-03 · P0 · Платежи и суммы сквозные
- Реализуется как BUG-055/056. **Приёмка:** E2E `TST-E2E-02/03/04` (ARCH_03) зелёные; Go-тест `finalizeCommandCarriesAllPaymentsAndTypes`.

### POS-04 · P0 · УНП и фискальный референс
- BUG-053. **Дополнительно:** модель данных чека POS: поля `unp`, `fiscalNumber`, `fiscalReference` раздельно в `LocalSuptoJournal.ts` и БД POS (миграция в `POS/…/migrations`, **найти актуальный каталог командой** `grep -rn "fiscal_ref" POS --include=*.sql -l`).
- **Приёмка:** `npx jest LocalSuptoJournal` ok; `grep -rn "fiscalRef: result.operationId" POS/pos-app/src` → пусто.

### POS-05 · P1 · Идемпотентность
- BUG-052/057. **Правило:** `Idempotency-Key` = `clientOperationId` = `operationId` для одной бизнес-операции; повтор после UNKNOWN использует тот же ключ; reconcile — новый ключ на попытку, но тот же `operationId`.
- **Приёмка:** Go `Submit_usesIdempotencyKeyAsOperationID`; jest `reconcileUsesNewKeyPerAttempt`; E2E `TST-E2E-17/18`.

### POS-06 · P1 · Ошибки, таймауты, UNKNOWN
- BUG-054. **Состояния UI/Store (`fiscalStore.ts`):** `PENDING → SENT → CONFIRMED | UNKNOWN | REJECTED`; `UNKNOWN` блокирует новую оплату того же чека и показывает кассиру процедуру (ARCH_06 §4).
- **Приёмка:** `npx jest fiscalStore` — таблица переходов; тест: из `UNKNOWN` нельзя перейти в `SENT` без reconcile.

### POS-07 · P1 · Политика установки и тест-режим
- BUG-058 + проверка Annex 29 п.20 (запрет тест/обучающего режима в боевой версии).
- **Шаги:** (1) перечислить значения `installationMode` и их влияние (`grep -rn installationMode POS --include=*.go --include=*.ts`); (2) в PROD-профиле (`APP_ENV=prod`) `AUTHORIZED_TEST` недопустим — старт отклоняется; (3) документировать в `88-pos-integration-rollout-checklist.md`.
- **Приёмка:** Go-тест `StartupRejectsAuthorizedTestInProd`; `rg -n AUTHORIZED_TEST POS/pos-gateway` — каждая ветка покрыта тестом (скрипт сравнивает список ветвей).

### POS-08 · P1 · Топология RabbitMQ
- BUG-059. **Файлы:** `POS/pos-gateway/internal/fiscal/result_consumer.go`, `POS/deploy/*` (найти определения очередей), `POS/scripts/check-rabbit-topology.sh` (создать).
- **Приёмка:** скрипт читает `rabbitmqctl list_queues name arguments` и падает, если у очередей `…fiscal-results…` нет `x-dead-letter-exchange`; integration-тест с контейнером RabbitMQ: невалидное сообщение попадает в DLQ ровно один раз.

### POS-09 · P0 · Секреты
- BUG-060. **Приёмка:** CI-джоба `secrets` (gitleaks) на POS и Fiscalisation; `git ls-files | grep -E 'credentials\.json|\.jks$|\.keystore$'` пусто.

### POS-10 · P1 · Деньги и округление
- BUG-023/061, ARCH-005. **Приёмка:** `npx jest decimal.golden` + Go/Kotlin golden.

### POS-11 · P1 · Момент возникновения УНП (Annex 29 п.9, 9.1)
- **Вопрос:** Annex 29 п.9/9.1 требует генерации и показа УНП при вводе первого товара. Путь (A) pos-app вызывает `openSaleWithLine` — соответствует; путь (B) шлюз финализирует весь чек в конце. **TO VERIFY:** какие потоки POS реально используются в проде и появляется ли УНП до оплаты в обоих. Нормативное толкование — через ARCH_06 (юрист).
- **Шаги:** (1) диаграмма последовательности для каждого сценария продажи (розница, ресторан/стол, онлайн-заказ из ERP) в `docs/MVP1/POS_SALE_FLOWS.md`; (2) отметить момент генерации УНП; (3) расхождения → задачи.
- **Приёмка:** `POS_SALE_FLOWS.md` содержит таблицу «сценарий / момент УНП / компонент / тест» без пустых ячеек (`awk` скрипт проверяет число колонок).

### POS-12 · P1 · Офлайн
- **Шаги:** (1) определить, что делает POS при недоступности шлюза/backend (очередь в SQLite?); (2) запретить печать «как фискальный» до подтверждения (BUG-053); (3) порядок отправки и дедупликация.
- **Тест:** E2E `TST-E2E-07/19`. **Приёмка:** jest `offlineQueue.order.test.ts` + E2E зелёные.

### POS-13 · P1 · Возврат/сторно из POS
- **Файлы:** `pos-app/src/application/workflows/RefundWorkflow*`, `fiscal_handler.go` (reverse).
- **Шаги:** возврат использует `unp` и `fiscalReference` **исходной** продажи, а не пересчитанные; сумма ≤ остатка; тип оплаты возврата соответствует правилам (ARCH_06).
- **Приёмка:** `TST-E2E-09/10/11`; jest `refund.originalReference.test.ts`.

### POS-14 · P2 · Корреляция и наблюдаемость
- `operationId` во всех логах pos-app/gateway/backend; заголовок `X-Request-Id` пробрасывается.
- **Приёмка:** E2E проверяет, что для одной операции в логах трёх сервисов найден один `operationId` (`scripts/e2e-trace-check.sh`).

### POS-15 · P2 · Документация
- Обновить `POS/docs/05-fiscal-contract.md`, `88-pos-integration-rollout-checklist.md`, `promt.md` (если используется как руководство) в соответствии с POS-01…POS-13.
- **Приёмка:** `scripts/check-doc-drift.sh` (ARCH_04 DOC-005) — нет `OPEN`.

## 4. Контрольный список перед HIL с POS
- [ ] POS-01…POS-04, POS-09 закрыты (P0).
- [ ] E2E TST-E2E-01…06, 08, 17, 20 зелёные на симуляторе в CI.
- [ ] Один сквозной прогон «POS → шлюз → backend → BlueCash (эмулятор Datecs)» сохранён как артефакт (`artifacts/e2e/<date>/`).
- [ ] `ruby scripts/check_plan_coverage.rb` (ARCH_07 R-01) → `unmapped: 0`.
