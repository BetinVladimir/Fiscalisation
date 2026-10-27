# NEXT_SESSION_PLAN — план для следующей сессии (обновлён 2026-10-27)

Ветка: `docs/mvp1-delivery-pipeline-and-test-review`

---

## Что закрыто за две сессии

### Перенесено в `docs/MVP1/fixed/` (29 задач)

**Из status.json DONE (13):** BP-01, BUG-009, BUG-060, CL-01, DOC-006, DOC-V3, DSY-01,
PIPE-01, S3D-02, SCOPE-01, SCR-37, TG-002, TST-META-01

**Завершено за сессии (16):**
- TST-GO-01…TST-GO-14 — все 14 Go-харнесов зелёные (15 passed)
- DEMO-OTP-001, DEMO-OTP-UI-20261027 — закоммичены в `716bec4`

### Созданные (не закоммиченные) артефакты

**Go-тесты (`fiscal-backend/internal/`):**
`domain/bug_002_test.go`, `domain/bug_008_test.go`, `domain/fiscal_operation_test.go`,
`domain/clock_sync_test.go`, `domain/repository_contract_test.go`,
`domain/payment_capability_test.go`, `domain/rounding_golden_test.go`,
`api/health_test.go`, `api/idempotency_test.go`

**Python-харнесы (`tests/mvp1/`):**
`test_tst_go_01_1.py` … `test_tst_go_14_1.py` (14 штук),
`test_bug_001_4.py`, `test_bug_001_5.py`,
`test_bug_002_3.py`, `test_bug_003_2_through_3.py`, `test_bug_003_4.py`,
`test_bug_008_3.py`, `test_bug_010_3.py`, `test_bug_026_2.py`

**Kotlin (`SmartDevices/bluecash-app/`):**
`BlueCashEngineTest.kt` — добавлены 2 теста BUG-026.2;
`BlueCashEngine.kt` — парсер поля документа cmd56/cmd43 исправлен на `getOrNull(1)?.takeIf { it.isNotBlank() }` (строки 265, 359)

---

## Открытые задачи с частичной реализацией

### BUG-026 · P0 · `SmartDevices/bluecash-app/`

**Сделано полностью:**
- 26.1 — удалён синтетический `"DATECS-"` (grep = 0)
- 26.2 — два Kotlin-теста + Python harness `test_bug_026_2.py`; Gradle 12/0/0 ✓
  - Исправление: `lastOrNull()` → `getOrNull(1)?.takeIf { it.isNotBlank() }` в двух местах BlueCashEngine.kt

**Осталось:**
- **26.3** — запустить полный набор (`./gradlew :bluecash-app:test`) и убедиться, что все
  варианты сборки зелёные; создать `test_bug_026_3.py` (harness с JUnit XML-проверкой
  `saleResultCarriesRealDocumentNumber` и `emptyCloseResponseIsNotSuccess` — эти имена взять из
  BUG-026.md, если они отличаются от 26.2, иначе 26.3 можно закрыть сразу).

```bash
cd SmartDevices && ./gradlew :bluecash-app:test --no-daemon 2>&1 | grep -E 'PASS|FAIL|tests'
```

---

### BUG-003 · P0 · `fiscal-backend/internal/domain/`

**Сделано:** subtasks 1-4 — `simulated_flag_test.go`, code уже правильный (5 мест в service.go),
харнесы `test_bug_003_2_through_3.py` и `test_bug_003_4.py` зелёные.

**Осталось — subtasks 5, 6:**
Прочитать `docs/MVP1/fixes/BUG-003.md`, найти что требуется в 5 и 6:
обычно это регрессия всего пакета domain + exports, и E2E/интеграционный тест.
Создать `test_bug_003_5.py` и `test_bug_003_6.py` по шаблону уже созданных харнесов.

```bash
python3 docs/MVP1/tools/task-pack.py BUG-003
```

---

### BUG-011 · P1 · `fiscal-backend/internal/auth/`

**BLOCKED в прошлой сессии.** `HMAC_ADMIN_FORBIDDEN` не существует в коде.

**Осталось (все 3 подзадачи):**
- **11.1** — в `auth.go:55-58` добавить: если токен HMAC и среда PROD и роль ADMIN/PLATFORM_* →
  вернуть ошибку `HMAC_ADMIN_FORBIDDEN`
