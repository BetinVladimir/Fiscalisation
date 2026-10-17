# Дополнение: порядок автоматического выполнения до подключения устройств

## Уточнение текущего контура

По прямому запросу пользователя отдельный Go-модуль `edge-agent/` удалён из репозитория. Его команды, генераторы и ссылки на тестовые evidence исключаются из действующего контура. Исторические результаты не подтверждают готовность S3. Текущий IoT runtime — `IoT/firmware/edge-agent-s3/idf`; обязательны actual-runtime тесты и сборка этой прошивки. Go oracle в `idf/tests/go_wire_host` использует `fiscal-backend/internal`, а не удалённый модуль, и остаётся применимым. Недостающие эквивалентные проверки S3 остаются OPEN до реализации, а не закрываются удалением reference tests.

## Активная цель и ускоренное исполнение

2026-10-17 пользователь поставил полную цель: закрыть все разрывы/ошибки, добавить и выполнить тесты, устранить регрессии, подготовить устройство-испытания и MiniPOS к применимой процедуре НАП/СУПТО. [Цель, три параллельных потока и общая приёмка](08_GOAL_AND_FAST_TRACK.md); [подготовка пакета НАП](09_SUPTO_NAP_PREPARATION.md). Сохраняется CASH → CARD/split/recovery, регуляторная дорожка начинается сразу. Пользователь разрешил делегирование; общий backlog, contracts и merged-snapshot gate обязательны. Это расширение цели, не свидетельство уже выполненных исправлений.

[План W0–W9](04_REMEDIATION_PLAN.md) · [Ревью](06_PLAN_REVIEW.md) · [Тесты](05_TESTS_AND_ACCEPTANCE.md)

**Статус: SPECIFICATION_ONLY.** Ниже требования к будущему runner и автоматическим исправлениям, не утверждение об их наличии. Пользователь на текущем этапе разрешил документацию; этот документ не начинает изменение исходного кода.

## Согласованный порядок

2026-10-17 пользователь принял **вариант 2: поэтапная автоматизация с контрольными точками**. Обязательный порядок результатов: **M0 baseline/decisions → M1 harness/red tests → M2 CASH → M3 CARD/split/recovery → M4 full software gate/build artifacts**. [Принятое решение и критерии milestones](04_REMEDIATION_PLAN.md) имеют приоритет над неопределённым порядком внутри пакетов. A0–A11 исполняются по применимому scope каждого milestone; CASH recovery и UNKNOWN обязательны в M2, карточное recovery добавляется в M3. Весь заявленный scope сохраняется, незакрытый BluePad spec не блокирует CASH и не позволяет объявить CARD готовым.

Направления transport identity/УНП/profiles/эмуляторов/CI согласованы; exact wire details и contract compatibility оформляются A2. Состояние плана ACCEPTED, реализации PLANNED. Новые make targets и manifests пока предстоит создать. Контрольные точки требуют evidence, а не повторного разрешения на каждую рутинную задачу в уже авторизованном этапе.

## Граница software-этапа

Финальный статус этого этапа — **PRE_HIL_SOFTWARE_READY** для явно перечисленных profiles и exact source/build snapshot. Обязательны no-hardware tests, cross-build artifacts и проверка эмуляторов. Physical flash, boot на плате, Android/iOS actual BLE, UART/USB electrical tests, банковская авторизация и vendor firmware остаются **HIL_PENDING**.

У каждого T-case должен быть phase `HOST`, `SERVICE_INTEGRATION`, `CROSS_BUILD` или `PHYSICAL_HIL`. Группы T-02/T-04/T-20/T-22 содержат cases разных phases: успешно выполненная host-часть не закрывает physical-часть. T-21 целиком physical; T-24 нужен только для включённого Tremol. Все HOST/SERVICE_INTEGRATION/CROSS_BUILD cases применимых T-01…T-23/T-25 обязательны; optional scope не пропускается неявно.

## A0–A11: последовательность задач

