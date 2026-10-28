# ARCH_03 — Тесты и E2E сценарии, которые ещё нужно написать

Формат: **ID · Что проверяет · Где создать · Шаги · Ожидаемый результат (автоматически проверяемый) · Команда запуска · Связанные баги/задачи**.
Общий DoD — CDOD из [ARCH_00_INDEX.md](ARCH_00_INDEX.md). Принцип: сначала красный тест, затем правка (TDD).

## 0. Слепые зоны анализа (честно)
- Не выполнялись `go test`, `gradlew`, `make iot-test`, `npx jest` (код не запускался; анализ статический).
- Не просматривались целиком: `BlueCashLocalHttpServer.kt` (частично), BLE-транспорт S3, `erp-connector`, `public-api-core`, тестовые каталоги `integration/`.
- Реальные ответы ФУ Datecs не анализировались: форматы — из `DatecsProtocol.kt` и `03_DEVICE_PROTOCOLS.md`.
- Любое утверждение «тестов нет» перепроверить `grep -rn "<TestName>"` перед созданием.

## 1. Юнит-тесты backend (Go) — `fiscal-backend/internal/...`

| ID | Файл (создать; ★ = файл уже существует — дополнить) | Тест | Покрывает |
|---|---|---|---|
| TST-GO-01 | `internal/domain/receipt_money_test.go` | `TestReceiptMoneyCentsRejectsSignedAndMalformed` | BUG-001 |
| TST-GO-02 | `internal/domain/service_money_test.go` | `TestValidMoney` | BUG-002 |
| TST-GO-03 | `internal/domain/simulated_flag_test.go` | `TestOperationSimulatedFlagFollowsDriver` | BUG-003 |
| TST-GO-04 | `internal/domain/fiscal_operation_test.go` | `TestFiscalOperationPrepareFailureMarksOperationFailed`, `TestFiscalOperationUsesBoundDriver` | BUG-004 |
| TST-GO-05 | `internal/domain/clock_sync_test.go` | `TestHasDailyClockSyncUsesSofiaBusinessDate`, `TestSyncDriftOver30sTriggersSet` | BUG-005 |
| TST-GO-06 | `internal/config/config_test.go` ★ | `TestValidateRejectsUnknownAppEnv`, `TestLoadDefaultsAreSafe` | BUG-009 |
| TST-GO-07 | `internal/api/health_test.go` | `TestReadyzFailsWhenRepositoryUnavailable` | BUG-010 |
| TST-GO-08 | `internal/auth/auth_test.go` ★ | `TestHMACTokenCannotCarryAdminRoleInProd` | BUG-011 |
| TST-GO-09 | `internal/api/idempotency_test.go` | `TestIdempotencyKeyReusableAfterPreSideEffect503` | BUG-012 |
| TST-GO-10 | `internal/domain/reversal_policy_test.go` ★ | `TestReversalPolicy_<Case>` по таблице | TG-009 |
| TST-GO-11 | `internal/domain/repository_contract_test.go` | `TestConcurrentFinalizeSameKeyProducesOneOperation` и др. | TG-001 |
| TST-GO-12 | `internal/domain/unp_allocator_test.go` ★ | `TestUNPAllocatorExhaustion`, `TestUNPFormatRegex` | TG-002 |
| TST-GO-13 | `internal/domain/payment_capability_test.go` | `TestPaymentTypeCapabilityMatrix` | BUG-042 |
| TST-GO-14 | `internal/domain/rounding_golden_test.go` | `TestMoneyRoundingGolden` | BUG-023 |

Для каждого: **Команда:** `cd fiscal-backend && go test ./internal/<pkg> -run '<TestName>' -race -count=1`. **Ожидание:** до исправления FAIL, после — ok.

Свойственные (property-based) тесты (`testing/quick` или `rapid`): `TestProperty_SumOfLinesEqualsTotal`, `TestProperty_UNPMonotonic`, `TestProperty_ReversalNeverExceedsOriginal`.

## 2. Юнит-тесты BlueCash (Kotlin) — `SmartDevices/bluecash-app/src/test/kotlin/...`

