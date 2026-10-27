# NEXT_SESSION_PLAN — план для следующей сессии (обновлён 2026-10-27)

Ветка: `docs/mvp1-delivery-pipeline-and-test-review`  
Последний коммит в Fiscalisation: `fdec71c`  
Последний коммит в POS: не коммичено (см. ниже)

---

## Шаг 0 — закоммитить накопленные правки (до любой работы)

В Fiscalisation есть незакоммиченные изменения:

```bash
# Проверить
cd /Users/freelancer/Documents/Beeloy/Fiscalisation
git status --short

# Закоммитить (по одному блоку):
git add fiscal-backend/internal/domain/money_test.go
git add fiscal-backend/cmd/fiscal-backend/repo.go fiscal-backend/cmd/fiscal-backend/main_test.go
git add IoT/common-modules/datecspay/tests/DatecsPayTest.cpp
git commit -m "Fix ARCH-004 money_test, add TG-001.6 repo guard, BP-02.3 void-purchase tag order"
```

В POS (отдельный git):
```bash
cd /Users/freelancer/Documents/Beeloy/POS
git add docs/contracts/ pos-gateway/internal/contracts/ Makefile
git commit -m "TG-007: FiscalIntentPayload schema, lock, contract tests, Makefile"
```

---

## Baseline — проверить перед работой

```bash
# 51 Python-харнес (1 known-fail: test_bug_026_2 — Gradle JUnit XML)
cd /Users/freelancer/Documents/Beeloy/Fiscalisation
python3 -m pytest tests/mvp1/ -q

# Go unit
cd fiscal-backend && go test ./internal/... -count=1 -timeout 120s

# Go cmd (TG-001.6)
go test ./cmd/fiscal-backend -count=1

# POS contract tests (TG-007)
cd /Users/freelancer/Documents/Beeloy/POS && make contract-test

# Datecs Pay C++ (BP-02.3)
bash IoT/common-modules/datecspay/run-tests.sh

# Protocol abstraction (DSY-02)
bash IoT/protocol-abstraction/run-tests.sh
```

---

## Что закрыто по результатам этой сессии (ещё не закоммичено)

| Задача | Статус | Файлы |
|--------|--------|-------|
| ARCH-004 | ✓ тест исправлен | `fiscal-backend/internal/domain/money_test.go` |
| BP-02.3 | ✓ tag-order тест | `IoT/common-modules/datecspay/tests/DatecsPayTest.cpp` |
| TG-001.6 | ✓ guard + тест | `fiscal-backend/cmd/fiscal-backend/repo.go`, `main_test.go` |
| TG-007 | ✓ ПОЛНОСТЬЮ | `POS/docs/contracts/fiscal-intent.v1.json`, `.lock`, `pos-gateway/internal/contracts/fiscal_intent_contract_test.go`, `POS/Makefile` |

---

## IN_PROGRESS — выполнить в следующей сессии (по приоритету)

### 1. BUG-001 · P0 · `fiscal-backend/internal/domain/`
Харнесы зелёные. Нужна реализация:
- `money.go`: добавить `var moneyPattern = regexp.MustCompile(`^[0-9]{1,7}\.[0-9]{2}$`)`
- `ValidateReceiptPlan`: суммировать Items через `Cents`, half-up; отклонять строки с «+» / «01»
- Регрессия: `go test ./internal/domain/ ./internal/api/ -count=1`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-001
```

### 2. BUG-003 · P0 · `fiscal-backend/internal/domain/`
Харнесы зелёные. Нужна реализация:
- `service.go`: добавить `func driverIsSimulated(d Driver) bool { _, ok := d.(*SimulatedDriver); return ok }`
- Заменить `Simulated: true` (5 мест в `service.go`) → `Simulated: driverIsSimulated(driver)`
- `exports.go:105,151`: `Simulated: true` → `Simulated: false`
- Регрессия: `go test ./internal/domain/ ./internal/api/ -count=1`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-003
```

### 3. ARCH-004 · P1 · `fiscal-backend/internal/domain/`
`money.go` создан и тесты исправлены. Осталась миграция ~20 вызовов:
```bash
grep -rn 'receiptMoneyCents\|parseFixed\|ParseFloat\|float64' \
  fiscal-backend/internal/domain/ --include='*.go'
```
Заменить на `ParseMoney`/`Cents`. Ни одного `float64` не должно остаться в пакете `domain`.

