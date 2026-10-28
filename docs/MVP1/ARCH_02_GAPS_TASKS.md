# ARCH_02 — Технологические разрывы и недоработки (декомпозиция для слабой модели)

Каждая задача: **ID · Приоритет · Файлы · Шаги (атомарные) · Тест · Приёмка (автоматическая) · DoD**.
Правила исполнения и общий DoD (CDOD) — [ARCH_00_INDEX.md](ARCH_00_INDEX.md). Баги — в [ARCH_01_BUGS.md](ARCH_01_BUGS.md); здесь только то, что **отсутствует** или не доведено.
Правило для слабой модели: один шаг = одна правка в одном файле; после каждого шага запускать указанную команду; не менять файлы вне списка.

| ID | P | Разрыв | Зависит |
|---|---|---|---|
| TG-001 | P0 | Персистентность: снимок целого состояния вместо транзакционных записей | — |
| TG-002 | P0 | Уникальность УНП и счётчика на уровне БД | TG-001 |
| TG-003 | P0 | Реальная синхронизация часов ФУ (MQTT-команда DEVICE_TIME) | BUG-005 |
| TG-004 | P0 | Типизированные ошибки транспорта Datecs + таймауты/SYN/NAK | BUG-028/029 |
| TG-005 | P0 | Команда отмены чека и восстановление состояния ФУ (BlueCash) | BUG-020 |
| TG-006 | P0 | Разбор ответов Datecs (номер документа, итоги, ошибки) по фикстурам вендора | BUG-026 |
| TG-007 | P0 | Сквозной контракт POS → шлюз → backend (платежи, типы, суммы) | BUG-050/051/055 |
| TG-008 | P1 | Электронные фискальные чеки (п.24) — после решения NORM-01 | — |
| TG-009 | P1 | Сторно: сроки, основания, партиальное сторно, ограничения Н-18 | BUG-008 |
| TG-010 | P1 | Рестарт и восстановление «висящих» операций (reconciler) | TG-001 |
| TG-011 | P1 | Наблюдаемость: метрики, алерты, аудит-лог без PII | — |
| TG-012 | P1 | Секреты, ротация ключей, mTLS MQTT, TLS локального HTTP | BUG-009/011/060 |
| TG-013 | P1 | Версионирование контрактов и lock-проверки в CI | — |
| TG-014 | P1 | HIL-стенд и сценарии с реальными ФУ | TG-004..006 |
| TG-015 | P2 | Миграции БД: проверка отката и совместимости | TG-001 |
| TG-016 | P2 | Нагрузочные и отказоустойчивые тесты | TG-001 |

---

### TG-001 · P0 · Персистентность состояния
- **Место:** `fiscal-backend/internal/domain/repository.go` (MemoryRepository), `internal/domain/pg_*`/`postgres_*` (версионированное хранилище), `database/fiscal/`.
- **Разрыв:** состояние сериализуется целиком (JSON-снимок) — при конкуренции записей возможна потеря обновлений («last write wins»), рост размера снимка, отсутствие межсервисных транзакций (операция + событие + replay-ключ + УНП должны фиксироваться атомарно).
- **Шаги:**
  1. Составить список агрегатов, меняемых одной бизнес-операцией: `grep -n "func (r \*MemoryRepository)" fiscal-backend/internal/domain/repository.go`; записать в `docs/MVP1/decisions/ADR-PERSIST-001.md` таблицу «агрегат → таблица → ключ → уникальные индексы».
  2. Написать миграции `database/fiscal/NNN_normalized_state.sql` (таблицы `operation`, `operation_event`, `unp_allocation`, `idempotency_record`, `outbox`); `UNIQUE(tenant_id, unp)`, `UNIQUE(tenant_id, idempotency_key)`.
  3. Реализовать `PostgresRepository` с методом `WithTx(ctx, func(tx Repository) error)`; все составные операции (`CommitOperationEvent`, `ReserveUNP`) выполнять внутри одной транзакции.
  4. Оставить `MemoryRepository` только для тестов; в `cmd/fiscal-backend/main.go` запретить его в `AppEnv != dev` (fatal).
  5. Включить параллельные тесты (-race) на двойную финализацию одного `idempotency_key`.
- **Тест:** `repository_contract_test.go` — один и тот же набор тестов запускается для Memory и Postgres (testcontainers): `TestConcurrentFinalizeSameKeyProducesOneOperation`, `TestUNPUniqueAcrossConcurrentAllocations`, `TestTxRollbackLeavesNoPartialState`.
- **Приёмка (автоматическая):**
  - `cd fiscal-backend && go test ./internal/domain -run 'RepositoryContract' -race -count=1` → ok для обеих реализаций;
  - `psql -c "\d unp_allocation"` показывает уникальный индекс (скрипт `scripts/check-db-schema.sh` падает, если индекса нет);
  - `grep -n "MemoryRepository" fiscal-backend/cmd/fiscal-backend/main.go` — только внутри ветки `AppEnv=="dev"`.
