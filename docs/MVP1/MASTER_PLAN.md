# MASTER PLAN — устранение всех разрывов и багов перед тестированием с реальными устройствами

**Это отправная точка для агента исправлений.** Читать первым. Детали каждой задачи — в ARCH_01…ARCH_06; критика плана 70 — в ARCH_07. Индекс и общий Definition of Done (CDOD) — [ARCH_00_INDEX.md](ARCH_00_INDEX.md).
Проверка полноты и путь к MVP: [ARCH_11](ARCH_11_MVP_COMPLETENESS_REVIEW.md). Вендорные разрывы: [ARCH_09](ARCH_09_VENDOR_DOCS_AND_PROTOCOL_GAPS.md); последние нормы НАП: [ARCH_10](ARCH_10_NAP_LATEST_CHANGES.md).
Раунд 3: решения [DECISIONS](DECISIONS.md); скрипты приёмки [ARCH_12](ARCH_12_ACCEPTANCE_SCRIPTS.md); компоненты [ARCH_13](ARCH_13_COMPONENTS_MINIPOS_ADMIN_ERP.md); тест-спецификации [ARCH_15](ARCH_15_TEST_SPECS_WITH_LOGIC.md); декомпозиция до функций `execution/decomp/`. Всего задач в `status.json`: 314, подзадач: 574; общих формулировок подзадач нет (проверка `python3 docs/MVP1/tools/check-doc-refs.py --status`). **Пакет задачи для слабой модели**: `python3 docs/MVP1/tools/task-pack.py <ID>` (≤6000 токенов; проверка всех: `--all --check`). Логика тестов: ARCH_15, ARCH_18 (60 тестов Given/When/Then); сценарии по путям S1…S8: ARCH_19; эмуляторы: ARCH_20.
Машинный трекер: [execution/status.json](execution/status.json) (все ID задач, волна, статус).

## 1. Цель и границы
> **Цель MVP зафиксирована в [MVP_GOAL.md](MVP_GOAL.md)** (пять путей P-A…P-E; POS+MiniPOS → MQTT/BLE). Пути и задачи — [ARCH_17](ARCH_17_MVP_GOAL_PATHS_GAPS.md). Формулировки ниже, сужающие MVP до BlueCash-50, устарели.

- **Цель:** довести Fiscalisation + BlueCash/Datecs + S3 + интеграцию с Beeloy POS до состояния «можно подключать реальные ФУ для лабораторного MVP-теста» и закрыть блокеры СУПТО/Н-18 (Приложение 29), которые закрываются кодом и тестами.
- **Не входит:** физические/электрические проверки, юридическая сертификация, решения вендора — они остаются `HIL_PENDING`/`EXTERNAL_PENDING` (см. ARCH_06 §2–3).
- Анализ, на котором основан план, был статическим (код не запускался). Поэтому **первое действие агента — зафиксировать фактическое состояние тестов** (волна 0).

## 2. Правила для агента (обязательно)
1. Одна задача = один scoped-коммит. Не смешивать задачи; не править файлы вне перечисленных в задаче.
2. Порядок: прочитать задачу → написать тест и **увидеть его красным** → исправить → тест зелёный → команды «Приёмка» → CDOD → обновить `status.json` → коммит.
3. Запрещено: удалять/ослаблять существующие тесты, `skip`, подгонять хэши contract-lock, менять ожидания теста под баг, вводить «временный» fallback, делающий операцию «успешной» без ответа ФУ.
4. Денежные пути: всегда проверять случаи **UNKNOWN/timeout** (UNKNOWN ≠ NOT_SENT; карта не сторнируется при неизвестном фискальном исходе; повтор с тем же ключом).
5. Нормативные вопросы (NORM-xx) и решения владельца (D-xx) **не угадывать**: пометить задачу `BLOCKED_SPEC`, перейти к следующей независимой.
6. Перед созданием теста — `grep -rn "<имя>"`: он может уже существовать (см. «Существующие Make-цели» в ARCH_00).
7. Пункты, помеченные TO VERIFY (например BUG-012, часть POS-xx), начинаются с теста-проверки гипотезы; если тест зелёный сразу — задача закрывается как `NOT_A_BUG` с ссылкой на тест.
8. Если задача оказалась крупнее описанной — разбить на подзадачи `ID.1/.2/…` (каждая ≤ 3 файлов, один инвариант), записать в `status.json`.
9. Не коммитить секреты. BUG-060 требует ротации ключей владельцем — агент только удаляет из индекса и добавляет защиту.