- **11.2** — проверить `exp-iat ≤ 24h` и поле `nbf`; харнес `test_bug_011_2.py`
- **11.3** — acceptance; харнес `test_bug_011_3.py`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-011
```

---

### BUG-005 · P1 · `fiscal-backend/internal/`

**Сделано:** subtask 5 — `TestDeviceTimeReturnsErrorWhenNotMeasured` в `client_test.go`.

**Осталось (subtasks 1-4, 6):**
- WARN при дрейфе >5 мин, BLOCK при >2 ч до card/fiscal I/O
- `business_date` Europe/Sofia с учётом DST
- host-clock fallback отвергнут (используется только часовой пояс ФУ)
- Тесты: `TestClockDriftOver5MinWarn`, `TestClockDriftOver2HBlock`, `TestBusinessDateTimezone`

```bash
python3 docs/MVP1/tools/task-pack.py BUG-005
```

---

### BUG-032 · P1 · `SmartDevices/bluecash-app/`

**Сделано:** BLUETOOTH_SCAN и BLUETOOTH_CONNECT в `AndroidManifest.xml`.

**Осталось (6 подзадач):**
- **32.1** — дополнить манифест: `FOREGROUND_SERVICE`, `FOREGROUND_SERVICE_CONNECTED_DEVICE`,
  `RECEIVE_BOOT_COMPLETED`, `POST_NOTIFICATIONS`, `WAKE_LOCK`; объявить `BlueCashRuntimeService`
  и `BootReceiver`; харнес `test_bug_032_1.py`
- **32.2** — создать `BlueCashRuntimeService : Service` со `startForeground`
- **32.3** — перенести `startManagedRuntime` из `MainActivity` → `RuntimeHost`
- **32.4** — тест (уточнить из BUG-032.md)
- **32.5** — пароль оператора: `inputType=textPassword` + Keystore-обёртка
- **32.6** — HTTP 8088: bind только на разрешённый интерфейс

```bash
python3 docs/MVP1/tools/task-pack.py BUG-032
```

---

## IN_PROGRESS в status.json (не трогали)

| ID | P | Описание |
|---|---|---|
| ARCH-004 | P1 | money.go — перевести 20 вызовов receiptMoneyCents/parseFixed |
| BUG-054 | P1 | POS: разбор ответа + UNKNOWN при таймауте AbortController |
| BUG-059 | P1 | POS-gateway: inbox + топология fiscal.operation.result.# |
| TG-009 | P1 | reversal_policy.go: осталось спец. УНП сторно |
| PA-03 | P0 | нормализация словаря статусов в общем пути |
| DSY-02 | P0 | Daisy-кодек: SEQ-диапазон на S3, CP1251 в C++ |
| BP-02 | P0 | 72 золотых кадра DatecsPay: тест библиотеки + сверка с PDF |
| SMS-01 | P1 | Smart S: ждёт DOC-V4/V4B |

---

## Приоритет следующей сессии

1. **BUG-026.3** (P0) — закрыть: прогнать `./gradlew :bluecash-app:test` и создать harness
2. **BUG-003.5-6** (P0) — дочитать BUG-003.md, создать 2 харнеса
3. **BUG-011.1-3** (P1) — реализация HMAC_ADMIN_FORBIDDEN в auth.go
4. **DSY-02** (P0) — SEQ-диапазон Daisy + CP1251 в C++
5. **BUG-005.1-4,6** (P1) — WARN/BLOCK clock sync
6. **BUG-032.1-2** (P1) — манифест + BlueCashRuntimeService

---

## Baseline-команды для проверки в начале сессии

```bash
# Python-харнесы (23 штуки должны пройти)
cd /Users/freelancer/Documents/Beeloy/Fiscalisation
python3 -m pytest tests/mvp1/ -q

# Go unit tests (domain + api)
cd fiscal-backend && go test \
  -run '^(TestMoneyRoundingGolden|TestReceiptMoneyCentsRejectsSignedAndMalformed\
|TestValidateReceiptPlanRejectsItemsTotalMismatch|TestValidMoney\
|TestOperationSimulatedFlagFollowsDriver|TestExportOperationIsNeverSimulated\
|TestReverseUsesFiscalDocumentTimestamp|TestReadyzFails|TestReadyzOK)$' \
  ./internal/domain/ ./internal/api/ -count=1

# Gradle (BlueCash)
cd /Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices
./gradlew :bluecash-app:testProdDebugUnitTest --no-daemon 2>&1 | tail -5
```

---

## Инварианты — не нарушать

1. Деньги только целые центы; half-up `(unit¢×qty_milli+500)/1000`.
2. UNKNOWN ≠ FAILED: после таймаута — только lookup, без повтора и без возврата.
3. Сторно-first: возврат по карте только после подтверждённого фискального сторно.
4. Повтор с тем же ключом не создаёт второй документ.
5. Ошибка хранилища останавливает денежный побочный эффект.
6. Номер документа — только из ответа ФУ.
7. Секреты не коммитить и не логировать.
8. Числа в тестах — из `docs/MVP1/fixtures/*` или золотого чека (1367¢ = 1200+167).