- **DoD:** CDOD + ADR + миграция применяется и откатывается (TG-015).

### TG-002 · P0 · Уникальность УНП и счётчика
- **Место:** `internal/domain/unp_allocator.go`; формат `AA123456-0001-0000001` (serial 8, оператор 4, последовательность 7).
- **Разрыв:** гарантия уникальности и монотонности держится на коде приложения; нет ограничения в БД; нет политики для дыр в нумерации (отменённое резервирование — BUG-040). *Уточнение по ARCH_08:* проверка переполнения 7-значного счётчика уже есть (`BGUNPSequenceMax` в `regulatory_identifier.go`, ошибка «UNP range exhausted» в `unp_allocator.go`) — нужен только тест.
- **Шаги:** (1) переполнение 7-значного счётчика уже обрабатывается (`BGUNPSequenceMax`, ошибка «UNP range exhausted» — ARCH_08): добавить только тест; (2) в БД — `UNIQUE (tenant_id, device_serial, operator_code, sequence)`; (3) документ `docs/SUPTO/UNP_GAPS_POLICY.md`: когда дыра допустима, как фиксируется (запись `unp_gap` с причиной); (4) тест на отсутствие повторного использования после рестарта.
- **Тест:** `TestUNPAllocatorExhaustion`, `TestUNPAllocatorNoReuseAfterRestart`, `TestUNPFormatRegex` (`^[A-Z0-9]{8}-[0-9]{4}-[0-9]{7}$`).
- **Приёмка:** `go test ./internal/domain -run UNP -race -count=1` ok; SQL-проверка уникального индекса.
- **DoD:** CDOD + политика дыр утверждена (ссылка на ADR).

### TG-003 · P0 · Реальная синхронизация часов
- См. BUG-005. Разбито на шаги для исполнения:
  1. `contracts/` — добавить в схему результата команды `DEVICE_TIME` поля `device_time` (RFC3339), `drift_seconds` (int); обновить `contracts/*.lock`.
  2. `internal/mqttclient/client.go` — `DeviceTime` публикует команду и ожидает результат с таймаутом 10 с (корреляция по `operation_id`).
  3. `service.go: SyncWorkstationClock` — использовать драйвер регистра; порог 30 с; запись события `CLOCK_SYNCED` с `drift_before/after`.
  4. Прошивка S3/BlueCash: обработчик `DEVICE_TIME` (чтение времени ФУ; для Datecs — команда чтения даты-времени, номер из PDF).
- **Тест:** `TestSyncWorkstationClockMeasuresDeviceClock`, `TestPaymentBlockedWithoutDailySync` (платёж отклонён кодом `CLOCK_SYNC_REQUIRED`).
- **Приёмка:** `go test ./internal/domain ./internal/mqttclient -run Clock -count=1`; `make iot-test`; `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Clock*'`.
- **DoD:** CDOD; пункт Приложения 29 (синхронизация времени) → `SOFTWARE_VERIFIED_HIL_PENDING` в `MVP_GATES.md`.

### TG-004 · P0 · Протокол Datecs: транспорт
- См. BUG-028/029. Шаги: типизированные ошибки → дедлайны → SYN → NAK → resync SEQ → тесты с эмулятором. Файлы: `SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt`, `DatecsAndroidPorts.kt`, `tests/host/DatecsCodecVendorHost.kt`.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*Datecs*'`; HIL `HIL-BC-02`.
- **DoD:** CDOD; закрыть GAP-011.

### TG-005 · P0 · Отмена чека и восстановление ФУ
- См. BUG-020/031. Дополнительно: при старте приложения выполнять «проверку состояния ФУ» (команда статуса): если есть открытый чек — отменить (с записью в журнал) до приёма новых команд.
- **Тест:** `startupCancelsDanglingReceipt`. **Приёмка:** gradle-тест ok; HIL `HIL-BC-05` (выдернуть питание между 49 и 56).
- **DoD:** CDOD.

### TG-006 · P0 · Разбор ответов Datecs
- **Файлы:** `DatecsProtocol.kt` (`DatecsResponses` — создать), `tests/host/fixtures/datecs/*.hex`.
- **Шаги:** (1) из PDF `Datecs FP protocol` выписать структуры ответов команд 48, 49, 53, 56, 43, 69, 70, 38/42/39 в `docs/DATECS_RESPONSES.md` (с номерами страниц); (2) для каждой — data class и парсер; (3) фикстуры реальных ответов (HIL-логи) → тесты; (4) парсер возвращает типизированную ошибку при неизвестном формате.
- **Тест:** `DatecsResponsesTest` — по тесту на команду + «битый» ответ.
- **Приёмка:** `./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsResponses*'` ok; `docs/DATECS_RESPONSES.md` содержит ссылки на страницы PDF (grep `стр.`).
- **DoD:** CDOD.