## 3. Решения владельца до старта волн (не блокируют волну 0–1)
| ID | Решение | Блокирует |
|---|---|---|
| D-1 | **Режим первого HIL** (лабораторный; BlueCash-50 в `MVP_GATES.md` = `EXCLUDED_FROM_PROD`) — GATE-01 | волна 9 |
| D-2 | **Линия отсечения MVP** (ARCH_07 R-05): что обязательно до HIL, что после | состав волн 6–8 |
| D-3 | Персистентность (TG-001/ARCH-001) — до лабораторного HIL или до пилота | волна 6 |
| D-4 | Политика округления (ADR-MONEY-ROUNDING-001): предложение — half-up на строку в целых центах | волна 2 |
| D-5 | Судьба `receipt_saga.go` (удалить или довести) — BUG-006 | волна 6 |
| D-6 | Ответы NORM-01…NORM-16 (NORM-13…16 — в ARCH_10: версия СУПТО при евро, чл.118 ЗДДС, ДВ 82/2026, сторно лв-документов)  ·  Ответы NORM-01…NORM-12 (юрист/НАП; NORM-09…12 — в ARCH_08 §6; **NORM-09 — классификация Beeloy POS** — ключевой) | TG-008/NAP-13, TG-009/NAP-01…03, TG-002, POS-11, NAP-04, NAP-05, NAP-06, NAP-12 |
| D-7 | Rotation keystore/HMAC-ключей (BUG-060, BUG-011) | закрытие BUG-060 |
| D-8 | Платёж картой: пинпад самого ФУ (cmd 53 Type=1 / cmd 55, ARCH_09 VG-14) или внешний Datecs Pay + сага (текущий путь) | VG-14; состав HIL карты |
| D-9 | Профиль слотов оплаты ФУ (CHEQUE/VOUCHER/NHIF/DEFERRED/INTERNAL_CONSUMPTION → PaidMode 3/4/5, печатные имена) по модели/прошивке | VG-05, BUG-042 |
| D-10 | Допустим ли режим валюты ФУ «2» (основная евро, доп. лев) или только «3» (ARCH_10 E1) | VG-09, NAP-16 |
| DOC-V1…V6 | Вендорные документы и сертификаты (ARCH_09 §1.2) — запросить у Datecs/Daisy/банка/БИМ | VG-09, VG-16, VG-17, GATE-01, HIL карты |
| D-11 | Входят ли в MVP1 `minipos/`, `AdminApp/`, `BeeFiscalApp/` (не покрыты анализом; ARCH_11 §6.2) | SCOPE-01 |

## 4. Волны (последовательность) и ворота
Задачи внутри волны можно выполнять параллельно, если не делят файлы. Переход к следующей волне — только после **ворот волны** (команды ниже должны завершаться exit 0).

| Волна | Тема | Результат |
|---|---|---|
| W0 | Подготовка и базовая линия | известно реальное состояние тестов; трекер и скрипты покрытия; ADR; contract-lock зелёный |
| W1 | Быстрые безопасные правки backend и секреты | деньги парсятся строго; нет симулятора по умолчанию; readiness/auth исправлены; секреты вне git |
| W2 | Деньги и единый контракт | одна политика округления во всех компонентах; схема intent/result |
| W3 | Ядро BlueCash/Datecs | транспорт, разбор ответов, отмена чека, безопасная компенсация карты, жизненный цикл Android |
| W4 | Время и УНП | реальная синхронизация часов, бизнес-дата Europe/Sofia, S3 без времени 0, дыры УНП |
| W5 | Интеграция POS ↔ Fiscal | корректные URL, платежи, УНП, идемпотентность, DLQ, E2E |
| W6 | Персистентность и восстановление | транзакционное хранение, reconciler, миграции |
| W7 | Функциональные разрывы | сторно-политика, электронный чек, типы оплат, экспорт Annex 29, нагрузка |
| W8 | Качество и архитектура | рефакторинг под защитой тестов, линтеры, документация кода, наблюдаемость |
| W9 | Ворота перед HIL | HIL runbook, ops-документы, полный прогон, статус готовности |

