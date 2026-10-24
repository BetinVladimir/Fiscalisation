# OPEN_WORK · актуальные работы после сверки с кодом (2026-10-24)

Сгенерировано `tools/gen-open-work.py` из `execution/reverify.json`. Проверка статическая (grep/чтение кода): на машине нет Go и Gradle, поэтому Go/Kotlin-тесты **не запускались**; наличие теста ≠ его зелёный прогон.

Итого: закрыто 13, частично 11, подтверждённо открыто 94, не проверялось кодом 196 (всего 314).

## 1. Закрыто (из очереди убрано)
Отмечены `DONE`. Для пяти из них закрытие подтверждено кодом при этой сверке: BUG-009, S3D-02, DSY-01, TG-002, BUG-060 (ротация пароля keystore — действие владельца). Остальные — задачи-документы/решения.

| Задача | Заметка |
|---|---|
| BUG-009 | Fixed: AppEnv validation, stub default false, staging parity, EMQX+stub error; 4 new tests pass / [reverify 2026-10-24 CLOSED] APP_ENV валидируется (d |
| BUG-060 | BLOCKED: No git repo in workspace. Requires git rm --cached credentials.json, .gitignore update, keystore rotation note / [reverify 2026-10-24 CLOSED] |
| TG-002 | [reverify 2026-10-24 CLOSED] unique(tenant_id,unp) в БД; unp_allocator_test.go: параллельные диапазоны, коллизии, неизменяемость / проверка: grep -c ' |
| DOC-V3 | ПОЛУЧЕНО 2026-10-23: DatecspayLowLevel_v3_2025.pdf и Pinpad_Commands v1.9 (docs/vendor/, разбор DATECSPAY_PROTOCOL.md). Открыты вопросы вендору Q-B1…Q |
| SCOPE-01 | решение D-11: входят ли minipos/, AdminApp/, BeeFiscalApp/ в MVP1; если да - расширить анализ (не покрыты) ; решено MVP_GOAL: MiniPOS в MVP, AdminApp  |
| SCR-37 | scripts/task-pack.py <ID>: генерирует контекст-пакет задачи ≤ 6000 токенов (карточка + подзадачи + решения D-n + выдержки кода ±40 строк по символам + |
| DOC-006 | Нормализация ссылок на код: полные пути от корня Fiscalisation (20 из 48 ссылок file:line используют неоднозначное имя `service.go`/`handler.go`); пер |
| DSY-01 | golden-кадры Daisy 2.0-4 / [reverify 2026-10-24 CLOSED] 17 золотых кадров Daisy проверены инструментом; тест кодека — TST-DEEP-13 / проверка: python3  |
| S3D-02 | GAP-010 LEN +0x2A: Kotlin и C++ / [reverify 2026-10-24 CLOSED] LEN=data+0x2A: C++ DatecsCodec и Kotlin DatecsFrameCodec / проверка: grep -c 'size()+0x |
| BP-01 | BluePay: DatecspayLowLevel.pdf v3 (DOC-V3) |
| CL-01 | D-13 решено владельцем 2026-10-23: ДА, нативный BLE в POS (задачи POS-BLE-01…03) |
| TST-META-01 | tools/check-test-matrix.py — проверка матрицы тестов и фикстур |
| PIPE-01 | tools/run-accept.py создан |

## 2. Закрыты ошибочно или не полностью → переоткрыты
BUG-001, BUG-002, BUG-003, BUG-010, BUG-011 стояли как `DONE` с пометкой «Fixed», но код этого не подтверждает (например `validMoney` по-прежнему принимает префикс +/−).


## 3. Частично сделано (осталось сузить)

| Задача | P | Кто | Что осталось / доказательство | Проверка |
|---|---|---|---|---|
| ARCH-004 | P1 | qwen | money.go (Cents, ParseMoney, FormatMoney, LineTotal) создан. Осталось: перевести 20 вызовов receiptMoneyCents/parseFixed | `grep -rn 'receiptMoneyCents\/parseFixed' fiscal-backend/internal --include=*.go / grep -v ` |
| BUG-001 | P0 | codex-first | знак теперь учитывается (−0.50→−50, −1.50→−150), но остались «+1.00», «01.00» и «1.-5»→95 без регэкспа ^[0-9]+\.[0-9]{2}$. Осталось: регэксп + тест. Статус DONE был завыш | `sed -n 96,122p fiscal-backend/internal/domain/receipt_saga.go` |
| BUG-003 | P0 | codex-first | Simulated берётся из s.simulated (SetSimulated); жёсткого true в service.go нет. Осталось: проверить exports.go:105,151 и добавить тест | `grep -rn 'Simulated' fiscal-backend/internal/domain/service.go` |
| BUG-010 | P1 | codex-first | /readyz и /healthz → h.ready, /livez → h.live. Осталось: тест TestReadyzFailsWhenRepositoryUnavailable | `grep -n 'readyz\/livez' fiscal-backend/internal/api/handler.go` |
| BUG-011 | P1 | qwen | проверки iss/aud/nbf/maxTTL (ParseTokenConfig) есть; осталось: откат на HMAC при ошибке OIDC (auth.go:55-58) и тест TST-GO-08 | `sed -n 55,58p fiscal-backend/internal/auth/auth.go` |
| BUG-054 | P1 | qwen | таймаут 4 с через AbortController есть. Осталось: разбор ответа и UNKNOWN при таймауте записи (тест TST-POS-05) | `cd ../POS && grep -n AbortController pos-app/src/infrastructure/runtime/Adapters.ts` |
| BUG-059 | P1 | qwen | потребитель результатов использует inbox и ключ fiscal.operation.result.#. Осталось: скрипт топологии (POS-08) | `cd ../POS && grep -n fiscal.operation.result pos-gateway/internal/fiscal/result_consumer.g` |
| TG-009 | P1 | qwen | reversal_policy.go и тесты есть (срок ошибки оператора). Осталось: спец. УНП сторно, остальные пункты карточки | `ls fiscal-backend/internal/domain/reversal_policy.go` |
| PA-03 | P0 | qwen | словарь статусов результата проверяется (SUCCEEDED/FAILED/RECONCILIATION_REQUIRED/CANCELLED). Осталось: нормализация в общем пути | `grep -n RECONCILIATION_REQUIRED ../BeeloyBackend/public-api-core/internal/adminerp/network` |
| DSY-02 | P0 | qwen | Daisy-кодек исправлен (LEN +0x24/−0x2B). Осталось: SEQ-диапазон на S3 (SEQ-01), CP1251 (в C++ только DaisyPrinter) | `grep -n "next_sequence_" IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp` |
| BP-02 | P0 | codex-first | 72 золотых кадра DatecsPay проверены инструментом. Осталось: тест библиотеки на кадры и сверка кода с PDF | `python3 docs/MVP1/tools/extract_vendor_frames.py --check datecspay` |

## 4. Подтверждённо открыто (в коде следов реализации нет или дефект на месте)

| Задача | P | Кто | Что осталось / доказательство | Проверка |
|---|---|---|---|---|
| ARCH-006 | P1 | codex-first | driverFor отсутствует | `grep -rn 'func (s \*Service) driverFor' fiscal-backend/internal` |
| BUG-002 | P1 | codex-first | validMoney по-прежнему принимает префикс +/− (service.go:1540-1542), хотя в статусе стояло «Fixed: rejects +/-»; задача была закрыта ошибочно | `sed -n 1531,1545p fiscal-backend/internal/domain/service.go` |
| BUG-043 | P1 | qwen | проверка сводится к total>0 | `sed -n 398p IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp` |
| TG-007 | P0 | qwen | POS/docs/contracts отсутствует | `ls ../POS/docs/contracts` |
| ARCH-007 | P1 | qwen | BlueCashRuntime отсутствует | `ls SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash / grep -i runtime` |
| BUG-020 | P0 | codex-first | нет отмены открытого чека при сбое | `grep -c 'CMD_CANCEL' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/` |
| BUG-021 | P0 | codex-first | компенсация карты при любой ошибке, независимо от unknown | `sed -n 274,278p SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueC` |
| BUG-024 | P1 | qwen | require(current == null) остался | `grep -n 'SALE_ALREADY_EXISTS' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/` |
| BUG-026 | P0 | codex-first | синтетический референс "DATECS-"+operationId | `grep -n '"DATECS-"' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/B` |
| BUG-027 | P1 | qwen | пустой ответ принимается как успех | `sed -n 379p SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashE` |
| BUG-028 | P0 | codex-first | классификация UNKNOWN по тексту сообщения | `grep -n 'DATECS_EOF' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/` |
| BUG-029 | P0 | codex-first | DatecsSocketProtocol.execute: нет SYN/NAK/таймаута | `sed -n 180,196p SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/Datec` |
| BUG-032 | P0 | codex-first | в манифесте нет сервиса и разрешений | `grep -c 'FOREGROUND_SERVICE' SmartDevices/bluecash-app/src/main/AndroidManifest.xml` |
| CODE-003 | P2 | qwen | DatecsLimits отсутствует | `grep -rn 'object DatecsLimits' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal` |
| BUG-005 | P0 | codex-first | DeviceTime возвращает time.Now().UTC() | `sed -n 98p fiscal-backend/internal/mqttclient/client.go` |
| BUG-008 | P1 | codex-first | OriginalDocumentAt: original.UpdatedAt | `grep -o 'OriginalDocumentAt: original.UpdatedAt' fiscal-backend/internal/domain/service.go` |
| BUG-040 | P1 | qwen | UNP резервируется до проверки готовности ФУ (строки 411 и 438) | `grep -n 'reserve_command\/executor_.ready' IoT/firmware/edge-agent-s3/idf/main/intent_proc` |
| BUG-041 | P0 | codex-first | now() возвращает 0 при часах раньше 2024-01-01, at=now() не проверяется | `sed -n 77,80p IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp` |
| CODE-007 | P2 | qwen | Clock интерфейса нет | `grep -rn 'type Clock interface' fiscal-backend/internal` |
| BUG-050 | P0 | qwen | POS вызывает :reconcile, шлюз принимает /reconcile | `cd ../POS && grep -n reconcile pos-app/src/infrastructure/runtime/Adapters.ts pos-gateway/` |
| BUG-053 | P0 | codex-first | unp берётся с откатом на fiscalReference | `cd ../POS && grep -n "result.unp" pos-app/src/infrastructure/runtime/Adapters.ts` |
| BUG-055 | P0 | codex-first | шлюз финализирует платёж как CASH | `cd ../POS && grep -c "\"type\": \"CASH\"" pos-gateway/internal/drain/idle.go` |
| BUG-057 | P1 | qwen | пустой ключ заменяется uuid.NewString() | `cd ../POS && sed -n 83p pos-gateway/internal/handler/fiscal_handler.go` |
| NAP-04 | P1 | qwen | ни один из целевых файлов не существует: NNN_sale_import_evidence.sql | `test -e NNN_sale_import_evidence.sql` |
| NAP-05 | P1 | qwen | ни один из целевых файлов не существует: POS/scripts/check-i18n-bg.mjs | `test -e POS/scripts/check-i18n-bg.mjs` |
| BUG-006 | P1 | qwen | ReserveReceiptSaga нигде не вызывается кроме определений | `grep -rn ReserveReceiptSaga fiscal-backend --include=*.go --exclude=*_test.go` |
| TG-010 | P1 | qwen | reconciler.go не существует | `ls fiscal-backend/internal/domain/reconciler.go` |
| BUG-042 | P1 | qwen | допустимы только CASH и CARD | `sed -n 390p IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp` |
| BUG-044 | P2 | qwen | PayloadConflict возвращает ошибку без result_sink_ | `sed -n 431,432p IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp` |
| NAP-02 | P1 | qwen | ни один из целевых файлов не существует: service.go | `test -e service.go` |
| NAP-06 | P1 | qwen | ни один из целевых файлов не существует: scripts/check-prod-artifact.sh | `test -e scripts/check-prod-artifact.sh` |
| TG-008 | P1 | qwen | ни один из целевых файлов не существует: electronic_receipt.go | `test -e electronic_receipt.go` |
| ARCH-002 | P1 | codex-first | service.go 1639 строк, не разделён | `wc -l fiscal-backend/internal/domain/service.go` |
| ARCH-003 | P1 | codex-first | handler.go 2125 строк, не разделён | `wc -l fiscal-backend/internal/api/handler.go` |
| CODE-005 | P2 | qwen | type OperationState отсутствует | `grep -rn 'type OperationState' fiscal-backend/internal` |
| NAP-07 | P1 | qwen | ни один из целевых файлов не существует: scripts/check-data-location.sh | `test -e scripts/check-data-location.sh` |
| TG-011 | P1 | qwen | /metrics не найден | `grep -rn promhttp fiscal-backend --include=*.go` |
| TG-014 | P1 | qwen | HIL_RUNBOOK.md и scripts/hil отсутствуют | `ls docs/MVP1/HIL_RUNBOOK.md scripts/hil` |
| NAP-19 | P1 | qwen | ни один из целевых файлов не существует: contracts/testdata/rounding.json, fiscal-backend/internal/domain/rounding_golden_test.go, POS/pos-app/src/__tests__/rounding.gold | `test -e contracts/testdata/rounding.json` |
| VG-11 | P1 | qwen | ни один из целевых файлов не существует: SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsClock.kt | `test -e SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsClock.k` |
| MC-01 | P1 | human | ни один из целевых файлов не существует: minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json, fiscal-backend/internal/domain/exec | `test -e minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_ve` |
| AA-01 | P0 | qwen | crypto.randomUUID() в AdminApp | `grep -c randomUUID AdminApp/App.tsx` |
| AA-03 | P1 | codex-first | фолбэк на localhost:8080 | `grep -n localhost:8080 BeeFiscalApp/App.tsx` |
| BF-01 | P0 | qwen | authenticate() заглушка CRYPTO_PROVIDER_NOT_AVAILABLE | `grep -n CRYPTO_PROVIDER_NOT_AVAILABLE BeeFiscalApp/src/smartDeviceBle.native.ts` |
| EC-02 | P0 | qwen | в erp-connector нет состояния UNKNOWN | `grep -c UNKNOWN ../POS/erp-connector/internal/orchestration/service.go` |
| EC-03 | P1 | qwen | нет url.PathEscape | `grep -c PathEscape ../POS/erp-connector/internal/orchestration/service.go` |
| EC-04 | P1 | qwen | _ = s.commitCredential | `grep -n "_ = s.commitCredential" ../POS/erp-connector/internal/orchestration/service.go` |
| EC-05 | P1 | qwen | opening_cash жёстко 0.00 | `grep -n opening_cash ../POS/erp-connector/internal/orchestration/service.go` |
| EC-07 | P2 | qwen | E2E_PROBE в боевом коде | `grep -n E2E_PROBE ../POS/erp-connector/internal/orchestration/service.go` |
| PA-01 | P1 | qwen | Nack(false,true): бесконечный requeue | `grep -n Nack ../BeeloyBackend/public-api-core/internal/adminerp/networkmessaging/fiscal_re` |
| PA-02 | P1 | qwen | payloadSha256 необязателен | `grep -n 'PayloadSHA256 != ""' ../BeeloyBackend/public-api-core/internal/adminerp/networkme` |
| BUG-072 | P0 | codex-first | нет блокировки между транспортами BlueCash | `grep -c 'Mutex' SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueC` |
| TST-DEEP-01 | P1 | qwen | ни один из целевых файлов не существует: fiscal-backend/internal/persistence/concurrent_test.go | `test -e fiscal-backend/internal/persistence/concurrent_test.go` |
| TST-DEEP-02 | P1 | qwen | ни один из целевых файлов не существует: то же | `test -e то же` |
| TST-DEEP-03 | P1 | qwen | ни один из целевых файлов не существует: internal/api/crash_replay_test.go | `test -e internal/api/crash_replay_test.go` |
| TST-DEEP-04 | P1 | qwen | ни один из целевых файлов не существует: fiscal-backend/internal/domain/ecdsa_vectors_test.go` + `fixtures/ecdsa_vectors.json | `test -e fiscal-backend/internal/domain/ecdsa_vectors_test.go` + `fixtures/ecdsa_vectors.js` |
| TST-DEEP-05 | P1 | qwen | ни один из целевых файлов не существует: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsGoldenFramesTest.kt | `test -e SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsGoldenF` |
| TST-DEEP-06 | P1 | qwen | ни один из целевых файлов не существует: Robolectric `JournalCrashRecoveryTest.kt | `test -e Robolectric `JournalCrashRecoveryTest.kt` |
| TST-DEEP-07 | P1 | qwen | ни один из целевых файлов не существует: minipos/.../webhook_race_test.go | `test -e minipos/.../webhook_race_test.go` |
| TST-DEEP-08 | P1 | qwen | ни один из целевых файлов не существует: POS/erp-connector/orchestration/crash_test.go | `test -e POS/erp-connector/orchestration/crash_test.go` |
| TST-DEEP-09 | P1 | qwen | ни один из целевых файлов не существует: public-api-core/.../fiscal_results_dlq_test.go | `test -e public-api-core/.../fiscal_results_dlq_test.go` |
| TST-DEEP-10 | P1 | qwen | ни один из целевых файлов не существует: tests/e2e/exactly_once_test.go | `test -e tests/e2e/exactly_once_test.go` |
| TST-DEEP-11 | P1 | qwen | ни один из целевых файлов не существует: internal/domain/business_date_test.go | `test -e internal/domain/business_date_test.go` |
| TST-DEEP-12 | P1 | qwen | ни один из целевых файлов не существует: FrameFuzzTest | `test -e FrameFuzzTest` |
| SCR-39 | P1 | human | ни один из целевых файлов не существует: scripts/check-mvp-cut.py | `test -e scripts/check-mvp-cut.py` |
| DSY-03 | P0 | qwen | профиль не-Datecs трактуется как Daisy | `sed -n 15p IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp` |
| MQ-02 | P0 | qwen | нативный BLE-модуль POS отсутствует | `grep -rln "react-native-ble-plx" ../POS/pos-app/package.json` |
| MQ-03 | P1 | qwen | ни один из целевых файлов не существует: fiscal-backend/internal/domain/device_result.go | `test -e fiscal-backend/internal/domain/device_result.go` |
| HILMX-01 | P0 | hil | ни один из целевых файлов не существует: scripts/check-hil-matrix.py | `test -e scripts/check-hil-matrix.py` |
| SCR-40 | P0 | human | ни один из целевых файлов не существует: scripts/check-hil-matrix.py | `test -e scripts/check-hil-matrix.py` |
| SMS-02 | P0 | codex-first | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| SMS-03 | P0 | qwen | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| SMS-04 | P0 | codex-first | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| SMS-05 | P0 | codex-first | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| SMS-07 | P1 | qwen | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| EMU-01 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| EMU-02 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| EMU-03 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| EMU-04 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| EMU-05 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| EMU-06 | P1 | qwen | фейк-эмулятор не найден | `find . -path ./node_modules -prune -o -iname "*Fake*" -print / head -3` |
| SMS-08 | P0 | qwen | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| SMS-09 | P0 | qwen | код отсутствует | `ls SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy` |
| POS-BLE-01 | P0 | codex-first | нативный BLE-модуль POS отсутствует | `grep -rln "react-native-ble-plx" ../POS/pos-app/package.json` |
| POS-BLE-02 | P0 | qwen | нативный BLE-модуль POS отсутствует | `grep -rln "react-native-ble-plx" ../POS/pos-app/package.json` |
| POS-BLE-03 | P0 | qwen | нативный BLE-модуль POS отсутствует | `grep -rln "react-native-ble-plx" ../POS/pos-app/package.json` |
| SEQ-01 | P0 | qwen | S3 wrap 0xFF для Datecs | `sed -n 23p IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp` |
| TST-E2E-21 | P0 | qwen | ни один из целевых файлов не существует: tests/e2e/pos_ble/ | `test -e tests/e2e/pos_ble/` |
| TST-E2E-22 | P0 | qwen | ни один из целевых файлов не существует: tests/e2e/smart_s/ | `test -e tests/e2e/smart_s/` |
| TST-E2E-23 | P0 | qwen | ни один из целевых файлов не существует: tests/e2e/bluepay/ | `test -e tests/e2e/bluepay/` |
| TST-E2E-24 | P0 | codex-first | ни один из целевых файлов не существует: tests/e2e/restart_cas/ | `test -e tests/e2e/restart_cas/` |
| TST-E2E-25 | P1 | qwen | ни один из целевых файлов не существует: tests/e2e/payment_slots/ | `test -e tests/e2e/payment_slots/` |
| TST-E2E-26 | P0 | qwen | ни один из целевых файлов не существует: tests/e2e/storno_order/ | `test -e tests/e2e/storno_order/` |
| TST-DEEP-14 | P1 | qwen | ни один из целевых файлов не существует: IoT/common-modules/datecspay/tests/DatecsPayGoldenFramesTest.cpp | `test -e IoT/common-modules/datecspay/tests/DatecsPayGoldenFramesTest.cpp` |

## 5. Не проверялось кодом (196 задач)
Юридические/нормативные, документные, решения и задачи, чьи признаки закрытия не извлекаются поиском. Считаются актуальными, пока не доказано обратное. Состав по префиксам: AA 2, ARCH 4, BF 1, BP 4, BUG 22, CL 1, CODE 4, DOC 12, DSY 5, EC 2, GATE 1, MP 7, MQ 1, NAP 19, NORM 16, PIPE 2, POS 15, PRB 5, S3D 2, SCOPE 5, SCR 1, SMS 2, STYLE 3, TG 10, TST 25, VG 22, W0 3.

Список: DOC-002, DOC-003, GATE-01, NAP-14, NAP-15, NORM-01, NORM-02, NORM-03, NORM-04, NORM-05, NORM-06, NORM-07, NORM-08, NORM-09, NORM-10, NORM-11, NORM-12, W0-1, W0-2, W0-3, BUG-004, BUG-012, POS-09, ARCH-005, BUG-023, BUG-061, BUG-062, POS-01, POS-10, TG-013, ARCH-008, BUG-022, BUG-025, BUG-030, BUG-031, CODE-001, CODE-002, TG-004, TG-005, TG-006, NAP-12, TG-003, BUG-051, BUG-052, BUG-056, BUG-058, POS-02, POS-03, POS-04, POS-05, POS-06, POS-07, POS-08, POS-11, POS-12, POS-13, POS-14, POS-15, ARCH-001, CODE-006, TG-001, TG-015, NAP-01, NAP-03, NAP-11, NAP-13, TG-016, TST-NAP-01, TST-NAP-02, ARCH-009, CODE-004, DOC-001, DOC-004, DOC-005, NAP-08, NAP-10, STYLE-001, STYLE-002, STYLE-003, TG-012, NAP-09, VG-19, NAP-N0, DOC-V1, DOC-V2, DOC-V4, DOC-V5, DOC-V6, NORM-13, NORM-14, NORM-15, NORM-16, VG-20, VG-01, VG-03, VG-02, VG-06, VG-05, VG-04, VG-07, VG-13, VG-09, NAP-16, VG-10, NAP-20, VG-08, VG-12, VG-15, VG-14, VG-16, VG-17, NAP-18, NAP-21, NAP-23, VG-18, NAP-17, NAP-22, BUG-063, BUG-064, BUG-065, BUG-066, BUG-067, BUG-068, BUG-069, BUG-070, TG-017, SCR-36, MP-01, MP-02, MP-03, MP-04, MP-05, MP-06, MP-07, AA-02, AA-04, BF-02, EC-01, EC-06, SCOPE-02, SCOPE-03, SCOPE-04, SCOPE-05, SCOPE-06, BUG-071, VG-23, TST-E2E-01, TST-E2E-02, TST-E2E-03, TST-E2E-04, TST-E2E-05, TST-E2E-06, TST-E2E-07, TST-E2E-08, TST-E2E-09, TST-E2E-10, TST-E2E-11, TST-E2E-12, TST-E2E-13, TST-E2E-14, TST-E2E-15, TST-E2E-16, TST-E2E-17, TST-E2E-18, TST-E2E-19, TST-E2E-20, VG-21, VG-22, PRB-01, PRB-02, PRB-03, PRB-04, PRB-05, DOC-V7, TST-CRIT-01, DSY-04, DSY-05, DSY-06, DSY-07, DSY-08, S3D-01, S3D-03, BP-03, MQ-01, CL-02, NAP-P-01, SMS-01, SMS-06, DOC-V4B, BP-04, BP-05, BP-06, TST-DEEP-13, TST-GATE-01, PIPE-02, PIPE-03
