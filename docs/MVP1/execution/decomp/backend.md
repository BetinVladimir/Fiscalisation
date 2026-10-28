# Декомпозиция до функций: backend

## BUG-001
Файл после D-5: правка в `receipt_plan.go` (если разделение ещё не сделано — в `receipt_saga.go:96-111`). Функции: `receiptMoneyCents(v string) (int64, error)`, `ValidateReceiptPlan(v ReceiptFinalizePlan) error`.

1. **Табличный тест `TestReceiptMoneyCentsRejectsSignedAndMalformed`: входы-ошибки `"-0.50"`,`"-1.50"`,`"+1.00"`,`"1.-5"`,`"1.5"`,`""`,`"1e2.00"`,`" 1.00"`,`"1.000"`; успех `"0.00"→0`,`"0.01"→1`,`"12.34"→1234`,`"99999.99"→9999999` (решение по `"01.00"`: допускать, фиксируем в тесте)**
   - файлы: `fiscal-backend/internal/domain/receipt_money_test.go`
   - проверка: до правки FAIL на "-0.50"
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestReceiptMoneyCentsRejectsSignedAndMalformed -count=1`
2. **Тест `TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly`: payments `-0.50`+`10.00`, ExpectedTotal `10.50` → ошибка `invalid payment amount`**
   - файлы: `fiscal-backend/internal/domain/receipt_money_test.go`
   - проверка: до правки FAIL (план проходит)
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestValidateReceiptPlanRejectsNegative -count=1`
3. **Добавить `var moneyPattern = regexp.MustCompile(`^[0-9]{1,7}\.[0-9]{2}$`)` и переписать тело `receiptMoneyCents`: при несовпадении `errors.New("invalid money")`; иначе `strings.Replace(v,".","",1)` → `strconv.ParseInt(...,10,64)`; удалить `strings.Split` и две `ParseInt`**
   - файлы: `fiscal-backend/internal/domain/receipt_saga.go`
   - проверка: оба теста зелёные
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run "TestReceiptMoneyCents|TestValidateReceiptPlanRejectsNegative" -count=1`
4. **Добавить в `ValidateReceiptPlan` проверку суммы строк `v.Items` через `lineTotalCents`/`discountedLineTotalCents` (service.go:1518,1530) == `expected`; ошибка `items total mismatch`; тест `TestValidateReceiptPlanRejectsItemsTotalMismatch` (items 10.00, payments 10.50, expected 10.50)**
   - файлы: `fiscal-backend/internal/domain/receipt_saga.go`, `fiscal-backend/internal/domain/receipt_money_test.go`
   - проверка: тест зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run ValidateReceiptPlan -count=1`
