# NEXT_SESSION_PLAN — план для следующей сессии (обновлён 2026-10-27 сессия 2)

Ветки:
- `Fiscalisation`: `docs/mvp1-delivery-pipeline-and-test-review` (ожидается коммит после сессии)
- `POS`: `main` (ожидается коммит после сессии)

---

## Baseline — проверить в начале сессии

```bash
# 51 Python-харнес (1 known-fail: test_bug_026_2 — Gradle JUnit XML)
cd /Users/freelancer/Documents/Beeloy/Fiscalisation
python3 -m pytest tests/mvp1/ -q

# Go fiscal-backend (все пакеты)
cd fiscal-backend && go test ./... -count=1 -timeout 120s

# POS contract tests
cd /Users/freelancer/Documents/Beeloy/POS && make contract-test

# POS pos-gateway
cd pos-gateway && go test ./... -count=1

# POS pos-app
cd pos-app && npx jest --no-coverage

# C++ Datecs Pay
bash IoT/common-modules/datecspay/run-tests.sh

# Protocol abstraction
bash IoT/protocol-abstraction/run-tests.sh

# BluePad BLE host
cd IoT/firmware/edge-agent-s3/idf/tests && python3 bluepad_host/run.py
```

---

## Что закрыто в этой сессии (сессия 2)

| Задача | Статус | Файлы |
|--------|--------|-------|
| **BUG-002** | ✓ уже было | `bug_002_test.go` зелёный, `validMoney` корректен |
| **BUG-005** | ✓ уже было | `DeviceTime` → `DEVICE_CLOCK_NOT_MEASURED`, 5 тестов green |
| **BUG-050** | ✓ | `Adapters.ts:109` `:reconcile` → `/reconcile`, тест green |
| **BUG-055** | ✓ | `finalize_cmd.go` + `idle.go` — реальные payments из intent, 3 теста green |
| **BUG-057** | ✓ | `resolveOperationID` + 422 на конфликт, 3 теста green |
| **DSY-03** | ✓ | `profile_orchestrator.cpp` Unconfigured guard + `profile_executor.cpp` switch; тест `profile_unconfigured_no_io` green |

---

## IN_PROGRESS — выполнить в следующей сессии

### 1. TG-001.1,1.3,1.4 · P0 · `fiscal-backend/internal/persistence/`
Postgres concurrent tests (требуется запущенный Postgres).
```bash
python3 docs/MVP1/tools/task-pack.py TG-001
```

### 2. TG-001.5 · P0 · `docs/decisions/`
ADR-PERSIST-001.md с разделом `## Decision` (choice между MemoryRepo и Postgres).

### 3. BUG-020/021/026/028/029/032/072 · P0 · BlueCash
BlueCash: нет mutex, синтетический референс, UNKNOWN по тексту, нет SYN/NAK/timeout, нет ForegroundService.
```bash
python3 docs/MVP1/tools/task-pack.py BUG-020
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
| ARCH-001 | P0 | 2 | domain: нормализованные таблицы (зависит от TG-001) |
| BUG-020 | P0 | 3 | BlueCash: нет отмены открытого чека при сбое |
| BUG-021 | P0 | 3 | BlueCash: компенсация карты при любой ошибке (нарушение UNKNOWN≠FAILED) |
| BUG-026 | P0 | 3 | BlueCash: синтетический референс "DATECS-"+operationId |
| BUG-028 | P0 | 3 | BlueCash: классификация UNKNOWN по тексту сообщения |
| BUG-029 | P0 | 3 | BlueCash: DatecsSocketProtocol.execute без SYN/NAK/таймаута |
| BUG-032 | P0 | 3 | BlueCash: AndroidManifest без ForegroundService разрешений |
| BUG-072 | P0 | 3 | BlueCash: нет блокировки между транспортами |
| EC-02 | P0 | 4 | erp-connector: нет состояния UNKNOWN |
| AA-01 | P0 | 4 | AdminApp: `crypto.randomUUID()` вместо серверного ID |
| BF-01 | P0 | 4 | BeeFiscalApp: `authenticate()` заглушка |
| MQ-02 | P0 | 5 | POS: нативный BLE-модуль отсутствует |

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