| ID | Тест | Покрывает |
|---|---|---|
| TST-KT-01 | `saleCancelsOpenReceiptWhenLineRejected`, `saleCancelsOpenReceiptWhenPaymentRejected` | BUG-020 |
| TST-KT-02 | `saleDoesNotReverseCardWhenFiscalResultUnknown`, `saleReversesCardWhenFiscalDefinitelyRejected` | BUG-021 |
| TST-KT-03 | `salePrevalidatesAllPayloadsBeforeCardPurchase`, `quantityScientificNotationRejected` | BUG-022 |
| TST-KT-04 | `MoneyRoundingGoldenTest` | BUG-023 |
| TST-KT-05 | `aggregateReplayAfterCrashReturnsRecoveryRequired` | BUG-024 |
| TST-KT-06 | `replayOfCompensatedSaleReturnsCompensated` | BUG-025 |
| TST-KT-07 | `closeReceiptWithoutParsableDocumentNumberIsUnknown` | BUG-026 |
| TST-KT-08 | `emptyDeviceResponseIsNotSuccess` | BUG-027 |
| TST-KT-09 | `everyTransportErrorAfterWriteIsUnknown` | BUG-028 |
| TST-KT-10 | `synBytesDoNotCorruptFrame`, `nakTriggersByteIdenticalRetransmit`, `silentDeviceTimesOutWithUnknown`, `concurrentCallersSerialized` | BUG-029 |
| TST-KT-11 | `acknowledgeWithWrongHashRejected` | BUG-030 |
| TST-KT-12 | `printerTestClosesNonFiscalReceiptOnFailure` | BUG-031 |
| TST-KT-13 | `RuntimeServiceLifecycleTest` | BUG-032 |
| TST-KT-14 | `journalChainBreakDetected` (подмена строки → `verifyChain` падает) | AndroidJournal |
| TST-KT-15 | `localHttpRejectsUnsignedAndReplayedRequests` | BlueCashLocalHttpServer |

**Команда:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*<Class>*'`. **Ожидание:** BUILD SUCCESSFUL.

## 3. Прошивка S3 (host-тесты) — `IoT/firmware/edge-agent-s3/idf/tests/`

| ID | Тест | Покрывает |
|---|---|---|
| TST-S3-01 | `device_unavailable_does_not_consume_unp`, `retry_after_unavailable_reuses_operation` | BUG-040 |
| TST-S3-02 | `intent_requires_trusted_time` | BUG-041 |
| TST-S3-03 | `payment_type_not_supported_rejected_before_io` | BUG-042 |
| TST-S3-04 | `sale_total_mismatch_rejected_before_reservation` | BUG-043 |
| TST-S3-05 | `payload_conflict_produces_reply` | BUG-044 |
| TST-S3-06 | `power_loss_between_reserve_and_execute_recovers` (сбой на каждой границе записи) | durable storage |

**Команда:** `make iot-test` (exit 0; имя теста присутствует в выводе — `make iot-test | grep -c PASS`). Новые native-тесты саги добавлять в существующие `idf/tests/receipt_saga_native.cpp` + `run_native_saga.sh`, money — в `idf/tests/money_host`/`run_money_host.sh`.

## 4. POS (TypeScript / Go)

| ID | Где | Тест | Покрывает |
|---|---|---|---|
| TST-POS-01 | `POS/pos-app/src/infrastructure/runtime/__tests__/HttpFiscalApiClient.test.ts` | reconcile URL `/reconcile` | BUG-050 |
| TST-POS-02 | там же | routing: метод → baseURL | BUG-051 |
| TST-POS-03 | там же | `reconcileUsesNewKeyPerAttempt` | BUG-052 |
| TST-POS-04 | `.../LocalReceiptOperations.test.ts` | пустой УНП → `NEEDS_ATTENTION`, печать не вызвана | BUG-053 |
| TST-POS-05 | `.../request.timeoutOnWriteMarksUnknown.test.ts` | таймаут записи = UNKNOWN | BUG-054 |
| TST-POS-06 | `POS/pos-gateway/internal/drain/idle_test.go` | `finalizeCommandCarriesAllPaymentsAndTypes` | BUG-055 |
| TST-POS-07 | `POS/pos-gateway/internal/handler/fiscal_handler_test.go` | `Submit_projectsTotalsFromPayload`, `Submit_doesNotAssume20PercentVAT`, `Submit_usesIdempotencyKeyAsOperationID`, `Submit_failsClosedOnPolicyLookupError` | BUG-056/057/058 |
| TST-POS-08 | `POS/pos-gateway/internal/fiscal/result_consumer_test.go` | `invalidEnvelopeGoesToDLQNotRequeue` | BUG-059 |
| TST-POS-09 | `POS/pos-app/src/shared/utils/decimal.golden.test.ts` | golden-вектора округления | BUG-023/061 |