5. **Регрессия: пакеты domain и api**
   - файлы: `fiscal-backend/internal/domain/receipt_saga.go`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain ./internal/api -count=1 && ! grep -n "ParseInt(parts\[0\]" internal/domain/receipt_saga.go`

## BUG-003
Символы: `type Driver interface` (service.go:33), `type Simulator struct` (:44), 5 литералов `Operation{… Simulated: true}` в service.go (внутри `reverseForTenant`, `reserveFiscalOperation`, `finalizeSaleForTenant`, `payForTenant`, `cancelSaleForTenant`) и 2 в exports.go (`CreateExport`, `CreatePeriodizedExport` — экспорт не использует ФУ ⇒ `Simulated:false`).

1. **Тест `TestOperationSimulatedFlagFollowsDriver` (табличный по 5 методам: `FinalizeSaleForTenant`,`PayForTenant`,`ReverseForTenant`,`FiscalOperation`,`CancelSaleForTenant`): Given `NewService(repo, NewSimulator(true))` → `op.Simulated==true`; Given `NewService(repo, &stubRealDriver{})` (методы `Execute`,`Probe`, не `*Simulator`) → `false`**
   - файлы: `fiscal-backend/internal/domain/simulated_flag_test.go`
   - проверка: до правки: вторая ветка FAIL
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestOperationSimulatedFlagFollowsDriver -count=1`
2. **Тест `TestExportOperationIsNeverSimulated`: `CreateExport` и `CreatePeriodizedExport` → `op.Simulated==false` при любом драйвере**
   - файлы: `fiscal-backend/internal/domain/simulated_flag_test.go`
   - проверка: FAIL до правки
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestExportOperationIsNeverSimulated -count=1`
3. **Добавить `func driverIsSimulated(d Driver) bool { _, ok := d.(*Simulator); return ok }` в service.go рядом с `Simulator`**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: компилируется
   - приёмка: `cd fiscal-backend && go build ./...`
4. **Заменить `Simulated: true` → `Simulated: driverIsSimulated(driver)` в 5 местах service.go (переменная `driver` после `s.boundDriver(...)`; в `cancelSaleForTenant`/`reserveFiscalOperation` — `s.driver`)**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: тест 1 зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestOperationSimulatedFlagFollowsDriver -count=1`
5. **Заменить `Simulated: true` → `Simulated: false` в exports.go (2 места)**
   - файлы: `fiscal-backend/internal/domain/exports.go`
   - проверка: тест 2 зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestExportOperationIsNeverSimulated -count=1`
6. **Контракт и регрессия**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: нет литералов
   - приёмка: `cd fiscal-backend && ! grep -n "Simulated: true" internal/domain/service.go internal/domain/exports.go && go test ./... -count=1 && cd .. && make contract-test`

## BUG-009
Файл `fiscal-backend/internal/config/config.go`: `Config` (стр. 12), `Load` (стр. 25-36: `getenv("APP_ENV","dev")`, `AllowStubAdapters`, `CORSAllowedOrigins`), `Validate` (стр. 38-100).

1. **Тесты `TestValidateRejectsUnknownAppEnv` (`production`,`PROD`,`stage`,`` → ошибка), `TestValidateProdChecksApplyToStaging` (staging + `CORS=*` → ошибка), `TestLoadDefaultsAreSafe` (пустое окружение → `AllowStubAdapters==false`, `CORSAllowedOrigins!="*"`), `TestStubAdaptersForbiddenWithBroker` (`EMQXBroker!=""&&AllowStubAdapters` → ошибка)**
   - файлы: `fiscal-backend/internal/config/config_test.go`
   - проверка: 4 теста красные
   - приёмка: `cd fiscal-backend && go test ./internal/config -count=1`
2. **В `Validate()` в начале: `switch c.AppEnv { case "dev","test","staging","prod": default: return fmt.Errorf("APP_ENV must be dev|test|staging|prod") }`**
   - файлы: `fiscal-backend/internal/config/config.go`
   - проверка: тест 1 зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/config -run UnknownAppEnv -count=1`
3. **Заменить условия `c.AppEnv == "prod"` на `c.strict()` (`return c.AppEnv=="prod"||c.AppEnv=="staging"`) во всех проверках Validate**
   - файлы: `fiscal-backend/internal/config/config.go`
   - проверка: тест 2 зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/config -run Staging -count=1`
4. **В `Load()` дефолт `AllowStubAdapters`=false, `CORSAllowedOrigins`=""; добавить проверку брокер+stub**
   - файлы: `fiscal-backend/internal/config/config.go`
   - проверка: тесты 3-4 зелёные
   - приёмка: `cd fiscal-backend && go test ./internal/config -count=1`
5. **Обновить `.env.example` и `compose.fiscalisation.dev.yaml` (явно `ALLOW_STUB_ADAPTERS=true`, `CORS_ALLOWED_ORIGINS=http://localhost:*` для dev)**
   - файлы: `fiscal-backend/.env.example`, `compose.fiscalisation.dev.yaml`
   - проверка: rg находит переменные
   - приёмка: `rg -n "ALLOW_STUB_ADAPTERS" compose.fiscalisation.dev.yaml fiscal-backend/.env.example`

