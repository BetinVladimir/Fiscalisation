# Полный план закрытия разрывов перед MVP испытаниями с реальными устройствами

Дата: **2026-10-21, Europe/Sofia**. Корень: `/Users/freelancer/Documents/Beeloy/Fiscalisation`.
Baseline исходников на момент составления: **394ef67dc8f95ed317d24652e6802e24bd681be2**. Перед исполнением обязательно обновить baseline по P00: работа и сборки продолжаются.

Ревью исполнимости выполнено против документа в commit `3e3c202`: [выводы и обязательные дополнения](#16-ревью-исполнимости-плана--2026-10-21). Исполнителю также обязательны разделы17–25; прежний короткий prompt без этих разделов недостаточен.

Документ предназначен для модели, которой нужны небольшие однозначные задания. Это **план исполнения и приёмки**, а не заявление о готовности. Он сохраняет полный согласованный объём: Fiscalisation/MiniPOS, S3 IoT, USB/COM/BLE/MQTT, Android/iOS, BlueCash, supported fiscal/payment devices, тесты/артефакты/evidence и подготовка применимого комплекта НАП.

## 1. Какой результат требуется

Закрыть все обязательные **software** разрывы; доказать поведение настоящих runtime модулей автоматическими тестами с независимыми wire oracle/stateful device emulators; собрать конкретные firmware/APK/iOS artifacts; подготовить безопасный воспроизводимый стенд и программу физического MVP тестирования.

Физическое напряжение/распиновка/PHY/радио/осциллятор/entropy/firmware производителя/эквайринг/внешняя применимость НАП не доказываются программными моками. До первого подключения эти пункты должны иметь точные процедуры, входные данные и stop criteria. Их результаты остаются `HIL_PENDING`/`EXTERNAL_PENDING`. Нельзя заявлять «все разрывы полностью закрыты», когда обязательная реализация или внешнее подтверждение ещё отсутствует.

**Неизменяемые ограничения:**

- Ключи текущего устройства хранятся **внутри ESP32-S3/NVS**. ATECC608A/608 не используется. Не добавлять внешний crypto/I2C/Qwiic/STEMMA модуль в код, BOM, wiring или обязательные tests.
- Удалённый корневой Go `edge-agent/` не восстанавливать. Production IoT — `IoT/firmware/edge-agent-s3/idf`, официальный ESP-IDF5.5.6; соседний Arduino skeleton не является доказательством готовности этого runtime.
- Никаких generic успешных `purchase()/fiscalize()/execute()` mocks для доказательства реального driver path. Подменяются только платформенные/физические границы, а stateful peer проверяет реальные bytes.
- UNKNOWN/possible-send не означает NOT_SENT. Ни timeout, ни 404 от другого маршрута, ни reconnect не разрешают второй debit/receipt.
- Intended electronic receipts, UNP/readiness на sale-entry, offline cycles, authenticated BLE и исходный scope не исключаются ради зелёного теста.
- Не прошивать/не устанавливать на реальное устройство/не совершать финансовые операции в ходе выполнения software-плана. Такие действия начинаются по отдельному HIL runbook после P38.

## 2. Источники и приоритет доказательств

Прочитать: [исходный реестр36GAP](02_GAP_REGISTER.md), [спецификациюT-01…T-27](05_TESTS_AND_ACCEPTANCE.md), [согласованный план/W9](04_REMEDIATION_PLAN.md), [автоматизациюA0–A11](07_AUTOMATION_EXECUTION.md), [цель](08_GOAL_AND_FAST_TRACK.md), [протоколы](03_DEVICE_PROTOCOLS.md), [текущую навигацию](README.md). Для конкретной задачи читать указанные далее source paths и тематические документы30…69.

Требуемое поведение определяют актуальные применимые нормативные требования и точный протокол производителя, затем согласованные продуктовые contracts/ADR. Actual current source+runtime evidence определяет, что реализовано и что реально проверено; ошибочная реализация не переопределяет требования. Исторические заметки помогают найти контекст, но не меняют contract. Конфликт не решать угадыванием. Создать `BLOCKED_SPEC` с противоречащими ссылками, минимальным контрпримером и конкретным вопросом. Самостоятельно продолжать независимые задачи.

Все пути ниже относительно корня. Запись `idf/main` означает `IoT/firmware/edge-agent-s3/idf/main`. Предлагаемые новые пути/entrypoints помечены явно; их наличие заранее не предполагается.

## 3. Что уже доказано и что заново проверять

| Область | Известное доказательство на baseline | Чего оно не закрывает |
|---|---|---|
| S3 target | c9aed74 full-IDF CI103Git files/7artifact hashes независимо проверены; source32ce5c0 target run39556851739 SUCCESS | Любое более позднее изменение требует соответствующего candidate/evidence |
| Новейший S3/transport | 2026-10-21 при составлении плана run39558958154 для8e35ee4 — completed SUCCESS | Root ещё должен скачать и независимо проверить именно эти artifacts/reports; не переносить103/7 от старой сборки |
| Actual crypto/lifecycle | Root GCC18+ASan/UBSan18, BLE proof/40interval cases, identity entropy/DRBG,8orchestrator tests PASS; remote crypto39558958200 SUCCESS | Provider/issuer/health/bootstrap/TLS runtime пока не включены; physical entropy/drift не проверены |
| CBOR/result/origin | Typed int64, strict JSON, actual queue→processor→SQLite→CBOR delivery, origin13cases; clean producer12/Go/Mini/browser66 и16negative gates PASS | Полная authenticated on-air BLE, offline authoritative outcome, все OS lifecycle |
| Ubuntu transport regression | Run39557126924 дал65PASS/1FAIL из-за fixed event-loop flush;606bfb6 заменяет его actual write-entry barrier, local fullclean66PASS | Требуется проверить corrected remote safe report, а не переписать failed историю как PASS |
| Integration E2E | Network/health исправлены, run39557126955 дошёл до HTTP400; actual clean PG reproduced missing fiscal.external_systems42P01 | Canonical integration fixture closure пока isolated candidate; source /tmp не означает committed/CI-ready |
| BlueCash | Actual prodDebug APK SHA256 c7549031baecfa5a854d6401da389b134038152bda7e42fd82c4eab3ede85da3; root69snapshot entries/14JUnit suites/43tests/manifest проверены | Debug build не production release, actual Datecs Binder service/hardware ещё не проверены |
| MiniPOS native | FullAppEntry Metro/native compilation начаты; первая isolated сборка получила Metaspace OOM; bounded memory retry выполнялся | Финальный APK не объявлять готовым без terminal result/hash. Tracked iOS lock Expo53/RN0.79 расходится с Expo54/RN0.81.5 |
| Регуляторика | Отдельные первичные поправки и публикационный охват проверены | Full current Art118/N18/e-receipt applicability и current MF PDF bytes/hash остаются OPEN |

Не переисполнять слепо уже исправленный дефект как новую разработку. Сначала reverify на новом candidate, затем исправлять только доказанный оставшийся gap.

## 4. Рабочий протокол слабой модели

Выполнять **одну подкарточку Pxx.n за раз**, с отдельным scoped commit на проверенный инвариант. Карточка Pxx является пакетом; её нельзя объявить закрытой после одной подкарточки. Перед началом применять обязательный Definition of Ready из раздела17. Не брать зависимую карточку, пока её prerequisites не имеют достаточного evidence. Внутри большой карточки сделать подзадачи `.1/.2/.3`, каждая не более одного инварианта/одного root cause.

1. Прочитать файлы карточки, найти actual caller/entrypoint и соответствующий existing test. Проверить, не закрыто ли это уже более поздним commit.
2. Выписать invariant и контрпример: вход, исходное state, допустимый результат, запрещённый irreversible action.
3. Получить RED по настоящему пути. Если failure вызван fixture/environment — исправить именно fixture/environment, не product security/contract.
4. Исправить минимальный root cause; сохранить idempotency/authority/ownership/durability. Обновить callers, schema/generated contracts и source inventory, если контракт изменился.
5. Запустить targeted positive+boundary+negative/fault cases. Для памяти/конкурентности — sanitizers/race и deterministic barriers. Для money/storage — actual DB/stateful peer.
6. Добавить meaningful counterfactual: удаление/поломка нужной проверки возвращает RED. Mutation выполняется в isolated disposable copy и после неё проверяется восстановление hashes.
7. Запустить зависимые suites и actual target/client build. Нельзя лечить regression удалением case, `.skip`, ослаблением assertion, fake success, permissive cert/time fallback или выключением lint.
8. Сохранить safe evidence и обновить эту задачу/inventory. Указать, чего именно тест не доказывает.
9. Reviewer проверяет diff, actual source match, cases и hashes. Только затем `SOFTWARE_VERIFIED_HIL_PENDING` для ограниченного scope.

Шаблон записи:

```yaml
id: Pxx.n
status: NOT_STARTED # INVESTIGATING / RED_REPRODUCED / FIX_IMPLEMENTED / SOFTWARE_VERIFIED_HIL_PENDING / BLOCKED_SPEC / FAILED
requirement_ids: []
source_snapshot: {commit: '', dirty_paths: [], file_sha256: {}}
actual_entrypoint: ''
invariant: ''
red_case: {input: '', initial_state: '', expected_failure: '', observed: ''}
changes: []
commands: [] # exact cwd/env (без secrets), argv, versions, exit_code
required_cases: []
passed: []
failed: []
skipped: [] # обязательный SKIP = gate FAIL
counterfactual: {mutation: '', actual_red: '', restored_sha256: ''}
artifacts: [] # реальные пути/hash/размер/target/signature
remaining: []
reviewer_result: ''
```

**Разрешено параллельно:** DB/backend; S3/transport; apps/build; нормативные sources. Владелец интеграции один. Не разрешать двум исполнителям одновременно менять один source/contract/generated file. Изолированные patches проходят `git apply --check`, hash review и independent tests до merge. Raw secrets/logs/fixtures не публиковать bulk artifact.

## 5. Порядок и dependencies

- Сначала P00–P02: baseline, полный inventory, profiles/sources.
- Параллельно можно готовить исследования/изолированные harness для потоков P03/P04 (DB), P06–P10 (S3/security), P23–P27 (protocol/emulator), P31 (native artifacts), P37 (актуальная нормативная база). Implementation/acceptance зависимой подкарточки разрешается только после её actual prerequisites; этот список не отменяет DAG.
- Затем P12–P22: money/UNP/offline/durability/recovery/results/UI. Общие profile/authority contracts фиксируются до параллельной реализации.
- P28–P36: весь объявленный operation scope, electronic receipts, audit/export, factory/release/security/resource coverage. Нормативные зависимости не угадывать.
- P38: окончательная проверка на exact integrated snapshot. P39: готовый runbook для первого физического подключения и полного MVP HIL.

Если карточка не применима к конкретному профилю, требуется profile-specific доказательство `UNSUPPORTED` и тест отказа до I/O. Это не разрешает вычеркнуть задачу из всего продукта.

## 6. Карточки исполнения

**Уточнение путей после ревью:** в каждой P00…P39 перечислены существующие файлы для actual implementation, тестов и contracts/build/docs. Существование каждого пути проверено по текущему дереву. Новые пути явно помечены `proposed` и не созданы этим документом. `IoT/common-drivers` исправлен на фактический `IoT/common-modules`; для дополнительных профилей используется существующее написание `termol` в имени каталога.

Пути являются стартовым impact map, не безусловным allowlist массовых правок. До редактирования найти symbol и actual caller через `rg -n`, записать точный scope Pxx.n; при переходе в новый caller добавить его путь в task record и этот plan. Не исправлять tests путём удаления assert и не заменять integration generic mock. Shared файлы из разных карточек меняет один владелец за раз.

### P00 — Зафиксировать фактический baseline и не потерять чужие изменения

**Зависимости:** нет. **Трассировка:** GAP-028/030; T-15/22/23.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [Makefile](/Users/freelancer/Documents/Beeloy/Fiscalisation/Makefile)
- [TOOLCHAIN.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/TOOLCHAIN.md)

**Тесты/gates — существующие файлы:**

- [scripts/verify_readiness_documentation.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_readiness_documentation.rb)

**Контракты/build/docs — читать и обновлять согласованно:**

- [.github/workflows/s3-idf.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/s3-idf.yml)
- [.github/workflows/integration-contract.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/integration-contract.yml)
- [docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/P00.1.md` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сохранить HEAD, branch, dirty/untracked paths и hashes затрагиваемых файлов. Не включать посторонние App.tsx/header/dist изменения в свои commits.
2. Снять список модулей, существующих Make targets, actual entrypoints, runtime profiles и всех открытых cases из JSON-контрактов. Проверить локальные инструкции AGENTS.md.
3. Прочитать текущие статусы CI по exact run IDs. Завершённую ошибку диагностировать; живой процесс наблюдать по тому же handle, не перезапускать только из-за timeout наблюдения.

**Тесты, которые должны существовать или быть добавлены:**

- Проверить, что каждый объявленный артефакт существует и его hash совпадает; отчёт без файла считается MISSING.
- Проверить, что baseline отражает dirty working tree, если тестировалась она, а не объявляет clean HEAD.

**Приёмка:** Создан immutable baseline manifest; для каждого результата известны source snapshot, host, tools, command, exit code, scope.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P01 — Свести все требования в один исполняемый backlog

**Зависимости:** P00. **Трассировка:** GAP-030/034; T-23; все UI.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [contracts/ui-acceptance-matrix.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/ui-acceptance-matrix.json)
- [contracts/bg-requirements-trace.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/bg-requirements-trace.json)
- [contracts/mvp-acceptance-v1.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/mvp-acceptance-v1.json)
- [contracts/roadmap-stage-acceptance.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/roadmap-stage-acceptance.json)

**Тесты/gates — существующие файлы:**

- [scripts/verify_readiness_documentation.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_readiness_documentation.rb)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/02_GAP_REGISTER.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/02_GAP_REGISTER.md)
- [docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md)
- [docs/remediation/2026-10-17/04_REMEDIATION_PLAN.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/04_REMEDIATION_PLAN.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/required-cases.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/task-inventory.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Создать case inventory по каждому GAP-001…036, T-01…027 и каждому текущему UI case. Сохранить исходные критерии, даже если узкая часть уже исправлена.
2. Связать requirement → actual source/caller → тест → сценарий → artifact/evidence. Разделить статический anchor, host runtime, OS instrumentation, target compile и physical evidence.
3. Исторический OPEN/PASS не переносить механически: заново проверить текущий путь и объём доказательства. Устаревшее описание пометить историческим, не стирать контрпример.

**Тесты, которые должны существовать или быть добавлены:**

- Намеренно удалить case/source/evidence из копии inventory: итоговый verifier обязан отказать.
- Проверить дубликаты ID, отсутствующие ссылки, обязательные skipped и неучтённые enabled capabilities.

**Приёмка:** Ни один исходный GAP, T-case, UI-case и добавленный в процессе дефект не потерян; coverage закрывается requirement-by-requirement.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P02 — Зафиксировать точные device profiles и применимые vendor sources

**Зависимости:** P00. **Трассировка:** GAP-009…014/029/032/033; T-05…08/24/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/provisioned_binding.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/provisioned_binding.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [BeeFiscalApp/src/deviceProfiles.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/deviceProfiles.ts)
- [IoT/protocol-abstraction/src/ProtocolFactory.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/ProtocolFactory.cpp)

**Тесты/gates — существующие файлы:**

- [IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md)
- [IoT/firmware/edge-agent-s3/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/README.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/profiles/registry.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/profiles/profile.schema.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Разделить Datecs DP-150 MX COM, Daisy Compact S 01 USB, BluePad-50 Plus BLE, BlueCash Android и конкретную плату S3. BluePay как торговое имя не считать автоматически BluePad: подтвердить модель/SDK/payment app.
2. Для каждого профиля сохранить model/hardware/firmware/region/currency/encoding/transport, UUID или VID/PID/interfaces, line settings, capabilities и unsupported commands.
3. Сопоставить каждый command/result/error/retry с разделом и страницей vendor PDF/SDK; сохранить URL, полученные bytes hash и дату. Datasheet BLE не использовать как TLV/refund specification.

**Тесты, которые должны существовать или быть добавлены:**

- Неприменимая firmware/region revision, неизвестная capability или отсутствие обязательного source hash должны запрещать включение профиля.
- Для полей без спецификации создать BLOCKED_SPEC с точным вопросом; не придумывать байты и не заимствовать команды другого производителя.

**Приёмка:** Есть проверяемая protocol-to-code таблица и фиксированные профили; неизвестный протокол не выдаётся за реализованную поддержку.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P03 — Закрыть canonical integration DB bootstrap, воспроизведённый HTTP400

**Зависимости:** P00,P01. **Трассировка:** T-16/18/19/20; integration run39557126955 failure.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [scripts/prepare-fiscal-pg-fixture.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/prepare-fiscal-pg-fixture.py)
- [scripts/e2e-server2server.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/e2e-server2server.sh)
- [.github/workflows/integration-contract.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/integration-contract.yml)
- [compose.fiscalisation.integration-ci.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/compose.fiscalisation.integration-ci.yaml)
- [fiscal-backend/internal/integration/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/service.go)
- [fiscal-backend/internal/integration/http.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/http.go)
- [fiscal-backend/internal/integration/rabbit.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/rabbit.go)
- [fiscal-backend/internal/integration/appauth.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/appauth.go)

**Тесты/gates — существующие файлы:**

- [scripts/test_prepare_fiscal_pg_fixture.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/test_prepare_fiscal_pg_fixture.py)
- [scripts/tests/test_integration_ci_topology.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/test_integration_ci_topology.py)
- [fiscal-backend/internal/integration/service_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/service_test.go)
- [fiscal-backend/internal/integration/appauth_postgres_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/appauth_postgres_test.go)

**Контракты/build/docs — читать и обновлять согласованно:**

- [database/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/README.md)
- [docs/remediation/2026-10-17/65_CI_INTEGRATION_FOLLOWUP.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/65_CI_INTEGRATION_FOLLOWUP.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/fixtures/fiscal-integration-canonical/provenance.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/fixtures/fiscal-integration-canonical/fixture.sql` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/prepare-integration-pg-fixture.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/canonical_fixture_postgres_test.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

**Внешние canonical inputs — только чтение/сверка provenance, не править в рамках этой карточки Fiscalisation:**

Trusted revision/closure определить по D04; наличие файлов само по себе не подтверждает пригодность версии. Если нужна product migration в BeeloyDB, оформить отдельную reviewed задачу, не исправлять её копированием упрощённой схемы в backend.

- [BeeloyDB/scripts/canonical-baseline-loader.js](/Users/freelancer/Documents/Beeloy/BeeloyDB/scripts/canonical-baseline-loader.js)
- [BeeloyDB/scripts/canonical-migration-loader.js](/Users/freelancer/Documents/Beeloy/BeeloyDB/scripts/canonical-migration-loader.js)
- [BeeloyDB/migrations/sqls/canonical-baseline-v1.json](/Users/freelancer/Documents/Beeloy/BeeloyDB/migrations/sqls/canonical-baseline-v1.json)
- [BeeloyDB/model/fiscal/tables/external_systems.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/external_systems.sql)
- [BeeloyDB/model/fiscal/tables/external_system_credentials.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/external_system_credentials.sql)
- [BeeloyDB/model/fiscal/tables/external_system_audit_log.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/external_system_audit_log.sql)
- [BeeloyDB/model/fiscal/tables/tenant_source_bindings.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/tenant_source_bindings.sql)
- [BeeloyDB/model/fiscal/tables/external_enrollment_challenges.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/external_enrollment_challenges.sql)
- [BeeloyDB/model/fiscal/tables/tenant_integration_credentials.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/tenant_integration_credentials.sql)
- [BeeloyDB/model/fiscal/tables/tenant_user_memberships.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/tenant_user_memberships.sql)
- [BeeloyDB/model/fiscal/tables/app_auth_challenges.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/app_auth_challenges.sql)
- [BeeloyDB/model/fiscal/tables/app_auth_sessions.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/app_auth_sessions.sql)
- [BeeloyDB/model/fiscal/tables/app_issued_tokens.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/app_issued_tokens.sql)
- [BeeloyDB/model/fiscal/tables/fiscal_email_outbox.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/fiscal_email_outbox.sql)
- [BeeloyDB/model/fiscal/tables/integration_commands.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_commands.sql)
- [BeeloyDB/model/fiscal/tables/integration_command_outbox.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_command_outbox.sql)
- [BeeloyDB/model/fiscal/tables/integration_resources.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_resources.sql)
- [BeeloyDB/model/fiscal/tables/integration_change_journal.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_change_journal.sql)
- [BeeloyDB/model/fiscal/tables/integration_idempotency_replays.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_idempotency_replays.sql)
- [BeeloyDB/model/fiscal/tables/integration_security_events.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_security_events.sql)
- [BeeloyDB/model/fiscal/tables/enrollment_conflict_decisions.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/enrollment_conflict_decisions.sql)
- [BeeloyDB/model/fiscal/tables/webhook_deliveries.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/webhook_deliveries.sql)
- [BeeloyDB/model/fiscal/tables/webhook_delivery_attempts.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/webhook_delivery_attempts.sql)
- [BeeloyDB/model/fiscal/tables/integration_retention_runs.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_retention_runs.sql)
- [BeeloyDB/model/fiscal/tables/integration_command_outbox_archive.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_command_outbox_archive.sql)
- [BeeloyDB/model/fiscal/tables/integration_commands_archive.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/integration_commands_archive.sql)
- [BeeloyDB/model/fiscal/tables/webhook_delivery_attempts_archive.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/webhook_delivery_attempts_archive.sql)
- [BeeloyDB/model/fiscal/tables/webhook_deliveries_archive.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/tables/webhook_deliveries_archive.sql)
- [BeeloyDB/model/fiscal/functions/integration_journal_immutable.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/functions/integration_journal_immutable.sql)
- [BeeloyDB/model/fiscal/functions/archive_integration_operational_rows.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/functions/archive_integration_operational_rows.sql)
- [BeeloyDB/model/fiscal/triggers/integration_change_journal__integration_change_journal_no_update.sql](/Users/freelancer/Documents/Beeloy/BeeloyDB/model/fiscal/triggers/integration_change_journal__integration_change_journal_no_update.sql)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Воспроизвести actual CreateSystem HTTP через настоящий Service/PG. Уже обнаружено отсутствие fiscal.external_systems, SQLSTATE42P01; не менять бизнес-валидацию ради HTTP201.
2. Найти и зафиксировать canonical BeeloyDB revision и полную DDL dependency closure: tables, PK/UNIQUE/CHECK/FK/indexes, functions, immutable trigger. Существующий isolated candidate ещё требует review; не считать /tmp файлы интегрированными.
3. Сделать воспроизводимый pinned CI fixture export с manifests/hash validation и явной границей external production migration dependency. Не создавать таблицы автоматически в Service.New и не ослаблять права. Canonical selected integration tables без RLS не объявлять tenant-RLS защищёнными.

**Тесты, которые должны существовать или быть добавлены:**

- Чистый PostgreSQL: old fixture RED42P01 → canonical fixture GREEN actual HTTP201.
- Реальные enrollment/OTP/tenant credential, Rabbit projection и обе session-concurrency проверки; FK/CHECK/UNIQUE и immutable journal negative tests.
- Foreign tenant operation недоступна; неверный credential/недостаточный scope отказ; non-owner DB role не получает доступа по умолчанию; missing/tampered DDL hash fail closed.
- Контейнер/сеть/volumes уникальны для run; cleanup удаляет только свои ресурсы, отсутствие контейнера проверяется.

**Приёмка:** Fresh Ubuntu integration workflow выполняет все E2E шаги, а не только health; production canonical dependency описана; сохраняются safe summaries.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P04 — Проверить реальные PostgreSQL схемы, миграции и persistence ownership

**Зависимости:** P03. **Трассировка:** GAP-016…020/024/028; T-10/11/13/15/16.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [fiscal-backend/internal/persistence/postgres.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/postgres.go)
- [fiscal-backend/internal/persistence/migrations.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/migrations.go)
- [minipos/beeminipos-backend/internal/persistence/postgres.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/postgres.go)
- [minipos/beeminipos-backend/internal/persistence/order_projection.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/order_projection.go)
- [database/fiscal/002_tenant_rls.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/002_tenant_rls.sql)
- [database/fiscal/022_runtime_typed_projection.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/022_runtime_typed_projection.sql)
- [database/minipos/002_tenant_rls.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/minipos/002_tenant_rls.sql)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/persistence/postgres_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/postgres_test.go)
- [fiscal-backend/internal/persistence/postgres_fixture_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/postgres_fixture_test.go)
- [fiscal-backend/internal/persistence/unp_allocator_integration_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/unp_allocator_integration_test.go)
- [minipos/beeminipos-backend/internal/persistence/postgres_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/postgres_test.go)
- [minipos/beeminipos-backend/internal/persistence/order_recovery_integration_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/order_recovery_integration_test.go)

**Контракты/build/docs — читать и обновлять согласованно:**

- [scripts/e2e-two-compose.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/e2e-two-compose.sh)
- [database/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/README.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Прогнать обе canonical typed схемы на чистой БД, отдельно legacy upgrade и повтор миграции. Проверить schemas/search_path, роли writer/runtime, grants и RLS там, где они заявлены.
2. Проверить source ownership коммерческого заказа, fiscal operation/receipt/payment projection, readiness/recovery fences, predecessor migration и реальные CAS/version conditions.
3. При uncertain commit не считать отсутствие локального ответа отсутствием записи; использовать confirmed snapshot/reconciliation. Не исправлять ошибки путём падения на публичные legacy таблицы.

**Тесты, которые должны существовать или быть добавлены:**

- Actual PG: cross-tenant read/write, role escalation, guessed UUID, wrong actor/register/session, concurrent update, duplicate idempotency key/payload conflict.
- Restart БД/backend, commit applied→connection lost, rollback, stale replica/read, deadlock/serialization failure и recovery fence.
- Точные денежные значения, VAT/discount/quantity, event chain и export dictionary сверяются с actual records, не только количеством rows.

**Приёмка:** Все включённые persistence paths проверены на настоящей PG с правильными ролями; доказана изоляция и восстановление, миграции имеют rollback/roll-forward drill.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P05 — Сквозной MQTT контракт, ACL и подтверждение исхода

**Зависимости:** P02,P04. **Трассировка:** GAP-001/002/024/025/035; T-01/12/13/26.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [fiscal-backend/internal/mqttclient/client.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/mqttclient/client.go)
- [fiscal-backend/internal/domain/admin.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/admin.go)
- [IoT/firmware/edge-agent-s3/idf/main/mqtt_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/mqtt_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashMqtt.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashMqtt.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWire.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWire.kt)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/mqttclient/client_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/mqttclient/client_test.go)
- [IoT/firmware/edge-agent-s3/idf/tests/run_native_mqtt.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_native_mqtt.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/run_sync_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_sync_host.sh)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWireTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWireTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [compose.fiscalisation.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/compose.fiscalisation.yaml)
- [compose.fiscalisation.dev.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/compose.fiscalisation.dev.yaml)

**Если меняется public API/DTO — source и generated consumers:**

- [docs/integration-kit/openapi.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/openapi.yaml)
- [docs/integration-kit/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/package.json)
- [docs/integration-kit/generate-go-validator.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/generate-go-validator.mjs)
- [docs/integration-kit/typescript/types.generated.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/typescript/types.generated.ts) — generated output; только штатная генерация.
- [fiscal-backend/internal/integration/resource_schemas_gen.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/resource_schemas_gen.go) — generated output; только штатная генерация.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Из signed binding определить adapter identity, fiscal endpoint и точные command/result/sync/ACK topics; обновлять publisher/subscriber/ACL/schema согласованно.
2. Передавать одни и те же operation UUID/digest/tenant/device/binding generation; immutable association фиксировать до publish и проверять при outcome.
3. Business ACK принимать только после проверки root signature, exact batch/device/hash/sequence; PUBACK не изменяет состояние fiscal operation или sync cursor.

**Тесты, которые должны существовать или быть добавлены:**

- Actual broker + backend publisher → actual S3/BlueCash ingress: правильный topic доставляет одну команду; чужой tenant/device/topic suffix отбрасывается.
- QoS duplicate, retained/stale message, broker restart, reconnect, lost ACK, out-of-order result, wrong association, root/key rotation.
- False financial result с валидным JSON на чужом topic не materialize sale; expiry и replay checks работают до I/O.

**Приёмка:** Command/result/ACK контракт един во всех runtime и broker ACL; failure/replay не создают вторую оплату/чек.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P06 — Замкнуть factory identity и provisioning без ручных NVS правок

**Зависимости:** P02,P04. **Трассировка:** GAP-003/004/005/031; T-02/03/22.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/encrypted_provisioning.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/encrypted_provisioning.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/provisioned_binding.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/provisioned_binding.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/provisioning_crypto.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/provisioning_crypto.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/app_main.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/app_main.cpp)
- [BeeFiscalApp/src/provisioningProtocol.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningProtocol.ts)
- [BeeFiscalApp/src/provisioningBle.native.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningBle.native.ts)
- [BeeFiscalApp/src/provisioningBle.web.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningBle.web.ts)
- [BeeFiscalApp/src/smartDeviceBle.native.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/smartDeviceBle.native.ts)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_provisioning_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_provisioning_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/run_identity_entropy_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_identity_entropy_host.sh)
- [BeeFiscalApp/src/provisioningProtocol.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningProtocol.test.ts)
- [IoT/device-manufacturing/tests/test_bundle.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/tests/test_bundle.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [IoT/device-manufacturing/src/beefiscal_factory/registration_client.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/src/beefiscal_factory/registration_client.py)
- [docs/remediation/2026-10-17/15_PROVISIONING_AND_BLUECASH_INTEGRATION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/15_PROVISIONING_AND_BLUECASH_INTEGRATION.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Использовать реальный P256 DeviceIdentity в ESP32-S3 NVS; enrollment key брать из trusted manufacturing registry, а не из BLE advertisement.
2. Проверить signed/encrypted package subject, tenant/location/register/device/key/generation/setup nonce; atomic NVS blob→commit→readback→signed install proof.
3. Приложение передаёт opaque bytes и показывает installed только после проверенного proof/authoritative lookup. Lost ACK не инициирует повторную финансовую операцию. Секреты не хранить в client logs.

**Тесты, которые должны существовать или быть добавлены:**

- Factory-empty, existing key, corrupt scalar/blob, wrong enrollment key/KID/subject/root, rollback generation, replay setup nonce.
- Разрывы на каждом fragment, commit/readback failure, reboot до/после commit, lost install ACK, lookup после reconnect; actual Kotlin/TS/S3 DTO совместимы.
- Неверный неподписанный пакет не уничтожает действующую authority; возможная mutation до error вызывает нужный quarantine.

**Приёмка:** Полный software factory/provisioning/reboot/probe сценарий проходит без ручного редактирования NVS; физическая entropy/boot квалификация остаётся HIL.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P07 — Согласовать peripheral BLE framing, buffers и lifecycle

**Зависимости:** P06. **Трассировка:** GAP-005…008/030; T-03/04/23/25; docs56/59/61.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_frame_layout.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_frame_layout.h)
- [IoT/firmware/edge-agent-s3/idf/main/ble_result_delivery.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_result_delivery.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp)
- [minipos/BeeMiniPOS/src/webBle.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/webBle.ts)
- [minipos/BeeMiniPOS/src/browserGatt.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/browserGatt.ts)
- [minipos/BeeMiniPOS/src/nativeBleTransport.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeBleTransport.ts)
- [minipos/miniposweb/src/fiscal/webBleRoute.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/src/fiscal/webBleRoute.ts)
- [minipos/miniposweb/src/fiscal/browserGatt.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/src/fiscal/browserGatt.ts)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_native_ble.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_native_ble.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py)
- [minipos/BeeMiniPOS/src/nativeBleTransport.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeBleTransport.test.ts)
- [minipos/miniposweb/tests/webBleLifecycle.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/tests/webBleLifecycle.test.ts)