**Команды:** `cd POS/pos-app && npx tsc --noEmit && npx jest <path>`; `cd POS/pos-gateway && go test ./internal/... -race -count=1`.

## 5. Контрактные тесты

| ID | Что | Где |
|---|---|---|
| TST-CT-01 | Golden округления: один JSON — три потребителя (Go/Kotlin/TS) | `Fiscalisation/contracts/money-rounding-golden-v1.json` |
| TST-CT-02 | OpenAPI ↔ фактические ответы handler'ов (schemathesis/`kin-openapi`) | `fiscal-backend/internal/api/contract_test.go` |
| TST-CT-03 | MQTT-конверты backend ↔ S3 ↔ BlueCash (CBOR/JSON векторы) | `contracts/mqtt-vectors/*.json` |
| TST-CT-04 | `FiscalIntentPayload` POS ↔ шлюз ↔ backend | `POS/docs/contracts/fiscal-intent.v1.json` |
| TST-CT-05 | Тест-вектора HMAC/CBOR подписи на трёх реализациях | `contracts/crypto-vectors-v1.json` |

**Приёмка:** `make contract-test` exit 0; изменение вектора ломает минимум по одному тесту каждого потребителя (проверка мутацией: `scripts/mutate-vector.sh` → ожидается FAIL).

## 6. Интеграционные тесты (docker compose)

| ID | Сценарий | Ожидаемый результат |
|---|---|---|
| TST-INT-01 | Рестарт backend между `PutOperation` и ответом клиенту, затем повтор с тем же ключом | один чек, один УНП |
| TST-INT-02 | Отключение PostgreSQL на 10 с во время финализации | клиенту 503 + `Retry-After`; после восстановления — повтор с тем же ключом успешен, без дублей (BUG-012) |
| TST-INT-03 | Потеря MQTT между publish и результатом | операция `EXECUTING` → reconciler → терминальное состояние (TG-010) |
| TST-INT-04 | Дубликат MQTT-результата (at-least-once) | идемпотентно, одно событие |
| TST-INT-05 | Результат с неверной HMAC подписью | отклонён, метрика `mqtt_bad_signature_total` ++ |
| TST-INT-06 | RabbitMQ недоступен 30 с | outbox POS накапливает; после восстановления доставка в порядке, без дублей на ФУ |
| TST-INT-07 | Параллельные 50 финализаций с одним ключом | ровно один `FISCALIZED` |

**Запуск:** существующие `make postgres-integration` и `make compose-e2e` (скрипты `scripts/postgres-integration.sh`, `scripts/e2e-two-compose.sh`); новые сценарии добавлять в них. Критерий: exit 0 каждой цели.

## 7. E2E (POS → шлюз → backend → симулятор / MQTT-эмулятор)

Размещение: `POS/tests/e2e/fiscal-integration/*.spec.ts` (есть transport/..); данные тест-вектора — `fixtures/`.