## TG-001
**Пере-определена** (DECISIONS §2): Postgres-персистентность уже есть (`persistence.Postgres.SaveDeltaVersioned`, `AllocateUNP`, типизированные проекции; `main.go:28-45`). Остаётся доказать и закрыть дефекты — подзадачи a–d.

1. **TG-001a-1: тест на реальном Postgres `TestRestartKeepsReservedOperation`: создать продажу → reserve → закрыть `NewPersistentRepository` → открыть заново → операция в `EXECUTING`, УНП тот же**
   - файлы: `fiscal-backend/internal/persistence/restart_integration_test.go`
   - проверка: закрепляет поведение
   - приёмка: `cd fiscal-backend && bash ../scripts/postgres-integration.sh`
2. **TG-001a-2: `TestConcurrentFinalizeSameKeyProducesOneOperation` (-race): 8 горутин `FinalizeSaleForTenant` с одним `ClientOperationID`; ожидание — одна запись `Operation`, вызовов `Driver.Execute` ровно 1**
   - файлы: `fiscal-backend/internal/domain/concurrent_finalize_test.go`
   - проверка: -race зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestConcurrentFinalizeSameKey -race -count=3`
3. **TG-001a-3: `TestTwoReplicasGenerationConflict`: два `PersistentRepository` над одной БД; B пишет → A при записи получает `ErrConcurrentState`, `restoreLocked` перечитывает, повтор проходит; ни одна операция не потеряна**
   - файлы: `fiscal-backend/internal/persistence/two_replica_test.go`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && bash ../scripts/postgres-integration.sh`
4. **TG-001b: бенчмарк `BenchmarkPersistLockedN` (N=1k,10k,100k): измеряет `persistLocked` (repository.go:392); порог p95 ≤ 50 мс при 10k — в `docs/MVP1/evidence/TG-001b.md`; при превышении — diff только затронутых коллекций (`persistLocked(touched ...string)`)**
   - файлы: `fiscal-backend/internal/domain/persist_bench_test.go`, `docs/MVP1/evidence/TG-001b.md`
   - проверка: бенч печатает ns/op
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run xxx -bench BenchmarkPersistLockedN -benchtime 3x`
5. **TG-001c (до пилота): ADR-PERSIST-001 — multi-replica: per-tenant поколение или единственный писатель; без кода**
   - файлы: `docs/decisions/ADR-PERSIST-001.md`
   - проверка: есть раздел Decision
   - приёмка: `grep -q "^## Decision" docs/decisions/ADR-PERSIST-001.md`
6. **TG-001d: вынести `selectRepository(cfg) (domain.Repository, error)` из `main.go:28`; `AppEnv` не dev/test и пустой `DatabaseURL` → ошибка; dev с нелокальным `PublicBaseURL` и пустым `DatabaseURL` → ошибка; тест `TestMainRefusesMemoryRepoOutsideDev`**
   - файлы: `fiscal-backend/cmd/fiscal-backend/main.go`, `fiscal-backend/cmd/fiscal-backend/main_test.go`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && go test ./cmd/fiscal-backend -run MemoryRepo -count=1`

## TG-002
Файлы: `internal/domain/unp_allocator.go`, `regulatory_identifier.go` (`BGUNPSequenceMax`), `internal/persistence/postgres.go:59` (`AllocateUNP`), `database/fiscal/013_supto_identifier_foundation.sql`.

1. **Прочитать `unp_allocator.go` и `AllocateUNP`; в `docs/MVP1/evidence/TG-002.md` — таблица «кто выделяет УНП: MemoryRepository.ReserveFiscalUNPRange vs Postgres.AllocateUNP; где вызывается» (rg `ReserveFiscalUNPRange|AllocateUNP`)**
   - файлы: `docs/MVP1/evidence/TG-002.md`
   - проверка: файл создан
   - приёмка: `test -f docs/MVP1/evidence/TG-002.md`
