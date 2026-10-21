# Полный план закрытия разрывов перед MVP испытаниями с реальными устройствами

Дата: **2026-10-21, Europe/Sofia**. Корень: `/Users/freelancer/Documents/Beeloy/Fiscalisation`.
Baseline исходников на момент составления: **394ef67dc8f95ed317d24652e6802e24bd681be2**. Перед исполнением обязательно обновить baseline по P00: работа и сборки продолжаются.

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

Приоритет: actual current source+runtime evidence → точный vendor/regulatory primary source → применимый approved contract/ADR → историческая заметка. Конфликт не решать угадыванием. Создать `BLOCKED_SPEC` с двумя противоречащими ссылками, минимальным контрпримером и конкретным вопросом. Самостоятельно продолжать независимые задачи.

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

Выполнять **одну карточку Pxx за раз** в отдельном scoped commit. Не брать зависимую карточку, пока её prerequisites не имеют достаточного evidence. Внутри большой карточки сделать подзадачи `.1/.2/.3`, каждая не более одного инварианта/одного root cause.

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
- Немедленно параллельно P03/P04 (DB), P06–P10 (S3/security), P23–P27 (protocol/emulator), P31 (native artifacts), P37 (актуальная нормативная база).
- Затем P12–P22: money/UNP/offline/durability/recovery/results/UI. Общие profile/authority contracts фиксируются до параллельной реализации.
- P28–P36: весь объявленный operation scope, electronic receipts, audit/export, factory/release/security/resource coverage. Нормативные зависимости не угадывать.
- P38: окончательная проверка на exact integrated snapshot. P39: готовый runbook для первого физического подключения и полного MVP HIL.

Если карточка не применима к конкретному профилю, требуется profile-specific доказательство `UNSUPPORTED` и тест отказа до I/O. Это не разрешает вычеркнуть задачу из всего продукта.

## 6. Карточки исполнения

### P00 — Зафиксировать фактический baseline и не потерять чужие изменения

**Зависимости:** нет. **Трассировка:** GAP-028/030; T-15/22/23.

