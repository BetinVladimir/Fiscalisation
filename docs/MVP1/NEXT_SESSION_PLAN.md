# NEXT_SESSION_PLAN — план для следующей сессии (обновлён 2026-10-27)

Ветка: `docs/mvp1-delivery-pipeline-and-test-review`  
Последний коммит: `fdec71c`

---

## Baseline — проверить в начале сессии

```bash
# 51 Python-харнес должны пройти (1 known-fail: test_bug_026_2 — Gradle JUnit XML)
cd /Users/freelancer/Documents/Beeloy/Fiscalisation
python3 -m pytest tests/mvp1/ -q

# Go unit
cd fiscal-backend && go test ./internal/... -count=1 -timeout 120s
```

---

## Что закрыто и закоммичено (по состоянию на fdec71c)

### В docs/MVP1/fixed/ (31 файл)
BP-01, BUG-005, BUG-009, BUG-060, CL-01, DEMO-OTP-001, DEMO-OTP-UI-20261027,
DOC-006, DOC-V3, DSY-01, DSY-02 (fix-файл), PIPE-01, S3D-02, SCOPE-01, SCR-37,
TG-002, TST-GO-01…TST-GO-14, TST-META-01

### Python-харнесы (tests/mvp1/ — 47 файлов, 51 теста)
TST-GO-01..14, BUG-001.1-5, BUG-002.3, BUG-003.1-6, BUG-005.1-6, BUG-008.3,
BUG-010.1-3, BUG-011.1-3, BUG-026.2-3, ARCH-004.1-3, TG-009.1-4,
DSY-02.1-2, SMS-01.1-2

### Новый production-код
- `fiscal-backend/internal/domain/money.go`: `type Cents int64`, `ParseMoney`, `LineTotal`
- `fiscal-backend/internal/domain/money_test.go`: `TestLineTotalFromGoldenFile` — исправлен вызов `discountedLineTotalCents`
- `fiscal-backend/internal/domain/reversal_policy_test.go`: `TestReversalPolicy_SR01..SR04`
- `docs/SUPTO/STORNO_RULES.md`: таблица 11 правил SR-01..11 (чл.31 Н-18)
- `scripts/check-storno-coverage.sh`: проверка storno_coverage=pass

---

## IN_PROGRESS — что осталось сделать