### TG-007 · P0 · Сквозной контракт POS ↔ шлюз ↔ backend
- Подробно — [ARCH_05](ARCH_05_POS_INTEGRATION.md). Шаги: (1) единая схема `FiscalIntentPayload` в `POS/docs/contracts/fiscal-intent.v1.json`; (2) генерация типов TS и Go из схемы; (3) контрактные тесты на обеих сторонах; (4) E2E `TST-E2E-*`.
- **Приёмка:** `cd POS && make contract-test` (создать цель) → exit 0; `npx tsc --noEmit` в pos-app; `go test ./...` в pos-gateway.
- **DoD:** CDOD + contract-lock обновлён.

### TG-008 · P1 · Электронные фискальные чеки (Прил. 29 п.24) — см. NAP-13 в ARCH_08
- **Основание (официальный текст):** чл. 3(18), 52з(17), Прил. 29 п.24.1–24.3, Прил. 32 п.2а. **QR в п.24 не упоминается** (первая редакция этой задачи ошибочно предполагала QR).
- **Сначала решение (D-6/NORM-01):** оставить вне MVP (статус `OUT_OF_MVP_SCOPE`, технический запрет режима) или реализовать.
- **Если реализовать:** (1) отправка сформированного чека на e-mail клиента или в его профиль (сайт/приложение); бумага — по требованию; (2) просмотр и экспорт по структуре п.24.2 (УНП, дата/время открытия и закрытия, сумма, идентификатор клиента, способ предоставления, e-mail получателя); (3) запись факта предоставления по правилам п.15 (УНП, дата, время, способ, адрес) — п.24.3; (4) выдача только после `FISCALIZED`.
- **Файлы:** `fiscal-backend/internal/domain` (новый `electronic_receipt.go`), `internal/emailworker`, `exports.go`, `contracts/supto-feature-scope.json`, `POS/pos-app` (ввод контакта клиента).
- **Тест:** `TestElectronicReceiptRequiresFiscalized`, `TestElectronicReceiptDeliveryRecorded`, `TestElectronicExportColumnsMatchAnnex29p24`, при отключении — `TestElectronicReceiptsRejectedWhenDisabled`; POS: `electronicReceipt.test.ts`.
- **Приёмка:** `go test ./internal/domain ./internal/api -run ElectronicReceipt -count=1`; `npx jest -t electronicReceipt`; E2E TST-E2E-16.
- **DoD:** CDOD.

### TG-009 · P1 · Сторно (чл. 31 Н-18) — см. NAP-01…03 в ARCH_08
- **Место:** `internal/domain/reversal_policy.go` (**срок «ошибка оператора» до 7-го числа следующего месяца уже реализован** в Europe/Sofia), `service.go:230-300`.
- **Разрыв (после сверки с текстом чл. 31):** нет специальных УНП (31(11,12)), нет обработки отказа ФУ по кассовой наличности (31(3)), не определён возврат на карту через ФУ (31(3а),(4)), не зафиксированы исключения 31(7,8); таблица правил с источником отсутствует.
- **Шаги:** (1) `docs/SUPTO/STORNO_RULES.md` — таблица правил с точными ссылками на алинеи (источник — ARCH_08 §3.2); (2) для каждой строки тест `TestReversalPolicy_<Case>`; (3) реализовать NAP-01…03 после решений NORM-04/NORM-10.
- **Приёмка:** `go test ./internal/domain -run ReversalPolicy -count=1` ok; число тестов ≥ числа строк таблицы (`scripts/check-storno-coverage.sh`, создать).
- **DoD:** CDOD.

### TG-010 · P1 · Reconciler
- **Место:** `internal/domain` (новый `reconciler.go`), `cmd/fiscal-backend/main.go`.
- **Разрыв:** операции в `EXECUTING`/`UNKNOWN` не пересматриваются автоматически.
- **Шаги:** (1) периодический воркер (интервал из конфига) выбирает операции старше N минут; (2) для каждой — запрос статуса у драйвера/устройства; (3) переход в терминальное состояние или эскалация (алерт); (4) лидер-выбор (advisory lock PG), чтобы не дублировать.
- **Тест:** `TestReconcilerResolvesExecutingOperations`, `TestReconcilerEscalatesAfterMaxAttempts`.
- **Приёмка:** `go test ./internal/domain -run Reconciler -race -count=1`.
- **DoD:** CDOD + метрика `fiscal_operations_stuck`.