**Где читать/менять:** `git status; Makefile; TOOLCHAIN.md; .github/workflows; docs/remediation/2026-10-17`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `02_GAP_REGISTER.md; 05_TESTS_AND_ACCEPTANCE.md; contracts/*.json; docs/SUPTO; этот план`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `03_DEVICE_PROTOCOLS.md; IoT/protocol-abstraction; IoT/common-drivers; SmartDevices; idf/main/provisioned_binding.cpp`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `database/fiscal; scripts/prepare-fiscal-pg-fixture.py; fiscal-backend/internal/integration; compose.fiscalisation.integration-ci.yaml; .github/workflows/integration-contract.yml`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `database/fiscal; database/minipos; fiscal-backend/internal/persistence; minipos/beeminipos-backend; scripts/e2e-two-compose.sh`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `fiscal-backend/internal/mqttclient; fiscal-backend/internal/domain/admin.go; idf/main/mqtt_runtime.cpp; SmartDevices BlueCashMqtt.kt; broker config`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `IoT/device-manufacturing; idf/main/device_identity.cpp; encrypted_provisioning.cpp; provisioned_binding.cpp; BeeFiscalApp/src/provisioningProtocol.ts; smartDeviceBle*`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/ble_runtime.cpp; ble_frame_layout.h; command_queue.*; minipos/BeeMiniPOS/src/webBle.ts; browserGatt.ts; nativeBleTransport.ts; miniposweb/src/fiscal`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/command_queue.cpp; command_queue.h; actual HTTP/BLE ingress; runtime host harness`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs60/63/66/68; idf/main/device_time_anchor.*; time_anchor_core.*; app_main.cpp; backend domain; новые runtime/monitor adapters`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `IoT/common-modules/ble-stream; idf/main/ble_certificate_proof.*; ble_runtime.cpp; backend ble_server*; actual native/browser transports`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `BeeFiscalApp/src; minipos/BeeMiniPOS/src/requestAuthFence.ts; nativeExecutionManager.ts; nativeExecutionAuthority.ts; App.tsx; PWA routes`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/intent_processor.cpp; execution_authorization.cpp; receipt_saga.*; backend prepare/dispatch; nativeExecutionAuthority.ts`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs43; docs/decisions/ADR-SUPTO-001-thin-pos.md; MiniPOS App.add; source order service; Fiscal sale preparation; S3 UNP allocator`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `MiniPOS App.add; sourceOrders.ts; checkoutJournal/outbox; native execution manager; MiniPOS backend import`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/durable_storage.*; receipt_saga.*; SmartDevices SQLite storage; source/Fiscal outbox`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/receipt_saga.cpp; IntentProcessor::recover_pending; recovery snapshot; terminal guard; Fiscal repository recovery`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/bluepad_ble_central.*; IoT/common-drivers datecs-pay; SmartDevices Datecs pinpad bridge; vendor SDK fixtures`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `receipt_saga.*; reversal executor; payment checkpoints; backend payment/reconciliation; MiniPOS split UI`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/go_json.cpp; go_wire.cpp; sync_runtime.*; backend sync/ACK validators; BlueCash sync`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/canonical_cbor.cpp; ble_result_delivery.cpp; intent_processor.cpp; MiniPOS cbor/bleResult; BlueCash result serialization; source order GET`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `MiniPOS backend orders/operations; fiscal-backend domain; integration worker/Rabbit; public APIs`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `contracts/ui-acceptance-matrix.json; BeeFiscalApp/App.tsx; minipos/BeeMiniPOS/App.tsx; miniposweb; actual browser/native tests`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/uart_runtime.*; IoT/common-drivers; IoT/protocol-abstraction; SmartDevices fiscal driver`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf/main/usb_cdc_runtime.cpp; Daisy runtime driver/codec; USB CDC managed component; Daisy fixtures`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `idf UART Datecs runtime; common datecs driver; SmartDevices bluecash-app/shared-sdk; Datecs Binder SDK`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `Предлагаемые новые tests/iot-device-emulation/{transports,devices,fixtures,scenarios}; existing idf/tests/*_host; Makefile`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `Предлагаемый versioned device registry/schema; driver capability inventory; CI; Makefile`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `Fiscal domain; MiniPOS shift/reversal screens; ReceiptExecutor; vendor profiles; report projections`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs30; device capability profile; Fiscal artifact/delivery/outbox API; MiniPOS/Fiscal UI; export`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs17…29/35/37; Fiscal/BlueCash/S3 export; SUPTO dictionaries; immutable audit; release dossier`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `BeeFiscalApp; minipos/BeeMiniPOS; SmartDevices; native-mvp prepare/build scripts; Gradle/Pod locks; docs41/69`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `IoT/device-manufacturing; idf partitions/build tools; edge-agent-s3/README.md; factory runbook`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs31; release manifest/SBOM/verifiers; idf/tools/build_idf.py; CI artifacts`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `backend auth/API; broker/mTLS; BLE bootstrap/TLS; mobile key stores; SQLite/export; release tooling`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `S3 queue/buffers/SQLite/NVS/SD; clients outbox; broker/backend PG; native memory; soak runners`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `Tremol BG profile; Daisy Smart app; Eltrade/Wiseasy docs; SmartDevices capabilities/UI`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `docs/SUPTO; docs09/16/30/49; contracts/bg-requirements-trace.json; primary source register`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `Makefile; current CI; requirement/case inventory; release/evidence manifest`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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

**Где читать/менять:** `HIL runbook/profile matrix; device inventory; factory manifests; report templates; safe read-only probes`. Точные существующие имена проверить через `rg --files`/`rg`, не создавать второй параллельный runtime.

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
Прочитай разделы1–5 и карточку Pxx документа
 docs/remediation/2026-10-17/70_FULL_PRE_HIL_REMEDIATION_PLAN_FOR_SMALL_MODEL.md.
Затем прочитай перечисленные в Pxx actual source/caller/test files.
Выполни только первую незакрытую подкарточку Pxx.n.

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
