# ARCH_04 — Архитектура, качество кода, стиль, документация кода

Формат: **ID · Sev · Место · Замечание · Как исправить (шаги) · Приёмка (автоматическая) · DoD**.
CDOD — в [ARCH_00_INDEX.md](ARCH_00_INDEX.md). Размеры файлов измерены `wc -l` на момент анализа (fiscal-backend: `domain/repository.go` 2061, `api/handler.go` 2089, `domain/service.go` 1579 строк, 61 экспортируемых метода `Service`; `BlueCashEngine.kt` 406; `intent_processor.cpp` 531; `MainActivity.kt` 335).
Рефакторинги — только с сохранением поведения: сначала characterization-тест, затем перенос кода.

| ID | Тип | Sev | Кратко |
|---|---|---|---|
| ARCH-001 | Архитектура | P0 | Хранение состояния: JSON-снимок под глобальным RWMutex (→ TG-001) |
| ARCH-002 | Архитектура | P1 | `Service` — «God object» (61 публичный метод, 1579 строк) |
| ARCH-003 | Архитектура | P1 | `handler.go` 2089 строк: маршруты, middleware, DTO, идемпотентность в одном файле |
| ARCH-004 | Архитектура | P1 | Четыре независимые реализации денег в Go-пакете |
| ARCH-005 | Архитектура | P1 | Три реализации округления в трёх языках (→ BUG-023) |
| ARCH-006 | Архитектура | P1 | Выбор драйвера: `s.driver` vs `boundDriver` — непоследовательно |
| ARCH-007 | Архитектура | P1 | Рантайм BlueCash привязан к `Activity` (→ BUG-032) |
| ARCH-008 | Архитектура | P2 | Два канала «правды» о фискальном исходе: журнал и агрегат |
| ARCH-009 | Архитектура | P2 | Контракты между POS/шлюз/backend без единого источника схемы (→ TG-007) |
| CODE-001 | Код | P1 | Классификация ошибок по тексту сообщения (Kotlin) |
| CODE-002 | Код | P1 | Дублирование `try/catch`-логики в четырёх методах `BlueCashEngine` |
| CODE-003 | Код | P2 | «Магические» значения (UNKNOWN-коды, лимиты Datecs, таймауты) |
| CODE-004 | Код | P2 | Тихое проглатывание ошибок (`runCatching`, `_ =`) в денежных путях |
| CODE-005 | Код | P2 | Состояния как строки (`"FISCALIZED"`) без типов |
| CODE-006 | Код | P2 | Мёртвый код: `receipt_saga.go`, неиспользуемые транспорты (→ BUG-006) |
| CODE-007 | Код | P2 | Время: `time.Now()` напрямую в доменной логике |
| STYLE-001 | Стиль | P2 | Линтеры не принуждают денежные правила |
| STYLE-002 | Стиль | P3 | Именование и комментарии на смеси RU/EN |
| STYLE-003 | Стиль | P3 | Форматирование/статанализ Kotlin и C++ |
| DOC-001 | Документация | P1 | Нет doc-комментариев инвариантов в денежных/саговых функциях |
| DOC-002 | Документация | P1 | Нет ADR по ключевым решениям (округление, сага, хранение, UNKNOWN) |
| DOC-003 | Документация | P1 | `verify-contract-lock.sh` падает (FISC-DOC-001) |
| DOC-004 | Документация | P2 | Нет карты компонентов и потоков данных POS ↔ Fiscal ↔ устройства |
| DOC-005 | Документация | P2 | Устаревшие/расходящиеся документы (README vs код) |

---

## Архитектура

### ARCH-001 · P0 · Хранение состояния
- **Место:** `fiscal-backend/internal/domain/repository.go` (`mu sync.RWMutex` — строка ~173; `persistLocked()` вызывается из каждого мутатора, строки ~148–166, 320–337); `internal/persistence/`.
- **Замечание:** каждая запись держит глобальную блокировку и сериализует состояние; рост данных → латентность растёт линейно; одновременные запросы разных арендаторов блокируют друг друга; неатомарность «операция + событие + replay-ключ».
- **Исправление:** см. TG-001 (нормализованные таблицы, `WithTx`). Предварительно — характеристический тест на 1000 операций с замером времени записи.
- **Приёмка:** `go test ./internal/domain -run 'TestRepositoryWriteLatencyDoesNotGrowLinearly' -count=1` (порог в тесте: p95 записи при 10 000 операций ≤ 3× p95 при 100); `go test -race ./...` ok.
- **DoD:** CDOD + ADR-PERSIST-001.