2. **Тесты `TestUNPAllocatorExhaustion` (9999999 → следующий `UNP range exhausted`), `TestUNPAllocatorNoReuseAfterRestart` (выделить 3, перезапуск, → 4), `TestUNPFormatRegex` (`^[A-Z0-9]{8}-[A-Z0-9]{4}-[0-9]{7}$`; кириллическая «О» отвергается)**
   - файлы: `fiscal-backend/internal/domain/unp_allocator_test.go`
   - проверка: результат фиксируется
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run UNP -race -count=1`
3. **Если уникального индекса `(tenant_id, fiscal_device_number, unp_sequence)` в `fiscal.unp_allocations` нет — миграция `database/fiscal/NNN_unp_unique.sql`; проверка `scripts/check-db-schema.sh` (DBS-001)**
   - файлы: `database/fiscal/NNN_unp_unique.sql`
   - проверка: индекс найден
   - приёмка: `bash scripts/check-db-schema.sh`
4. **Документ `docs/SUPTO/UNP_GAPS_POLICY.md`: когда допустима дыра (отменённое резервирование — BUG-040), запись `unp_gap` с причиной; тест `TestCancelledReservationRecordsGap`**
   - файлы: `docs/SUPTO/UNP_GAPS_POLICY.md`, `fiscal-backend/internal/domain/unp_gap_test.go`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestCancelledReservationRecordsGap -count=1`

## TG-003
Объединено с BUG-005 и VG-21: на BC-50 команда установки времени (61) **не применима** — только чтение (62) и контроль дрейфа. Символы: `Service.SyncWorkstationClock` (service.go:450), `HasDailyClockSync` (:483), `clockCapableDriver`, `Bridge.DeviceTime/SetDeviceTime` (mqttclient/client.go:94-104).

1. **Тест `TestBridgeDeviceTimeFailsClosedWithoutMeasurement`: `Bridge` без ответа устройства → `DeviceTime()` = `ErrDeviceClockNotMeasured`**
   - файлы: `fiscal-backend/internal/mqttclient/client_clock_test.go`
   - проверка: красный
   - приёмка: `cd fiscal-backend && go test ./internal/mqttclient -run Clock -count=1`
2. **`Bridge.DeviceTime()`: вместо `time.Now().UTC()` вернуть `ErrDeviceClockNotMeasured` пока нет команды `DEVICE_TIME`; `SetDeviceTime` — `ErrDeviceClockSetUnsupported` для профиля BC-50**
   - файлы: `fiscal-backend/internal/mqttclient/client.go`
   - проверка: тест зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/mqttclient -run Clock -count=1`
3. **Контракт: в схему результата `DEVICE_TIME` поля `device_time` (RFC3339), `drift_seconds` (int); обновить `*.lock`**
   - файлы: `fiscal-backend/contracts/device_command_result.json`
   - проверка: contract-test зелёный
   - приёмка: `make contract-test`
4. **`SyncWorkstationClock`: константы `ClockDriftMax = 2*time.Hour` (Н-18 чл.25 ал.12), `ClockDriftWarn = 5*time.Minute`; ветка `SetDeviceTime` только для драйверов с `clockSetter`; `BusinessDate` через `sofiaLoc` (`time.LoadLocation("Europe/Sofia")` один раз; `time/tzdata` уже импортирован в `reversal_policy.go`)**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: TestHasDailyClockSyncUsesSofiaBusinessDate: 2026-10-21T22:30Z → 2026-10-22
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run "ClockSync|DailyClock" -count=1`
5. **`HasDailyClockSync`: O(1) через ресурс `clock_sync|<workstation>|<business_date>` вместо перебора `Resources("device_clock_sync_event")`; тест на 10k событий (< 5 мс)**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run HasDailyClockSync -count=1`
6. **Тесты порогов: `TestClockDriftOver2hBlocksSales` (2h01m → `CLOCK_DRIFT_BLOCK`), `TestClockDrift5minWarnsOnly`**
   - файлы: `fiscal-backend/internal/domain/clock_drift_test.go`
   - проверка: зелёные
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run ClockDrift -count=1`

