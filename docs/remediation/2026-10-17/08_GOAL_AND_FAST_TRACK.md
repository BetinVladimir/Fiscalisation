# Цель и ускоренная стратегия достижения

## Уточнение текущего контура

По прямому запросу пользователя отдельный Go-модуль `edge-agent/` удалён из репозитория. Его команды, генераторы и ссылки на тестовые evidence исключаются из действующего контура. Исторические результаты не подтверждают готовность S3. Текущий IoT runtime — `IoT/firmware/edge-agent-s3/idf`; обязательны actual-runtime тесты и сборка этой прошивки. Go oracle в `idf/tests/go_wire_host` использует `fiscal-backend/internal`, а не удалённый модуль, и остаётся применимым. Недостающие эквивалентные проверки S3 остаются OPEN до реализации, а не закрываются удалением reference tests.

## Цель

Закрыть все выявленные разрывы Fiscalisation/MiniPOS, исправить обнаруженные ошибки, покрыть изменения автоматическими тестами, выполнить тестирование и устранить регрессии; подготовить систему, воспроизводимые сборки и evidence к испытаниям с реальными USB/COM/BLE устройствами; подготовить MiniPOS и связанные компоненты к применимой процедуре НАП для СУПТО. Новые дефекты, обнаруженные во время реализации и тестов, также входят в backlog до достижения acceptance criteria. Нельзя считать «все разрывы закрыты» только по исчерпанию исходного списка из34 записей.

Согласованная последовательность сохраняется: baseline → эмуляторы/red tests → CASH → CARD/split/recovery → полная программная проверка и сборки → HIL. Регуляторный пакет готовится с начала работ, не после окончания программирования.

## Самый быстрый обоснованный путь

**Короткий общий контракт → три независимых потока → ранний вертикальный CASH → CARD/split → единый final-snapshot gate.** Ускорение достигается разделением владения кодом, ранним воспроизведением дефектов, переиспользованием независимых protocol fixtures и выборочными relevant tests. Безопасность денежных операций и достоверность evidence не сокращаются.

Не ждать реализации всего W9: сначала минимальный actual-runtime harness для Datecs COM, достаточный воспроизвести codec/command/error/recovery. Затем добавить Daisy USB и BluePad BLE в общий registry. Codec golden tests, MQTT namespace tests, MiniPOS UNKNOWN regression, SQLite atomicity и Kotlin vectors начинают приносить результат до полного стенда. Host harness использует actual runtime modules; отдельный production cross-build подтверждает target/linkage, но сам по себе не доказывает поведение ESP32 binary.

До фиксации точных ADR разрешены baseline, independent fixtures применимых опубликованных протоколов, toolchain preflight и воспроизведение ошибок. Неоднозначные wire/authority/payment решения зависимые агенты не выбирают независимо; integrator закрепляет их в A2. Неизвестный BluePad profile блокирует CARD, но не останавливает CASH.

## Распараллеливание и владение

До четырёх активных участников одновременно: **координатор-интегратор + три рабочих потока**. Пользователь явно разрешил параллельную работу. Все задачи сохраняют единый backlog/контракт и общие критерии; агенты не создают самостоятельные несовместимые «версии решения».

| Поток | Исключительная область изменений | Что стартует независимо | Интеграционные зависимости |
|---|---|---|---|
| L1 — protocols и device emulation | `IoT/protocol-abstraction/**`, `IoT/common-modules/**`, profile executor/device central files IDF, device models и transport mocks W9 | Datecs/Daisy vendor vectors, codec failures/fixes, command parsers, minimal COM harness, затем USB/BLE models | Typed DeviceIo/ExecutionResult/PaymentOutcome contract с L2; exact vendor spec для CARD |
| L2 — IoT durability/runtime | Остальные IDF runtime files: intent processor, durable storage, sync, MQTT/BLE ingress, provisioning; `IoT/device-manufacturing/**` | Atomic projection+outbox, failpoints, frozen batches/ACK, UNP recovery snapshot и software bootstrap | Контракты L1 и MQTT/identity A2; actual stateful device для crash/reconcile |
| L3 — services, POS и BlueCash | `fiscal-backend/**`, `minipos/**`, `BeeFiscalApp/**`, `AdminApp/**`, `SmartDevices/**` | MQTT topic/ID regressions, три backend failures, UNKNOWN route persistence, Kotlin Datecs vectors | Shared wire/sync/lookup semantics L2; отдельное Android evidence |
| I — интегратор / независимая приёмка | `contracts/**`, shared registry/schema, root `Makefile`, gate scripts, docs/status/evidence; общие interfaces до передачи owner | Baseline, A2 решения, CI manifest, актуальная SUPTO traceability и draft пакета НАП | Принимает небольшие patches и запускает merged-path gates; статус READY выдаёт только по общему snapshot |