### ARCH-002 · P1 · Разделить `Service`
- **Место:** `internal/domain/service.go` (1579 строк; методы продаж, сторно, смен, отчётов, касс, часов).
- **Исправление (шаги, безопасные):**
  1. Создать характеристические тесты публичного API (`TestServiceBehaviorSnapshot_*`) — запись входов/выходов для 10 основных сценариев.
  2. Выделить внутренние сервисы по файлам (`sale_service.go`, `reversal_service.go`, `shift_service.go`, `clock_service.go`, `report_service.go`) **без изменения публичных сигнатур**: `Service` делегирует.
  3. Общие зависимости передавать структурой `deps`, не глобалами.
  4. После каждого шага `go test ./internal/domain -count=1`.
- **Приёмка:** `wc -l internal/domain/service.go` ≤ 600; `go test ./... -race -count=1` без изменений результатов; `go vet ./...` ok.
- **DoD:** CDOD; публичный API не изменён (`go doc ./internal/domain Service | sha256sum` совпадает до/после — записать в PR).

### ARCH-003 · P1 · Разделить `handler.go`
- **Место:** `internal/api/handler.go` (2089 строк: маршруты `~140–200`, CORS, auth-middleware, идемпотентность `~530–600`, DTO, ответы).
- **Исправление:** вынести `routes.go`, `middleware_cors.go`, `middleware_idem.go`, `middleware_auth.go`, `dto_*.go`, `responses.go`; без изменения путей.
- **Тест:** существующий `handler_test.go` (1085 строк) должен проходить без правок; добавить `TestRouteTableSnapshot` — список «метод + путь + роль» сравнивается с golden `testdata/routes.golden`.
- **Приёмка:** `go test ./internal/api -count=1` ok; `wc -l internal/api/handler.go` ≤ 500; golden неизменен.
- **DoD:** CDOD.

### ARCH-004 · P1 · Единый модуль денег в Go
- **Место:** `service.go:1489-1528` (`validMoney`, `parseFixed`, `lineTotalCents`), `receipt_saga.go:96-111` (`receiptMoneyCents`), `edge_sync*.go`, `exports.go`.
- **Исправление:** новый файл `internal/domain/money.go`: `type Cents int64`, `ParseMoney(string) (Cents, error)`, `FormatMoney(Cents) string`, `LineTotal(price Cents, qtyMilli int64) (Cents, error)` с проверкой переполнения (`math/bits`/`checked`). Все остальные реализации удалить, заменить вызовами.
- **Тест:** `money_test.go` — таблица (BUG-001/002), fuzz `FuzzParseMoney`, golden из TST-CT-01.
- **Приёмка:** `go test ./internal/domain -run 'Money|Fuzz' -count=1`; `grep -rn 'strconv.ParseInt' internal/domain | grep -i 'amount\|money'` → пусто вне `money.go`.
- **DoD:** CDOD.

### ARCH-005 · P1 · Единая политика округления
- См. BUG-023 (ADR + golden + три реализации). **Приёмка:** `scripts/check-rounding-golden.sh` запускает три набора и падает, если любой не использует `contracts/money-rounding-golden-v1.json` (`grep -l money-rounding-golden` по трём тест-деревьям).

### ARCH-006 · P1 · Выбор драйвера
- **Место:** `service.go` (`s.driver`, `s.boundDriver(...)`; `SyncWorkstationClock`, `executeReservedFiscalOperation`).
- **Исправление:** единственная точка `driverFor(ctx, registerID)`; прямое обращение `s.driver` разрешено только в `driverFor`.
- **Приёмка:** `grep -n 's\.driver\b' internal/domain/service.go | grep -v 'func (s \*Service) driverFor'` → пусто; тест `TestEveryOperationUsesBoundDriver`.

### ARCH-007 · P1 · Рантайм BlueCash вне Activity
- См. BUG-032; архитектурно: `BlueCashRuntime` (чистый Kotlin, без Android-зависимостей) + `RuntimeService` (Android) + `MainActivity` (UI). **Приёмка:** `grep -n 'import android' src/main/kotlin/.../BlueCashRuntime*.kt` → пусто; JVM-тесты рантайма не требуют Robolectric.

### ARCH-008 · P2 · Журнал и агрегат
- **Место:** `AndroidJournal.kt`, `BlueCashComplianceIntent.kt` (`processed[...]`, `aggregate`).
- **Замечание:** исход чека хранится и в журнале (хэш-цепочка), и в агрегате (`store.put`); при расхождении (BUG-024) нет правила, что приоритетнее.
- **Исправление:** ADR «Источник истины исхода = журнал; агрегат — проекция, восстанавливаемая из журнала»; функция `rebuildAggregateFromJournal()` при старте.
- **Тест:** `aggregateRebuildMatchesJournal` (рандомные последовательности событий).
- **Приёмка:** gradle-тест ok.