## ARCH-002
Механическое разделение `service.go` (1579 строк) на файлы **без изменения сигнатур**. Каждая подзадача: вырезать перечисленные функции целиком (с комментариями) в новый файл пакета `domain`; `go build` и тесты зелёные; публичный хэш API не меняется. Номера строк — на момент аудита (искать по имени).

1. **Шаг 0: характеристические тесты `TestServiceBehaviorSnapshot_*` (10 сценариев: open shift, create sale+lines, pay cash, pay card (Simulator), finalize, reverse, cancel, reconcile, readiness, clock sync) → `testdata/behavior/*.golden`; `go doc -all ./internal/domain Service | sha256sum > testdata/service_api.sha256`**
   - файлы: `fiscal-backend/internal/domain/service_behavior_test.go`, `fiscal-backend/internal/domain/testdata/service_api.sha256`
   - проверка: зелёные
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestServiceBehaviorSnapshot -count=1`
2. **`clock_service.go`: `RefreshReadiness`, `CurrentReadiness`, `signReadiness`, `SyncWorkstationClock`, `HasDailyClockSync`, `Readiness`, `ReadinessForTenant`, `activeDeviceForTenant` (service.go:416-491, 1457-1487)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/clock_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
3. **`ble_service.go`: `SetBLESigningKey`, `BLESession`, `RefreshBLE`, `RevokeBLE`, `bleSessionForTenant`, `bleResponse` (service.go:409, 492-541, 621-683)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/ble_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
4. **`shift_service.go`: `OpenShift`, `activeOperatorResourceForTenant`, `CloseShift`, `CloseShiftForTenant`, `GetShift`, `Shifts`, `OpenWorkstationSession`, `LogoutWorkstationSession`, `WorkstationSession` (service.go:171-223, 704-767)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/shift_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
5. **`device_binding_service.go`: `boundDriver`, `registerHasActiveFiscalDevice`, `activeFiscalDeviceSnapshot`, `registerHasActivePaymentTerminal`, `bindingIsActive`, `activeOperatorForTenant`, `Connectivity`, `GetConnectivityProbe`, `ExpireDeviceCommand`, `CompleteDeviceCommand*` (service.go:120-167, 377-408, 542-620, 1206-1259)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/device_binding_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
6. **`reversal_service.go`: `Reverse*`, `reverseForTenant`, `CancelSale*`, `cancelSaleForTenant`, `ReconcileOperation*`, `reconcileOperationForTenant`, `FiscalOperation`, `reserveFiscalOperation`, `executeReservedFiscalOperation` (service.go:224-376, 1309-1346)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/reversal_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
7. **`sale_service.go`: `OpenSaleWithFirstLine`, `CreateSale`, `snapshotSaleNomenclature`, `AddLine*`, `ChangeLine*`, `CancelLine*`, `SalesForTenant`, `addLineForTenant`, `Pay*`, `payForTenant`, `FinalizeSaleForTenant`, `PrepareFinalizeSaleForTenant`, `finalizeSaleForTenant`, `GetSale*`, `GetOperation*`, `Operations`, `Receipt*`, `receiptForTenant`, `saleForTenantMutation`, `operationForTenantMutation` (service.go:768-1200, 1260-1308, 1347-1410)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/sale_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
8. **`money.go`: `validTax`, `validMoney`, `validQuantity`, `parseFixed`, `lineTotalCents`, `discountedLineTotalCents`, `validDiscount`, `saleTotal`, `formatFixed`, `contains` (service.go:1488-1578) — единое место для D-4**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/money.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
9. **`outbox_service.go`: `QueueFiscalEvent`, `fiscalOperationEvent`, `fiscalCommandEvent`, `reconciliationRequiredEvent`, `PendingOutbox`, `UpdateOutbox`, `Replay`, `ClaimReplay`, `PutReplay`, `PutAmbiguousReplay` (service.go:1411-1456)**
   - файлы: `fiscal-backend/internal/domain/service.go`, `fiscal-backend/internal/domain/outbox_service.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