### W0 — Подготовка
| Задача | Что | Где |
|---|---|---|
| W0-1 | Снять базовую линию: запустить `make test-race vet contract-test security-test iot-test` и gradle-тесты BlueCash, сохранить вывод в `execution/baseline/<дата>.txt`; перечень красных тестов → `execution/baseline/red.md` | ARCH_00 CDOD |
| W0-2 | Создать `execution/coverage-map.v1.json` и скрипт `scripts/check_plan_coverage.rb` (ARCH_07 R-01) | ARCH_07 |
| W0-3 | Создать `scripts/check-test-matrix.sh` (ARCH_03 §10) | ARCH_03 |
| DOC-003 | Починить `verify-contract-lock.sh` (владелец контракта подтверждает изменение) | ARCH_04 |
| DOC-002 | ADR: MONEY-ROUNDING, PERSIST, SAGA, UNKNOWN-OUTCOME, TIME-BUSINESS-DATE, PAYMENT-CAPABILITY (после D-4/D-5) | ARCH_04 |
| GATE-01, NORM-01…NORM-12 | запрос решений (создать `NORM_DECISIONS.md`) | ARCH_06, ARCH_08 §6 |
| VG-19, DOC-V1…V6 | вендорный манифест + скрипт хешей; запросы документов (ARCH_09 §1) | ARCH_09 |
| NAP-N0, NORM-13…16 | заморозка нормативного baseline (повтор на W9); вопросы юристу | ARCH_10 |
| NAP-14, NAP-15 | зафиксировать исключения объёма; актуализировать trace и локальную нормативную базу (`ANNEX29_CURRENT_DELTA.md`) | ARCH_08 |

**Ворота W0:** `ls execution/baseline/*.txt` непусто; `ruby scripts/check_plan_coverage.rb` запускается (допустим `unmapped>0` на этом этапе); `sh scripts/verify-contract-lock.sh` exit 0; `bash scripts/check-vendor-docs.sh` exit 0; `docs/SUPTO/nap-baseline-<date>.md` существует.

### W1 — Быстрые безопасные правки
| Задача | Приоритет | Тест (имя) |
|---|---|---|
| BUG-001, BUG-002, ARCH-004 | P0/P1 | `TestReceiptMoneyCentsRejectsSignedAndMalformed`, `TestValidMoney` |
| BUG-003, ARCH-006, BUG-004 | P0/P1 | `TestOperationSimulatedFlagFollowsDriver`, `TestFiscalOperation*` |
| BUG-009 | P0 | `TestValidateRejectsUnknownAppEnv`, `TestLoadDefaultsAreSafe` |
| BUG-010 | P1 | `TestReadyzFailsWhenRepositoryUnavailable` |
| BUG-011 | P1 | `TestHMACTokenCannotCarryAdminRoleInProd` |
| BUG-012 | P1 (TO VERIFY) | `TestIdempotencyKeyReusableAfterPreSideEffect503` |
| BUG-060, POS-09 | P0 | `scripts/check-no-secrets.sh` |

**Ворота W1:** `cd fiscal-backend && go vet ./... && go test ./... -race -count=1`; `scripts/check-no-secrets.sh` exit 0; `grep -n 'Simulated: true' fiscal-backend/internal/domain/service.go` пусто.

### W2 — Деньги и контракт
| Задача | Тест / артефакт |
|---|---|
| BUG-062 | `TestUNPGoldenVectors` (Go/Kotlin/S3 на `contracts/bg-unp-v1-golden.json`) |
| BUG-023, ARCH-005, TST-CT-01 | `contracts/money-rounding-golden-v1.json` + три golden-теста (Go/Kotlin/TS) |
| BUG-061, POS-10 | `decimal.golden.test.ts`, eslint-правило |
| BUG-043 | `sale_total_mismatch_rejected_before_reservation` (S3) |
| NAP-19 | `contracts/testdata/rounding.json` + Go/Kotlin/TS golden-тесты; ADR привязан к ЗВЕРБ чл.12–13 |
| POS-01, TG-007, TG-013 | `POS/docs/contracts/fiscal-intent.v1.json` + контрактные тесты обеих сторон; `make contract-test` |

**Ворота W2:** три golden-набора зелёные на одном JSON; `make contract-test` exit 0; `cd POS/pos-app && npx tsc --noEmit && npx jest contract.fiscalIntent`; `cd POS/pos-gateway && go test ./internal/handler -run FiscalIntentContract`.