Перед каждым coding dispatch закрепить exact file ownership, включая shared headers, CMake и registry entries: один файл — один writer. Новые эмуляторы L1 регистрируются через integrator-owned registry patch. Поток L3 большой: ранний приоритет backend/POS safety и Kotlin codec, затем onboarding/admin; косметические изменения не задерживают CASH. При достижении milestones перераспределять освободившиеся слоты на CARD, mobile lifecycle и независимое review, а не открывать произвольное число конфликтующих задач.

Для изоляции можно применять отдельные task branches/worktrees, если подготовка не теряет dirty baseline. Если участники работают в общем checkout, изменения файлов строго раздельны; запрещены reset/cleanup чужих изменений. Build dirs, SQLite DB, broker namespaces, service ports и artifact paths отдельные на task/run; никакой общей `/tmp` БД или shared executable output, перезаписываемой двумя suites.

## Волны и агрегирование

| Волна | Одновременная работа | Общая контрольная точка |
|---|---|---|
| V0 / M0 | I фиксирует source/decisions/profile scope и regulatory inventory; L1 — golden red tests; L2 — storage/sync repro; L3 — backend/UNKNOWN/Kotlin repro | B0: один identity/topic/UNP/UNKNOWN contract; красный baseline сохранён, fixtures независимы |
| V1 / M1 | L1 строит minimal Datecs harness и codecs; L2 закрывает checkpoint/outbox/sync; L3 стыкует MQTT и исправляет POS/backend failures; I создаёт case manifest/gates | B1: actual runtime + golden oracle; corrupt LEN/topic/checkpoint делает suite красной |
| V2 / M2 | L1 добавляет Daisy USB и protocol coverage; L2 закрывает cash recovery/bootstrap; L3 замыкает sale/domain/WebHook/POS; I принимает обе CASH трассы | B2: MQTT/direct BLE → actual S3 runtime → Datecs/Daisy emulator → journal/sync → backend/POS, включая restart/lost reply |
| V3 / M3 | L1 BluePad BLE; L2 per-tender recovery/compensation; L3 BlueCash Android card/split и client lifecycle; I дополняет regulatory cases/build manifests | B3: approve→lost reply→restart, второй tender fail, late notify, compensation/reconcile; 0 duplicate debit |
| V4 / M4 | Параллельно cross-builds и изолированные service/UI suites; независимый reviewer проверяет golden/mutations/evidence | B4/B5: no-mock production linkage + полный gate на одном объединённом source snapshot |
| V5 | HIL по готовым profiles и сверка final Annex29/НАП пакета; новые findings возвращаются владельцу с regression test | PHYSICAL_HIL evidence и REGULATORY_PACKAGE_READY оцениваются отдельно |

Это зависимости результатов, а не требование остановить всё до окончания каждой волны. Независимые package tests выполняются сразу; переход в общий READY зависит от применимых барьеров. Нельзя объявлять готовность всего продукта по CASH milestone. CASH recovery, UNKNOWN и durable outbox обязательны уже в V2; откладывается только дополнительная карточная сложность.

## Три уровня gates

- **Change gate:** после каждого небольшого исправления — воспроизводящий тест, профильные unit/wire/negative tests и затронутые type/contract checks. Сначала красный контрпример, затем исправление.
- **Lane integration gate:** перед приёмкой пакета — actual runtime и независимый emulator; настоящий broker/PostgreSQL/SQLite по применимому пути; cross-language vectors, failpoints и restart.
- **Final pre-HIL gate:** на объединённом фиксированном source snapshot — все required HOST/SERVICE_INTEGRATION/CROSS_BUILD cases, clean artifacts, mutation controls и независимая проверка manifest. Изменение snapshot требует повторной применимой проверки и итогового gate.