**Контракты/build/docs — читать и обновлять согласованно:**

- [IoT/firmware/edge-agent-s3/idf/main/CMakeLists.txt](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/CMakeLists.txt)
- [docs/remediation/2026-10-17/56_BLE_CALLBACK_LIFECYCLE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/56_BLE_CALLBACK_LIFECYCLE.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Прогнать реальные callers: advertising budget, negotiated MTU, BFF1 header, fragment sizes, total length/hash, CCCD completion, assembly reset, deadlines.
2. Сохранять captured connection handle+generation через ingress→owned queue→processor→result sink→publisher. Старый callback не получает новое owner значение.
3. Отделить provisioning, trusted-time bootstrap, operational TLS и BluePad central state; не смешивать buffers/handles/UUID. Реализовать SDK reset/callback cleanup с фактическими NimBLE API.

**Тесты, которые должны существовать или быть добавлены:**

- MTU ниже минимального, partial/duplicate/out-of-order frames, max/oversize payload, wrong hash/UUID, unexpected CCCD state.
- Disconnect/reconnect handle reuse, notification after abort, completion from old write, device wrapper ABA, concurrent two devices, reset during queued execution.
- После disconnect выполнение, которое могло произойти, не повторяется; late notify может быть отброшен, durable result сохраняется.

**Приёмка:** Нет cross-session delivery, UAF или unbounded assembly; exact SDK callbacks проверены host boundary и target build, radio ещё HIL_PENDING.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P08 — Исправить настоящие deadlines и ownership CommandQueue

**Зависимости:** P07. **Трассировка:** GAP-007/008/017/025; T-03/11/14/20.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/command_queue.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/command_queue.h)
- [IoT/firmware/edge-agent-s3/idf/main/local_http_server.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/local_http_server.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/38_HTTP_TOTAL_DEADLINES.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/38_HTTP_TOTAL_DEADLINES.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/command_queue_host/queue_deadline_native.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/command_queue_host/run.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. В enqueue_and_wait найден fallback portMAX_DELAY после timeout для защиты stack result pointer. Спроектировать owned/refcount completion object; worker не пишет в stack завершившегося caller.
2. Разделить cancel-before-accept, accepted/possible-send и late completion. Timeout освобождает caller за общий deadline, но не отменяет/повторяет финансовое действие, которое могло уйти.
3. Проверить start failure cleanup queue/task/semaphore, full queue, shutdown ownership, освобождение payload exactly once. Не лечить зависание удалением UAF-защиты.

**Тесты, которые должны существовать или быть добавлены:**

- Worker никогда не отвечает; caller выходит по deadline без UAF, а pending operation остаётся reconcile.
- Timeout одновременно с finish; caller destruction; queue-full; allocation/task-create/semaphore failure; late result; concurrent enqueues; ASan/UBSan и leak counters.
- Одна принятая команда вызывает не больше одного исполнения; late completion обновляет durable result/outbox один раз.

**Приёмка:** Все public ingress calls имеют bounded total deadline, корректное владение памятью и различают NOT_SENT/UNKNOWN; infinite wait в caller path устранён.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P09 — Завершить production trusted-time vertical

**Зависимости:** P04,P06,P07. **Трассировка:** GAP-004/008/025; T-02/10/18/20; docs57/60/66/68.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/device_time_anchor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_time_anchor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/device_time_anchor.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_time_anchor.h)
- [IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.h)
- [IoT/firmware/edge-agent-s3/idf/main/installed_authority_lifecycle.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/installed_authority_lifecycle.h)
- [IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/startup_entropy.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/startup_entropy.h)
- [IoT/firmware/edge-agent-s3/idf/main/app_main.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/app_main.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_certificate_proof.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_certificate_proof.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp)
- [fiscal-backend/internal/domain/ble_server_challenge.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_challenge.go)
- [fiscal-backend/internal/domain/ble_server_ca.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_ca.go)
- [BeeFiscalApp/src/provisioningBle.native.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningBle.native.ts)
- [BeeFiscalApp/src/provisioningProtocol.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/provisioningProtocol.ts)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/time_anchor_crypto_host/crypto_native.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/time_anchor_crypto_host/crypto_native.cpp)
- [fiscal-backend/internal/domain/time_anchor_crypto_interop_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/time_anchor_crypto_interop_test.go)
- [scripts/verify-device-time-anchor-crypto-host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify-device-time-anchor-crypto-host.sh)
- [scripts/run-ble-certificate-crypto.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/run-ble-certificate-crypto.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/60_DEVICE_TIME_PROOF_VERIFIER_CONTRACT.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/60_DEVICE_TIME_PROOF_VERIFIER_CONTRACT.md)
- [docs/remediation/2026-10-17/66_TRUSTED_TIME_PRODUCTION_VERTICAL.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/66_TRUSTED_TIME_PRODUCTION_VERTICAL.md)
- [IoT/firmware/edge-agent-s3/idf/main/CMakeLists.txt](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/CMakeLists.txt)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_time_runtime.h` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_time_runtime.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/time_nonce_source.h` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/time_nonce_source.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/authorityclock/clock.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/authorityclock/linux_monitor.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/authorityclock/policy.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/device_time_anchor.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/api/device_time_anchor.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Не переписывать уже проверенные crypto/core/lifecycle части. Подключить production boot owner, independent nonce DRBG под startup entropy, monotonic adapter и interval provider; никакого NVS восстановления авторитетного UTC.
2. Реальный серверный AuthorityClock должен иметь проверенный authenticated source/health/error policy, immutable health epoch и атомарную выдачу при current binding/actor/source. time.Now, успешный SNTP и OS synchronized не являются таким доказательством.
3. Реализовать purpose-specific issuer exact17/22 canonical fields/private permit и bounded opaque BLE bootstrap/phone relay с captured owner; не пропускать oversized envelope через старый1024-byte read. Миграцию execution/local-token clocks выполнить явно, не включать fake healthy default.

**Тесты, которые должны существовать или быть добавлены:**

- Actual monitor protocol: stale/replayed metrics, source mismatch, clock step/leap/suspend, daemon restart, uncertainty overflow/health-loss epoch.
- Signed hostile fields, nonce/request/boot/root/actor/binding mismatch, replay/delay, max RTT, holdover/drift boundaries, expiry during sign/publication.
- Actual installer/key/coordinator ABA, failed NVS write/commit/readback, controlled restart; old response/cancel не меняет нового owner; fresh crypto markers18 обязательны.
- Full bootstrap→actual root signature→S3 accept→BLE certificate proof; cold boot, lost refresh и unsigned reply fail closed.

**Приёмка:** Полный vertical работает с реально configured/qualified clock source; отсутствие любого dependency даёт UNAVAILABLE. Target build/host tests не заменяют physical oscillator/entropy tests.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P10 — Включить стандартный аутентифицированный BLE operational channel

**Зависимости:** P07,P09. **Трассировка:** GAP-004/008/027; UI-MP-026/UI-FISC-012; T-04/18/23.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_certificate_proof.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_certificate_proof.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/device_identity.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/app_main.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/app_main.cpp)
- [IoT/common-modules/ble-stream/stream.c](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/stream.c)
- [IoT/common-modules/ble-stream/driver.c](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/driver.c)
- [IoT/common-modules/ble-stream/boundary.c](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/boundary.c)
- [IoT/common-modules/ble-stream/stream.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/stream.ts)
- [fiscal-backend/internal/domain/ble_server_ca.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_ca.go)
- [fiscal-backend/internal/domain/ble_server_challenge.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_challenge.go)
- [fiscal-backend/internal/domain/ble_server_repository.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_repository.go)
- [minipos/BeeMiniPOS/src/nativeBleTransport.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeBleTransport.ts)
- [minipos/BeeMiniPOS/src/webBle.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/webBle.ts)
- [minipos/miniposweb/src/fiscal/webBleRoute.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/src/fiscal/webBleRoute.ts)

**Тесты/gates — существующие файлы:**

- [IoT/common-modules/ble-stream/reproduce.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/reproduce.py)
- [IoT/common-modules/ble-stream/counterfactuals.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/ble-stream/counterfactuals.py)
- [fiscal-backend/internal/domain/ble_server_tls_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_tls_test.go)
- [fiscal-backend/internal/domain/ble_server_ca_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/ble_server_ca_test.go)
- [scripts/verify-ble-certificate-proof-host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify-ble-certificate-proof-host.sh)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/46_BLE_STANDARD_CHANNEL_OPTIONS.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/46_BLE_STANDARD_CHANNEL_OPTIONS.md)
- [docs/remediation/2026-10-17/52_RELIABLE_GATT_STREAM.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/52_RELIABLE_GATT_STREAM.md)
- [docs/remediation/2026-10-17/53_BLE_SERVER_CERTIFICATE_ISSUANCE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/53_BLE_SERVER_CERTIFICATE_ISSUANCE.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_tls_channel.h` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_tls_channel.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/bleTlsChannel.ts` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Подключить reliable GATT stream и стандартный TLS1.3 implementation в реальный S3/runtime и actual Android/iOS/browser adapters; standalone feasibility не считать интеграцией.
2. Реализовать CA/pinning/name/chain validation, same-NVS-key PoP, certificate install/renewal/expiry/revocation, session binding и client authority. Оба направления обязаны быть защищены.
3. Убрать OPEN_MVP/random32 как production readiness evidence. Нельзя отключать сертификатную проверку ради bootstrap; bootstrap использует отдельный purpose proof.

**Тесты, которые должны существовать или быть добавлены:**

- Unknown CA/name, wrong SPKI/device, expired/not-yet-valid/revoked cert, signature corruption, downgrade/plaintext/replay, TLS record fragmentation.
- Reconnect/resume с отозванной authority; binding/key/session change в середине handshake/response; certificate cached path и time-health loss.
- Реальный native/WASM implementation ↔ S3 actual TLS engine, не SHA-only/transport/auth stub; buffers/backpressure/deadlines/memory bounded.

**Приёмка:** Команды и результаты криптографически привязаны к устройству/session; active runtime вызывает проверенные modules. Подписанная команда сама по себе не закрывает аутентификацию результата.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P11 — Привести все client auth/callback ownership к одной модели

**Зависимости:** P04,P07. **Трассировка:** GAP-008/026/027; UI-MP-014/016/017/020/028; T-04/14/18.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [BeeFiscalApp/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/App.tsx)
- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)
- [BeeFiscalApp/src/requestAuthFence.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/requestAuthFence.ts)
- [BeeFiscalApp/src/emailOtpAuth.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/emailOtpAuth.ts)
- [BeeFiscalApp/src/http.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/http.ts)
- [BeeFiscalApp/src/smartDeviceBle.native.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/smartDeviceBle.native.ts)
- [minipos/BeeMiniPOS/src/requestAuthFence.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/requestAuthFence.ts)
- [minipos/BeeMiniPOS/src/useManagementAccess.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/useManagementAccess.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionManager.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionManager.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts)
- [minipos/BeeMiniPOS/src/http.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/http.ts)
- [minipos/BeeMiniPOS/src/emailAuth.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/emailAuth.ts)

**Тесты/gates — существующие файлы:**

- [BeeFiscalApp/src/requestAuthFence.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/requestAuthFence.test.ts)
- [minipos/BeeMiniPOS/src/requestAuthFence.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/requestAuthFence.test.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionManager.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionManager.test.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionAuthority.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionAuthority.test.ts)
- [minipos/BeeMiniPOS/__tests__/management-access-current.test.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/__tests__/management-access-current.test.tsx)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/47_HTTP_SESSION_OWNERSHIP.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/47_HTTP_SESSION_OWNERSHIP.md)
- [docs/remediation/2026-10-17/55_UI_COMMIT_OWNERSHIP_AND_TEST_DISCOVERY.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/55_UI_COMMIT_OWNERSHIP_AND_TEST_DISCOVERY.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Каждый async request/callback захватывает exact bearer/session/config/device/register owner до await. После await проверяет тот же owner; stale cleanup не отменяет новую сессию.
2. Проверить committed render authority: interrupted React render не выдаёт административные права. Logout/config change отменяет только свои listeners/tasks и стирает transient secrets.
3. Сохранить существующие requestAuthFence/management fixes; устранить оставшиеся unbounded discovery/connect/body/native bridge waits и error-path leaks.

**Тесты, которые должны существовать или быть добавлены:**

- Old401 после нового login, token ABA, Header override, current401, auth loss во время OTP/network/native write; финансового retry нет.
- Unmount/background/process death, reconnect к тому же физическому device с другим JS wrapper, cancel old after replacement, denied permission.
- Actual App tests + real native adapter boundary, а не только detached helper; required cases не skipped.

**Приёмка:** Нет публикации/действий по stale authority и неограниченного ожидания; UI/backend permissions согласованы.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P12 — Закрыть денежную валидацию и immutable execution authority

**Зависимости:** P04,P05,P10. **Трассировка:** GAP-015/016/025; T-10/13/18.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/money_plan.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/money_plan.h)
- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [fiscal-backend/internal/domain/execution_authorization.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/execution_authorization.go)
- [fiscal-backend/internal/domain/composite_execution.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/composite_execution.go)
- [minipos/beeminipos-backend/internal/domain/execution_prepare.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/execution_prepare.go)
- [minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionAuthority.ts)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_money_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_money_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/execution_auth_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/execution_auth_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/authority_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/authority_host/run.py)
- [fiscal-backend/internal/domain/authorization_actor_contract_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/authorization_actor_contract_test.go)
- [minipos/BeeMiniPOS/src/nativeExecutionAuthority.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionAuthority.test.ts)