| ID | Зависит от | Ограниченная задача / deliverable | Проверяемый выход |
|---|---|---|---|
| A0 | — | Immutable baseline: HEAD, hash tracked diff + всех используемых untracked source files, contracts, dependency locks/toolchains, текущие test results | baseline manifest; известные failures сохранены. Красный baseline допустим, потеря чужих изменений недопустима |
| A1 | A0 | Dependency и profile inventory: standalone fixtures, licenses, source revisions, enabled runtime capabilities; разделить no-hardware и HIL cases | Матрица profiles/cases; missing SDK/spec отмечены BLOCKED_SPEC/DEPENDENCY |
| A2 | A1 | Зафиксировать применимые ADR-A…F через contract decision process; fixtures exact topics/IDs/commands/UNP/preimage/security/recovery | Versioned decision records со статусом ACCEPTED; incompatible вариант не выбирается тестовым mock |
| A3 | A1; A2 для protocol-dependent cases | Harness bootstrap W9: transport seams, independent device models/clock/scheduler/device storage; actual runtime linkage manifest | Runnable failing fixture для actual C++ runtime; production binary без mocks |
| A4 | A2/A3 | Red tests и fixes GAP-009…012/015/021; vendor codec/payload/parser/result semantics | T-05/06/07/10 зелёные; mutations codecs/SEQ/status делают тесты красными |
| A5 | A2/A3 | Red tests и fixes GAP-001/002/024; actual publisher→consumer→sync→domain | T-01/13, broker integration; adapter/fiscal IDs и external/internal IDs различаются |
| A6 | A2/A3; applicable payment spec | BluePad GATT/payment mapping и per-tender saga GAP-013/014/019 | T-08/09, typed references/late responses; неподтверждённый spec не подменён fake-success |
| A7 | A4/A5; A6 для CARD | Durability/UNP/recovery/sync GAP-016…020/022/023/025 | T-10/11/12; все named failpoints и restart cases выполнены |
| A8 | A2/A3 | Provisioning/network/session software GAP-003…008/004/031; no-hardware protocol/config tests | Host parts T-02/03/04/20/22; device boot/radio остаются HIL_PENDING |
| A9 | A5/A7 | POS UNKNOWN/outbox + backend regression GAP-026…028; auth/UI/webhook/DB | T-14…19 и native adapter host cases; real PostgreSQL, необходимые race/concurrency checks |
| A10 | A3…A9 для scope | Build/gate/evidence GAP-029/030/033/034; T-25 registry coverage, clean cross-build, readiness validator | Все required cases учтены; artifact hashes, no-mock linkage, software verifier negative tests |
| A11 | A10 | Независимая сверка final manifest/cases/invariants, без изменения expected outputs | PRE_HIL_SOFTWARE_READY либо FAIL/BLOCKED с точными причинами; physical coverage явно pending |

Tremol/GAP-032 включается отдельной A4/A3 profile-задачей только при выбранном scope. Для каждого шага фиксировать уникальный task ID; таблица не заменяет декомпозицию W5/W9 на исполняемые case IDs. Не нужно ждать A2 для механического baseline, установки доступного toolchain или independent vectors уже опубликованных применимых протоколов. Заблокированный BluePad не останавливает Datecs/Daisy cash ветки; весь заявленный cash+card scope не получает итоговый READY, пока его dependency не закрыта.

## Обязательная запись задачи и case inventory

Будущий machine-readable manifest должен содержать для каждой задачи: `task_id`, GAP/REV IDs, profile/version, predecessors, accepted decision/source IDs, allowed source files, repro case IDs, ожидаемые до/после состояния, required commands с cwd/env/timeout, required artifacts, failpoints/mutations и status/evidence refs. C/R риски сначала получают воспроизводящий тест/contract clarification; отсутствие failing test не оправдывает произвольную замену реализации.

Для каждого case: stable ID, T-group, phase, profile/capability, protocol source/hash, input fixture, initial persisted adapter/device state, clock/seed, fault ID, ожидаемые wire bytes/переходы/счётчики/terminal state, required flags и output refs. `NOT_APPLICABLE` допускается только из явно отсутствующей capability профиля с причиной и negative capability test. Required case со SKIP/missing/BLOCKED не даёт READY.

Named failpoints минимум: `before_checkpoint_commit`, `after_checkpoint_commit_before_send`, `after_partial_write`, `after_device_execute_before_reply`, `after_reply_before_payment_commit`, `after_payment_commit_before_fiscal_send`, `after_fiscal_close_before_result_commit`, `inside_terminal_projection_outbox_transaction`, `after_sync_server_commit_before_ack`, `after_ack_verify_before_cursor_commit`. Для каждого применимого failpoint определить persisted state, разрешённый recovery query и запрещённые side effects; массив «random fault happened» не считается полным покрытием.

Device emulator и adapter process имеют независимые persistent stores. В `after_device_execute_before_reply` перезапускается adapter, а emulator сохраняет receipt/payment и action count. Reset обоих одновременно проверяет другой сценарий. Virtual clock/scheduler детерминирует SYN/NAK, write completions, delayed notifications и reconnect; отдельный seeded stress дополняет deterministic cases, не заменяет их. Host sanitizers для native buffers и concurrency tests должны входить в применимые build variants, где поддерживаются toolchain.