### TG-011 · P1 · Наблюдаемость
- **Разрыв:** нет метрик Prometheus по операциям, задержкам MQTT, UNKNOWN-результатам; нет корреляции `operation_id`/`unp` в логах; риск PII (EGN, e-mail) в логах.
- **Шаги:** (1) `/metrics`; счётчики: `fiscal_operations_total{state}`, `fiscal_unknown_total`, `mqtt_command_latency_seconds`; (2) middleware логирования с маской PII; (3) алерты в `docs/runbooks/ALERTS.md`.
- **Тест:** `TestMetricsEndpointExposesOperationCounters`, `TestLogMaskingHidesEmailAndEGN`.
- **Приёмка:** `go test ./internal/api -run 'Metrics|LogMasking' -count=1`; `curl -s localhost:8080/metrics | grep fiscal_operations_total`.
- **DoD:** CDOD.

### TG-012 · P1 · Секреты и каналы
- Объединяет BUG-009/011/032/060: ротация HMAC, TLS+mTLS для MQTT (`EMQX`), TLS локального HTTP BlueCash, хранилище пароля оператора ФУ, EAS secrets.
- **Приёмка:** `scripts/check-no-secrets.sh` exit 0; `go test ./internal/config -count=1`; проверка конфигурации EMQX (`emqx_ctl listeners`) в `scripts/check-mqtt-tls.sh`.
- **DoD:** CDOD + runbook ротации.

### TG-013 · P1 · Контракты и CI
- **Место:** `.github/workflows`, `contracts/`, `Makefile`.
- **Разрыв:** контракт-lock проверяется не во всех компонентах (S3, BlueCash, POS-gateway).
- **Шаги:** добавить в каждый компонент цель `contract-check`, подключить в CI-матрицу, хранить SHA схем в `contracts/*.lock`.
- **Приёмка:** `make contract-test` exit 0 локально; в CI джоба `contracts` обязательна (`required` в branch protection — проверить вручную, зафиксировать в `docs/`).
- **DoD:** CDOD.

### TG-014 · P1 · HIL-стенд
- Сценарии — ARCH_03 (раздел HIL). Шаги: (1) `docs/MVP1/HIL_RUNBOOK.md` — подключение, тестовые данные, чек-лист; (2) скрипт `scripts/hil/run.sh <scenario>` сохраняет логи и журнал в `artifacts/hil/<date>/`; (3) шаблон протокола результатов.
- **Приёмка:** `scripts/hil/run.sh --dry-run HIL-BC-01` → exit 0 и создаёт каталог артефактов; после реального прогона — файл `result.json` со статусом `PASS|FAIL` валидируется схемой `contracts/hil-result.schema.json`.
- **DoD:** CDOD + заполнены `MVP_GATES.md`.

### TG-015 · P2 · Миграции
- **Шаги:** проверка применения «с нуля» и «с предыдущей версии», откат; CI-джоба `db-migrations`.
- **Приёмка:** `scripts/check-migrations.sh` (up → down → up) exit 0.

### TG-016 · P2 · Нагрузка и отказоустойчивость
- **Шаги:** k6/vegeta-сценарий: 50 rps финализации с повторами; «хаос»: рестарт БД/EMQX/RabbitMQ посреди нагрузки; проверка: нет дублей УНП, нет потерянных операций.
- **Приёмка:** `scripts/loadtest/run.sh` завершается с `duplicates=0 lost=0`.

### TG-017 · P0 · Вендорные разрывы (сводная ссылка)
Технологические разрывы, найденные при сверке с вендорной спецификацией (ARCH_09, VG-01…VG-20) и последними нормами НАП (ARCH_10, NAP-16…NAP-23, NAP-N0), оформлены там полными карточками и подзадачами (`execution/status.json`, поле `subtasks`). Порядок: W0 (VG-19, NAP-N0, запросы DOC-V*) → W3 (VG-20, VG-01, VG-02, VG-03, VG-04, VG-05, VG-06, VG-07, VG-13) → W4 (VG-09, VG-10, VG-11) → W7 (VG-08, VG-12, VG-14…VG-17) → W9 (NAP-17, NAP-22, повтор NAP-N0).
- **Приёмка (автоматическая):** `python3 -c "import json;d=json.load(open('docs/MVP1/execution/status.json'));print(all(any(t['id']==f'VG-{i:02d}' for t in d['tasks']) for i in range(1,21)))"` печатает `True`.
- **DoD:** CDOD; все VG-* в статусе DONE, NOT_A_BUG или BLOCKED_SPEC с владельцем в `notes`.