**Если меняется public API/DTO — source и generated consumers:**

- [docs/integration-kit/openapi.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/openapi.yaml)
- [docs/integration-kit/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/package.json)
- [docs/integration-kit/generate-go-validator.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/generate-go-validator.mjs)
- [docs/integration-kit/typescript/types.generated.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/typescript/types.generated.ts) — generated output; только штатная генерация.
- [fiscal-backend/internal/integration/resource_schemas_gen.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/resource_schemas_gen.go) — generated output; только штатная генерация.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. До reservation/payment/fiscal I/O проверить schema, action, exact binding generation, tenant/register/device/actor, expiry, root/client signatures и capability.
2. Использовать checked integer money/quantity/tax/discount arithmetic. Typed int64 wire не означает разрешение передать unsafe cJSON бизнес-числа или заведомо недопустимую сумму.
3. Сравнить backend, S3, BlueCash и UI одинаковыми vectors. Один intent хранит UUID/digest/UNP/authority неизменными при MQTT/HTTP/BLE переключении.

**Тесты, которые должны существовать или быть добавлены:**

- Negative/zero/overflow/boundary totals, rounding/tax/quantity/discount mismatch, currency/profile mismatch, duplicate item/tender, oversized text.
- Wrong actor/register/device/key/action, stale binding, replay grant, wrong PoP, expired ticket before/after admission; до отказа device action counter0.
- Tampered signature/domain/hash, duplicate JSON keys/invalid UTF8/NUL, CBOR nonminimal/extra/trailing/type confusion; valid equivalent payload проходит.

**Приёмка:** Любой invalid intent отвергается до денег/фискализации; валидные equivalent paths дают одинаковый canonical commitment.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P13 — Согласовать readiness и UNP до регулируемого ввода продажи

**Зависимости:** P02,P04,P12,P37. **Трассировка:** GAP-016; UI-MP-005; T-10/17; docs43.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)
- [minipos/BeeMiniPOS/src/sourceOrders.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/sourceOrders.ts)
- [minipos/beeminipos-backend/internal/domain/native_order_lines.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/native_order_lines.go)
- [minipos/beeminipos-backend/internal/domain/execution_prepare.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/execution_prepare.go)
- [fiscal-backend/internal/domain/unp_allocator.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/unp_allocator.go)
- [fiscal-backend/internal/domain/delegated_sale_prepare.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/delegated_sale_prepare.go)
- [fiscal-backend/internal/domain/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/service.go)
- [IoT/firmware/edge-agent-s3/idf/main/authority_unp.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/authority_unp.h)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/domain/open_sale_supto_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/open_sale_supto_test.go)
- [fiscal-backend/internal/domain/unp_allocator_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/unp_allocator_test.go)
- [fiscal-backend/internal/persistence/unp_allocator_integration_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/unp_allocator_integration_test.go)
- [minipos/beeminipos-backend/internal/domain/native_order_lines_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/native_order_lines_test.go)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/decisions/ADR-SUPTO-001-thin-pos.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/decisions/ADR-SUPTO-001-thin-pos.md)
- [docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md)
- [database/fiscal/013_supto_identifier_foundation.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/013_supto_identifier_foundation.sql)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. По актуальным применимым требованиям определить точный sale-entry момент. Source-first ownership сохранить; не подменять решение возвратом direct UI→root API без ADR.
2. Спроектировать выдачу fiscal identity/UNP и readiness до этого момента: backend/S3 владеет allocation, привязкой device/register/operator/shift и durable continuity.
3. Запретить произвольную генерацию UNP в UI и выдачу неподтверждённых offline reservations. Не переносить момент на оплату без нормативного основания.

**Тесты, которые должны существовать или быть добавлены:**

- Первый товар/скан/изменение количества при ready/not-ready/unknown device; source HTTP down, S3 unreachable, shift closed.
- Concurrent tabs/phones, allocation lost response, crash после allocation до первой строки, reopen после restart, rollback/conflict; UNP не дублируется и не меняется.
- Связать UI-MP-005 с actual calls/DB/NVS state и нормативным point, а не только новым source anchor.

**Приёмка:** Утверждённый timing/readiness contract реализован и проверен в обоих online/offline путях; нормативный вопрос не скрыт сменой теста.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P14 — Реализовать новый durable offline cart и последующую синхронизацию

**Зависимости:** P10,P12,P13. **Трассировка:** GAP-016/023/027; UI-MP-013/027; T-14/17.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)
- [minipos/BeeMiniPOS/src/sourceOrders.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/sourceOrders.ts)
- [minipos/BeeMiniPOS/src/checkoutJournal.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutJournal.ts)
- [minipos/BeeMiniPOS/src/checkoutRecovery.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutRecovery.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionManager.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionManager.ts)
- [minipos/beeminipos-backend/internal/domain/native_order_lines.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/native_order_lines.go)
- [minipos/beeminipos-backend/internal/domain/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/service.go)
- [minipos/beeminipos-backend/internal/persistence/order_projection.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/order_projection.go)

**Тесты/gates — существующие файлы:**

- [minipos/BeeMiniPOS/src/checkoutJournal.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutJournal.test.ts)
- [minipos/BeeMiniPOS/src/checkoutRecovery.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutRecovery.test.ts)
- [minipos/BeeMiniPOS/src/nativeExecutionManager.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/nativeExecutionManager.test.ts)
- [minipos/beeminipos-backend/internal/domain/native_order_lines_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/native_order_lines_test.go)
- [minipos/miniposweb/tests/outbox-recovery.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/tests/outbox-recovery.test.ts)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/offlineCommercialDraft.ts` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/offlineCommercialDraft.test.ts` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/offline_order_import.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/offline_order_import_test.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Отдельно от checkout существующего заказа реализовать создание нового local commercial draft при полном отсутствии source HTTP, с принятой S3-owned fiscal identity/readiness.
2. Draft/lines/discount/tenders/authority snapshot/outcome сохранять атомарно и восстанавливать после reload/kill. Не очищать единственный positive evidence до authoritative import confirmation.
3. Синхронизировать exact immutable identity/amount/line/payment commitment идемпотентно в source/Fiscal. Conflict или отозванная authority ведёт в quarantine/reconciliation, не в новую продажу.

**Тесты, которые должны существовать или быть добавлены:**

- Cold offline launch с valid cache; отсутствующий/expired/revoked cache; каталог/налоги stale; смена actor/register/shift.
- Создать новый cart offline→cash/card where supported→kill→restart→online import→duplicate import; device action count1, source order1, fiscal receipt1.
- Lost local result, lost import ACK, partial outbox, order conflict, interrupted multi-tender update; ни потери approved tender, ни второго debit.

**Приёмка:** Fresh offline cycles доказаны separately from existing-order cached checkout; сохранены UNP/result до подтверждённого sync.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P15 — Проверить journal, transactions и power-loss windows

**Зависимости:** P04,P12. **Трассировка:** GAP-017/018/019/023; T-09/11/12/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.h)
- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt)
- [fiscal-backend/internal/persistence/postgres.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/postgres.go)
- [minipos/beeminipos-backend/internal/persistence/postgres.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/postgres.go)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_storage_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_storage_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/run_saga_storage_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_saga_storage_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/run_tender_recovery_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_tender_recovery_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/storage_host/runtime_storage_test.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/storage_host/runtime_storage_test.cpp)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Перед каждым irreversible I/O должен быть durable checkpoint с exact intent/receipt/payment/route/authority snapshot. Terminal projection + outbox выполняются атомарно.
2. Проверить результаты всех open/begin/prepare/bind/step/commit/fsync/readback; storage error не маскируется ESP_OK и не разрешает следующий monetary action.
3. Для каждого crash window описать authoritative recovery source. Рестарт host process и reopen SQLite должны использовать сохранённое stateful device state.

**Тесты, которые должны существовать или быть добавлены:**

- Inject allocation/SQL errors/disk-full/readonly/corruption/busy/commit uncertain/SD detach в каждую boundary до send, после send, после device result, до outbox.
- Crash/restart на каждой boundary: не более1 physical action; approved/payment references не теряются; receipt/outbox согласованы.
- Race duplicate UUID same/different payload; durable conflict не перезаписывается; writes завершаются bounded.

**Приёмка:** Таблица всех checkpoints покрыта исполняемыми failpoint cases; нет side effect после неподтверждённой mandatory durability.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P16 — Закрыть recovery readiness, ownership и uncertain storage

**Зависимости:** P15. **Трассировка:** GAP-016/019/020/021; T-09/11/17; docs51/54.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/payment_reconciliation.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/payment_reconciliation.cpp)
- [fiscal-backend/internal/domain/repository_recovery.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/repository_recovery.go)
- [fiscal-backend/internal/domain/payment_reconciliation.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/payment_reconciliation.go)
- [minipos/beeminipos-backend/internal/domain/recovery_readiness.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/recovery_readiness.go)
- [minipos/beeminipos-backend/internal/domain/recovery_snapshot.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/recovery_snapshot.go)
- [minipos/beeminipos-backend/internal/domain/checkout_lookup.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/checkout_lookup.go)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/terminal_guard_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/terminal_guard_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/reconciliation_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/reconciliation_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py)
- [fiscal-backend/internal/domain/repository_recovery_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/repository_recovery_test.go)
- [minipos/beeminipos-backend/internal/domain/recovery_snapshot_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/recovery_snapshot_test.go)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/RECOVERY_SNAPSHOT_SHAPE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/RECOVERY_SNAPSHOT_SHAPE.md)
- [docs/remediation/2026-10-17/54_SOURCE_RECOVERY_AUTHORITY.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/54_SOURCE_RECOVERY_AUTHORITY.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Recovery запускается после readiness/installed identity/device probe; сопоставляет operation/receipt session/UNP/device/each tender и не отменяет чужой открытый чек.
2. Использовать документированные read/status queries. NOT_FOUND/timeout/foreign route не доказывают NOT_SENT. Неполный legacy snapshot quarantined.
3. Проверить applied-but-unacknowledged payment/fiscal commit, replacement actor/config/device и подтверждённый source snapshot после failover.

**Тесты, которые должны существовать или быть добавлены:**

- Own open receipt vs foreign open receipt, closed receipt, shifted session, unknown serial, stale binding, absent legacy fields.
- Repeated restart/recovery, source DB readiness epoch lost/recovered, delayed old callback/error, recovery→UNKNOWN→COMMITTED atomic projection.
- Реальный SQLite и real executor против stateful emulator, а не recover() returning canned success.

**Приёмка:** Recovery не создаёт второй чек/списание и не трогает чужую session; unresolved состояние остаётся доступным для authorized reconcile.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P17 — Доказать BluePad/BluePay vendor flow и correlation

**Зависимости:** P02,P07,P12,P15. **Трассировка:** GAP-013/014/019; T-08/09/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.h)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/common-modules/datecspay/DatecsPay.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecspay/DatecsPay.cpp)
- [IoT/common-modules/datecspay/DatecsPay.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecspay/DatecsPay.h)
- [IoT/common-modules/datecspay/DatecsPayBleProfile.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecspay/DatecsPayBleProfile.h)
- [IoT/common-modules/datecspay/DatecsPayBleTransport.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecspay/DatecsPayBleTransport.h)
- [IoT/protocol-abstraction/src/DatecsPayTerminalAdapter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/DatecsPayTerminalAdapter.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsPinpadProtocol.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsPinpadProtocol.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsAndroidPorts.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsAndroidPorts.kt)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/bluepad_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/bluepad_host/run.py)
- [IoT/common-modules/datecspay/tests/DatecsPayTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecspay/tests/DatecsPayTest.cpp)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsPinpadProtocolTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsPinpadProtocolTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/BLUEPAD_CENTRAL_HANDOVER.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/BLUEPAD_CENTRAL_HANDOVER.md)
- [docs/remediation/2026-10-17/BLUEPAD_ADMIN_RECONCILIATION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/BLUEPAD_ADMIN_RECONCILIATION.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. По applicable SDK подтвердить discover→connect→service/CCCD→POWER/WAKE→write completion→notifications и реальные chunk limits. Не применять generic19byte limit без profile source.
2. Проверять typed STAN/RRN/auth/terminal reference, amount/currency/command/session correlation; vendor identifiers не приводить к unsafe JSON number.
3. Определить реально доступные purchase/status/void/refund/cancel capabilities и acquirer restrictions. Отсутствующий lookup/reversal не имитировать успехом.

**Тесты, которые должны существовать или быть добавлены:**

- Approve/decline/user cancel/unknown, wrong amount/currency/STAN, malformed TLV/checksum, duplicate/late/out-of-order response.
- Wake/CCCD/partial write/disconnect during send, reconnect, lost approved response, concurrent fiscal BLE peripheral traffic.
- Положительный outcome только при точном vendor evidence; неверная correlation не прикрепляется к другому tender.

**Приёмка:** Actual vendor driver/wake/correlation path покрыт независимыми bytes и stateful BLE model; acquirer/device payment evidence остаётся physical gate.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P18 — Проверить split tenders и компенсацию

**Зависимости:** P15,P16,P17. **Трассировка:** GAP-019/020/024; T-09/11/13/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/payment_reconciliation.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/payment_reconciliation.cpp)
- [fiscal-backend/internal/domain/receipt_saga.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/receipt_saga.go)
- [fiscal-backend/internal/domain/payment_reconciliation.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/payment_reconciliation.go)
- [minipos/beeminipos-backend/internal/fiscalintegration/payment_reconciliation.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/fiscalintegration/payment_reconciliation.go)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/run_tender_recovery_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_tender_recovery_host.sh)
- [fiscal-backend/internal/domain/payment_reconciliation_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/payment_reconciliation_test.go)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/GAP024_REVERSAL_HANDOVER.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/GAP024_REVERSAL_HANDOVER.md)
- [docs/remediation/2026-10-17/BLUEPAD_DURABLE_QUARANTINE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/BLUEPAD_DURABLE_QUARANTINE.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Каждый tender хранит собственные amount/state/reference/STAN/RRN/auth и compensation state; общий receipt reference не заменяет per-tender evidence.
2. При approved card + fiscal failure выбрать документированный recovery/compensation путь; повторная compensation не повторяет возврат.
3. Если vendor не поддерживает query/void/refund, сохранить unresolved/quarantine с operator/acquirer evidence; не выдумывать reversal.

**Тесты, которые должны существовать или быть добавлены:**

- CASH+CARD, CARD+CARD, second-card decline/unknown, partial approved, compensation response lost, repeated restart.
- Fault before/after each payment/compensation durable checkpoint, mismatched original reference/amount, stale ADMIN grant.
- Sum of tenders/refunds точно соответствует outcome; receipt result не подтверждает все платежи по одному selected tender.

**Приёмка:** Ни один approved tender не потерян/не смешан; сумма и количество irreversible actions проверены independent emulator counters.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P19 — Замкнуть canonical sync, frozen batches и business ACK

**Зависимости:** P05,P15. **Трассировка:** GAP-002/022/023/025; T-12/13/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/go_json.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/go_json.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/go_wire.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/go_wire.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)
- [fiscal-backend/internal/domain/edge_sync.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/edge_sync.go)
- [fiscal-backend/internal/domain/edge_sync_v2.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/edge_sync_v2.go)
- [fiscal-backend/internal/domain/edge_sync_repository.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/edge_sync_repository.go)
- [fiscal-backend/internal/domain/edge_sync_server_authority.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/edge_sync_server_authority.go)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWire.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWire.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/CanonicalJson.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/CanonicalJson.kt)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/go_json_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/go_json_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/go_wire_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/go_wire_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/run_sync_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_sync_host.sh)
- [fiscal-backend/internal/domain/edge_sync_v2_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/edge_sync_v2_test.go)
- [fiscal-backend/internal/mqttclient/sync_int64_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/mqttclient/sync_int64_test.go)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWireTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashSyncWireTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/GAP022_CANONICAL_SYNC_HANDOVER.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/GAP022_CANONICAL_SYNC_HANDOVER.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сравнить точный canonical hash preimage между Go/C++/Kotlin: key order, Unicode, escaping, integer semantics, arrays/null/missing.
2. Frozen in-flight batch сохраняется до подтверждённого ACK. Retry после ambiguous response отправляет exact same bytes/seq/hash, не пересобранный журнал.
3. Проверять подписанный ACK actor/device/batch/range/hash и durable cursor advance; partial acceptance/replay не пропускают journal rows.

**Тесты, которые должны существовать или быть добавлены:**

- Independent Go oracle, boundary integers, invalid Unicode/NUL/duplicate keys, byte-level golden hashes.
- Power loss после send/ACK до cursor commit; broker offline, duplicate/foreign/stale/out-of-order ACK; restart/retry exact bytes.
- Backpressure/bounded batches/retention при длительном offline, disk full без silent drop.

**Приёмка:** Цепь journal→batch→ACK→cursor непрерывна, повторное наблюдение не приводит к duplicate materialization.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P20 — Проверить result encoder, references и authoritative final outcome

**Зависимости:** P10,P12,P15. **Трассировка:** GAP-021/024/036; UI-MP-013; T-13/14/17/27.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/canonical_cbor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/canonical_cbor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_result_delivery.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_result_delivery.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [minipos/BeeMiniPOS/src/cbor.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/cbor.ts)
- [minipos/BeeMiniPOS/src/bleResult.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/bleResult.ts)
- [minipos/BeeMiniPOS/src/cloudCheckoutConfirmation.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/cloudCheckoutConfirmation.ts)
- [minipos/BeeMiniPOS/src/checkoutJournal.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutJournal.ts)
- [minipos/miniposweb/src/fiscal/webBleRoute.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/src/fiscal/webBleRoute.ts)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashCanonicalCbor.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashCanonicalCbor.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/cbor_encoder_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/cbor_encoder_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py)
- [minipos/BeeMiniPOS/src/cborDelivery.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/cborDelivery.test.ts)
- [minipos/BeeMiniPOS/src/cloudCheckoutConfirmation.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/cloudCheckoutConfirmation.test.ts)
- [minipos/miniposweb/tests/webBleLifecycle.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/tests/webBleLifecycle.test.ts)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashCanonicalCborTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashCanonicalCborTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/64_TYPED_CBOR_RESULT_ENCODER.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/64_TYPED_CBOR_RESULT_ENCODER.md)
- [tests/ble-transport/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/ble-transport/run.py)

**Если меняется public API/DTO — source и generated consumers:**

- [docs/integration-kit/openapi.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/openapi.yaml)
- [docs/integration-kit/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/package.json)
- [docs/integration-kit/generate-go-validator.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/generate-go-validator.mjs)
- [docs/integration-kit/typescript/types.generated.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/typescript/types.generated.ts) — generated output; только штатная генерация.
- [fiscal-backend/internal/integration/resource_schemas_gen.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/resource_schemas_gen.go) — generated output; только штатная генерация.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сохранить уже исправленные typed int64/strict JSON/UUID-owner paths; прогнать полные actual producer→wire→client projected result.
2. Fiscal success требует реальных device serial/receipt/UNP/time/evidence по profile. Synthetic EDGE-reference, PUBACK/HTTP202 не являются fiscal success.
3. FAILED/DECLINED/REJECTED не превращать автоматически в UNKNOWN. BLE authenticated outcome должен попасть в authoritative source projection; journal cleanup только после нужного durable confirmation.

**Тесты, которые должны существовать или быть добавлены:**

- Maximum text/reference length, quotes/backslashes/Cyrillic/invalidUTF8/NUL, truncation, malformed/overflow/fraction/exponent/-0 JSON.
- 12 full-width int64 transport vectors, exact status mapping, old-session delivery, unknown UUID, foreign operation/reference.
- Kill после полученного success до source sync, duplicate result, late rejection after known commit; positive evidence не стирается.

**Приёмка:** Включённые ingress/client paths одинаково интерпретируют результат; нет ложного COMMITTED и потери единственного evidence.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P21 — Проверить source/Fiscal order/payment/reversal материализацию

**Зависимости:** P04,P05,P18,P19,P20. **Трассировка:** GAP-024/026/028/035; T-13/15/16/19/26.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [minipos/beeminipos-backend/internal/domain/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/service.go)
- [minipos/beeminipos-backend/internal/domain/execution_prepare.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/execution_prepare.go)
- [minipos/beeminipos-backend/internal/domain/checkout_lookup.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/checkout_lookup.go)
- [minipos/beeminipos-backend/internal/domain/recovery_snapshot.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/recovery_snapshot.go)
- [minipos/beeminipos-backend/internal/api/order_operation.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/api/order_operation.go)
- [minipos/beeminipos-backend/internal/api/order_recovery.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/api/order_recovery.go)
- [minipos/beeminipos-backend/internal/persistence/order_projection.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/order_projection.go)
- [fiscal-backend/internal/domain/delegated_execution.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/delegated_execution.go)
- [fiscal-backend/internal/domain/composite_execution.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/composite_execution.go)
- [fiscal-backend/internal/integration/operations.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/operations.go)
- [fiscal-backend/internal/integration/rabbit.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/rabbit.go)

**Тесты/gates — существующие файлы:**

- [minipos/beeminipos-backend/internal/api/order_operation_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/api/order_operation_test.go)
- [minipos/beeminipos-backend/internal/api/order_recovery_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/api/order_recovery_test.go)
- [minipos/beeminipos-backend/internal/persistence/order_recovery_integration_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/persistence/order_recovery_integration_test.go)
- [fiscal-backend/internal/domain/delegated_execution_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/delegated_execution_test.go)
- [scripts/e2e-two-compose.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/e2e-two-compose.sh)