10. **Финал: в `service.go` остаются `Simulator`, `Service`, `NewService`, интерфейсы Driver*, `newID`, `pad7`, `Hash`, `CountryPolicy`, `TaxGroups`, `SetCompositeBindingPublisher`, `SetRequireHardwareSyncSignatures`; проверка размера и хэша API**
   - файлы: `fiscal-backend/internal/domain/service.go`
   - проверка: wc ≤ 300; хэш API совпал
   - приёмка: `cd fiscal-backend && test $(wc -l < internal/domain/service.go) -le 300 && go doc -all ./internal/domain Service | sha256sum | diff - internal/domain/testdata/service_api.sha256 && go test ./... -race -count=1`

## ARCH-003
Механическое разделение `internal/api/handler.go` (2089 строк). Хэндлеры — методы `*Handler`; пути и поведение не меняются.

1. **Шаг 0: `TestRouteTableSnapshot` — список «метод + путь + роль» из `newHandler` (handler.go:141-221) сравнивается с `testdata/routes.golden` (обновление флагом `-update`)**
   - файлы: `fiscal-backend/internal/api/routes_snapshot_test.go`, `fiscal-backend/internal/api/testdata/routes.golden`
   - проверка: зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/api -run TestRouteTableSnapshot -count=1`
2. **`middleware_idem.go`: `fiscalCapturedResponse` (+3 метода), `fiscalIdempotency`, `fiscalReplayHeaders`, `applyFiscalReplayHeaders`, `fiscalIdempotencyFingerprint`, `replay`, `saveReplay`, `saveEmptyReplay`, `replayKey` (handler.go:517-649, 1918-1959, 1977)**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/middleware_idem.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
3. **`middleware_http.go`: `version`, `recoverer`, `cors`, `SignWebhook`, `problem`, `write`, `readBody`, `decodeStrict`, `paginate`, `optionalTime`, `tenantID`, `actorSubject`, `ifMatch`, `hasAnyRole` (handler.go:36, 330, 896, 1905-2012, 2031-2089)**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/middleware_http.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
4. **`handlers_devices.go`: `manufacturingDeviceRegister`, `platformDevices`, `platformDevice`, `deviceActivationLookup`, `deviceBootstrap*`, `deviceActivationConfirmation`, `device` (handler.go:222-436, 1749-1904)**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/handlers_devices.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
5. **`handlers_sales.go`: `sales`, `openSaleWithLine`, `sale`, `operations`, `operation`, `authorizeSale`, `shifts`, `shift`, `register` (handler.go:920-1296, 1297-1448, 1715-1748, 2018-2030)**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/handlers_sales.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
6. **`handlers_reports.go`: `reports`, `report`, `auditEvents`, `exports`, `periodizedExports`, `export`, `exportExtension` (handler.go:1449-1640)**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/handlers_reports.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
7. **`handlers_platform.go`: `webhookEndpoints`, `webhookEndpoint`, `typedWebhookCreated`, `taxGroups`, `countryPolicy`, `organization`, `resourceCollection`, `resourceItem`, `live`, `ping`, `version`-handler, `connectivityProbe`, `bleSession`, `sync`, `workstation`**
   - файлы: `fiscal-backend/internal/api/handler.go`, `fiscal-backend/internal/api/handlers_platform.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/api -count=1`
8. **Финал: `handler.go` ≤ 500 строк (тип `Handler`, конструкторы, таблица маршрутов); golden маршрутов не изменился**
   - файлы: `fiscal-backend/internal/api/handler.go`
   - проверка: wc ≤ 500
   - приёмка: `cd fiscal-backend && test $(wc -l < internal/api/handler.go) -le 500 && go test ./internal/api -count=1`

## BUG-006
По D-5: разделить `receipt_saga.go`. Подзадачи механические.

1. **Перенести `ReceiptFinalizePlan`, `ValidateReceiptPlan`, `supportedPaymentType`, `receiptMoneyCents` в новый `receipt_plan.go` (поведение неизменно)**
   - файлы: `fiscal-backend/internal/domain/receipt_saga.go`, `fiscal-backend/internal/domain/receipt_plan.go`
   - проверка: build+тесты
   - приёмка: `cd fiscal-backend && go build ./... && go test ./internal/domain -count=1`
2. **Удалить `receipt_saga.go` целиком (ReceiptSaga*, MemoryReceiptSagaStore, allowedReceiptTransition, ReceiptPlanHash) и тесты, ссылающиеся только на удалённое (`rg -l MemoryReceiptSagaStore --glob "*_test.go"`)**
   - файлы: `fiscal-backend/internal/domain/receipt_saga.go`
   - проверка: файла нет, тесты зелёные
   - приёмка: `cd fiscal-backend && test ! -f internal/domain/receipt_saga.go && go test ./... -count=1`
3. **Защита от возврата: правило в `scripts/check-lint-rules.sh` (SCR-17) — запрет `MemoryReceiptSagaStore`**
   - файлы: `scripts/check-lint-rules.sh`
   - проверка: rg пусто
   - приёмка: `! rg -n "MemoryReceiptSagaStore" fiscal-backend`

## BUG-023
D-4: half-up на строку в целых центах. Символы: `lineTotalCents(price Money, quantity string)` (service.go:1518), `discountedLineTotalCents` (:1530), `saleTotal` (:1553), `parseFixed` (:1501); Kotlin `moneyMinor` (BlueCashEngine.kt:384); POS — `rg "toFixed|Math.round" POS`.

1. **Эталон `docs/MVP1/fixtures/rounding_golden.json`: ≥ 30 кейсов {unit_cents, qty_milli, discount_cents, expect_line_cents}, включая границы 0.005 (`0.01 × 0.500` → 1) и 3 набора для PRB-03**
   - файлы: `docs/MVP1/fixtures/rounding_golden.json`
   - проверка: jq length>=30
   - приёмка: `jq -e "length>=30" docs/MVP1/fixtures/rounding_golden.json`
2. **Go-тест `TestRoundingGolden` читает JSON, проверяет `lineTotalCents`/`discountedLineTotalCents`; печатает `ROUND OK <n>`**
   - файлы: `fiscal-backend/internal/domain/rounding_golden_test.go`
   - проверка: красный при расхождении
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestRoundingGolden -count=1`
3. **`lineTotalCents` → `(unit_cents*qty_milli + 500)/1000` целочисленно, без float; отрицательные — отдельная ветка**
   - файлы: `fiscal-backend/internal/domain/money.go`
   - проверка: TestRoundingGolden зелёный
   - приёмка: `cd fiscal-backend && go test ./internal/domain -run TestRoundingGolden -count=1`
4. **Kotlin `RoundingGoldenTest` на той же фикстуре (`DatecsPayloads` + `moneyMinor`)**
   - файлы: `SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/RoundingGoldenTest.kt`
   - проверка: зелёный
   - приёмка: `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests "*RoundingGolden*"`
5. **POS (TS): `roundingGolden.test.ts` на той же фикстуре**
   - файлы: `POS/pos-app/src/__tests__/roundingGolden.test.ts`
   - проверка: зелёный
   - приёмка: `cd POS/pos-app && npx jest roundingGolden`