| ID | Сценарий | Проверки |
|---|---|---|
| TST-E2E-01 | Продажа наличными, 1 строка, НДС 20% | `FISCALIZED`, УНП формата, чек напечатан с УНП |
| TST-E2E-02 | Продажа картой (Borica-эмулятор) | платёж CARD доходит до backend, а не CASH (BUG-055) |
| TST-E2E-03 | Смешанная оплата CASH+CARD | суммы совпадают, оба платежа в чеке |
| TST-E2E-04 | НДС 9% и 0%, смешанные группы | разбивка по группам = POS |
| TST-E2E-05 | Дробное количество 0.333 × 1.99 | одинаковый итог в POS/backend/ФУ-эмуляторе (BUG-023) |
| TST-E2E-06 | Скидка на строку и на чек | порядок округления по ADR |
| TST-E2E-07 | Offline POS → накопление → синхронизация | порядок, без дублей, УНП уникальны |
| TST-E2E-08 | Таймаут ответа ФУ (UNKNOWN) | карта НЕ сторнируется; reconcile приводит к терминальному состоянию (BUG-021) |
| TST-E2E-09 | Сторно полное | исходный УНП, дата документа, новый УНП |
| TST-E2E-10 | Сторно частичное и повторное сторно | второе — отказ; сумма сторно ≤ остатка |
| TST-E2E-11 | Сторно после срока / другого оператора | отказ с кодом политики |
| TST-E2E-12 | Продажа без синхронизации часов за день | отказ `CLOCK_SYNC_REQUIRED` (BUG-005) |
| TST-E2E-13 | Z-отчёт при незавершённых операциях | отказ/предупреждение, данные не теряются |
| TST-E2E-14 | X-отчёт, служебное внесение/изъятие | суммы в журнале и отчёте |
| TST-E2E-15 | Смена даты через полночь Europe/Sofia | `business_date` корректен |
| TST-E2E-16 | Электронный чек | выдан только после `FISCALIZED`, содержит УНП (TG-008) |
| TST-E2E-17 | Повтор команды POS с тем же `Idempotency-Key` | тот же результат, нет нового УНП |
| TST-E2E-18 | Конфликт: тот же ключ — другой payload | 409/`IDEMPOTENCY_CONFLICT` |
| TST-E2E-19 | Недоступность шлюза → POS в режиме очереди | статус «ожидает», печать как не-фискальной запрещена |
| TST-E2E-20 | Пустой УНП в ответе | чек не выдаётся как фискальный (BUG-053) |

**Критерий приёмки каждого:** `npx playwright test|jest <spec> -t <ID>` → PASS; отчёт JUnit в `artifacts/e2e/`.
**DoD набора:** все 20 сценариев автоматизированы, зелёные в CI на симуляторе; список непокрытых — в `MVP_GATES.md` с причиной.

## 8. HIL (реальные устройства)

Общая процедура — `docs/MVP1/HIL_RUNBOOK.md` (создать, TG-014). Результат каждого прогона — `artifacts/hil/<date>/<ID>/result.json` (схема `contracts/hil-result.schema.json`).

| ID | Устройство | Сценарий | Проходной критерий |
|---|---|---|---|
| HIL-BC-01 | BlueCash+Datecs | Продажа наличными | документ на ФУ, номер совпадает с `fiscal_reference` |
| HIL-BC-02 | BlueCash+Datecs | Занятость ФУ (SYN), NAK | команда завершена без ошибки/дубля |
| HIL-BC-03 | BlueCash+Datecs | Продажа с картой (pinpad) | карта и ФУ согласованы |
| HIL-BC-04 | BlueCash+Datecs | Отказ карты | чек не открыт |
| HIL-BC-05 | BlueCash+Datecs | Обрыв питания/кабеля между 49 и 56 | после рестарта чек отменён или завершён, журнал согласован |
| HIL-BC-06 | BlueCash+Datecs | Сторно по cmd 43 | документ сторно с корректной ссылкой на исходный |
| HIL-BC-07 | BlueCash | Перезагрузка планшета | сервис поднят ≤2 мин, READY (BUG-032) |
| HIL-BC-08 | BlueCash+Datecs | X, Z, внесение/изъятие | суммы сходятся |
| HIL-CLK-01 | ФУ | Сдвиг часов ±2 мин | sync выставил, дрейф ≤30 с |
| HIL-S3-01 | S3+ФУ | Продажа по MQTT | как BC-01 |
| HIL-S3-02 | S3 | Продажа по BLE | как BC-01 |
| HIL-S3-03 | S3 | Нет NTP/сети | продажа отклонена `TIME_NOT_TRUSTED` (BUG-041) |
| HIL-S3-04 | S3 | Питание off между reserve и execute | recovery без дубля УНП |
| HIL-NAP-01 | ФУ | Передача данных в НАП (тестовая среда) | подтверждение приёма |
| HIL-NAP-02 | ФУ | Ошибка связи с НАП | накопление и повторная отправка |