**Если меняется public API/DTO — source и generated consumers:**

- [docs/integration-kit/openapi.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/openapi.yaml)
- [docs/integration-kit/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/package.json)
- [docs/integration-kit/generate-go-validator.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/generate-go-validator.mjs)
- [docs/integration-kit/typescript/types.generated.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/typescript/types.generated.ts) — generated output; только штатная генерация.
- [fiscal-backend/internal/integration/resource_schemas_gen.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/resource_schemas_gen.go) — generated output; только штатная генерация.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сопоставить source order ID, fiscal sale ID, receipt ID, execution operation ID и parent reversal; no conflation GUIDs.
2. Проекция из immutable authoritative event проверяет tenant/device/actor/session/payment commitment и current recovery fence.
3. CAS/idempotency/version conflict обрабатываются явно; webhooks/outbox deliver-at-least-once не выполняют финансовую команду повторно.

**Тесты, которые должны существовать или быть добавлены:**

- Actual HTTP/API/broker/PG integration CASH/CARD/split/reversal/offline import; duplicate/delayed/out-of-order/mismatched events.
- Missing actor/operator, stale source version, wrong shift/source order, foreign operation GET404, unauthorized write.
- Запросы UI read authoritative state после acceptance; ошибка lookup не трактуется как операция отсутствует.

**Приёмка:** Однозначная source↔fiscal↔device цепочка и идемпотентная проекция доказаны настоящими сервисами.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P22 — Проверить UI all45 cases и реальные финансовые запреты

**Зависимости:** P11,P13,P14,P20,P21. **Трассировка:** GAP-026/027; T-14/17/18; все UI.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)
- [BeeFiscalApp/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/App.tsx)
- [BeeFiscalApp/src/screens/OperationsScreen.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/screens/OperationsScreen.tsx)
- [BeeFiscalApp/src/screens/DevicesScreen.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/screens/DevicesScreen.tsx)
- [BeeFiscalApp/src/screens/ReportsScreen.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/screens/ReportsScreen.tsx)
- [BeeFiscalApp/src/screens/AdministrationScreen.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/screens/AdministrationScreen.tsx)
- [BeeFiscalApp/src/screens/AuditScreen.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/screens/AuditScreen.tsx)
- [BeeFiscalApp/src/components/FiscalControls.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/src/components/FiscalControls.tsx)
- [minipos/BeeMiniPOS/src/useManagementAccess.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/useManagementAccess.ts)

**Тесты/gates — существующие файлы:**

- [minipos/BeeMiniPOS/__tests__/management-access-current.test.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/__tests__/management-access-current.test.tsx)
- [minipos/BeeMiniPOS/e2e/web-interaction.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/web-interaction.mjs)
- [minipos/miniposweb/tests/webBleLifecycle.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/miniposweb/tests/webBleLifecycle.test.ts)
- [scripts/test_verify_ui_acceptance.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/test_verify_ui_acceptance.rb)
- [scripts/verify_ui_acceptance.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_ui_acceptance.rb)

**Контракты/build/docs — читать и обновлять согласованно:**

- [contracts/ui-acceptance-matrix.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/ui-acceptance-matrix.json)
- [docs/remediation/2026-10-17/39_UI_STATIC_ACCEPTANCE_REVIEW.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/39_UI_STATIC_ACCEPTANCE_REVIEW.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Для каждого case из appendix ниже выполнить actual render/caller test на current source. Source anchor EVIDENCE не переводить в runtime PASS.
2. Проверить CASH/CARD/split/reversal/shift reports/readiness/operator/config/auth/session/recovery/permissions, включая first-sale/offline границы.
3. UNKNOWN блокирует оплату/смену маршрута до authority reconciliation; recovery permissions приходят от authoritative contract, не локального allowed_actions:[READ].
4. Проверить accessibility names, focus, keyboard, tap bounds48dp/primary56dp как заявлено контрактом; menu/header/theme/locale/pagination не теряют состояние при pending requests.

**Тесты, которые должны существовать или быть добавлены:**

- Cashier без config не видит/не вызывает management; role loss закрывает уже открытый editor; backend тоже denies.
- Double tap, reload/kill/background/logout/device switch при pending payment; не более1 submit; FAILED остаётся final rejection.
- Actual browser reports и Android/iOS interaction screenshots с source/build hash; test selectors не заменяют outcome assertions.

**Приёмка:** Все current45 cases имеют runtime evidence или честный OPEN/BLOCKED_SPEC; ни одна unresolved monetary/security case не допускается в pre-HIL ready.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P23 — Проверить UART/RS232 stream parser и driver composition

**Зависимости:** P02. **Трассировка:** GAP-009…012; T-05/06/07/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/serial_exchange.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/serial_exchange.h)
- [IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/protocol-abstraction/src/FrameCodec.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/FrameCodec.cpp)
- [IoT/protocol-abstraction/src/CommandPayload.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/CommandPayload.cpp)
- [IoT/protocol-abstraction/src/CommandRegistry.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/CommandRegistry.cpp)
- [IoT/common-modules/datecs/DatecsProtocol.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecs/DatecsProtocol.cpp)
- [IoT/common-modules/daisy/DaisyProtocol.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/daisy/DaisyProtocol.cpp)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh)
- [IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp)
- [IoT/protocol-abstraction/tests/FrameCodecTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/FrameCodecTest.cpp)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Actual executor должен использовать проверенный codec, а не параллельную устаревшую копию. Сопоставить compile/link sources с host test sources.
2. Реализовать bounded stream parser: noise/resync, partial reads/writes, SYN/busy/NAK, sequence wrap/reset, wrong command/SEQ, typed vendor status/ErrorCode.
3. Sequence/connection state принадлежит конкретному device session; транспортный retry допустим только по vendor duplicate semantics, а не как новый receipt/payment.

**Тесты, которые должны существовать или быть добавлены:**

- Все варианты разбиения frames, short write, timeout до/после bytes, corrupt LEN/BCC, noise before/inside packet.
- Busy/NAK flood, late old sequence, device restart, sequence wrap, unrecognized status, two concurrent callers.
- Golden bytes независимы от production encoder; mutation LEN/BCC/SEQ должен сделать тест красным.

**Приёмка:** Actual UART→parser→driver→saga path соответствует profile; ошибки не становятся успехом и не запускают blind monetary retry.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P24 — Daisy USB: setup, descriptors и fiscal conformance

**Зависимости:** P02,P23. **Трассировка:** GAP-009/011/012/021; T-05…07/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/usb_cdc_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/usb_cdc_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp)
- [IoT/common-modules/daisy/DaisyProtocol.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/daisy/DaisyProtocol.cpp)
- [IoT/common-modules/daisy/DaisyProtocol.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/daisy/DaisyProtocol.h)
- [IoT/common-modules/daisy/DaisyPrinter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/daisy/DaisyPrinter.cpp)
- [IoT/protocol-abstraction/src/DaisyFiscalAdapter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/DaisyFiscalAdapter.cpp)

**Тесты/gates — существующие файлы:**

- [IoT/common-modules/daisy/tests/DaisyProtocolTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/daisy/tests/DaisyProtocolTest.cpp)
- [IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh)
- [IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp)

**Контракты/build/docs — читать и обновлять согласованно:**

- [IoT/firmware/edge-agent-s3/idf/main/idf_component.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/idf_component.yml)
- [IoT/firmware/edge-agent-s3/idf/dependencies.lock](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/dependencies.lock)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/usb_cdc_host/usb_lifecycle_native.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/usb_cdc_host/run.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Проверить фактический USB class/interface/endpoints/VID/PID и необходимость CDC vs bridge driver для конкретного Daisy. Подключать USB Host, не USB-Serial-JTAG.
2. Mock setup проверяет enumeration/claim/line coding/transfer completion/detach/reattach; применимость CP1251 и команд зафиксировать vendor profile.
3. Проверить fiscal open/line/discount/tender/close/status/X/Z/storno/identity/EJ только для supported capabilities.

**Тесты, которые должны существовать или быть добавлены:**

- Unsupported descriptor/interface, transfer short/zero/error, hot detach during receipt, reopen same physical device, stale callback.
- Daisy golden frame/status/error vectors; invalid quantity/tax/encoding/currency и неизвестная команда отвергаются до I/O.
- Stateful receipt emulator подтверждает точный command order/result reference и recovery после lost close reply.

**Приёмка:** Actual USB boundary и fiscal codec проходят stateful host suite; питание/VBUS/PHY/cable/real firmware остаются физическими tests.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P25 — Datecs COM и BlueCash Datecs fiscal path conformance

**Зависимости:** P02,P23. **Трассировка:** GAP-010…012/021/029; T-05…07/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/serial_exchange.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/common-modules/datecs/DatecsProtocol.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecs/DatecsProtocol.cpp)
- [IoT/common-modules/datecs/DatecsProtocol.h](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecs/DatecsProtocol.h)
- [IoT/common-modules/datecs/DatecsPrinter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecs/DatecsPrinter.cpp)
- [IoT/protocol-abstraction/src/DatecsFiscalAdapter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/DatecsFiscalAdapter.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsAndroidPorts.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsAndroidPorts.kt)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/VendorAdapters.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/VendorAdapters.kt)

**Тесты/gates — существующие файлы:**

- [IoT/common-modules/datecs/tests/DatecsProtocolTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/datecs/tests/DatecsProtocolTest.cpp)
- [IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/RuntimeCodecVendorTest.cpp)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocolTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocolTest.kt)
- [SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/tests/host/DatecsCodecVendorHost.kt)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Подтвердить LEN/BCC/SEQ и profile-specific command payload/operator/till/payment/tax/reason/encoding для DP-150 MX; не объединять разные версии PM протокола без compatibility table.
2. Одинаковые независимые vectors прогнать через actual C++ runtime и Kotlin bridge. Binder wrapper JAR не доказывает наличие vendor service на BlueCash.
3. Проверить genuine fiscal receipt reference/status/query и аппаратную identity, а не synthetic result.

**Тесты, которые должны существовать или быть добавлены:**

- Golden good/corrupt frames, maximum Cyrillic fields, reserved control bytes, NAK/SYN/ErrorCode, real status bits.
- Binder unavailable/service disconnect/exception/late callback, boot reconnect; no fake Fiscal approved when service missing.
- Receipt close applied response lost→status/EJ recovery with exact original identity; no second receipt.

**Приёмка:** Оба actual paths согласованы с applicable Datecs spec; SDK service availability и model revision вынесены в HIL checklist.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P26 — Создать/довести интегральный stateful IoT device emulator runner