### W3 — Ядро BlueCash/Datecs (порядок важен)
0. **VG-20** (golden-векторы из спецификации) → **VG-01** (windows-1251) → **VG-02** (STAT/пустой ответ/каталог ошибок; закрывает BUG-027) — до остальных правок протокола.
1. BUG-028 (типизированные ошибки) → CODE-001 → BUG-029/TG-004 (таймауты, SYN, NAK).
2. VG-06, VG-05 (после D-9), VG-13; BUG-026 + **VG-07** (FiscalEvidence), TG-006 (разбор ответов по фикстурам вендора; номера команд брать **только из PDF вендора**, иначе `BLOCKED_SPEC`).
3. **VG-03** (SEQ), **VG-04** (DatecsRecovery: cmd 74/76/103/60 + параметр UNP) вместе с BUG-020, TG-005, BUG-031 (отмена чека, восстановление при старте), затем BUG-021 (карта при UNKNOWN), BUG-022 (предвалидация), BUG-024, BUG-025, BUG-030, CODE-002, CODE-003.
4. BUG-032, ARCH-007, ARCH-008 (жизненный цикл Android, источник истины).

**Ворота W3:** `cd SmartDevices && ./gradlew :bluecash-app:testProdDebugUnitTest :bluecash-app:testDemoDebugUnitTest --no-daemon` exit 0; в выводе присутствуют тесты `TST-KT-01…13` (ARCH_03 §2); `grep -rn 'e.message in' SmartDevices/bluecash-app/src/main` пусто; `grep -n '"DATECS-' …/BlueCashEngine.kt` пусто.

### W4 — Время и УНП
| Задача | Тест |
|---|---|
| BUG-005, TG-003, CODE-007 | `TestSyncWorkstationClockMeasuresDeviceClock`, `TestHasDailyClockSyncUsesSofiaBusinessDate` (имена с `Clock` — подхватывает `make supto-time-test`) |
| BUG-008 | `TestReverseUsesFiscalDocumentTimestamp` |
| BUG-041 | `intent_requires_trusted_time` (S3 host) |
| BUG-040, TG-002 | `device_unavailable_does_not_consume_unp`, `TestUNPAllocatorExhaustion` |
| NAP-12 | `TestUNPRangesDisjointAcrossEdgeAndOnline` |
| VG-09 (=NAP-16), VG-10, VG-11 | `*DeviceCurrency*`, `*DatecsReadiness*`, `*DatecsClock*`; backend `Readiness` |

**Ворота W4:** `make supto-time-test iot-test`; `go test ./internal/domain -run UNP -race -count=1`; Kotlin `*Clock*` тесты.

### W5 — Интеграция POS (ARCH_05)
Дополнительно: NAP-20 (валюта/курс в проекции чека). Дополнительно (ARCH_08): NAP-04 (доказательства импорта, после NORM-09), NAP-05 (покрытие POS требованиями Приложения 29). Порядок: BUG-050/051/POS-02 → BUG-055/056/POS-03 → BUG-053/POS-04 → BUG-052/057/POS-05 → BUG-054/POS-06 → BUG-058/POS-07 → BUG-059/POS-08 → POS-11, POS-12, POS-13, POS-14, POS-15.

**Ворота W5:** `cd POS/pos-app && npx tsc --noEmit && npx jest`; `cd POS/pos-gateway && go vet ./... && go test ./... -race -count=1`; E2E TST-E2E-01…06, 08, 17, 20 на симуляторе (`POS/tests/e2e/fiscal-integration`); `POS/scripts/check-rabbit-topology.sh` exit 0.

### W6 — Персистентность и восстановление (объём по D-2/D-3)
TG-001 + ARCH-001 → TG-015 → TG-010 → BUG-006 (по D-5) → CODE-006.
**Ворота W6:** `go test ./internal/domain -run RepositoryContract -race -count=1` для Memory и Postgres; `make postgres-integration`; `scripts/check-migrations.sh` (up/down/up) exit 0.

### W7 — Функциональные разрывы
TG-009 + NAP-01/NAP-02/NAP-03 (после NORM-04/NORM-10) + VG-08 + NAP-18 (после NORM-16) + VG-12 + VG-15 + VG-16/VG-17 (после DOC-V3/V4) + VG-14 (решение D-8) + NAP-21 + NAP-23 → BUG-042 → BUG-044 → TG-008/NAP-13 (после NORM-01) → NAP-06 → NAP-11 → TST-NAP-01 → TST-NAP-02 → TG-016.
**Ворота W7:** `go test ./internal/domain -run 'ReversalPolicy|Export|PaymentTypeCapability' -count=1`; `scripts/check-prod-artifact.sh` exit 0; `scripts/loadtest/run.sh` → `duplicates=0 lost=0`.

