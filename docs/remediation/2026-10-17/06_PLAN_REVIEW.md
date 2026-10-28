# Ревью плана: готовность к автоматическому исправлению без устройств

Дата: 2026-10-17. Проверены W0–W9, GAP-001…034, T-01…25, gates и реальные build/test entrypoints репозитория. Это review документации и orchestration, без нового полного запуска suites и без изменения кода.

**Вердикт: исходный план достаточен как инженерный backlog, но недостаточен как полностью исполняемое автоматическое задание.** Можно начинать воспроизведение ошибок, независимые protocol fixtures и test harness. Полностью автоматически исправлять все модули и объявлять готовность к устройствам пока нельзя: есть unresolved contracts/vendor dependencies и ещё не созданные исполняемые gates. Дополнение [07_AUTOMATION_EXECUTION.md](07_AUTOMATION_EXECUTION.md) задаёт порядок и границы автоматизации. Оно является спецификацией будущей реализации, а не существующим runner.

## Замечания по приоритету

| ID | Приоритет | Основание | Почему мешает автоматизации | Требуемое закрытие |
|---|---|---|---|---|
| REV-01 | P0 | План: ADR-A…F; GAP-001/003/014/016/022/024 | Topic identity, UNP authority, canonical bytes, provisioning/security и payment references оставлены решениями на будущее. Автоисправление может выбрать несовместимые варианты и написать эмулятор под свой же неверный выбор | Сначала decision records с exact schemas/bytes, legacy policy и источниками. BLOCKED_SPEC до фиксации; запрещено «угадывать протокол» ради PASS |
| REV-02 | P0 | W2 gate factory-reset; G3 reboot; G4/T-21 physical smoke; T-22 flash/boot | Gate «до устройств» требует подключённую плату и мобильный hardware. Полный план не имеет достижимого без устройств terminal status | Разделить PRE_HIL_SOFTWARE_READY, HIL_PENDING и PHYSICAL_HIL_PASS. T-02/04/20/22 разделить на HOST/BUILD и PHYSICAL cases; pre-HIL не зависит от последних |
| REV-03 | P0 | W9/T-25; `Makefile::iot-test`; `idf/tests/run_native_saga.sh` | Эмуляторы/registry/runner пока предложения. Текущий native runner компилирует orchestrator/FrameCodec, но не заменяет integration harness actual profile_executor + transports + storage. Отсутствует runnable T-25 | Отдельный bootstrap harness с API seams, source/link manifest, независимыми fixtures и негативным контролем; существующий make iot-test не называть эквивалентом T-25 |
| REV-04 | P1 | W0 baseline; W3/W4/W5 fixes; G0 «все suites зелёные» | Если требовать green baseline до начала fixes, исправление трёх известных failures и новых failing regressions не начнётся. Source cleanup может потерять dirty изменения пользователя | Сначала immutable baseline с разрешённым красным результатом; затем воспроизводящий тест → фикс → gate. Чистый acceptance-run на отдельном фиксированном срезе, без reset чужих изменений |
| REV-05 | P1 | T-22 «после проверки tasks»; Makefile smart-device-test; scripts/run_mvp1_software_gate.sh | Нет одного no-hardware entrypoint. Старый gate требует нефлаворные release artifacts, конфликтующие с STORE_BLOCKED; копирует зависимости из соседнего repo. Повтор запуска дважды сам по себе не делает среду чистой | Ввести новый pre-HIL entrypoint/manifest, exact tasks/outputs/timeouts, pinned dependencies и самостоятельную поставку разрешённых fixtures; старые gates адаптировать без снятия production guard |
| REV-06 | P1 | Все GAP, broad T suites; W5 «в каждом crash window» | Слишком крупные задачи: нет полного case inventory, точек инъекции, state transitions, отчёта об exact required count. Runner может выполнить удобное подмножество и выдать PASS | Atomic task manifest и bidirectional GAP→task→case→profile→artifact matrix; named failpoints и scenario expansion; missing/skip обязательного case означает FAIL |
| REV-07 | P1 | W9 независимый oracle; V-03/04/06 и BluePad spec pending | Эмулятор можно реализовать как зеркало production codec, а неподтверждённые UUID/descriptors/STAN семантики придумать. Это взаимное подтверждение двух ошибок | Golden bytes с происхождением, independent reference parser, mutation negative controls; неполный vendor profile BLOCKED_SPEC, а не искусственный approve |
| REV-08 | P1 | T-08/09/11/25 concurrency/restart | Seed/clock заявлены, но не определены scheduler, persistent device state и process boundary. Простое исключение из функции не моделирует crash после фактического исполнения устройством | Virtual scheduler/clock, независимые device/adapter storage, process kill/reopen cases, late event/queue ordering и explicit persisted actions counters |
| REV-09 | P1 | Новые banners; scripts/verify_readiness_documentation.rb; contracts/CONTRACT_LOCK.md | Readiness verifier ищет старый SOFTWARE_COMPLETE_HIL_PENDING и печатает external-only gaps. Он может пройти при новых P0. Контракты locked: нельзя молча менять schema ради исправления topics/IDs | Актуализировать status machine/validator и contract change process, hashes/generated types/overlays. Различать contract drift, missing dependency и runtime failure |
| REV-10 | P2 | План, dependency paragraph и Definition of Done | При предыдущем дополнении появились дубли фраз W9 и device registry requirement. Для человека мелочь, для generated task inventory потенциальная неоднозначность | Удалено в этом review; stable task/case IDs должны быть уникальны |