**Зависимости:** P02,P15,P17,P23,P24,P25. **Трассировка:** GAP-009…025/030; T-25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_orchestrator.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/uart_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/usb_cdc_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/usb_cdc_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp)
- [Makefile](/Users/freelancer/Documents/Beeloy/Fiscalisation/Makefile)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/receipt_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/receipt_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/bluepad_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/bluepad_host/run.py)
- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/04_REMEDIATION_PLAN.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/04_REMEDIATION_PLAN.md)
- [docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/run.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/transports/uart_mock.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/transports/usb_host_mock.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/transports/ble_sdk_mock.cpp` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/devices/datecs_model.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/devices/daisy_model.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/devices/bluepad_model.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/scenarios/required-cases.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сначала инвентаризировать existing harness и переиспользовать настоящие modules. Новый runner запускает actual queue/processor/saga/storage/driver; подменяет только UART/USB/BLE SDK boundary, время и необходимые platform services.
2. Отдельная независимая device model разбирает TX bytes и проверяет LEN/BCC/TLV/SEQ/command/payload/order. Oracle не вызывает production encode/decode для expected bytes.
3. У модели сохраняются physical action counters, open receipt, each tender/reference и device memory после host restart. Crash host не обнуляет симулированный уже выполненный debit/receipt.
4. Прогонять и MQTT ingress, и BLE peripheral ingress; BluePad central работает одновременно. Непредвиденный byte/command/transition немедленно FAIL, не generic success response.

**Тесты, которые должны существовать или быть добавлены:**

- CASH Datecs, CASH Daisy, CARD/split Datecs+BluePad, supported BlueCash compositions; direct handler shortcut не заменяет ingress test.
- Для каждого сценария fault до send/partial send/after execution/before checkpoint/lost result, restart+recovery; assertions на wire, result, journal, outbox и device action count.
- Сломанные LEN, mapping, wake, completion, duplicate purchase mutation обязаны дать RED; production build не содержит emulator/test key.

**Приёмка:** Единый actual-runtime EMULATED_RUNTIME gate с независимым wire oracle покрывает все enabled device profiles; это не CPU/RTOS/physical emulation claim.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P27 — Сделать добавление нового устройства расширяемым и обязательным для CI

**Зависимости:** P26. **Трассировка:** GAP-030/032/033; T-23/24/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [Makefile](/Users/freelancer/Documents/Beeloy/Fiscalisation/Makefile)
- [.github/workflows/iot-protocols.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/iot-protocols.yml)
- [.github/workflows/s3-idf.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/s3-idf.yml)
- [IoT/protocol-abstraction/src/ProtocolFactory.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/ProtocolFactory.cpp)

**Тесты/gates — существующие файлы:**

- [IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/profiles/registry.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/profiles/profile.schema.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/check_coverage.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/test_registry.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/README.md` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Registry хранит stable profile ID/version, model/firmware/region/currency/encoding, transport config, sources hashes, capability set, fixtures/scenarios и coverage status.
2. Общий runner discovers profiles из registry; новая модель добавляет data/model plugin, а не ручной if в каждом тесте.
3. CI сравнивает enabled runtime profiles/capabilities и registry: пропущенные profile/scenario/source/hash либо unexpectedly skipped case запрещают success. Unsupported command отвергается до I/O.

**Тесты, которые должны существовать или быть добавлены:**

- Новый test profile через тот же runner; enabled production profile без model/fixture →RED.
- Unknown registry version, duplicate ID, missing required case, wrong firmware binding, corrupted golden vector/source hash →RED.
- Removal of one driver test/capability check меняет gate на FAIL; расширение не ломает старые scenarios.

**Приёмка:** Документирован device onboarding template; включение поддержки невозможно без обязательных conformance/fault/recovery tests.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P28 — Проверить storno/refund/cash-in-out/reports/shift lifecycle

**Зависимости:** P12,P18,P21,P24,P25. **Трассировка:** GAP-012/019/020/024; T-07/09/10/13/17.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [fiscal-backend/internal/domain/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/service.go)
- [fiscal-backend/internal/domain/receipt_saga.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/receipt_saga.go)
- [fiscal-backend/internal/domain/payment_reconciliation.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/payment_reconciliation.go)
- [minipos/beeminipos-backend/internal/domain/service.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/service.go)
- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/receipt_saga.cpp)
- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/domain/receipt_saga_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/receipt_saga_test.go)
- [fiscal-backend/internal/domain/payment_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/payment_test.go)
- [minipos/beeminipos-backend/internal/domain/shift_close_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/shift_close_test.go)
- [minipos/beeminipos-backend/internal/domain/order_actions_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/domain/order_actions_test.go)
- [IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Каждая операция имеет собственный authorization/action/scope, original receipt/payment identity, operator/shift и reason. Support определяется конкретным fiscal/payment device.
2. Shift open/close/X/Z readiness требует действительной device/report evidence; source order deletion не заменяет storno fiscal receipt.
3. Повтор команд после possible-send только через lookup/recovery. При неподдержанном refund/VOID explicit unsupported/quarantine.

**Тесты, которые должны существовать или быть добавлены:**

- Wrong original serial/receipt/UNP/date/amount/reason, closed shift, cashier without permission, concurrent close and sale.
- Lost Z/storno/refund response, duplicate request, restart, stale source projection, source/fiscal/device disagreement.
- Результат отчёта, journal/export и displayed totals соответствуют реальным tender/fiscal states.

**Приёмка:** Все declared MVP operations проверены actual profile paths; unsupported operations отсутствуют в разрешённых UI/actions.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P29 — Реализовать настоящие электронные фискальные чеки в заявленном scope

**Зависимости:** P02,P20,P21,P28,P37. **Трассировка:** docs30; Annex29.24; T-07/13/17/18.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/profile_executor.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp)
- [fiscal-backend/internal/domain/exports.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/exports.go)
- [fiscal-backend/internal/integration/email.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/email.go)
- [fiscal-backend/internal/dossier/export.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/dossier/export.go)
- [minipos/BeeMiniPOS/App.tsx](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/App.tsx)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/domain/exports_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/exports_test.go)
- [fiscal-backend/internal/dossier/export_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/dossier/export_test.go)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/30_ELECTRONIC_RECEIPT_GAP.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/30_ELECTRONIC_RECEIPT_GAP.md)
- [contracts/bg-requirements-trace.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/bg-requirements-trace.json)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/electronic_receipt.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/electronic_receipt_test.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/api/electronic_receipt.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/api/electronic_receipt_test.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/electronic_receipt.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/electronic_receipt_integration_test.go` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

**Если меняется public API/DTO — source и generated consumers:**

- [docs/integration-kit/openapi.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/openapi.yaml)
- [docs/integration-kit/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/package.json)
- [docs/integration-kit/generate-go-validator.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/generate-go-validator.mjs)
- [docs/integration-kit/typescript/types.generated.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/integration-kit/typescript/types.generated.ts) — generated output; только штатная генерация.
- [fiscal-backend/internal/integration/resource_schemas_gen.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/resource_schemas_gen.go) — generated output; только штатная генерация.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Требование electronic_fiscal_receipts включено; не исключать его ради зелёного MVP без явного решения пользователя и актуального нормативного основания.
2. Получить authentic device-generated electronic receipt; связать immutable content/hash с sale/operation/serial/receipt/UNP. Software PDF/email summary не заменяет исходный fiscal artifact.
3. Сделать durable acquisition/delivery queue, recipient authorization, retrieval/expiry/retention/audit, paper fallback без повторной fiscalization.

**Тесты, которые должны существовать или быть добавлены:**

- Unsupported device mode deny before I/O; wrong receipt/device/sale/hash rejected.
- Acquisition applied/delivery lost ACK, duplicate delivery/webhook, restart, corrupt/missing artifact, cross-tenant recipient access, expired link.
- Переключение paper/electronic не создаёт второй чек; original content доступен в export и контрольной сверке.

**Приёмка:** Весь software lifecycle реализован по applicable legal/device profile; реальный device-generated content/delivery отдельно подтверждается HIL.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P30 — Проверить audit, exports, retention и локальный комплект НАП

**Зависимости:** P04,P15,P19,P21,P29. **Трассировка:** GAP-016/018/022/025; T-11/12/16/18/22.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [fiscal-backend/internal/dossier/export.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/dossier/export.go)
- [fiscal-backend/internal/domain/exports.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/exports.go)
- [fiscal-backend/internal/persistence/postgres.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/persistence/postgres.go)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/dossier/export_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/dossier/export_test.go)
- [fiscal-backend/internal/dossier/audit_predecessor_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/dossier/audit_predecessor_test.go)
- [fiscal-backend/internal/domain/exports_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/exports_test.go)
- [scripts/dossier-evidence-postgres-integration.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/dossier-evidence-postgres-integration.sh)

**Контракты/build/docs — читать и обновлять согласованно:**

- [database/fiscal/017_supto_full_event_model.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/017_supto_full_event_model.sql)
- [database/fiscal/019_audit_immutability.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/019_audit_immutability.sql)
- [database/fiscal/020_supto_reporting_catalogs.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/020_supto_reporting_catalogs.sql)
- [database/fiscal/021_audit_predecessor_compatibility.sql](/Users/freelancer/Documents/Beeloy/Fiscalisation/database/fiscal/021_audit_predecessor_compatibility.sql)
- [docs/remediation/2026-10-17/17_S3_SQLITE_EXTRACTION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/17_S3_SQLITE_EXTRACTION.md)
- [docs/remediation/2026-10-17/19_BLUECASH_SQLITE_EXTRACTION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/19_BLUECASH_SQLITE_EXTRACTION.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Для каждой операции проверить actor/operator/tenant/register/device/time/UNP/receipt/tenders/reversal/changes audit; typed projections и predecessor continuity сверяются с actual DB.
2. Экспортировать реальные PostgreSQL/SQLite данные и manifest/dictionary; не substitute synthetic report table. Redaction не должна разрушать необходимые reconciliation fields.
3. Проверить retention/backup/restore, chain hashes, неизменяемость audit и authorized deletion policy по применимому нормативному profile. Private identity scalar/NVS secrets не экспортируются.

**Тесты, которые должны существовать или быть добавлены:**

- Actual extraction on all3 persistence contours; empty/malformed/foreign DB, unsupported schema version, corrupt predecessor/hash, missing tender.
- Nonowner unauthorized export/write/delete, cross-tenant range/filter, huge pagination, interrupted export, resume without duplicates.
- Restore backup и сравнение logical counts/IDs/amounts/chain с исходным snapshot; content hash manifest verifier rejects tampering.

**Приёмка:** Полный traceable audit/export/dossier воспроизводится и не содержит ключей; срок хранения/формат подкреплены нормативной applicability.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P31 — Собрать actual Android/iOS приложения и BlueCash artifacts

**Зависимости:** P00,P06,P10,P11,P22. **Трассировка:** GAP-027/029/030/033; T-04/22; UI-BUILD-001.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [Makefile](/Users/freelancer/Documents/Beeloy/Fiscalisation/Makefile)
- [minipos/BeeMiniPOS/e2e/native-mvp-prepare.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-prepare.mjs)
- [minipos/BeeMiniPOS/e2e/native-mvp-build.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-build.mjs)
- [minipos/BeeMiniPOS/e2e/native-mvp-inputs.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-inputs.mjs)
- [minipos/BeeMiniPOS/e2e/native-mvp-provenance.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-provenance.mjs)
- [minipos/BeeMiniPOS/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/package.json)
- [minipos/BeeMiniPOS/package-lock.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/package-lock.json)
- [BeeFiscalApp/package.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/package.json)
- [BeeFiscalApp/package-lock.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/package-lock.json)
- [SmartDevices/bluecash-app/build.gradle.kts](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/build.gradle.kts)
- [SmartDevices/gradle/wrapper/gradle-wrapper.properties](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/gradle/wrapper/gradle-wrapper.properties)
- [minipos/BeeMiniPOS/ios/Podfile](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/ios/Podfile)
- [minipos/BeeMiniPOS/ios/Podfile.lock](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/ios/Podfile.lock)

**Тесты/gates — существующие файлы:**

- [minipos/BeeMiniPOS/e2e/native-mvp-inputs.test.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-inputs.test.mjs)
- [minipos/BeeMiniPOS/e2e/native-mvp-guards.test.mjs](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/e2e/native-mvp-guards.test.mjs)
- [SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngineTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/41_NATIVE_HARNESS_PREPARATION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/41_NATIVE_HARNESS_PREPARATION.md)
- [docs/remediation/2026-10-17/69_NATIVE_APPLICATION_BUILD_CHECKPOINT.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/69_NATIVE_APPLICATION_BUILD_CHECKPOINT.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/ios/Podfile` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/BeeFiscalApp/ios/Podfile.lock` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Сборка использует actual Expo AppEntry→App.tsx и production native modules. Отдельный instrumentation harness/Metro-only debug не выдаётся за full application build.
2. Снять hashes всех local shared UI/design modules, package/lock/resolved Gradle/Pods/native toolchain и source working tree. Не stub отсутствующий sibling dependency. HOME/global SDK не менять, caches task-local.
3. Для Android получить self-contained JS bundle в APK, manifest/permissions/package/flavor/signature/launcher; для iOS fresh matching Expo54/RN0.81.5 prebuild/Pods и simulator/device-development artifact.
4. Не отключать lint/tests из-за Metaspace OOM. Сохранить terminal failure и bounded retry с обоснованным memory budget. Текущий MiniPOS Podfile.lock Expo53/RN0.79 требует актуализации в review.

**Тесты, которые должны существовать или быть добавлены:**

- Чистая воспроизводимая build environment, actual startup smoke emulator/simulator, login/config/checkout/recovery deep entrypoints.
- BLE permission denied/BT off/background restrictions/local-network permission/secure key APIs (OS tests); min/max supported OS.
- BlueCash actual unit43 + vendor service absence; own Gradle wrapper. Final artifacts/source/dependency hashes и debug/release flags независимо проверены.

**Приёмка:** Готовы конкретные MiniPOS Android/iOS, FiscalApp Android/iOS и BlueCash профили для стенда; debug signing явно обозначен, production guard сохранён.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P32 — Завершить factory images, wiring и предохранители перед flash

**Зависимости:** P02,P06,P31. **Трассировка:** GAP-004/031; T-02/21/22.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/device-manufacturing/src/beefiscal_factory/flash_manifest.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/src/beefiscal_factory/flash_manifest.py)
- [IoT/device-manufacturing/src/beefiscal_factory/bundle.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/src/beefiscal_factory/bundle.py)
- [IoT/device-manufacturing/src/beefiscal_factory/cli.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/src/beefiscal_factory/cli.py)
- [IoT/device-manufacturing/src/beefiscal_factory/evidence.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/src/beefiscal_factory/evidence.py)
- [IoT/firmware/edge-agent-s3/idf/tools/build_idf.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tools/build_idf.py)

**Тесты/gates — существующие файлы:**

- [IoT/device-manufacturing/tests/test_flash_manifest.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/tests/test_flash_manifest.py)
- [IoT/device-manufacturing/tests/test_bundle.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/tests/test_bundle.py)
- [IoT/device-manufacturing/tests/test_evidence.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/tests/test_evidence.py)
- [IoT/firmware/edge-agent-s3/idf/tests/test_official_build_gate.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/test_official_build_gate.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [IoT/firmware/edge-agent-s3/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/README.md)
- [IoT/firmware/edge-agent-s3/idf/tools/build.lock.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tools/build.lock.json)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Различать app-only/bootloader/partition/merged image и offsets по exact target; нельзя прошивать app.bin как merged at0.
2. Проверить exact S3 board flash/PSRAM/SD/camera/strap pins. Datecs UART17/18 в примере не считать универсальной распиновкой. TTL↔RS232 module должен давать3.3V TTL; DB9 male/female не определяет DTE/DCE.
3. Daisy подключать к USB host D+/D-/protected VBUS по board schematic; не к USB-JTAG. BluePad BLE GPIO не требует; ATECC/I2C модуль не подключается.
4. Подготовить erase/provision/verify/rollback/operator instructions без выполнения hardware writes в software stage.

**Тесты, которые должны существовать или быть добавлены:**

- Manifest offsets/target/partition overlap/wrong-size/wrong-chip/hash mismatch →factory rejects.
- Отсутствующая схема/неизвестный power pin/TTL voltage →BLOCKED_HARDWARE_SPEC до подключения.
- Dry-run flash plan и actual firmware artifact provenance совпадают; factory test fixtures не включены в release image.

**Приёмка:** Есть проверяемый flash/wiring checklist конкретной платы и кабелей; software stage не утверждает, что плата уже загружается.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P33 — Замкнуть release provenance, SBOM, signing и evidence trust

**Зависимости:** P01,P27,P30,P31,P32. **Трассировка:** GAP-030/031/034; T-22/23; docs31.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [scripts/generate_release_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/generate_release_evidence.rb)
- [scripts/verify_release_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_release_evidence.rb)
- [scripts/verify_hil_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_hil_evidence.rb)
- [scripts/external_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/external_evidence.rb)
- [IoT/firmware/edge-agent-s3/idf/tools/build_idf.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tools/build_idf.py)
- [.github/workflows/s3-idf.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/s3-idf.yml)

**Тесты/gates — существующие файлы:**

- [scripts/tests/release_npm_inventory_test.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/release_npm_inventory_test.rb)
- [scripts/tests/verify_external_evidence_test.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/verify_external_evidence_test.rb)
- [scripts/tests/verify_hil_evidence_test.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/verify_hil_evidence_test.rb)
- [scripts/test-release-evidence-signing.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/test-release-evidence-signing.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/test_official_build_gate.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/test_official_build_gate.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/31_RELEASE_DEPENDENCY_INVENTORY.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/31_RELEASE_DEPENDENCY_INVENTORY.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Inventory охватывает Go/npm/local sources/Gradle+vendor JAR/Pods/native libraries/IDF managed components/bootloader/partition/app и точные tools, не только npm lock union.
2. Привязать reports/scans к exact artifact digest и tested source config. Подписывать manifest trusted release identity; verifier получает public trust anchor извне bundle.
3. CI artifact upload только reviewed safe allowlist. Raw issuer logs/JWT/private fixtures/NVS scalars не публиковать; local diagnostic hashes допускаются.

**Тесты, которые должны существовать или быть добавлены:**

- Self-signed evidence key from same bundle, tampered artifact/source/scan/SBOM, foreign scan, unsigned release, unknown issuer/time →reject.
- Test flag/root override/emulator key в production linkage →reject; missing required artifact/case/host scope →reject.
- Clean rebuild sameinputs с documented nondeterminism; source drift после tests обнаруживается финальным verifier.

**Приёмка:** Release candidate имеет независимую цепь доверия и полное описание tested composition; SBOM count не используется как критерий полноты.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P34 — Провести сквозной security negative regression

**Зависимости:** P04,P05,P06,P09,P10,P11,P12,P29,P33. **Трассировка:** T-18/19/20/23; UI-MP-020/026/028; GAP-004/008/034/035.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [fiscal-backend/internal/auth/oidc.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/auth/oidc.go)
- [fiscal-backend/internal/integration/http.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/integration/http.go)
- [fiscal-backend/internal/api/execution_authorization.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/api/execution_authorization.go)
- [minipos/beeminipos-backend/internal/api/execution_authorization.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/beeminipos-backend/internal/api/execution_authorization.go)
- [IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/execution_authorization.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/local_token_validator.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp)
- [minipos/BeeMiniPOS/src/requestAuthFence.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/requestAuthFence.ts)

**Тесты/gates — существующие файлы:**

- [fiscal-backend/internal/auth/oidc_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/auth/oidc_test.go)
- [fiscal-backend/internal/domain/operator_security_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/operator_security_test.go)
- [fiscal-backend/internal/domain/authorization_actor_contract_test.go](/Users/freelancer/Documents/Beeloy/Fiscalisation/fiscal-backend/internal/domain/authorization_actor_contract_test.go)
- [IoT/firmware/edge-agent-s3/idf/tests/authority_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/authority_host/run.py)
- [minipos/BeeMiniPOS/src/requestAuthFence.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/requestAuthFence.test.ts)

**Контракты/build/docs — читать и обновлять согласованно:**

- [compose.fiscalisation.yaml](/Users/freelancer/Documents/Beeloy/Fiscalisation/compose.fiscalisation.yaml)
- [contracts/ui-acceptance-matrix.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/ui-acceptance-matrix.json)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Проверить attack surfaces по actual current entrypoints: enrollment/provisioning/token/grants/admin/refund/export/sync/time/OTA/factory. Test matrix должна указывать source caller и required authority.
2. Проверить revocation online и offline: session/actor/binding/device/certificate/source readiness. Не обещать мгновенный offline отзыв без канала/короткой authority lease и принятой политики.
3. Crash/error/log paths не раскрывают private keys/JWT/credentials и не переключают систему в permissive fallback; bootstrap/time/control нельзя использовать как arbitrary signing oracle.

**Тесты, которые должны существовать или быть добавлены:**

- Cross-tenant/actor/register/device, guessed UUID, scope escalation, replay, downgrade/MITM, cert/root substitution, malformed/bounded bodies.
- Revocation between precheck/sign/confirm/publish, A→B→A, clock/source health lost/recovered, stale UI callbacks, restore old grant/config.
- Constant required-case inventory, real crypto/libs/PG/SDK boundary; negative mutation of signature/owner/check demonstrably RED.

**Приёмка:** Все заявленные trust boundaries имеют actual negative evidence; unresolved authority/authenticated-result gap блокирует pre-HIL software readiness.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P35 — Проверить ресурсные пределы, нагрузку и длительную работу

**Зависимости:** P08,P15,P19,P26,P31. **Трассировка:** GAP-007/008/025/030; T-11/12/20/23/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/command_queue.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/durable_storage.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/sync_runtime.cpp)
- [IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/main/time_anchor_core.cpp)
- [SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt)
- [minipos/BeeMiniPOS/src/checkoutJournal.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutJournal.ts)

**Тесты/gates — существующие файлы:**

- [IoT/firmware/edge-agent-s3/idf/tests/run_storage_host.sh](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/run_storage_host.sh)
- [IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/idf/tests/ble_origin_host/run.py)
- [minipos/BeeMiniPOS/src/checkoutJournal.test.ts](/Users/freelancer/Documents/Beeloy/Fiscalisation/minipos/BeeMiniPOS/src/checkoutJournal.test.ts)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/68_INSTALLED_TIME_LIFECYCLE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/68_INSTALLED_TIME_LIFECYCLE.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/scenarios/resource-budgets.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/iot-device-emulation/scenarios/soak.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Зафиксировать profile-specific budgets: payload sizes, queues, buffers, heap/stack, DB/outbox rows, disk reserve, retry/timeouts, nonce/sign budget и retention.
2. Для очередей/registry/fragment assembly/outbox обеспечить bounded admission/backpressure; capacity failure не теряет paid receipt и не превращается в успех.
3. Сравнить host/target composition; host memory test не доказывает реальную SRAM/PSRAM/RTOS stack. Для устройства подготовить measurable thresholds и measurement command.

**Тесты, которые должны существовать или быть добавлены:**

- Maximum legal cart/text/split/multi-device connections + simultaneous BLE central/peripheral/MQTT/sync.
- Long offline growth→near-full storage, burst reconnect/replay,>10000time refresh, sequence/generation boundary, hung device/body/write.
- Soak с deterministic seeds и random schedules: no duplicate action, monotonic durable counters, no leaks/deadlock/unbounded wait.

**Приёмка:** Host stress/resource software gates зелёные; реальные target budgets/temperature/drift/power помечены HIL_PENDING с числовыми порогами, не словами «стабильно».

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P36 — Опциональные устройства и placeholders без скрытого сужения scope

**Зависимости:** P02,P27. **Трассировка:** GAP-032/033; T-24/25.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/common-modules/termol/TremolProtocol.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/termol/TremolProtocol.cpp)
- [IoT/common-modules/termol/TremolPrinter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/common-modules/termol/TremolPrinter.cpp)
- [IoT/protocol-abstraction/src/TremolFiscalAdapter.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/TremolFiscalAdapter.cpp)
- [IoT/protocol-abstraction/src/ProtocolFactory.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/src/ProtocolFactory.cpp)
- [SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy/DaisySmartStub.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/daisy-smart-app/src/main/kotlin/com/beeloy/fiscal/daisy/DaisySmartStub.kt)

**Тесты/gates — существующие файлы:**

- [IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/protocol-abstraction/tests/ProtocolFacadeTest.cpp)
- [SmartDevices/daisy-smart-app/src/test/kotlin/com/beeloy/fiscal/daisy/DaisySmartStubTest.kt](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/daisy-smart-app/src/test/kotlin/com/beeloy/fiscal/daisy/DaisySmartStubTest.kt)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md)
- [SmartDevices/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/README.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Для каждого упомянутого устройства установить, входит ли оно в согласованный MVP matrix. Не считать наличие PDF/JAR/placeholder объявленной поддержкой.
2. Tremol включать только с applicable BG protocol/version, real driver/model/golden/recovery/HIL plan. Daisy Smart app не выдавать за Daisy USB adapter path.
3. Если capability входит в первоначальную цель, отсутствующая реализация остаётся OPEN; исключить можно только явно согласованным scope decision. Unsupported paths fail before I/O.

**Тесты, которые должны существовать или быть добавлены:**

- Enabled-but-uncovered profile →registry gate RED; unsupported command отвергается без physical action.
- Каждый включённый optional profile проходит тот же набор protocol/stateful/fault/security/artifact cases, а не только compilation.

**Приёмка:** Нет advertised-but-placeholder функций и неучтённых enabled profiles; согласованный scope не урезан ради зелёного результата.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P37 — Зафиксировать актуальный нормативный baseline и комплект НАП

**Зависимости:** P00,P02; выполняется параллельно. **Трассировка:** Annex29/BG trace; docs09/16/30/49; T-18/22.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [docs/remediation/2026-10-17/09_SUPTO_NAP_PREPARATION.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/09_SUPTO_NAP_PREPARATION.md)
- [docs/remediation/2026-10-17/16_NAP_OFFICIAL_PROCEDURE_RECHECK.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/16_NAP_OFFICIAL_PROCEDURE_RECHECK.md)
- [docs/remediation/2026-10-17/49_NAP_PRIMARY_REQUIREMENTS_RECHECK.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/49_NAP_PRIMARY_REQUIREMENTS_RECHECK.md)
- [docs/remediation/2026-10-17/30_ELECTRONIC_RECEIPT_GAP.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/30_ELECTRONIC_RECEIPT_GAP.md)
- [contracts/bg-requirements-trace.json](/Users/freelancer/Documents/Beeloy/Fiscalisation/contracts/bg-requirements-trace.json)
- [docs/SUPTO/index.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/SUPTO/index.md)

**Тесты/gates — существующие файлы:**

- [scripts/verify_legal_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_legal_evidence.rb)
- [scripts/verify_readiness_documentation.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_readiness_documentation.rb)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/43_OFFLINE_CART_AND_UNP_TIMING.md)

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Получить актуальные первичные тексты ЗДДС ст.118 и Н18/Приложение29 на дату candidate, поправки/даты вступления/переходные нормы; сохранить bytes/hash/URL/страницы. Наличие ссылки Минфина не равно прочитанному PDF.
2. Особо сверить старую/новую118(3), перенос3a→3b, electronic receipt и non-present card режимы: не делать вывод об отмене/разрешении по одному renumbering.
3. Составить requirement→feature→test→evidence→external confirmation matrix: sale-entry/UNP/readiness, operator identity, audit/storage/export, cash/card/refund/electronic receipt, software version/change control.
4. Уточнить фактически применимую процедуру НАП/СУПТО и роли производителя/пользователя/сервисной организации/эквайера; собрать заявление/описание/инструкции/контрольные данные, не подавать автоматически.

**Тесты, которые должны существовать или быть добавлены:**

- Каждый regulatory PASS поддержан актуальной applicability и конкретным software/device evidence; stale source и отсутствующая полная норма →OPEN.
- Проверить trace contradictions: intended electronic receipts=true при implementation missing не NOT_APPLICABLE.
- Внешний legal/device/acquirer outcome нельзя сгенерировать host tests или подписать собственным evidence key.

**Приёмка:** Актуальный нормативный dossier имеет reviewable источники и открытые вопросы с owner; pre-HIL software readiness не переименовывается в сертификацию.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P38 — Выполнить окончательный интеграционный pre-HIL software gate

**Зависимости:** P01…P37 для всех обязательных profiles. **Трассировка:** Все GAP/T/UI; отдельный итоговый manifest.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [Makefile](/Users/freelancer/Documents/Beeloy/Fiscalisation/Makefile)
- [.github/workflows/s3-idf.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/s3-idf.yml)
- [.github/workflows/integration-contract.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/integration-contract.yml)
- [.github/workflows/ble-certificate-crypto.yml](/Users/freelancer/Documents/Beeloy/Fiscalisation/.github/workflows/ble-certificate-crypto.yml)
- [tests/ble-transport/run.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/ble-transport/run.py)
- [scripts/run-ble-certificate-crypto.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/run-ble-certificate-crypto.py)
- [scripts/verify_release_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_release_evidence.rb)

**Тесты/gates — существующие файлы:**

- [tests/ble-transport/test_gates.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/tests/ble-transport/test_gates.py)
- [scripts/tests/test_ble_certificate_crypto_gate.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/test_ble_certificate_crypto_gate.py)
- [scripts/tests/verify_external_evidence_test.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/verify_external_evidence_test.rb)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/run-pre-hil-software-gate.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify-pre-hil-evidence.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/test_pre_hil_gate.py` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/pre-hil-candidate.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Заморозить exact candidate после всех repairs. Запустить required unit/boundary/negative, real PG/broker/service integration, actual-runtime emulator, native build и full target compile на одном согласованном snapshot.
2. Запустить existing Make gates из раздела команд ниже и реализованные новые gate targets. Не выполнять выдуманное имя как существующий target; отсутствие mandatory orchestrator — отдельная задача, не skip.
3. Для каждой строки inventory проверить actual case execution/assertion/artifact. Missing/zero/skipped/mutant still green блокирует результат. Узкий green static verifier не закрывает product requirement.
4. Независимо скачать/проверить remote artifacts, Git blobs/source hashes, linked source units, manifests и safe reports. Исправление после failed run требует релевантного повторного gate.

**Тесты, которые должны существовать или быть добавлены:**

- Удалённый обязательный case, чужой artifact, stale HEAD/config/lock, truncated report, missing marker/negative control →pre-HIL verifier RED.
- Full CASH/CARD/split/reversal/recovery/offline matrix по enabled devices×ingress×client, без canned execute/payment success.
- Проверить отсутствие software OPEN по required scope и исключений без явного решения пользователя; внешние HIL items остаются separately listed.

**Приёмка:** Только после полной проверки допустим PRE_HIL_SOFTWARE_READY для exact profiles/builds; это разрешение начать контролируемое подключение, не physical acceptance.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

### P39 — Подготовить контролируемую программу первого подключения и MVP HIL

**Зависимости:** P32,P33,P38. **Трассировка:** T-02/04/21/22/25; G3/G4 hardware portion.

**Точные пути для этой карточки.** Ниже список точек изменения по найденному RED, а не утверждение, что каждый файл сейчас содержит ошибку. Уже проверенные исправления сохранять; менять только затронутый инвариант.

**Исправление/integration — существующие файлы:**

- [IoT/firmware/edge-agent-s3/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/firmware/edge-agent-s3/README.md)
- [IoT/device-manufacturing/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/README.md)
- [SmartDevices/README.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/SmartDevices/README.md)
- [scripts/verify_hil_evidence.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/verify_hil_evidence.rb)

**Тесты/gates — существующие файлы:**

- [scripts/tests/verify_hil_evidence_test.rb](/Users/freelancer/Documents/Beeloy/Fiscalisation/scripts/tests/verify_hil_evidence_test.rb)
- [IoT/device-manufacturing/tests/test_flash_manifest.py](/Users/freelancer/Documents/Beeloy/Fiscalisation/IoT/device-manufacturing/tests/test_flash_manifest.py)

**Контракты/build/docs — читать и обновлять согласованно:**

- [docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/05_TESTS_AND_ACCEPTANCE.md)
- [docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md](/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/03_DEVICE_PROTOCOLS.md)

**Новые файлы — ПРЕДЛАГАЕТСЯ, пока отсутствуют:**

- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/HIL_FIRST_CONNECTION_RUNBOOK.md` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/hardware-inventory.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.
- `/Users/freelancer/Documents/Beeloy/Fiscalisation/docs/remediation/2026-10-17/execution/hil-required-cases.v1.json` — proposed path; создать после contract/design review, подключить к actual caller и сборке. Не существует на момент уточнения путей.

Generated outputs (если затронуты) обновлять штатным генератором из source contract; не редактировать вручную. Новую database migration добавлять отдельным reviewed файлом с актуальным номером, а не переписывать уже применённую migration только ради CI.

**Доработки и порядок:**

1. Собрать конкретные S3 board/Datecs/Daisy/BluePad/BlueCash/Android/iPhone serial+firmware+OS/cables/power inventory и exact approved artifacts.
2. Разбить HIL: wiring/voltage/boot/read-only identification → provisioning/reboot/network → read-only status/time → минимальный CASH → CARD/acquirer-approved flow → split/recovery/fault/soak.
3. Каждая стадия имеет stop criteria: неизвестный провод/voltage, wrong identity/firmware/clock, storage fault, unexpected financial action, ambiguous outcome. Не продолжать денежные циклы после UNKNOWN до reconcile.
4. Подготовить capture timing/wire/journal/export/counters/IDs без секретов; акты производителя/эквайера/НАП хранить отдельно от software reports.

**Тесты, которые должны существовать или быть добавлены:**

- Android+iOS MiniPOS и Fiscal setup; MQTT и direct BLE; Datecs COM и Daisy USB; simultaneous S3peripheral+BluePadcentral; BlueCash actual app/vendor service.
- Отключения сети/BT/USB/COM/питания в согласованных точках, app kill/background/reconnect, device reset, lost result, reconciliation; никаких blind повторов оплаты.
- Каждый HIL PASS подписан/привязан к independent trusted tester identity и exact hardware/artifacts, а не автоматически из simulator report.

**Приёмка:** Готов полный пошаговый HIL пакет, но hardware tests не отмечены выполненными до фактического проведения.

**Evidence:** exact source/build/config hashes; required case IDs; команды/exit codes; actual reports; safe summary; failure/skip inventory. Статус не менять на основании присутствия функции или зелёного detached helper.

## 6.1. Как дробить крупные карточки, чтобы модель не потеряла инварианты

Сначала выполнять указанные подкарточки последовательно. Каждая получает собственную запись RED/fix/test/evidence из раздела4. Между подкарточками нельзя включать новую capability по умолчанию до завершения полного vertical.

| Карточка | Последовательные подкарточки |
|---|---|
| P03 DB | .1 actual clean PG42P01 RED → .2 canonical revision/28-file dependency closure review → .3 reproducible export+hash verifier → .4 actual schema/constraints/permissions negative → .5 actual enrollment/projection/session E2E → .6 clean Ubuntu+owned cleanup proof. Если closure изменилась, число28 не критерий успеха: перечислить actual files/functions/dependencies. |
| P06 provision | .1 identity/enrollment trust → .2 strict package/nonce/generation validation → .3 fragment ingress → .4 atomic install/readback/quarantine → .5 proof/lookup/lostACK → .6 reboot compatibility regression. |
| P08 queue | .1 hung-worker RED → .2 completion ownership design+lock order → .3 before-accept cancellation → .4 timeout-after-accept/late result → .5 allocation/task failure cleanup → .6 sanitizers+actual ingress tests. |
| P09 trusted time | .1 source/health/error policy contract (BLOCKED_SPEC до конкретных inputs) → .2 actual monitor+epoch tests → .3 backend private issuer permit+atomic actor/source/health confirmation → .4 startup nonce/monotonic/runtime owner → .5 exact opaque bootstrap/fragment protocol → .6 actual phone owner-fenced relay → .7 interval-provider→certificate-proof wiring → .8 full actual crypto/target/negative vertical. Checked core/lifecycle не переписывать. |
| P10 BLE TLS | .1 immutable identity/certificate profile → .2 actual SDK reliable stream wiring → .3 actual standard TLS engine integration → .4 native+browser counterpart → .5 install/renewal/revocation lifecycle → .6 downgrade/MITM/ownership/resource tests. |
| P13 UNP | .1 primary normative timing+ADR reconciliation → .2 owner/allocation contract → .3 durable source/S3 integration → .4 first-entry UI readiness → .5 concurrency/crash/replay tests. |
| P14 offline | .1 versioned durable draft schema → .2 readiness/UNP reservation owner → .3 actual fresh-offline add/edit → .4 checkout/outcome persistence → .5 idempotent source import/conflict → .6 kill/restart/duplicate/revocation tests. |
| P15 storage | .1 checkpoint inventory → .2 injectable failpoint boundary → .3 atomic result/outbox → .4 per-tender durable evidence → .5 restart with preserved peer state → .6 complete crash-window matrix. |
| P17 terminal | .1 applicable vendor profile → .2 scan/CCCD/wake/completion → .3 exact TLV/request-response correlation → .4 approve/decline/unknown semantics → .5 documented lookup/void/refund capability → .6 simultaneous BLE central/peripheral tests. |
| P26 emulator | .1 runtime-source manifest → .2 UART/USB/BLE SDK mocks → .3 independent Datecs model → .4 independent Daisy model → .5 independent BluePad model → .6 actual ingress+queue+saga+SQLite runner → .7 crash/wire/action-counter checks → .8 mutation negatives and CI binding. |
| P29 e-receipt | .1 applicable device/legal capability → .2 authentic acquisition/result binding → .3 durable original artifact → .4 authorized recipient/delivery/retrieval → .5 paper fallback/idempotent recovery → .6 audit/export/security tests. |
| P31 native | .1 source/dependency/toolchain manifests → .2 actual BlueCash build+JUnit → .3 actual MiniPOS Android full bundle → .4 actual FiscalApp Android → .5 fresh matching iOS prebuild/Pods+both apps artifacts → .6 emulator/simulator startup/permissions/key tests → .7 exact artifacts handoff. |
| P33 release | .1 complete dependency/material inventory → .2 artifacts/report hash binding → .3 external verifier trust policy → .4 tamper/missing/foreign negative → .5 reviewed safe publication → .6 independently verified candidate. |
| P38 final | .1 freeze candidate → .2 required-case discovery check → .3 software suites+actual DB/broker → .4 emulator full matrix → .5 native/target artifacts → .6 independent source/artifact/evidence verification → .7 requirement-by-requirement readiness decision. |

Нельзя поручать слабой модели «сделай весь P09» без этих промежуточных проверок. При ошибке более ранней подкарточки вернуться к её RED; не добавлять fallback в следующую, чтобы скрыть причину.

## 7. Полнота исходного GAP register: ни один пункт не потерять

Ниже routing, а не новые PASS. Исходные критерии каждого GAP обязательно прочитать в реестре. Частичный fix закрывает только проверенную часть.

| Исходный GAP | Карточки нового плана |
|---|---|
| GAP-001 — MQTT command namespace не совпадает | P05 |
| GAP-002 — S3 business ACK не доходит | P05,P19 |
| GAP-003 — BeeFiscalApp → S3 onboarding отсутствует как совместимый цикл | P06,P07,P31 |
| GAP-004 — Нет замкнутого bootstrap сети, trust и secrets S3 | P06,P09,P10,P32 |
| GAP-005 — Provisioning BFPE требует одного oversized GATT write | P06,P07 |
| GAP-006 — Legacy advertising S3 превышает budget | P07 |
| GAP-007 — BLE MTU, framing и timeout расходятся | P07,P08,P35 |
| GAP-008 — BLE flow/session isolation не замкнуты | P07,P08,P10,P11 |
| GAP-009 — Daisy runtime codec не соответствует протоколу | P23,P24,P26 |
| GAP-010 — Datecs runtime codecs ошибаются в LEN на четыре | P23,P25,P26 |
| GAP-011 — Serial exchange не реализует полноценный NAK/SYN/error parser | P23,P24,P25 |
| GAP-012 — Sequence reset и vendor mapping/encoding | P02,P23,P24,P25,P28 |
| GAP-013 — BluePad central не исполняет vendor BLE wake/flow | P17,P26 |
| GAP-014 — BluePad correlation и reference types не доказаны | P17,P18 |
| GAP-015 — Неполная проверка S3 intent до оплаты | P12 |
| GAP-016 — S3 UNP allocation и recovery snapshot расходятся | P04,P13,P14,P15,P16 |
| GAP-017 — Storage errors не останавливают денежный side effect | P08,P15 |
| GAP-018 — Terminal state и outbox не атомарны | P15,P19,P30 |
| GAP-019 — Split-card evidence и compensation не индивидуальны | P16,P17,P18 |
| GAP-020 — Recovery стартует раньше readiness и может отменить чужой чек | P16,P18,P28 |
| GAP-021 — Synthetic fiscal_reference вместо device evidence | P16,P20,P24,P25 |
| GAP-022 — S3 sync hash preimage несовместим с Go map serialization | P19,P30 |
| GAP-023 — S3 не фиксирует неизменяемый in-flight sync batch | P14,P15,P19 |
| GAP-024 — Backend/S3 command types, result events и sale ID расходятся | P05,P18,P20,P21,P28 |
| GAP-025 — S3 command expiry и bounded storage не закрыты | P08,P09,P12,P19,P35 |
| GAP-026 — MiniPOS Web UNKNOWN может повторно submit | P11,P20,P21,P22 |
| GAP-027 — Native checkout и outbox нужно доказать на реальном lifecycle | P10,P11,P14,P20,P22,P31 |
| GAP-028 — MiniPOS backend regression suite красный | P03,P04,P21 |
| GAP-029 — BlueCash build/readme не соответствуют composition | P02,P25,P31 |
| GAP-030 — Software PASS не доказывает physical runtime path | P00,P01,P26,P27,P31,P33,P38,P39 |
| GAP-031 — Factory flash не различает app и merged image | P06,P32,P33 |
| GAP-032 — Tremol не подтверждён отдельным conformance contour | P02,P27,P36 |
| GAP-033 — Daisy Smart app и placeholders не являются готовым продуктом | P02,P27,P31,P36 |
| GAP-034 — HIL verifier доверяет ключу из самого evidence bundle | P01,P33,P34 |
| GAP-035 — MQTT outcome принимался без immutable tenant/device association | P05,P21,P34 |
| GAP-036 — Result JSON обрезался и не экранировал device references | P20 |

## 7.1. Все исходные тестовые направления T-01…T-27

| Test ID/название из исходной спецификации | Карточки |
|---|---|
| T-01 — Сквозной MQTT contract | P05,P19 |
| T-02 — Factory-reset BLE provisioning | P06,P32,P39 |
| T-03 — S3 GATT transport boundaries | P07,P08 |
| T-04 — Native BLE Android/iOS lifecycle | P07,P10,P11,P31,P39 |
| T-05 — Независимые Daisy/Datecs codec golden vectors | P23,P24,P25 |
| T-06 — Physical stream parser, timeout и SEQ | P23 |
| T-07 — Profile payload/result conformance | P12,P24,P25,P28 |
| T-08 — BluePad vendor BLE и correlation | P17 |
| T-09 — Card saga, split и компенсация | P18 |
| T-10 — Monetary validation, authority и UNP | P12,P13 |
| T-11 — Durable failpoints и recovery | P15,P16 |
| T-12 — Cross-language sync, lost ACK и chain | P05,P19 |
| T-13 — Domain materialization sale/reversal/offline | P21,P28,P29 |
| T-14 — Durable UNKNOWN и route switching | P11,P14,P20,P22 |
| T-15 — MiniPOS backend contract regressions | P04,P21 |
| T-16 — Real PostgreSQL migrations, RLS, concurrency | P03,P04 |
| T-17 — Outbox/reference/offline reconciliation UI | P14,P16,P20,P22 |
| T-18 — Admin/auth/security capability | P06,P11,P12,P22,P29,P34,P37 |
| T-19 — WebHook/API orchestration faults | P03,P21,P34 |
| T-20 — Deploy/runtime health и transport failure | P08,P09,P35 |
| T-21 — Минимальный физический smoke перед основными циклами | P32,P39 |
| T-22 — Reproducible artifacts и factory/release gate | P31,P32,P33,P39 |
| T-23 — Gate quality и runtime composition | P00,P01,P27,P33,P38 |
| T-24 — Tremol conformance при включении в scope | P36 |
| T-25 — IoT build + расширяемая эмуляция USB/COM/BLE устройств | P26,P27 |
| T-26 — Scoped MQTT outcome | P05,P21 |
| T-27 — Device reference JSON integrity | P20 |

Эта таблица не заменяет исходные test preconditions/oracles. Новый required test добавляется в inventory и gate, а не только в текст.

## 8. Полный UI inventory текущего контракта

Baseline содержит45cases. Для каждого нужен actual caller/render/runtime test; EVIDENCE/source anchor не равен PASS. Если контракт изменился, inventory обновляется без удаления исходного requirement. P22 отвечает за весь список; дополнительные владельцы указаны рядом.

| Case | Что требуется проверить (invariant текущего JSON) | Дополнительно |
|---|---|---|
| UI-MP-001 | Interactive controls meet 48dp and primary CTA 56dp minimums. | P22 |
| UI-MP-002 | Actions have semantic roles and changing status is announced. | P22 |
| UI-MP-003 | Critical journeys expose stable cross-platform selectors. | P22 |
| UI-MP-004 | Unknown outcome freezes payment; reconciliation is rendered only from backend READ permission and uses lookup/reconcile APIs without repeating payment. | P20/P21 |
| UI-MP-005 | The first product intent obtains fresh workstation/device readiness and a server-owned session before the atomic sale open; failures block the sale. | P13 |
| UI-MP-006 | Web BLE is capability-gated and initiated by an explicit user action. | P22 |
| UI-MP-007 | MiniPOS uses public APIs with personal OIDC authority and submits one aggregate fiscal finalize command with one durable client operation id. | P22 |
| UI-MP-008 | The product editor preserves optional barcode and scanner/keyboard submit resolves an exact barcode before adding the item to an open shift. | P22 |
| UI-MP-009 | The UI treats a shift as closed only when MiniPOS returns a final Fiscal Z operation and fiscal reference; missing evidence fails closed. | P22 |
| UI-MP-010 | Production sales resolve exactly one active employee from a personal email OTP session; static tokens and free employee selection cannot start a shift, and logout is durably submitted before credentials are discarded. | P22 |
| UI-MP-011 | The touch UI keeps the canonical original receipt reference distinct from storno evidence, renders reversal only from server-owned REVERSE permission, submits a documented reason through the MiniPOS public API and never presents an ambiguous result as success. | P22 |
| UI-MP-012 | The touch UI validates a positive partial cash amount, derives the exact EUR card remainder, preserves CASH then CARD order and submits the ordered payment array in one aggregate finalize command. | P22 |
| UI-MP-013 | The thin UI consumes the Fiscal operation result: UNKNOWN freezes and reconciles, FAILED is final rejection, and authoritative sale state is reloaded after every accepted payment. | P14/P20 |
| UI-MP-014 | An expired, failed, logged-out or configuration-invalidated BLE session loses READY immediately, cancels local authority, attempts public-API revoke while credentials remain available and permits explicit re-issuance with a new client key. | P22 |
| UI-MP-015 | Every MiniPOS and Fiscal public-API call has a bounded abort deadline, so a hung route cannot retain the touch UI busy state indefinitely. | P22 |
| UI-MP-016 | The cashier email session schedules rotating refresh before expiry; absent or failed refresh clears persisted personal credentials and returns the UI to login fail closed. | P22 |
| UI-MP-017 | Email session expiry or either public API returning 401 clears cashier authority, drops local BLE READY immediately and attempts revocation with the previous in-memory bearer before returning to personal login. | P22 |
| UI-MP-018 | After restart or re-authentication MiniPOS restores the active employee/register shift and only its server-owned DRAFT/OPEN source order; an existing durable checkout remains in reconciliation and authority loss clears its projection. | P22 |
| UI-MP-019 | A recovered blocked Z shift disables sales and exposes reconciliation only from server-owned actions; the UI calls the dedicated reconciliation route rather than repeating shift close or reporting success without final Z evidence. | P22 |
| UI-MP-020 | A production cashier neither downloads the employee directory nor sees object/report editors; supervisor/admin roles explicitly unlock management, and authority loss closes any previously visible administrative surface. | P11/P34 |
| UI-MP-021 | The commercial report always supplies the required ordered RFC3339 period and renders the exact EUR gross returned by the public API. | P22 |
| UI-MP-022 | MiniPOS rejects blank or alias register/device identifiers before shift, BLE, readiness or configuration calls; every public Fiscal resource identity is a canonical UUID. | P22 |
| UI-MP-023 | Cloud checkout resolves the bound end device from the configured register through the public Fiscal API and fails closed for mismatched, inactive, unbound or malformed registry state; deployment does not require a hidden device ID. | P22 |
| UI-MP-024 | Shift recovery is never broadened to all registers when workstation configuration is absent or invalid; MiniPOS clears the server projection and requires an exact configured Fiscal register. | P22 |
| UI-MP-025 | Supervisor/admin discount controls require current committed management authority, submit a versioned source-order line mutation and derive the displayed total from the returned server projection. | P22 |
| UI-FISC-001 | Fiscal administration lists runtime data only through the public API, follows every documented cursor page and never sends a non-UUID register filter. | P22 |
| UI-FISC-002 | Cloud/Edge/driver/final-device readiness is shown as distinct hops. | P22 |
| UI-FISC-003 | Report actions call the Fiscal public API, expose accessible controls and render returned state. | P22 |
| UI-FISC-004 | Fleet navigation, live status, filters and readiness actions expose accessible semantics and compliant touch targets. | P22 |
| UI-FISC-005 | Location, register, operator, DEV fiscal-device and binding editors use optimistic public API writes and enforce the staged device activation lifecycle. | P22 |
| UI-FISC-006 | Fiscal operation actions are rendered and submitted only when present in the server-provided allowed_actions array; a missing action fails closed. | P22 |
| UI-FISC-007 | Every Fiscal administration public-API call has a bounded abort deadline and returns control to the operator on a hung route. | P22 |
| UI-FISC-008 | Production administration requires a personal email OTP session, ignores public static bearer tokens, makes no pre-login API request and provides explicit logout. | P22 |
| UI-FISC-009 | The administrator refreshes rotating email-session authority before expiry; missing or failed refresh clears local authority and returns all public API access to fail-closed login state. | P22 |
| UI-FISC-010 | OIDC expiry or a Fiscal 401 clears administrator authority and returns every public API operation to the fail-closed personal login state. | P22 |
| UI-FISC-011 | Production navigation and commands derive from fail-closed OIDC roles: only ADMIN sees editors, ADMIN/AUDITOR read the audit journal, AUDITOR has report read-only access, and report execution or reconciliation requires SUPERVISOR or ADMIN in addition to server-owned allowed actions. | P22 |
| UI-FISC-012 | BeeFiscal exposes role-gated provisioning, redacted diagnostics/capabilities, REST-issued proof-of-possession BLE session preparation, separate transport/final-device metrics and distinct fiscal-device versus optional-terminal configuration through public contracts. | P06/P10 |
| UI-FISC-013 | ADMIN and AUDITOR can filter and inspect the immutable public-API audit journal by period, actor, action, UNP and object with timestamp and hash evidence. | P22 |
| UI-FISC-014 | An absent register means an unfiltered collection read, while every supplied collection filter, report, BLE, probe, binding or resource path is UUID-validated before request construction. | P22 |
| UI-PAGE-001 | Displayed paginated collections traverse all opaque cursor pages with bounded, fail-closed loop protection. | P22 |
| UI-BUILD-001 | Both applications have release-gated Web, Android and iOS bundle paths; MiniPOS also fails if the noble package-exports fallback returns. | P31 |
| UI-E2E-001 | A real browser executes one aggregate barcode cash/split/card checkout, canonical commercial report, known rejection, production OIDC fail-closed login, proves public MiniPOS/Fiscal static tokens are absent from the PROD bundle, reconciles ambiguous outcomes and performs BeeFiscal readiness, provisioning, BLE, diagnostics, audit, composite binding lifecycle, reporting and administration without direct database access. | P22/P38 |
| UI-MP-026 | Android, iOS and web BLE READY and financial outcomes require real client proof of possession and authenticated device replies bound to the active session and exact operation. | P10 |
| UI-MP-027 | With valid offline authority and an authenticated S3 route, a cashier can create a new durable commercial cart and obtain S3-owned fiscal identity before the regulated sale-entry boundary, recover after restart and later synchronize without creating a second sale or payment. | P13/P14 |
| UI-MP-028 | An unauthorized response may revoke only the auth session that dispatched the request; late responses from an old session cannot log out a new operator or cause a financial retry. | P11 |

## 9. Обязательная сквозная матрица до устройств

Для каждой допустимой строки прогнать: нормальный успех; документированный final rejection; ошибка до send; partial/possible-send; device executed+response lost; DB checkpoint failure; process kill/restart; duplicate UUID; wrong authority; late old-owner result. Проверять одновременно wire order, physical-model counters, authoritative result, journal/tender/outbox/source projection.

| Device composition | Ingress/клиент | Операции |
|---|---|---|
| S3 + Datecs COM | backend MQTT; MiniPOS Web BLE; MiniPOS Android BLE; MiniPOS iOS BLE | CASH; CARD/split только с подтверждённым BluePad profile; supported storno/X/Z/cash-in-out/recovery |
| S3 + Daisy USB | те же применимые маршруты | CASH и supported fiscal operations; CARD только с явно добавленным совместимым payment endpoint/profile |
| S3 peripheral + BluePad central одновременно | MiniPOS BLE и backend MQTT с cross-route UUID | CARD/split/recovery; одновременные notifications не смешивают ownership |
| BlueCash Android actual app + Datecs vendor service boundary | MQTT/local route согласно actual app capability | CASH/CARD/split/recovery в реально включённом profile; отсутствующий Binder service не даёт success |
| BeeFiscalApp setup/management | Web там, где поддержан WebBluetooth; Android; iOS native | scan/connect/provision/reboot/status/config/authorization loss; без неподтверждённого raw key trust |
| Source/Fiscal/backend integration | actual HTTP+PG+Rabbit/MQTT | orders/prepare/dispatch/outcome/offline import/refund/report/export/OTP/session concurrency |

Не объявлять поддержанным iOS WebBluetooth, если платформа его не предоставляет. iOS native BLE проверяется отдельно. Для unsupported platform-route нужен явный корректный отказ/UI alternative, не пропуск обязательного native сценария.

## 10. Универсальные параметры fault injection

Каждая комбинация проверяется там, где параметр применим. Не создавать тысячи бессмысленных Cartesian tests: pairwise покрывает независимые параметры, все критические money/authority/crash boundaries проходят явно.

| Граница | Инъекции | Запрещённый результат |
|---|---|---|
| Bytes/parser | empty/max/max+1, any fragmentation, bad LEN/BCC/TLV/SEQ/CMD, invalid UTF8/NUL, duplicate/trailing/noncanonical | accepted invalid command; false fiscal/payment success |
| Transport | connect/discovery/CCCD/wake fail, partial write, hanging completion, late/duplicate/out-of-order notify, detach/reconnect | blind financial retry; cross-session delivery; unbounded wait |
| Authority | wrong tenant/actor/device/register/key/purpose, expiry, revocation and A→B→A at each await/sign/commit | stale authority publication; arbitrary signing oracle |
| Time | no anchor/coldboot, stale nonce, source health epoch ABA, drift/RTT/holdover boundary, clock jump/sleep | SNTP/wallclock fallback; resurrected old anchor |
| Storage | allocation/full/readonly/busy/corrupt/commit uncertainty/readback mismatch, crash at every checkpoint | lost approved tender; success without durability; lost only result |
| Service | invalid/expired JWT, stale401/new login, body hang, HTTP error taxonomy, Rabbit/broker offline, DB restart | logout new session;404 interpreted as NOT_SENT; retry on unrelated route |
| Resource | queue/buffer/outbox limits, long offline, burst reconnect, max cart, nonce/SEQ/generation exhaustion | overflow/wrap, silent drop, UAF/deadlock/leak, repeated debit |
| Evidence | missing case/marker/artifact, wrong hash/profile/HEAD, self-provided trust key, skipped test | green gate despite missing proof |

## 11. Команды: существующие и предлагаемые

Команды запускать **из корня**, после чтения их текущих recipes. Make target может требовать SDK/environment/services; отсутствие dependency фиксировать как ENVIRONMENT_MISSING, не заменять stub. Подготовить toolchain по `TOOLCHAIN.md`, `idf/tools/build.lock.json`, `tests/ble-transport/README.md`. Не использовать чужие/глобальные caches/SDK как необъяснённый proof.

Существующие entrypoints на baseline:

```sh
make test test-race vet
make governance-test readiness-doc-test
make contract-test typecheck roadmap-acceptance-test
make driver-coverage-test fault-regression-test soak-regression-test security-test
make ui-acceptance-test ui-interaction-test
make iot-protocol-test time-anchor-test
make iot-cbor-encoder-test iot-cbor-encoder-interop-test
make ble-certificate-crypto-test ble-stream-test
make iot-test
make iot-build-script-test iot-build-check
make iot-build S3_BUILD_DIR=/absolute/new/empty/build-directory
make iot-build-verify S3_BUILD_DIR=/absolute/new/empty/build-directory
make pg-fixture-preparation-test typed-projection-test postgres-integration
make compose-check compose-e2e
make minipos-outbox-test native-harness-test native-cloud-confirmation-test
make smart-device-host-test smart-device-test
make web-build native-bundle android-build ios-build
make factory-test evidence-test
make nap-local-extraction-test nap-postgres-extraction-test
make nap-minipos-postgres-extraction-test nap-evidence-postgres-extraction-test
make supto-trace-test country-profile-test supto-document-policy-test
make supto-unp-test supto-sale-lifecycle-test supto-readiness-test supto-time-test
make supto-audit-test supto-export-test supto-offline-equivalence-test
make supto-software-acceptance
make regression
make full-regression
```

Не требуется запускать всё заново после правки одной опечатки. После source fix — targeted/dependent tests; перед P38 — полный обязательный integrated gate. `android-build/ios-build` проверить на actual entrypoint/variant: команда в Makefile сама по себе не доказывает нужный full-app artifact.

Финальные production/legal/HIL verifiers могут быть красными из-за отсутствующих внешних доказательств. Это честный результат; не создавать фиктивные акты. `supto-full-acceptance`, `supto-hil-verify`, `supto-release-verify`, `supto-legal-verify` не объявлять пройденными без всех inputs соответствующего scope.

**Предлагаемые orchestrators, наличие проверить, при отсутствии реализовать в P27/P38:** `iot-emulation-test`, `pre-hil-software-gate`, `pre-hil-evidence-verify`. Они обязаны перечислять required profiles/cases и отказать при missing/skip. До реализации эти названия не являются командами готовой системы.

## 12. Разрешение начать подключение: точные критерии

Все пункты обязательны для заявленного MVP profile, если нет отдельного явного scope решения пользователя:

- [ ] Полный baseline и requirement inventory; все36GAP/27T/45UI учтены, новые findings добавлены.
- [ ] Нет unresolved P0/P1 **software** defects по required scope; нет `BLOCKED_SPEC` в разрешаемом financial/security path.
- [ ] Actual protocol drivers, independent vendor vectors и stateful USB/COM/BLE models покрывают все enabled devices/capabilities.
- [ ] Реальные сервисы/PG/broker/SQLite выполняют CASH/CARD/split/reversal/recovery/offline flows идемпотентно и с правильной authority.
- [ ] UNP/readiness timing и fresh offline cart закрыты по принятому актуальному contract; journal сохраняет подтверждённый outcome до нужного sync.
- [ ] Operational BLE защищён в обоих направлениях; time/provider/root/PoP/revocation lifecycle реально интегрированы, не только helpers.
- [ ] Обязательные tests не skipped, negative controls дают RED; ошибки test discovery/environment не скрывают missing execution.
- [ ] Есть exact matching S3 bootloader/partition/app, BlueCash APK и actual MiniPOS/Fiscal native artifacts; sources/deps/config/tools/signatures/hashes проверены независимо.
- [ ] Canonical schema deployment dependency/fixture закрыта, clean CI E2E выполняет enrollment/projection/session tests.
- [ ] Audit/export/electronic receipt и intended capabilities не вычеркнуты ради readiness; их software lifecycle покрыт.
- [ ] Evidence trust anchored externally, no bulk private logs/keys; release manifest точно связывает tests/artifacts/profile.
- [ ] Конкретные wiring/power/flash manifests проверены; hardware inventory, stop rules и HIL procedures готовы.
- [ ] Нормативный baseline актуализирован настолько, чтобы безопасно определить software scope; оставшиеся внешние подтверждения перечислены явно и не объявлены полученными.

Итог: `PRE_HIL_SOFTWARE_READY`, exact candidate/profile list, список `HIL_PENDING/EXTERNAL_PENDING`. Если хотя бы одна обязательная software строка не доказана — `NOT_READY`, с case IDs и следующим действием. `PASS` у общего checker, который проверяет только согласованность слов/JSON, не меняет этот результат.

## 13. После первого подключения: отдельная физическая приёмка

P39 описывает порядок. Начать с read-only identity/status и электрической безопасности; только затем согласованные денежные циклы. Отдельно измерить actual startup entropy/drift/stack/heap/PSRAM/SD/power recovery, real USB enumeration, UART voltage/DB9 wiring, native OS radio/background behavior, BluePad/acquirer reference semantics и vendor SDK service availability.

Результат хранить как `PHYSICAL_HIL`, не `EMULATED_RUNTIME`. Первые green smoke не закрывают длительные split/reversal/power/network recovery tests. Применимая НАП/СУПТО процедура, manufacturer/acquirer подтверждения и production release approval остаются отдельными проверяемыми deliverables.

## 14. Формат итогового отчёта исполнителя

1. Candidate commit+dirty/source hashes, device/profile scope и список артефактов.
2. Таблица всех требований: состояние, actual test/caller, command/exit code, evidence, remaining external item.
3. Все failures, skips, BLOCKED_SPEC и непроверенные предположения; никаких «в основном работает».
4. Число физических действий модели/устройства и фактические money/receipt invariants для каждого fault cycle.
5. Независимая проверка artifact/source/dependency/evidence hashes и trust anchor.
6. Однозначный вывод `NOT_READY` либо profile-specific `PRE_HIL_SOFTWARE_READY`; отдельные HIL/legal production статусы.

**Первое действие следующего исполнителя:** P00, затем P01; после этого выбрать первую незакрытую карточку с выполненными prerequisites. Не начинать с массового refactor, переименования статусов или переписывания всех тестов.

## 15. Готовая инструкция для запуска слабой модели

Скопировать этот prompt, подставив одну карточку. Не выдавать модели сразу40 карточек на реализацию:

```text
Репозиторий: /Users/freelancer/Documents/Beeloy/Fiscalisation.
Прочитай разделы1–5,16–19,21–23,25 и карточку Pxx документа
 docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md.