### W8 — Качество и архитектура (только под защитой тестов)
NAP-07 (расположение БД), NAP-08 (версии/обновления), NAP-10 (профиль аудитора), ARCH-002, ARCH-003 (по одному шагу; после каждого полный `go test`), ARCH-009, CODE-004, CODE-005, STYLE-001…003, DOC-001, DOC-004, DOC-005, TG-011, TG-012.
**Ворота W8:** `make lint` (создать) exit 0; `wc -l fiscal-backend/internal/domain/service.go` ≤ 600 и `internal/api/handler.go` ≤ 500; `scripts/check-doc-invariants.sh`, `scripts/check-doc-drift.sh` exit 0.

### W9 — Ворота перед HIL
VG-18 (Tremol, при D-1); NAP-17 (HIL смена режима валюты); NAP-22 (версия СУПТО); **повторить NAP-N0** (заморозка baseline) непосредственно перед HIL.
NAP-09 (досье и экстрактор), TG-014 (`HIL_RUNBOOK.md`, `scripts/hil/run.sh`; раздел «Место проведения» — NAP-06), ops-документы (ARCH_06 §4), актуализация `contracts/supto-annex29-trace.json`.
**Ворота W9 (= готовность к лабораторному HIL):**
- `make regression` (существующая агрегирующая цель) exit 0, плюс `make postgres-integration compose-e2e`;
- `ruby scripts/check_plan_coverage.rb` → `unmapped: 0`; `scripts/check-test-matrix.sh --severity P0` exit 0;
- `ruby scripts/verify_supto_trace.rb` exit 0; `scripts/check-ops-docs.sh` exit 0;
- `bash scripts/check-vendor-docs.sh && bash scripts/check-golden-vectors.sh && bash scripts/check-nap-baseline.sh` exit 0;
- DOC-V1 (прошивка с параметром валюты) и DOC-V5 (свидетельства типа) получены, иначе VG-09 = `BLOCKED_SPEC` и HIL не начинается;
- в `MVP_GATES.md` есть решение D-1 («First HIL mode»).

## 5. Алгоритм цикла агента
```
1. python3 -c "import json;d=json.load(open('docs/MVP1/execution/status.json'));print([t['id'] for t in d['tasks'] if t['status']=='NOT_STARTED' and t['wave']==<текущая>][:5])"
2. Выбрать задачу, у которой все `depends` в статусе DONE (поле depends в status.json; если пусто — по порядку из §4).
3. Открыть описание задачи по ссылке (поле doc/anchor), выполнить правила §2.
4. Прогнать «Приёмку» задачи, затем CDOD.
5. Обновить статус: NOT_STARTED → IN_PROGRESS → DONE | BLOCKED_SPEC | NOT_A_BUG | FAILED (с причиной в notes).
6. Коммит (одна задача), сообщение: "<ID>: <суть> (tests: <имена>)".
7. Когда все задачи волны DONE/NOT_A_BUG/BLOCKED_SPEC(с владельцем) — прогнать ворота волны; при FAIL — вернуться к виновной задаче.
```
**Остановка и эскалация:** три подряд неудачные попытки одной задачи; обнаруженное противоречие между документом вендора/нормативом и тестом; необходимость менять контракт без владельца; любой риск реального финансового действия — остановиться и записать в `notes` вопрос.

## 6. Критичный путь (кратко)
`W0 → W1 (BUG-001/003/009/060) → W2 (округление+контракт) → W3 (BlueCash) → W4 (время/УНП) → W5 (POS) → W9`. Волны W6–W8 идут параллельно после W2 в части, не затрагивающей общие файлы; их состав определяется D-2/D-3.

## 7. Сопоставление с планом 70
Карточки плана `docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md` остаются «как проверять и доказывать» (протокол Pxx.n, outcome-таксономия, Definition of Ready). Данный план задаёт **что и в каком порядке чинить**. Соответствие карточек и задач — [ARCH_07 §6](ARCH_07_PLAN70_CRITICAL_REVIEW.md). Новые карточки P40–P44 (ARCH_07 R-02) реализуются волнами W5, W1, W3, W2, W9 соответственно.

Раунд 5: конвейер поставки (1 разработчик + qwen + codex) [ARCH_21](ARCH_21_DELIVERY_PIPELINE.md); ревью тестов и e2e, новые сценарии TST-E2E-21…26 [ARCH_22](ARCH_22_TEST_E2E_REVIEW_ADDENDUM.md); письма вендорам `../vendor/LETTER_DAISY.md`, `../vendor/LETTER_DATECS.md`.

Раунд 6: сверка с кодом 2026-10-24 — [OPEN_WORK](OPEN_WORK.md) (очередь только актуальных работ), доказательства в `execution/reverify.json`.