### ARCH-009 · P2 · Единая схема контрактов
- См. TG-007/TG-013: источник — JSON Schema; генерация типов Go/Kotlin/TS; проверка в CI. **Приёмка:** `make contract-test`; `git diff --exit-code` после генерации.

---

## Код

### CODE-001 · P1 · Ошибки по тексту сообщения
- **Место:** `BlueCashEngine.kt:170,206,274,366` (`e.message in setOf(...)`); аналог в `BlueCashComplianceIntent.kt`.
- **Исправление:** sealed-иерархия `DatecsFailure` (BUG-028); везде `when (e) { is … }`.
- **Приёмка:** `grep -rn 'e.message in' src/main` → пусто; тест BUG-028.

### CODE-002 · P1 · Дублирование логики исключений
- **Место:** `BlueCashEngine.kt` — `sale`, `reverse`, `printerTest`, `report` содержат похожие `catch`.
- **Исправление:** единая функция `inline fun <T> guardedFiscalCall(ctx, block): Outcome<T>` с классификацией (NotSent/Rejected/Unknown), записью в журнал и условной отменой чека.
- **Тест:** параметризованный по всем четырём методам.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*BlueCashEngineTest*'`; число `catch (` в файле уменьшилось (`grep -c 'catch (' BlueCashEngine.kt` ≤ 4).

### CODE-003 · P2 · Магические значения
- **Место:** лимиты в `DatecsProtocol.kt:87-130` (72 символа, 99999.99, 100.000), таймауты в `DatecsAndroidPorts.kt`, коды `DATECS_*`.
- **Исправление:** `object DatecsLimits { const val MAX_NAME_LEN = 72 … }` с комментарием-ссылкой на раздел PDF; `enum class DatecsErrorCode`.
- **Приёмка:** `grep -nE '\b(72|99999|100\.000)\b' DatecsProtocol.kt` показывает только `DatecsLimits`.

### CODE-004 · P2 · Проглатывание ошибок
- **Место:** `grep -rn 'runCatching' SmartDevices/bluecash-app/src/main` и `grep -rn '_ = ' fiscal-backend/internal --include=*.go | grep -v _test`.
- **Исправление:** для каждого вхождения в денежном пути — логирование с `operation_id` и метрика; список допустимых исключений — комментарий `// best-effort: <причина>`.
- **Приёмка:** скрипт `scripts/check-swallowed-errors.sh` падает на вхождениях без комментария `best-effort:`.

### CODE-005 · P2 · Типы состояний
- **Место:** строки `"FISCALIZED"`, `"EXECUTING"`, `"UNKNOWN"` по Go и Kotlin.
- **Исправление:** `type OperationState string` + константы + `func (s OperationState) Terminal() bool`; Kotlin `enum class`.
- **Тест:** `TestOperationStateTerminalSet` (одна таблица — единственный источник).
- **Приёмка:** `grep -rn '"FISCALIZED"' fiscal-backend/internal --include=*.go | grep -v _test | grep -v 'domain/state.go'` → пусто.

### CODE-006 · P2 · Мёртвый код
- См. BUG-006; найти неиспользуемое: `staticcheck -checks U1000 ./...` (Go), `./gradlew lint`/`detekt` (Kotlin).
- **Приёмка:** `staticcheck ./...` exit 0 (либо список исключений в `staticcheck.conf` с обоснованиями).

### CODE-007 · P2 · Время
- **Место:** `grep -rn 'time.Now()' fiscal-backend/internal/domain --include=*.go | grep -v _test`.
- **Исправление:** внедрить `Clock` интерфейс (`Now() time.Time`) в `Service`; в тестах — фиксированные часы; бизнес-дата — только через `BusinessDate(clock.Now())` (Europe/Sofia).
- **Приёмка:** предыдущая команда → пусто вне `clock.go`; тесты BUG-005 используют `fakeClock`.

---

## Стиль

### STYLE-001 · P2 · Линтеры с денежными правилами
- **Файлы:** `fiscal-backend/.golangci.yml` (создать/дополнить), `POS/pos-app/.eslintrc*`, `SmartDevices/bluecash-app/detekt.yml`.
- **Правила:** запрет `float64` в доменных пакетах (кроме `metrics`), запрет `Number(...)*Number(...)` для сумм в TS (`no-restricted-syntax`), запрет `Double/Float` в Kotlin-коде денег.
- **Тест:** файл-нарушитель в `testdata/lint/` должен падать (`golangci-lint run ./testdata/lint/...` → exit ≠ 0, зафиксировано в скрипте `scripts/check-lint-rules.sh`).
- **Приёмка:** `make lint` exit 0; `scripts/check-lint-rules.sh` exit 0.

### STYLE-002 · P3 · Язык комментариев
- **Решение:** комментарии в коде — английский, документация для операторов — русский/болгарский; зафиксировать в `docs/CONTRIBUTING.md` (создать).
- **Приёмка:** файл существует и содержит раздел «Language policy» (`grep -c 'Language policy'` ≥ 1).

### STYLE-003 · P3 · Форматирование
- **Файлы:** `.editorconfig`, `ktlint`/`spotless`, `clang-format` для `IoT/`.
- **Приёмка:** `gofmt -l . | wc -l` = 0; `./gradlew ktlintCheck` ok; `clang-format --dry-run -Werror IoT/firmware/edge-agent-s3/idf/main/*.cpp` exit 0.

---

## Документация

### DOC-001 · P1 · Инварианты в doc-комментариях
- **Место:** функции денег и саги: `receiptMoneyCents`, `ValidateReceiptPlan`, `finalizeSaleForTenant`, `reserveFiscalOperation`, `SyncWorkstationClock`, `UnpAllocator.*`; Kotlin: `BlueCashCommandProcessor.sale`, `DatecsSocketProtocol.execute`, `AndroidTransactionJournal.append/acknowledge`; C++: `IntentProcessor::execute`.
- **Требование:** для каждой — блок «Invariants / Failure modes / Idempotency / Side effects» (3–8 строк), без пересказа кода.
- **Приёмка:** `scripts/check-doc-invariants.sh` (создать): список функций из `docs/MVP1/doc-required-functions.txt`; для каждой ищет блок `Invariants:` в предшествующем комментарии; exit 0 только если все найдены.
- **DoD:** CDOD; список ≥ 20 функций.

### DOC-002 · P1 · ADR
- **Создать в `docs/decisions/`:** `ADR-MONEY-ROUNDING-001.md`, `ADR-PERSIST-001.md`, `ADR-SAGA-001.md`, `ADR-UNKNOWN-OUTCOME-001.md`, `ADR-TIME-BUSINESS-DATE-001.md`, `ADR-PAYMENT-CAPABILITY-001.md`. Шаблон: Контекст · Решение · Последствия · Тесты, обеспечивающие решение (имена).
- **Приёмка:** `ls docs/decisions/ADR-*-001.md | wc -l` ≥ 6; `scripts/check-adr-links.sh` проверяет, что каждое имя теста из ADR найдено `grep -r`.

### DOC-003 · P1 · Contract lock
- **Место:** `scripts/verify-contract-lock.sh` (падает; FISC-DOC-001 в `docs/fiscal-contour-review-20261021/01-review-and-status.md`).
- **Шаги:** воспроизвести падение; определить, какой файл контракта изменился без обновления lock; решение принимает владелец контракта (не «чинить» подгонкой хэша). 
- **Приёмка:** `sh scripts/verify-contract-lock.sh` exit 0 на чистом дереве; CI-джоба `contracts` обязательна.

### DOC-004 · P2 · Карта компонентов
- **Создать:** `docs/MVP1/COMPONENT_MAP.md` — диаграмма (Mermaid) POS-app → pos-gateway → RabbitMQ → fiscal-backend → MQTT → BlueCash/S3 → ФУ; для каждой стрелки: протокол, аутентификация, идемпотентность, таймаут, поведение при UNKNOWN.
- **Приёмка:** файл содержит блок ```mermaid и таблицу «стрелка/таймаут/идемпотентность» минимум из 8 строк (`grep -c '^|' ≥ 10`).

### DOC-005 · P2 · Расхождения документов
- **Шаги:** сверить `README.md`, `docs/BLUECASH_POS_INTEGRATION.md`, `docs/EXTERNAL_POS_INTEGRATION_PROTOCOL.md`, `POS/docs/05-fiscal-contract.md`, `88-pos-integration-rollout-checklist.md` с кодом: URL (`:reconcile` vs `/reconcile`), заголовки, коды ошибок, форматы УНП/референса. Расхождения — в таблицу `docs/MVP1/DOC_DRIFT.md` с решением «править документ/править код».
- **Приёмка:** `scripts/check-doc-drift.sh` — для каждой строки `DOC_DRIFT.md` статус `RESOLVED`; пустых `OPEN` нет к моменту HIL.