Затем прочитай перечисленные в Pxx actual source/caller/test files.
Найди относящиеся к ней CT cases раздела20 и interoperation cases раздела24.
Выполни только первую незакрытую подкарточку Pxx.n, прошедшую Definition of Ready.
Если Definition of Ready не выполнен, выполни только discovery/design этой подкарточки
и зафиксируй точные missing inputs вместо предположений.

Перед правкой:
- зафиксируй current HEAD/dirty paths/file hashes;
- проверь prerequisites по реальному evidence, а не слову PASS;
- найди invariant и воспроизведи RED actual runtime test;
- если это уже исправлено, reverify и не переписывай работающий код.

Выполнение:
- исправляй root cause, не создавай параллельную заглушку;
- не меняй чужие UI/dist/untracked файлы;
- не ослабляй authorization, time/cert validation, durability или assertions;
- UNKNOWN/possible-send никогда не превращай в разрешение повторной оплаты;
- все keys остаются ESP32-S3/NVS, ATECC не добавляется, Go edge-agent не возвращается;
- не прошивай реальные устройства и не делай реальные денежные операции;
- используй только проверенные существующие команды либо явно реализуй proposed target;
- для зависимого неизвестного protocol/legal вопроса запиши BLOCKED_SPEC с конкретным вопросом;
- не останавливай независимые безопасные проверки из-за отдельного unresolved внешнего вопроса.

После правки:
- targeted positive/boundary/negative/fault + relevant integration/build;
- missing/skip обязательного теста означает FAIL;
- meaningful mutation должна вернуть RED в isolated copy;
- проверь восстановление source hashes после mutation;
- сохрани запись задачи по шаблону раздела4 и safe evidence;
- укажи exact scope: что доказано, что осталось OPEN/HIL_PENDING;
- не объявляй весь MVP/НАП готовым по одному зелёному тесту.