P0 здесь — блокер **автоматического исполнения плана**, не новая классификация аппаратного дефекта. REV находки не закрывают GAP; fixes в runtime по-прежнему не выполнены.

## Что уже хорошо покрыто

В плане верно поставлены независимые wire vectors, stateful emulation USB/COM/BLE, shared runtime sources, possible-send/UNKNOWN, per-tender recovery, SQLite failpoints, frozen sync batches, отдельный physical evidence и обязательное расширение registry для новых устройств. Эти требования сохраняются. Недостаток — степень формализации и исполнимость, а не отсутствие основных направлений проверки.

## Что можно выполнять автоматически сейчас

Снять baseline/источники/toolchains, проверить локальную доступность зависимостей, воспроизвести текущие failures, добавить independent Datecs/Daisy fixtures и красные runtime regression tests. Создавать транспортный harness можно с clearly provisional seams, не объявляя непроверенные vendor details обязательным standard. После фиксации decision records — исправлять соответствующий компонент и последовательно исполнять тесты.

**Не разрешать автоматическому исполнителю:** обходить locked contract, менять vendor fixtures под текущий encoder, отключать failing required cases, ослаблять operator/tenant/UNP/idempotency checks, автоматически отправлять деньги/flash/publish/deploy в рамках no-hardware gate, заменять UNKNOWN успешным/отрицательным результатом без evidence или закрывать hardware gate программной эмуляцией.

## Условие достаточности перед стартом полного auto-fix

План станет достаточным для ограниченного автоматического исправления и software acceptance, когда подготовлены: (1) применимые approved decision records и vendor profiles; (2) task/case manifests; (3) runnable no-hardware gate; (4) versioned fixtures и actual-runtime emulation harness; (5) независимый verifier результата. После этого можно получить **PRE_HIL_SOFTWARE_READY** на фиксированном срезе. Это аргументированная готовность начать HIL, не доказательство совместимости реального устройства.

Сейчас эти prerequisites в основном описаны, но не реализованы. Поэтому вердикт по готовности к полностью автоматическому запуску — **NOT_READY_FOR_FULL_AUTOFIX**; по готовности начать подготовительные программные задачи — **READY_FOR_BASELINE_AND_HARNESS**.