Не запускать весь широкий regression после каждой правки. Дедупликация одинаковой проверки допускается только по command/env/toolchain/source fingerprint и неизменным inputs; ссылки на требования сохраняются. Финальный acceptance-run не заменяется cached результатами старого среза. PASS отдельных потоков не складывается автоматически в общий PASS.

## Контракт передачи результата от агента

Один пакет содержит: task/GAP/requirement IDs; touched files; source baseline/result hashes; воспроизводящий case; решение и contract effects; commands/cwd/env/toolchain/exit codes; case/profile/transport/build inventory; wire traces и счётчики receipt/payment actions; DB/outbox/cursor assertions; artifact hashes; remaining risks/blockers. Неподтверждённый факт маркируется как риск, внешний dependency — как BLOCKED_SPEC/DEPENDENCY.

Интегратор проверяет diff, совместимость contracts, сохранность пользовательских изменений, затем применяет пакет и запускает стыковые tests. Shared source changes принимаются в одном порядке; агент не правит owned файл другого потока, а передаёт запрос/patch владельцу. Новые дефекты получают стабильные IDs и required tests. Failures integration приоритетнее новых capabilities; повторный прогон включает контрпример, не только зелёные старые suites.

## SUPTO идёт параллельной дорожкой

Подготовка раскрыта в [09_SUPTO_NAP_PREPARATION.md](09_SUPTO_NAP_PREPARATION.md). С первого изменения поддерживать trace `норма/версия → компонент → case → artifact`, описания модулей/DB/каналов и Bulgarian operator/auditor documentation. Не переписывать документацию о реализации раньше появления evidence. Отдельно проверить регуляторную допустимость offline/authority модели до её закрепления как SUPTO-compliant.

Выполнены два независимых анализа стратегии/gates и агрегированы в этот документ. После разрешения параллельной работы запущена первая изолированная волна:

- L1: actual C++ FrameCodec Datecs/Daisy, независимые vendor vectors, негативные и граничные tests. Получен patch: baseline воспроизводит 6 ошибок; исправленный код проходит suite и ASan/UBSan, тесты обнаруживают 6 намеренных регрессий. Kotlin остаётся отдельной задачей.
- L2: actual SQLite/intent processor, fail-closed checkpoint, атомарность terminal projection/outbox и сохранность per-tender references. Дополнительно воспроизведён дефект PRAGMA synchronous внутри transaction; результат ещё требует интеграционной приёмки.
- L3: durable possible-send marker и атомарный claim MiniPOS, UNKNOWN lookup-only, запрет resend после неавторитетной ошибки lookup. Подготовлен patch с regression tests; реальные CLOUD/BLE lookup остаются незавершённой зависимостью, поэтому безопасная блокировка повтора не означает завершённого recovery.

Работа выполняется в отдельных временных копиях. Пакеты на момент этой записи не приняты в общий source snapshot: результаты отдельных потоков не являются общим PASS. Следующий обязательный шаг — review каждого patch, интеграция с сохранением текущих пользовательских изменений и объединённые проверки. Статусы GAP сохраняются OPEN до соответствующего evidence.

## Когда цель считается достигнутой

1. Все известные software GAP и новые findings имеют accepted disposition и исполняемые regressions; C/R гипотезы подтверждены/опровергнуты evidence. Исключённая необязательная возможность программно недоступна и документирована; исключением нельзя закрыть обязательный заявленный scope.
2. Полный согласованный CASH/CARD/split/recovery scope получает PRE_HIL_SOFTWARE_READY по одному exact snapshot, без required SKIP, false success, duplicate receipt/debit, lost tender или потерянного terminal outbox.
3. Готовы воспроизводимые firmware/APK/native builds, factory/provisioning/runbooks, стенд и case inventory для реальных устройств; actual physical execution отмечена отдельно.
4. Готов проверенный пакет к применимой процедуре НАП, source-versioned requirements, техническая матрица и открытые внешние confirmations. Подготовка не приравнивается к подаче/включению в список.

Срок полного завершения не обещан: реальные SDK/profiles, состояние toolchains и внешние подтверждения ещё требуют проверки. После первого воспроизводимого вертикального CASH slice дать оценку по фактически закрытым cases и оставшемуся критическому пути, а не числу файлов/проценту unit coverage.