## 9. Негативные и безопасностные тесты
- Fuzz: `go test -fuzz=FuzzReceiptMoneyCents ./internal/domain -fuzztime=60s`; `FuzzDatecsFrameDecoder` (Kotlin, jazzer или property-тест); `fuzz_cbor_decode` (C++).
- Auth: просроченный/чужой `aud`/`iss` токен; HMAC-токен с ролью ADMIN (BUG-011).
- MQTT: replay старой команды (нонс/время), команда на чужой tenant.
- Local HTTP BlueCash: без подписи, повтор, слишком большое тело.
- **Приёмка:** существующий `make security-test` (`ruby scripts/verify_security_regression.rb` + `scripts/verify_sensitive_data.sh`) расширить проверкой секретов (BUG-060) и подключить `make fuzz-short` (создать: `go test -fuzz` с `-fuzztime=30s` для fuzz-функций выше) → обе цели exit 0.

## 10. Метрики готовности тестирования (проверяются скриптом)
- `scripts/check-test-matrix.sh` читает этот файл, ищет каждый `TST-*` ID и проверяет, что в коде есть тест с соответствующим именем (`grep -rn`); выход ≠ 0, если что-то отсутствует.
- Покрытие: Go `go test ./... -cover` ≥ 70% для `internal/domain`; Kotlin — отчёт Jacoco ≥ 60% для `BlueCashEngine`, `DatecsProtocol`.

## 11. Вендорные тесты (добавлено по ARCH_09)

Эталон — `vendor/datecs_golden_vectors.json` (11 запросов, 2 ответа из примеров спецификации; независимо воспроизведены энкодером). Тесты пишутся **красными вперёд**.

| ID | Что проверяет | Где (создать/расширить) | Команда |
|---|---|---|---|
| TST-VENDOR-01 | Побайтное совпадение кадров cmd 33/48(×2)/49/53/56/60/62/74/76/103 с спецификацией; ответы 56/76 — BCC и STAT | `SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt` | `python3 SmartDevices/bluecash-app/tests/host/run-datecs-codec.py` |
| TST-VENDOR-02 | То же в C++ | `IoT/common-modules/datecs/tests/DatecsProtocolTest.cpp`, `IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp` | `bash IoT/common-modules/datecs/run-tests.sh && bash IoT/protocol-abstraction/run-tests.sh` |
| TST-VENDOR-03 | Кириллица → windows-1251 (вектор cmd 49), отказ для эмодзи, длина в байтах | `DatecsPayloadsTest.kt` | `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsPayloads*' --no-daemon` |
| TST-VENDOR-04 | Daisy: CP1251 и кириллические налоговые буквы А–З, SYN каждые 100 мс | `IoT/common-modules/daisy/tests/` | `bash IoT/common-modules/daisy/run-tests.sh` |
| TST-VENDOR-05 | Пустой ответ = отказ; каждый бит STAT из §6 и каждая категория ошибок §7 | `DatecsStatusTest.kt`, `DatecsOkTest.kt` | `… --tests '*DatecsStatus*' --tests '*DatecsOk*'` |
| TST-VENDOR-06 | Эмулятор ФУ с кэшем последнего ответа по SEQ: рестарт приложения не теряет команду | `FakeDatecsDevice.kt`, `DatecsSequenceTest.kt` | `… --tests '*DatecsSequence*'` |
| TST-VENDOR-07 | Восстановление: COMMITTED/NOT_COMMITTED по `IsOpen`, `Number`, параметру `UNP`; осиротевший чек → cmd 60 | `DatecsRecoveryTest.kt` | `… --tests '*DatecsRecovery*'` |
| TST-VENDOR-08 | Режим валюты ФУ ≠ евро → отказ **до** списания по карте; расхождение часов | `DeviceCurrencyTest.kt`, `DatecsClockTest.kt` | `… --tests '*DeviceCurrency*' --tests '*DatecsClock*'` |
| TST-VENDOR-09 | Каталог ошибок полон (число кодов = числу строк раздела 7 PDF) | `scripts/check-datecs-error-catalog.sh` | `bash scripts/check-datecs-error-catalog.sh` |
| TST-VENDOR-10 | Манифест вендорных PDF: SHA256 совпадают | `scripts/check-vendor-docs.sh` | `bash scripts/check-vendor-docs.sh` |

HIL-сценарии, добавляемые этим разделом: `HIL-BC-03` (печать кириллицы), `HIL-BC-04` (обрыв связи на cmd 56 → разрешение через cmd 76/`UNP`), `HIL-BC-05` (Z-отчёт), `HIL-BC-06` (сторно по данным VG-07), `HIL-BC-EUR-01` (режим валюты), `HIL-BC-EUR-02` (смена режима валюты, ARCH_10 NAP-17).