Ответ: изменённые файлы, invariant/root cause, RED, exact commands/exit codes,
required cases, artifacts/hashes, ограничения, следующая зависимая подкарточка.
```

Для следующей модели передавать текущую запись задачи и ссылки на evidence, а не только разговорное «всё готово». После нескольких подкарточек интегратор сверяет общий candidate и запускает dependent regression; P38 остаётся обязательным общим gate.

## 16. Ревью исполнимости плана — 2026-10-21

**Вердикт:** исходный вариант достаточно полно перечислял направления, но не был самодостаточной инструкцией для полностью автономного закрытия всех разрывов слабой моделью. Большая карточка всё ещё требовала архитектурного выбора, детализации fixture/команды, выбора независимого oracle и интерпретации юридически/аппаратно неизвестных условий. Разделы16–25 обязательны и дополняют P00…P39; scope не сокращается.

| Найденный пробел | Почему мешает самостоятельному исполнению | Исправление |
|---|---|---|
| Source ошибочно стоял выше normative/vendor contract | Модель могла узаконить баг изменением ожидания | Исправлен раздел2; required behavior и actual behavior разделены |
|40карточек содержали крупные design tasks | Модель могла сразу писать disconnected helper либо менять несколько trust boundaries | Definition of Ready; пакет разбивается на исполнимые Pxx.n с reviewed contract/caller/oracle |
| Нет frozen решения по части протоколов/time/offline/receipt | Модель могла придумать wire bytes, UTC guarantees, UNP или fake receipt | Реестр D01…D12, допустимые входы/выходы и fail-closed до решения |
| Generic «fault tests» без exact postconditions | Можно получить green assertion только по HTTP/ESP_OK | CT01…CT24, state/counters/durable invariants и evidence schema |
| Нет единой outcome taxonomy | Timeout мог превратиться в retry/FAILED/NOT_SENT | Раздел19: outcome knowledge отдельно от actual device action и authoritative projection |
| Названия gates выглядели как готовый runner | Модель могла создать shell с echo PASS | Точные условия новых orchestrators, expected-case equality и отрицательные проверки gate |
| Огромный список Make commands без environment manifest | Модель могла использовать ambient SDK/npm/DB либо запустить destructive target | Раздел21: recipe на подзадачу, prerequisite inventory, owned disposable services и no-hardware guard |
| Не определена инвалидация старого evidence | Старый green мог закрыть изменённый code/config | Раздел23: impact/retest matrix и immutable candidate rules |
| Широкие cards/DAG и параллельная подготовка смешивались | Можно merge незавершённую authority dependency | Dependency readiness действует на implementation/acceptance, исследования допускаются отдельно |
| Не было лимита цикла «попробуй ещё» | Бесконечный rerun и замена flaky теста sleep | Failure taxonomy, deterministic barriers, bounded diagnostic/retry policy |
| Отсутствовали требования к каждому новому emulator/profile | Модель могла скопировать encoder в oracle | Plugin/fixture contract и negative controls в разделе22 |
| Не был определён независимый финальный reviewer | Самоотчёт исполнителя становился acceptance | Раздел25: проверка source/caller/report/artifact/coverage, без обхода missing proof |

После дополнений модель может последовательно исполнять **готовые к исполнению** подзадачи и проверять результат. Документ не создаёт отсутствующую vendor specification, аппаратную схему, health SLA источника времени или внешнее юридическое подтверждение. Такие зависимости нужно получить и зафиксировать; при их отсутствии соответствующая задача остаётся BLOCKED_SPEC/EXTERNAL_PENDING. Утверждать полную автономную готовность всего продукта до их получения нельзя.

## 17. Definition of Ready: обязательный вход для каждой подкарточки

P01 должен добавить в task record следующие поля. Это **схема будущего inventory**, а не уже существующий API/файл. Запись хранить, например, в новом `docs/remediation/2026-10-17/execution/Pxx.n.md` и машиночитаемом inventory, формат которого фиксируется до реализации gate.

```yaml
task_id: Pxx.n
requirements: []
profile_ids: []
depends_on: []
prerequisite_evidence: []
required_behavior_sources: [] # revision/hash/page или approved contract
actual_entrypoint: ''
actual_caller_chain: []       # UI/API/ingress → queue/service → driver/store → result
source_files: []              # точные существующие relative paths, без wildcard ...
allowed_change_files: []
proposed_new_files: []        # цель каждого нового файла и actual caller
frozen_contract: ''          # payload/types/errors/state/ownership/limits
fixture_inputs: []           # path/hash/provenance; synthetic secret marker
independent_oracle: ''
initial_state: ''
red_test_id: ''
required_test_ids: []
expected_state_transitions: []
forbidden_side_effects: []
commands: []                 # cwd/argv/env names/versions/timeout/expected exit
cleanup_resources: []        # exact owned project/container/volume/cache IDs
acceptance_assertions: []
retest_dependents: []
review_risk: ''
```

**Запрет старта implementation:** отсутствуют exact caller, применимый required contract, independent oracle, RED/expected postconditions, bounded execution recipe или prerequisite evidence. Вместо догадки выполнить отдельную `.design`/`.discovery` подзадачу, сохранить найденные источники и минимальный contract diff. Неизвестный file path разрешается найти `rg`, но найденный путь необходимо внести в record до изменения.

**Definition of Done подкарточки:** accepted required behavior реализовано в actual caller; old RED проходит; boundary/negative/dependent gates выполнены; meaningful mutant RED; source restoration проверено; нет unexplained failures/skips; source/config/dependency/artifact hashes совпадают; limitations перечислены. Только затем статус SOFTWARE_VERIFIED_HIL_PENDING. Parent Pxx остаётся OPEN, пока каждая обязательная подкарточка не завершена.

Модель не обязана заново запрашивать разрешение на каждую routine правку в уже авторизованном scope. Design review здесь нужен для фиксации correctness/trust boundaries, а не как дополнительный общий permission flow. Если решение уже принято в существующем contract/сессии, сослаться на него и продолжить.

## 18. Реестр решений, которые нельзя придумывать

Довести каждое решение до frozen артефакта до зависимого implementation. Для всех D-записей фиксировать выбранное значение, источники с hashes, версии consumers, совместимость, negative tests и reviewer. `TBD` не проходит readiness. Без новых вводных безопасный default — соответствующая capability UNAVAILABLE/UNSUPPORTED, а задача остаётся открытой; это временное поведение, не закрытие scope.

| Decision | Что необходимо получить/зафиксировать | Выходной артефакт и зависимые задачи |
|---|---|---|
| D01 MVP profile/capabilities | Exact Datecs/Daisy/BluePad/BlueCash model, firmware/SDK/payment app/region/currency, transports и required operations; каждый изначально заявленный пункт учтён | Versioned registry/profile manifest; P02/P17/P24/P25/P27/P36. Нельзя самостоятельно исключить обязательный профиль |
| D02 Vendor wire | Applicable command IDs, fields, encoding, LEN/BCC/SEQ/TLV/status, retry/duplicate/query semantics и independent bytes | Profile command table+golden fixture provenance; P17/P23…P26. Datasheet не закрывает wire contract |
| D03 Board/electrical | Exact S3 schematic/module revision, reserved pins/PSRAM/SD, RS232 TTL voltage/DTE/DCE/cable, USB host/VBUS/current/protection | Wiring+power manifest с источниками; P32/P39. Hardware facts подтверждает владелец стенда/производитель, не код |
| D04 Canonical database | Trusted BeeloyDB revision, complete dependency closure, exporter transformation, roles/grants/RLS applicability и actual deployment migration route | Reproducible source/SQL manifest, CI fixture profile и deployment dependency doc; P03/P04. Fixture не равна production migration |
| D05 MQTT/API identity | Adapter vs fiscal device vs terminal identity, versioned topics/DTO/errors, operation/source/fiscal IDs, binding generation и ACK preimage | Frozen contract+publisher/subscriber compatibility tests; P05/P19/P21. Existing tested contract переиспользовать |
| D06 Trusted UTC health | Конкретный authenticated upstream, доверенная error/health policy, monitor protocol и ownership, источник bounded uncertainty; operational acceptance его guarantees | AuthorityClock deployment profile + monitor/issuer rules; P09/P10. Не выбирать «здоровое время» по time.Now/SNTP или выдуманной uncertainty |
| D07 BLE bootstrap/TLS | Exact characteristic/endpoint/framing limits, handshake/client/device identity, standard TLS engine, certificate trust/renewal/revocation; ownership граница | Reviewed protocol/API/state diagrams+compat tests; P07/P09/P10. Не вводить произвольный custom crypto scheme |
| D08 Sale-entry/UNP/offline | Applicable нормативный момент ввода, source-first commercial ownership, readiness/UNP authority и durable offline reservation/import policy | ADR amendment+schemas/state/cases; P13/P14/P37. Отсутствие нормативного текста не означает исключение требования |
| D09 Financial reconciliation | Actual final statuses, authoritative NOT_SENT evidence, source/device conflict policy, exact per-tender reference/void/refund/query capabilities | Outcome mapping+recovery/ADMIN contract; P16/P18/P20/P21/P28. UNKNOWN не разрешает retry |
| D10 Electronic receipt | Device-generated format/acquisition command, immutable ownership/hash, legal recipient/delivery/retention/fallback requirements | End-to-end receipt contract+profile vectors; P29/P30/P37. Software-generated PDF не является substitute |
| D11 Platform/build matrix | Supported Android/iOS/BlueCash OS/API versions, native entrypoints, full bundle mode, signing/permissions/key custody, Expo/RN/Pods compatibility | Versioned build recipe+source/dependency material manifest; P11/P31/P33. Harness build не закрывает приложение |
| D12 External procedure | Current applicable НАП/Н18/Art118 baseline, procedure/actors, manufacturer/acquirer acceptance и exact evidence requirements | Dossier applicability/trace matrix с external reviewer, P37/P39. Не подменять документ подписанным собственным JSON |

Если противоречие относится только к одному profile, изолировать его и продолжать другие, но итог P38 не может автоматически объявить весь исходный MVP scope готовым. Scope change требует явного решения пользователя и обновления всех dependent contracts/tests/UI.

## 19. Outcome и state-machine правила: что именно проверять

Следующие названия — **концептуальные test states**, не новая product enum. Для каждого actual wire/API/Kotlin/C++/TS status сделать explicit mapping в D09. Не вводить эти строки в production DTO без contract review.

| Наблюдение | Что известно | Допустимые действия | Что запрещено |
|---|---|---|---|
| До admission, все границы доказывают отсутствие send | NOT_SENT, если это подтверждено actual transport/queue state | Исправить ввод; повторить по договорённой idempotency policy | Объявить NOT_SENT только по UI/network error |
| Queue приняла intent, но caller потерял ответ | ACCEPTED/POSSIBLE_SEND; исполнение могло начаться | Read-only lookup того же immutable operation; durable pending | Новый operation UUID/route ради «повтора» |
| Partial write или disconnect после send | UNKNOWN/POSSIBLE_SEND | Recovery/query по exact device/session/reference | Blind resend financial command или показ decline/success без proof |
| Terminal подтвердил approve, fiscal close неизвестен | PAYMENT_APPROVED + FISCAL_UNKNOWN | Сохранить each-tender evidence; documented recovery/compensation | Потерять approved tender, повторить purchase, отменить чужой receipt |
| Authoritative final rejection при доказанном отсутствии commit | FINAL_REJECTED в scope того же operation | Показать причину; разрешённые contract actions | Считать rejection одновременно committed либо переписать unrelated pending |
| Подтверждён genuine fiscal result | DEVICE_COMMITTED, затем source sync может быть pending | Сохранить original evidence до durable confirmation; idempotent materialization | Стереть единственный result при потере cloud, повторить чек |
| Source и device дают противоречащие final facts | CONFLICT/QUARANTINED | Authorized reconcile с фактами; immutable audit | Last response wins, автоматический rollback/новая финансовая операция |
| Неверная/чужая/старая подпись или owner | UNTRUSTED_PACKET, не финансовый исход | Отбросить packet; сохранить текущую pending/healthy authority по contract | Превратить packet в FAILED/success или очистить новую session |

**Обязательные свойства для всех финансовых cases:**

1. `logical operation UUID + digest + authority + UNP` неизменны при retry/route/restart; одинаковый UUID с другим payload — conflict до I/O.
2. Для каждого `payment_id` число irreversible approved debits `<=1`, кроме отдельной явно новой авторизованной операции; transport attempts и approved debit counters считать отдельно.
3. Для одного обычного sale intent число созданных fiscal receipts `<=1`. Storno/refund — отдельная linked operation, не «второй close» первого sale.
4. Для CASH сценария card debit count `0`. Для отказа до I/O все peer action counters `0`. Valid complete CASH имеет один receipt и matching amount/UNP/reference.
5. Для split каждый approved tender имеет собственный immutable evidence; selected tender не представляет все платежи. `sum(tenders)` и `sum(compensation)` сверяются в exact profile units, checked integer math.
6. Durable checkpoint precedes irreversible action; подтверждённый terminal state и outbox не расходятся. Commit uncertainty не превращается в NOT_COMMITTED по одному exception.
7. После restart проверять состояние **того же сохранённого peer**, не новой пустой модели. Device counter не сбрасывается перезапуском host.
8. Status/read/query/EJ recovery сами по себе не увеличивают debit/receipt counter. Если vendor query имеет side effect, это явно отражается в profile/fixture, не угадывается.
9. Authenticated result и authoritative source projection — разные этапы. Valid device result нельзя терять из-за unavailable source; неподтверждённая projection не разрешает финансовый повтор.
10. Смена владельца закрывает admission/publication старого request; stale error/cleanup не отбирает authority у нового owner. Сравнение только одинакового token/string/handle не доказывает отсутствие ABA.

## 20. Конкретные mandatory test blueprints

Каждый CT ниже развернуть в реальные named tests со своим `profile/ingress/client/fault_point` ID и записью expected state/counters/rows. Эта таблица — требование к будущему case inventory; она **не означает**, что тесты уже реализованы. Expected vendor bytes берутся из D02, не вычисляются тестируемым encoder.

| ID / владелец | Given → When | Обязательные assertions / negative control |
|---|---|---|
| CT01 / P03 | Чистый PG, текущие Fiscal fixtures без integration schema → actual authenticated CreateSystem | RED SQLSTATE42P01 воспроизводится. После exact canonical closure actual HTTP201; actual enrollment/projection/session tests выполняются. Пустая table-only stub без constraints/functions не проходит остальные cases |
| CT02 / P04 | Два tenant/actor и runtime DB role → foreign ID read/write/export/concurrent CAS | Wrong scope denied, чужие rows неизменны, correct owner succeeds. PG policy tests относятся к заявленным RLS tables; integration owner-only таблицы отдельно. Удаление tenant predicate/policy даёт RED |
| CT03 / P05 | BindingA/deviceA, зарегистрирован pending operation → valid result приходит от topic/deviceB | Projection/cursor/journal не меняются; тот же result на correct immutable association принимается once. Mutant с проверкой только operation UUID даёт RED |
| CT04 / P05,P19 | Actual broker повторяет command/result/businessACK после lost network reply | Повтор transport доставлен, но irreversible action count1 и materialized row1; ACK cursor только для exact signed batch/device/hash/seq. PUBACK-only mutant RED |
| CT05 / P06 | Trusted enrolled key и действующий package → package subject/key/generation/setup nonce изменены | До NVS mutation packet rejected, старая authority сохранена. Correct envelope commit/readback/proof succeeds. Не использовать advertisement public key как root |
| CT06 / P06,P09 | Actual installer: новый package, NVS commit applied → readback error | Install не объявлен verified; time authority boot-quarantined, pending/prepared invalidated; ordinary reopen/trusted_start_boot не снимает quarantine. Durable bytes не удалены ради fake rollback |
| CT07 / P07 | Connection handleN/genA послал intent; disconnect, handleN/genB → workerA завершился | Actual durable operationA завершена один раз; notifyA не поступает B. Правильный B reply работает. Mutant с current-connection fallback RED |
| CT08 / P07 | Реальный fragment stream → duplicates/out-of-order/max+1/hash mismatch/partial final | Никакой command admission до valid complete message; all buffers bounded/released; correct fragmentation succeeds. Одна isolated codec проверка недостаточна |
| CT09 / P08 | Queue accepted intent, worker blocked на deterministic barrier → caller deadline | Caller выходит bounded; no stack/UAF; pending остаётся UNKNOWN. После release worker получает один durable final result; no duplicate execution. ASan/UBSan + deadline assertion |
| CT10 / P09 | Cold boot без verified anchor; SNTP/NVS/mobile присылают время | Provider UNAVAILABLE; purpose proof не выпускается. После actual root nonce-bound response от healthy source interval valid; wallclock-only mutant RED |
| CT11 / P09 | Pause после actual root verification → binding/owner A→B→A либо failed commit → release | Late signed response не adopts authority; replacement pending/healthy owner не повреждается. Same-address coordinator placement-new и static-exit отдельные cases |
| CT12 / P09,P10 | Actual proof admitted с valid interval → signing/transport delay пересёк expiry/health epoch | Publication отвергнута, включая cached proof; ошибка не подменяет interval midpoint. Exact lower/upper boundaries сверяются независимым rational oracle |
| CT13 / P10 | Full real TLS peers → wrong CA/name/SPKI/device/expiry/downgrade/replay | No authenticated operational session/result; correct profile succeeds. Signature-valid command на OPEN channel не закрывает этот test |
| CT14 / P11 | Request captured sessionA → loginB → delayed401A; затем current401B | A не logout B/не retry; B отзывает только B. Token ABA/Header override и interrupted React admin render проверяются actual caller |
| CT15 / P12 | Signed intent с mismatch total/tax/discount/quantity/currency/actor/binding → admission | Ни payment, ни fiscal I/O; device counters0, no false committed rows. Valid neighbouring boundary vector succeeds. Checked overflow mutant RED |
| CT16 / P13 | First regulated sale input при not-ready/UNP unavailable → add/scan/change line | Поведение соответствует frozen D08 до regulated boundary; UI не создаёт произвольный UNP. Valid allocation после lost response/restart сохраняет identity, concurrent actors не делят один sequence |
| CT17 / P14 | Полностью отключён source HTTP, fresh app с valid offline authority+S3 → новый cart/checkout/kill/restart/import twice | Draft/UNP/tenders/outcome survive; receipt/debit counters1 where applicable, source order1. Existing-order cached checkout не засчитывается как fresh-cart case |
| CT18 / P15,P16 | Device выполнил close/debit; process crash до result/outbox commit → restart с тем же peer state | Recovery queries original identity, no second debit/receipt; terminal projection/outbox converge atomically; foreign open receipt не cancel. Mutant сбрасывает model state → gate отвергает fixture |
| CT19 / P17,P18 | CARD1 approved, CARD2 unknown/declined, fiscal failure → recovery/compensation | Evidence обоих tenders раздельно; exact amount/reference; documented compensation count<=1 each, unsupported capability unresolved. Aggregate-reference-only mutant RED |
| CT20 / P20 | Actual C++ producer выпустил maximum escaped/int64 result → Go+real TS/Kotlin projection | Exact bytes/semantic fields/status survive, no truncation/cross-owner result. All required12 transport vectors и both named client cases executed, not skipped |
| CT21 / P21,P22 | Device committed, source import ACK lost → UI reload/lookup/route switch | No financial resend; journal retains positive evidence; authoritative matching GET permits cleanup. Foreign404/body error не доказывает NOT_SENT; locally forged READ permission не grants backend action |
| CT22 / P24,P25,P26 | USB/UART/BLE peer принимает реальные commands → corrupt length/status/NAK/wake/completion/partial write | Exact sequence/profile payload/checksum checked independently; wrong command немедленно FAIL; no canned execute success. Production encode mutation RED |
| CT23 / P29,P30 | Genuine receipt artifact привязан sale/device/hash → delivery lost/restart/cross-tenant retrieval | Original immutable content survives, delivery idempotent, unauthorized access denied, fallback не создаёт sale2. Software summary как fiscal artifact отвергается |
| CT24 / P27,P33,P38 | Enabled profile/requiredcase/artifact отсутствует либо bundle сам приносит trust key → final verify | NOT_READY с exact IDs; foreign/stale report/scan/hash/root rejected. Zero tests или echo PASS не дают readiness, даже если command exit0 |

**Для каждого fault CT добавить контроль:** до injection был валидный working fixture; injected fault действительно сработал (`fault_hit=1`); named assertion выполнена; clearing injection возвращает positive case. Иначе test может быть зелёным потому, что не дошёл до проверяемого участка.

## 21. Execution recipes, окружение и обработка ошибок

### 21.1. Обязательный recipe перед запуском

Для каждой Pxx.n сохранить: exact `cwd`, argv (массив, без shell concatenation секретов), required env **names**, resolved versions, source/dependency hashes, owned paths/services, timeout, expected named cases, expected exit/result files, cleanup и command для повторной проверки. Не копировать private credentials в report. Synthetic fixtures помечать как disposable test-only.

| Контур | Где взять реальный recipe/inputs | Что проверить до запуска |
|---|---|---|
| Backend Go | Makefile, fiscal-backend/go.mod/go.sum, minipos/beeminipos-backend/go.mod/go.sum | Exact current Go module/toolchain, working directory; external DB env и actual tests selection; -count=1 где cache мог скрыть re-execution |
| PostgreSQL/Compose | .github/workflows/integration-contract.yml; scripts/prepare-fiscal-pg-fixture.py; compose.fiscalisation{,.dev,.e2e,.integration-ci}.yaml; verify-integration-ci-topology.py | Exact same COMPOSE_FILE/COMPOSE_PROJECT_NAME для up/exec/down; SQL hash manifest; DSN credentials match; canonical integration dependency; no чужих volumes |
| Full S3 | idf/tools/build.lock.json, build_idf.py, .github/workflows/s3-idf.yml | Official clean SDK/submodules/compiler/USB component; IDF_PATH/IDF_TOOLS_PATH/IDF_PYTHON_ENV_PATH; новый empty build dir; no flash target |
| Actual crypto | scripts/run-ble-certificate-crypto.py; verify-device-time-anchor-crypto-host.sh; verify-ble-certificate-proof-host.sh | Реальные pinned mbedTLS/cJSON и библиотеки той же сборки, required markers, host macros изолированы от production; timeout из runner |
| BLE client/CBOR | tests/ble-transport/{README.md,run.py,package-lock.json,check_lock.py} | Node22.14.0/npm10.9.2 actual, empty private npm cache/config, exact11source copies, real native producer; 66baseline cases плюс новые mandatory IDs |
| BlueCash/Daisy Smart | SmartDevices/gradlew, gradle/wrapper, settings/build files и actual variant | Собственный wrapper, exact JDK/SDK/AGP; task-local GRADLE_USER_HOME/SDK copy; variant/signature/vendor JAR hashes |
| Full Expo | minipos/BeeMiniPOS и BeeFiscalApp package/locks/app config; native-mvp scripts после review | Actual AppEntry/App.tsx, all local shared modules tracked, current native/Pod lock compatibility, JS bundle inside artifact; HOME не переопределяется |
| Evidence/release | existing evidence/verifier tests и Make recipes; release source/material manifest | Внешний trusted verifier key; exact digest-bound reports; safe artifact allowlist, не весь /tmp каталог |

Большой список команд раздела11 — каталог gates, **не одна paste-and-run shell команда** и не разрешение выполнять `upload/flash/deploy` targets. Разбить на recipes и prerequisites. Если pipe используется для логов, результат всей проверки должен отражать первый failed процесс; использовать корректный `pipefail` либо явную проверку всех returncodes, а не успешного tee/последней команды.

### 21.2. Failure taxonomy и следующий шаг

| Failure | Что делать | Что не делать |
|---|---|---|
| PRODUCT_RED | Найти exact invariant/caller; минимальный fix; target+dependent regression | Менять expected result, чтобы принять баг |
| FIXTURE_RED | Проверить applicable schema/profile/source/hash; исправить fixture по trusted source; сохранить прежний RED | Добавлять permissive table/mock или fake vendor response |
| ENVIRONMENT_MISSING | Зафиксировать недостающий dependency/version/permission; подготовить task-local среду по recipe | Использовать случайный global SDK либо объявлять SKIP успешным |
| RESOURCE_FAILURE | Подтвердить OOM/disk/process error; сохранить log hash; дождаться/управляемо завершить owned process; bounded retry с обоснованным budget | Выключать lint/test, запускать второй competing build пока первый live |
| FLAKY/SCHEDULING | Найти незафиксированный event/barrier; ждать actual admission/completion; repeat same named case с recorded schedule | Увеличивать произвольный sleep или rerun до случайного green |
| BLOCKED_SPEC | Записать D-ID, источники/конфликт, минимальный вопрос, dependent tasks | Выдумывать command/time/UNP/legal policy |
| LIVE_WAIT | Poll exact confirmed process/run handle с bounded timeout; сохранить progression | Restart только из-за истечения observation timeout |
| EXTERNAL_PENDING | Зафиксировать hardware/legal/acquirer evidence и owner/получение | Сгенерировать «акт» программным тестом |

Две одинаковые неудачные попытки без нового диагностического evidence — сигнал остановить слепой retry, классифицировать причину и пересмотреть подход; это не автоматическое объявление всей цели blocked. Закрывать процесс/ресурс можно только по его фактическому state и ownership. Нельзя запускать `down -v`, `rm -rf`, reset/clean по глобальному проекту/корню.

## 22. Формальные contracts для новых test orchestrators/emulators

Это требования к будущей реализации P27/P38; сейчас новых schemas/entrypoints этим текстом не создаётся. Их отсутствие остаётся задачей, не assumed PASS.

### 22.1. Profile и scenario

Profile должен содержать: `profile_id/version`, vendor/model/hardware/firmware/protocol applicability, exact transport/config/UUID/VIDPID/interface, encoding/currency/unit rules, capabilities/unsupported operations, authoritative source hashes/pages, linked production module IDs, emulator plugin ID/version, independent golden fixture IDs, mandatory scenario IDs и declared physical-only properties.

Scenario должен содержать: stable ID, requirement/P/CT IDs, profile/build ID, input hash, initial host+peer state, clock/seed/schedule, ingress/client, fault point+expected fault_hit, expected TX/RX/allowed command automaton, irreversible action counters, durable state/outbox/source assertions, expected outcome **и** permitted unresolved classification. End-of-test проверять, что все ожидаемые peer commands/assertions действительно consumed; trailing unexpected TX не игнорировать.

Emulator plugin обязан поддерживать:

- Byte-stream input и независимый parser, не production codec в expected oracle.
- Stateful receipt/payment identity и separate attempt/approved/fiscal/compensation counters.
- Persist/restore **peer state** независимо от host/SQLite restart; snapshot format versioned/hash-bound.
- Explicit fault injection hooks с exact boundary и `fault_hit` counters.
- Список поддержанных commands/transitions и отказ на неожиданный command/order.
- Reviewed source/profile applicability и доказательство, что новые golden bytes не вычислены тестируемым encoder.

### 22.2. Report и coverage equality

Предлагаемый report содержит как минимум: schema_version, run_id, task/requirement/case/profile IDs, source/config/dependency hashes, actual entrypoints/linked units, command/tool versions, assertion IDs, executed/passed/failed/skipped, fault_hit/mutation results, seed/schedule, peer counters, artifact digests и scope `HOST_ACTUAL_RUNTIME`/`TARGET_BUILD`/`OS_INSTRUMENTATION`/`EMULATED_RUNTIME`/`PHYSICAL_HIL`. Каждый scope проверяется отдельно.

Финальная проверка вычисляет expected required cases из frozen inventory и enabled profiles. Требование: **множество executed required IDs равно expected required IDs**, без дубликатов/unknown/missing, и каждый обязательный assertion проверен. Total passed>=66 само по себе не доказывает, что новые обязательные cases не исчезли; существующий66-floor сохраняет compatibility baseline, P38 должен сравнивать exact required IDs текущего candidate.

`pre-hil-software-gate`: запускает реальные mandatory recipes, сохраняет все exitcodes и report hashes; error не поглощается. `pre-hil-evidence-verify`: независимо валидирует snapshot/trust/cases/assertions/profile/artifact linkage и выдаёт NOT_READY при любом missing proof. Orchestrator не вправе сам написать missing report=PASS. Фиктивный report/удалённый required case/подменённый profile/missing artifact/unknown schema/self-provided key должны ломать verifier в его собственных тестах.

## 23. Boundary values, numerical budgets и повторная проверка

### 23.1. Числа брать из frozen applicable contract

Для каждого limit проверить `min-1,min,min+1,max-1,max,max+1`, пустое/нулевое/непредставимое значение и checked conversion между units. Где один byte/секунда/копейка меняет predicate, добавить explicit regression с независимым expected result. Для expiry указать inclusive/exclusive semantics.

Известные исходные ориентиры, которые перед реализацией перепроверить по source:

- Actual typed CBOR input/output budget8192bytes и depth16: это предел протокола/реализации, не monetary maximum. INT64_MIN/MAX и2^53±1 проверяются losslessly; business cJSON safe53 guard не снимается глобально.
- Accepted time contract60: exact17 request/22response fields, exact4-key envelope, max4096bytes envelope; криптографические domains/formats и canonical JSON из contract60 не заменяются RFC8785/float map.
- TimeAnchorPolicy на baseline: max_rtt5000ms, holdover30000ms, uncertainty250ms, width6000ms, drift1000ppm; history128. Это engineering/source defaults, не измеренные гарантии физического oscillator и не server UTC SLA. Consumer inclusive interval/exclusive expiry проверяется независимо.
- Реальное BLE fragment allowance определяется negotiated payload minus actual header; iOS limit получается из runtime API, а не Android constant. Vendor terminal chunking определяется конкретным profile.
- Денежные bounds/precision/currency задаются vendor+product contract; полные int64 transport test values **не** разрешают такую сумму покупки.
- Resource capacity и mandatory soak budgets фиксируются до P35 acceptance. Если board budget/accepted workload неизвестен — явно pending measurement/profile decision, не «unlimited».

### 23.2. Impact/retest matrix

| Изменение | Минимальная зависимая перепроверка |
|---|---|
| Protocol field/codec/money/encoding | Independent golden+malformed+all applicable driver consumers, stateful monetary/recovery, client result projection |
| Ownership/queue/async callback | Actual deterministic race/ABA/deadline, ASan/UBSan или race для соответствующего языка, caller abort/reconnect/recovery; no duplicate actions |
| Identity/NVS/binding/time | Actual key create/reload/readback, installers/quarantine, crypto/lifecycle/interval, default unavailable, inherited certificate proof, full S3 target |
| DB schema/roles/persistence | Fresh/upgrade PG, constraints/rights/RLS applicability, uncertain commits, enrollment/order/projection/recovery, export/backup restore |
| DTO/HTTP/topic | Actual sender/receiver across Go/C++/Kotlin/TS, generated contracts, broker/API E2E, negative auth/scope/idempotency |
| App auth/UI/result lifecycle | Actual App/caller tests, all relevant current45 cases, browser/native lifecycle and full affected artifact |
| Dependencies/toolchain/build config | Lock/material origin checks, clean install, full affected build/tests, source/artifact/provenance verification; no reuse old binary |
| Emulator/fixtures/runner/verifier | Independent oracle provenance, mutation controls, fault_hit, case discovery/exact inventory equality, all affected profiles and gate negative tests |
| Documentation только | Coverage/link/command/decision consistency; no claim that documentation repairs product behavior |

После любого dependent source/config/lock change прежний тестовый отчёт остаётся историческим. Его можно переиспользовать только если verifier доказывает неизменность полной relevant input closure и scope явно совпадает; простой одинаковый top-level HEAD/filename/count недостаточен. Новый firmware/client build включает source materials того же candidate; test-only изменения не дают права подписать старый artifact как новую сборку.

## 24. Межоперационные gaps, которые легко забыть

Внести в P01 обязательные cases для этих связок, даже если узкие unit suites уже зелёные:

1. Source order→Fiscal preparation→S3 admission→each tender→fiscal close→durable result→source materialization→journal cleanup: trace identity/amount/UNP на каждом переходе.
2. Одновременный MQTT и BLE retry **того же** intent: serialized idempotent reservation, no second debit/receipt. Другой payload того же UUID — conflict; новый UUID не является retry.
3. Auth/config/binding/time-health смена после precheck и до sign/commit/publication: atomic ownership confirmation, а не «ещё один GET после операции».
4. Actual device approved, client/backend offline: immutable evidence не теряется; новый offline cart использует новую identity по D08 и не переписывает pending предыдущего.
5. Outbox/event order и fiscal/source projections при duplicate/delay/restart: terminal facts не регрессируют от старого сообщения; конфликт не скрывается last-write-wins.
6. Reversal/compensation/EJ/status используют original scoped identities; unauthorized caller не превращает recovery endpoint в write/payment oracle.
7. Fresh schema и legacy upgrade дают одинаковые constraints/ownership semantics; CI-only transformations не выдаются за canonical production deployment.
8. App source-built feature действительно присутствует в APK/IPA/firmware и вызывается UI/ingress; linked helper без caller не закрывает requirement.
9. Release default не включает host test roots, injected clocks, emulator publisher, placeholder backend, permissive transport или synthetic receipt reference.
10. Нормативный/electronic receipt/first-entry/offline contract отражён в UI/API/storage/export совместно. Закрытие одного anchor не отменяет другой требуемый lifecycle.

## 25. Независимая приёмка и ответ на вопрос об автономности

Перед P38 reviewer получает не только итоговый PASS, а frozen source/material manifest, requirement/case inventory, выбранные D01…D12 decisions, actual reports и binaries. Проверяет:

1. Required behavior подтверждён applicable source/approved contract; нет игнорируемого conflict/TBD.
2. Actual callers всех включённых профилей используют именно tested code; native/firmware артефакты соответствуют source/config/deps и expected entrypoints.
3. Required tests реально executed, named assertions и fault_hit присутствуют; mandatory skip/missing не скрыт общим passing count.
4. RED reproduced→fix→negative/mutant→positive restoration доказаны; oracle независим, harness не подменяет execute/payment/fiscal success.
5. Money/authority/durability invariants раздела19 сохраняются в CT cases и всей applicable matrix, включая crash/restart/possible-send.
6. External trust anchor не взят из проверяемого bundle; source/artifact/report/scan hashes совпадают, unsafe logs/keys не опубликованы.
7. Все software requirements исходного scope закрыты либо остаются NOT_READY с exact IDs; нет самовольного scope reduction.
8. HIL/electrical/entropy/drift/native-radio/vendor/acquirer/legal внешние результаты отделены и имеют owner/procedure/evidence requirement.

Результат ревью — profile-specific `PRE_HIL_SOFTWARE_READY` или `NOT_READY` со списком blockers. Если reviewer обнаружил missing case/невалидный contract, задача возвращается к соответствующей Pxx.n, а не получает waiver формулировкой «для MVP достаточно».

**Практический предел:** после этих дополнений план задаёт полный маршрут, шаблоны inputs/outputs, тестовые оракулы и правила приёмки для самостоятельной работы модели. Но гарантировать, что слабая модель без проверенных D01…D12 inputs и независимой проверки сама закроет *все* ошибки, невозможно. Она должна уметь остановить зависимую задачу на отсутствии фактов и продолжать готовые. Реальное оборудование и внешняя применимость не становятся software-complete от подробности документа.