### 1. BUG-001 · P0 · w1 · `fiscal-backend/internal/domain/`
**Харнесы есть и зелёные.** Нужна реализация:
- **1.3** — добавить `var moneyPattern = regexp.MustCompile(`^[0-9]{1,7}\.[0-9]{2}$`)` в `money.go`
- **1.4** — в `ValidateReceiptPlan`: проверка суммы Items через `Cents`, суммирование через half-up
- **1.5** — регрессия: `go test ./internal/domain/ ./internal/api/ -count=1`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-001
```

### 2. BUG-003 · P0 · w1 · `fiscal-backend/internal/domain/`
**Харнесы 1-6 есть и зелёные.** Нужна реализация:
- **3.3** — добавить `func driverIsSimulated(d Driver) bool { _, ok := d.(*SimulatedDriver); return ok }`
- **3.4** — в `service.go` (5 мест): `Simulated: true` → `Simulated: driverIsSimulated(driver)`
- **3.5** — в `exports.go` (2 места): `Simulated: true` → `Simulated: false`
- **3.6** — регрессия domain+api

```bash
python3 docs/MVP1/tools/task-pack.py BUG-003
```

### 3. ARCH-004 · P1 · w1 · `fiscal-backend/internal/domain/`
**money.go создан.** Нужна миграция:
- ~20 вызовов `receiptMoneyCents`/`parseFixed` в `service.go`, `edge_sync*.go`, `exports.go`,
  `receipt_saga.go`, `delegated_sale_prepare.go` → заменить на `ParseMoney`/`Cents`
- Ни один `strconv.ParseFloat` или `float64` не должен остаться в пакете

```bash
grep -rn 'receiptMoneyCents\|parseFixed\|ParseFloat\|float64' fiscal-backend/internal/domain/
```

### 4. DSY-02 · P0 · w3 · `IoT/protocol-abstraction/`, `IoT/firmware/edge-agent-s3/`
**Харнесы DSY-02.1-2 зелёные.** Нужна реализация:
- **DSY-02.1** — `FrameCodec.cpp::DaisyCodec::decode`: валидация SEQ 0x20…0xFF (не 0x7F!)
  и LEN = data+0x24 (не 0x2A); текст CP1251
- **DSY-02.2** — `serial_exchange.cpp`: тот же SEQ-диапазон на уровне S3

```bash
python3 docs/MVP1/tools/task-pack.py DSY-02
bash IoT/protocol-abstraction/run-tests.sh
```

### 5. BP-02 · P0 · w3 · `IoT/firmware/edge-agent-s3/idf/main/`
**BP-02.1 сделан (72 золотых кадра в fixtures).** Нужно:
- **BP-02.2** — `bluepad_ble_central.cpp::packet()`: сверить XOR с golden-файлом
- **BP-02.3** — `startVoidPurchase(amountCents,rrn,authId)` → теги 81, DF01, DF02

```bash
python3 docs/MVP1/tools/task-pack.py BP-02
bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh
```

### 6. BUG-011 · P1 · w1 · `fiscal-backend/internal/auth/`
**Харнесы 1-3 зелёные.** Нужна реализация в `auth.go`:
- **11.1** — если токен HMAC + среда PROD + роль ADMIN/PLATFORM_* → ошибка `HMAC_ADMIN_FORBIDDEN`
- **11.2** — проверить `exp-iat ≤ 24h` и поле `nbf`
- **11.3** — acceptance

```bash
python3 docs/MVP1/tools/task-pack.py BUG-011
```

### 7. BUG-054 · P1 · w5 · `POS/pos-app/` (TypeScript)
- **54.2** — сделан `AbortController` 4 с на чтение (уточнить)
- **54.3** — таймаут записи → `FiscalUnknownError(operationId)` (UNKNOWN ≠ FAILED!)

```bash
python3 docs/MVP1/tools/task-pack.py BUG-054
cd POS/pos-app && npx jest
```

### 8. BUG-059 · P1 · w5 · `POS/pos-gateway/` (Go)
- **59.1-59.4** — DLX `beeloy.dlx`, DLQ `pos-gateway.fiscal-results.dlq`;
  `Nack(false,false)` для невалидных конвертов

```bash
python3 docs/MVP1/tools/task-pack.py BUG-059
```

---

## NOT_STARTED с высоким приоритетом (волна ≤ 2)

| ID | P | w | Суть |
|---|---|---|---|
| W0-1 | P0 | 0 | MASTER_PLAN.md — критический обзор |
| W0-2 | P0 | 0 | ARCH_07 — критический обзор план 70% |
| W0-3 | P0 | 0 | ARCH_03 — E2E тест-матрица |
| NAP-N0 | P0 | 0 | Скачать актуальный Н-18, ДВ 115/2025, ДВ 82/2026 |
| SCR-40 | P0 | 0 | HIL-скрипт: fixtures + парсер ARCH_19 |
| TG-007 | P0 | 2 | POS: FiscalIntentPayload JSON-схема + contract-test |
| TG-001 | P0 | 2 | Postgres: `TestRestartKeepsReservedOperations` + WithTx |
| ARCH-001 | P0 | 2 | domain: нормализованные таблицы (зависит от TG-001) |

---

## Заблокировано (не трогать)

| ID | Причина |
|---|---|
| DOC-V4 / SMS-01.2 | ждёт vendor-ответов Q-S1..Q-S8, Q-P1..Q-P6 |
| PA-03 | внешний репо `../BeeloyBackend/` |
| TG-009 SR-05..11 | GAP — специальные УНП, нужна инструкция НАП |

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