### 4. DSY-02 · P0 · `IoT/`
Daisy-кодек исправлен (LEN). Осталось:
- SEQ-диапазон 0x20…0xFF на уровне S3: `IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp`
- CP1251 для Daisy в C++ (только `DaisyPrinter` — проверить)

```bash
python3 docs/MVP1/tools/task-pack.py DSY-02
bash IoT/protocol-abstraction/run-tests.sh
```

### 5. BUG-011 · P1 · `fiscal-backend/internal/auth/`
Харнесы зелёные. Нужна реализация `auth.go`:
- HMAC + PROD + роль ADMIN/PLATFORM_* → `HMAC_ADMIN_FORBIDDEN`
- `exp-iat ≤ MaxTTL (24h)` + проверка `nbf`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-011
```

### 6. BUG-054 · P1 · `POS/pos-app/` (TypeScript)
AbortController 4 с есть. Осталось:
- Таймаут записи → `FiscalUnknownError(operationId)` (инвариант UNKNOWN ≠ FAILED)
- Тест TST-POS-05

```bash
python3 docs/MVP1/tools/task-pack.py BUG-054
cd /Users/freelancer/Documents/Beeloy/POS/pos-app && npx jest
```

### 7. BUG-059 · P1 · `POS/pos-gateway/` (Go)
- DLX `beeloy.dlx` + DLQ `pos-gateway.fiscal-results.dlq`
- `Nack(false,false)` для невалидных конвертов

```bash
python3 docs/MVP1/tools/task-pack.py BUG-059
```

### 8. BP-02.2 · P0 · `IoT/firmware/edge-agent-s3/idf/`
- `bluepad_ble_central.cpp::packet()`: тест chunked assembly с XOR-сверкой по golden-файлу

```bash
python3 docs/MVP1/tools/task-pack.py BP-02
bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh
```

---

## NOT_STARTED — высокий приоритет (волна ≤ 2)

| ID | P | w | Суть |
|---|---|---|---|
| W0-1 | P0 | 0 | MASTER_PLAN.md — критический обзор |
| W0-2 | P0 | 0 | ARCH_07 — критический обзор план 70% |
| W0-3 | P0 | 0 | ARCH_03 — E2E тест-матрица |
| NAP-N0 | P0 | 0 | Скачать актуальный Н-18, ДВ 115/2025, ДВ 82/2026 с nap.bg |
| SCR-40 | P0 | 0 | HIL-скрипт: `scripts/check-hil-matrix.py` |
| TG-001.1,1.3,1.4 | P0 | 2 | Postgres concurrent tests (требуется запущенный Postgres) |
| TG-001.5 | P0 | 2 | `docs/decisions/ADR-PERSIST-001.md` с разделом `## Decision` |
| ARCH-001 | P0 | 2 | domain: нормализованные таблицы (зависит от TG-001) |
| BUG-002 | P1 | 1 | `validMoney` по-прежнему принимает префикс +/− в `service.go:1540` |
| BUG-050 | P0 | 5 | POS вызывает `:reconcile`, шлюз принимает `/reconcile` |
| BUG-055 | P0 | 5 | шлюз финализирует платёж как CASH (`pos-gateway/internal/drain/idle.go`) |

---

## Заблокировано (не трогать)

| ID | Причина |
|---|---|
| DOC-V4 / SMS-01.2 | ждёт vendor-ответов Q-S1..Q-S8, Q-P1..Q-P6 |
| PA-03 | внешний репо `../BeeloyBackend/` |
| TG-009 SR-05..11 | GAP — специальные УНП, нужна инструкция НАП |
| NAP-N0 | требуется ручное скачивание с nap.bg |

---

## Инварианты — не нарушать

1. Деньги — только целые центы; half-up `(unit¢×qty_milli+500)/1000`.
2. UNKNOWN ≠ FAILED: после таймаута — только lookup, без повтора и без возврата.
3. Сторно-first: возврат по карте только после подтверждённого фискального сторно.
4. Повтор с тем же ключом не создаёт второй документ.
5. Ошибка хранилища останавливает денежный побочный эффект.
6. Номер документа — только из ответа ФУ.
7. Datecs: LEN=data+0x2A, SEQ 0x20…0x7F. Daisy: LEN=data+0x24, SEQ 0x20…0xFF.
8. Секреты не коммитить и не логировать.
9. Числа в тестах — из `docs/MVP1/fixtures/*` или золотого чека (1367¢ = 1200+167).
10. Парсеры не бросают исключения наружу: кадр или типизированная ошибка.