## Команды и будущие entrypoints

Существующие команды — частичные проверки, не новый итоговый gate: `make iot-test`, `make contract-test`, `make typecheck`, `make postgres-integration`; `go test -count=1 ./...` в каждом Go module и имеющиеся npm suites. До запуска проверить source-mutating generate/build steps и output paths; сохранять настоящий exit status, не status последнего `tail`/`tee`.

Существующий IDF target определён в `IoT/firmware/edge-agent-s3/idf/platformio.ini`: env `edge-agent-s3-idf`, platform `espressif32@6.10.0`. Cross-build из корня: `platformio run -d IoT/firmware/edge-agent-s3/idf -e edge-agent-s3-idf` с зафиксированным binary/toolchain и обязательными trust/config prerequisites. Работоспособность команды на clean acceptance environment пока не подтверждена этим review. Default Arduino env из соседнего `platformio.ini` не подменяет target.

В реализации добавить и документировать следующие **новые, пока не существующие** targets:

| Предлагаемый entrypoint | Обязательное поведение |
|---|---|
| `make iot-emulation-test` | Registry validate + actual-runtime build + все required USB/COM/BLE scenarios + independent golden/mutation checks; T-25 case report |
| `make pre-hil-software-gate` | A0/A1 preflight, все применимые no-hardware suites, service containers, cross-builds, contract/status verification и финальный manifest |
| `make pre-hil-evidence-verify` | Проверяет completeness/hashes/source snapshot/required case inventory и trusted attestations, а не просто строку PASS |

Исполнитель создаёт точные commands/cwd/env/outputs/timeout для подшагов до первого принятого acceptance run. Missing target/toolchain/dependency не преобразуется в PASS. Images и tools pin по version/digest; packages и legal vendor fixtures доступны вне абсолютных путей workstation. Production keys не нужны; применять fixture-only test identities и локальные service endpoints. Runner не выполняет flash, production payment, store upload или deployment.

Старый `make smart-device-test` и `run_mvp1_software_gate.sh` требуют отдельной актуализации: flavors, STORE_BLOCKED, layout outputs, optional/excluded STUB и no-hardware phases. Нельзя снять release guard только ради прохождения software gate. Readiness validator должен читать фактический structured status и open required gaps; наличие исторической строки SOFTWARE_COMPLETE_HIL_PENDING больше не критерий готовности.

## Протокол выполнения исправления

Одна task: воспроизвести контрпример → зафиксировать независимый expected result → изменить только относящиеся к задаче sources → выполнить relevant tests → выполнить dependent integration tests → сохранить diff/evidence. Если failing fixture противоречит locked contract/vendor source, задача BLOCKED_SPEC до разбора, а не обновление fixture под код.

Не выполнять весь широкий gate после каждого малого изменения. Relevant suites достаточны для локального шага; итоговый полный no-hardware gate обязателен на final snapshot. Любая новая правка после verification инвалидирует соответствующий manifest/artifact hashes и требует повторной проверки затронутых suites и итогового gate. Исключения scopes/requirements не принимать автоматически, чтобы сделать отчёт зелёным.

Baseline сохраняет уже существующие изменения пользователя. Для acceptance собрать фиксированный срез без потери dirty источников; manifest включает фактические bytes, а не только HEAD. Generated outputs и build directories отделены от source identity. Реальные случайно попавшие секреты не выводить в отчёты.

## Итоговый статус

`PRE_HIL_SOFTWARE_READY` допустим только когда все включённые profiles имеют accepted spec, emulators и required case evidence; P0/P1 software tasks закрыты; required HOST/SERVICE_INTEGRATION/CROSS_BUILD suites и mutation controls проходят без skipped/missing cases; source/build/profile hashes совпадают и artifacts проверены. Unresolved spec/toolchain/vendor dependency означает BLOCKED для профиля, а не успех без теста. Если READY получен для суженного scope, он явно перечислен; READY для CASH не объявляет готовность CARD.

Отчёт сохраняет отдельно `software_status`, `profiles`, `source_snapshot`, `build_artifacts`, `required_cases`, `executed_cases`, `failures`, `blocked_dependencies`, `excluded_scope` и `hil_status=HIL_PENDING`. Этот документ определяет, что нужно реализовать для автоматизации; он не свидетельствует, что реализация уже достаточна или выполнена.
