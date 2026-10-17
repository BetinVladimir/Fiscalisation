# План доработок до аппаратной приёмки

## Активная цель и ускоренное исполнение

2026-10-17 пользователь поставил полную цель: закрыть все разрывы/ошибки, добавить и выполнить тесты, устранить регрессии, подготовить устройство-испытания и MiniPOS к применимой процедуре НАП/СУПТО. [Цель, три параллельных потока и общая приёмка](08_GOAL_AND_FAST_TRACK.md); [подготовка пакета НАП](09_SUPTO_NAP_PREPARATION.md). Сохраняется CASH → CARD/split/recovery, регуляторная дорожка начинается сразу. Пользователь разрешил делегирование; общий backlog, contracts и merged-snapshot gate обязательны. Это расширение цели, не свидетельство уже выполненных исправлений.

[Навигация](README.md) · [GAP register](02_GAP_REGISTER.md) · [Тестовая спецификация](05_TESTS_AND_ACCEPTANCE.md)

Цель этапа: сделать целевые runtime paths совместимыми, восстанавливаемыми и проверяемыми программно. **Изменения ниже — задания на следующий этап, не выполненные исправления.** Оценка сроков до фиксации exact device/SDK profiles ненадёжна; использовать зависимости и gates. Все GAP в реестре OPEN.

## Принятое решение — вариант 2, 2026-10-17

**Согласовано пользователем:** поэтапная автоматизация с обязательными контрольными точками. Первый проверяемый результат — **CASH**, затем **CARD, split и recovery**. Полный ранее заявленный объём сохраняется; CASH milestone не означает готовность всего решения. Статус решения — **ACCEPTED**; статус реализации — **PLANNED**, GAP остаются открытыми до подтверждения тестами.

### Зафиксированные решения

| Область | Согласованное направление |
|---|---|
| Transport identity | Команда адресуется IoT-адаптеру; конечное фискальное устройство указывается отдельно. Один `client_operation_id` и immutable digest сохраняются между MQTT/BLE; internal/external sale IDs различаются явно |
| Device profiles | Версионируемые profiles Datecs, Daisy и BluePad со спецификацией/fixtures. Неподтверждённые детали BluePad блокируют соответствующий CARD scope, но не CASH |
| Authority/УНП | Cloud-reserved УНП и offline allocation различаются явно. Уже назначенный УНП неизменяем; authority, диапазон и reservation сохраняются до device I/O |
| Эмуляция | Единый registry-driven USB/COM/BLE runner W9/T-25, независимые vendor vectors, stateful device models и настоящий код IoT runtime; новое поддерживаемое устройство обязательно расширяет coverage |
| Исправления | Небольшие задачи: воспроизводящий красный тест → исправление → негативные проверки → зависимая интеграционная проверка; затем общий gate на фиксированном срезе |
| CI | Реализовать `iot-emulation-test`, `pre-hil-software-gate`, `pre-hil-evidence-verify`; обязательный skipped/missing case блокирует успех |
| Результат | `PRE_HIL_SOFTWARE_READY` только для явно перечисленных profiles/capabilities и exact build. Реальные устройства, radio/electrical interoperability и acquirer evidence остаются `HIL_PENDING` |

Это согласование архитектурного направления, а не утверждение ещё не составленных wire schemas/UUID/TLV. Exact topic syntax, compatibility policy, hash preimages, provisioning handshake и vendor payment details должны быть оформлены в decision records A2 на основе контрактов и источников. Исполнитель может готовить baseline/harness и выполнять однозначные задачи; при противоречии источников зависимая задача получает BLOCKED_SPEC. Повторное согласование уже принятого направления не требуется; контрольная точка — проверяемый результат и отчёт, а не обязательный запрос разрешения после каждой задачи.

### Контрольные результаты и порядок реализации

| Milestone | Содержание и зависимости | Критерий завершения |
|---|---|---|
| M0 — Baseline и исполняемая спецификация | A0–A2: сохранить фактический source snapshot/dirty changes, текущие failures; profiles, accepted decisions, task/case manifests и software/HIL phases | Исходные ошибки воспроизводятся; required cases и sources определены. Неподтверждённые CARD детали явно BLOCKED_SPEC |
| M1 — Стенд и красные regression tests | A3/W9: actual runtime + USB/COM/BLE mocks, independent oracle, device state и fault scheduler; воспроизводящие тесты codec/topic/durability defects | Runner исполняет реальные modules, неверные bytes/commands и mutation controls проваливают тесты; production linkage без mocks |
| M2 — Первый результат: CASH | Применимые A4/A5/A7/A8/A9 и A10/A11: S3+Datecs COM и S3+Daisy USB, MQTT/direct BLE ingress, журнал/sync/domain/MiniPOS. Software provisioning, UNKNOWN, idempotency и cash recovery входят в этот milestone | Required CASH cases проходят, restart/lost reply не печатает второй чек, result содержит device evidence; build/evidence verifier проходит. READY ограничен CASH profiles, physical checks HIL_PENDING |
| M3 — CARD, split и полное recovery | После M2 и applicable BluePad spec: A6 и расширение A7/A9/W9. BLE wake/flow/references, CARD/split, per-tender journal, compensation и recovery | Approve/decline/unknown/late reply/crash cases проходят; отсутствуют повторные debit и потерянные approved tenders. Неопределённость остаётся RECONCILE |
| M4 — Полный software gate и сборки к HIL | A10/A11 для всего согласованного scope, включая отдельный BlueCash Kotlin контур и MiniPOS builds; общая regression matrix и независимая проверка evidence | PRE_HIL_SOFTWARE_READY для полного заявленного scope либо честный FAIL/BLOCKED; exact firmware/APK/native build artifacts и перечень pending physical cases |

Recovery не откладывается целиком до M3: безопасность CASH, durable result/outbox, UNKNOWN и восстановление после потери ответа обязательны уже в M2. M3 расширяет recovery на карточные и смешанные платежи. Для BlueCash отдельный Android processor/bridge проверяется собственными suites; PASS S3 не переносится на него. Часть работ с подтверждённой спецификацией можно готовить раньше milestone, но следующие этапы не получают READY за счёт непроверенного предыдущего.

Текущий запрос фиксирует план. Реализация программного кода, создание эмуляторов и выполнение исправлений этим этапом не начаты.

## Результат ревью: автоматическое выполнение

**До подключения устройств целевой результат — PRE_HIL_SOFTWARE_READY; весь исходный план ещё не является исполняемым auto-fix runner.** [Ревью с замечаниями REV-01…10](06_PLAN_REVIEW.md) и [обязательное дополнение A0–A11](07_AUTOMATION_EXECUTION.md) определяют зависимости, точные manifest/case требования и будущие no-hardware entrypoints.

Сначала baseline и harness; красный baseline допустим. Unresolved protocol/contract/payment decisions блокируют зависимые fixes, а не подменяются предположениями эмулятора. Все physical flash/boot/mobile-radio/acquirer cases вынесены в HIL_PENDING и не блокируют выполнение software-подэтапа; они остаются обязательными до аппаратной приёмки. Требование зелёных G0–G4 ниже относится к итоговым ступеням, а не запрету начать исправления при красных тестах.

## Общие инварианты

1. Один business intent сохраняет UUID, digest, authority и UNP при Cloud/MQTT/Local HTTP/BLE; payload conflict не создаёт side effect.
2. После possible-send неопределённость означает lookup/reconcile, пока не доказан NOT_SENT; сетевой timeout/404 из другого маршрута не доказательство отсутствия исполнения.
3. До каждого денежного/фискального I/O существует durable checkpoint. После него ошибки хранилища не скрываются, terminal projection и outbox атомарны.
4. Каждая CARD tender имеет собственный reference/amount/state; approvals и compensation никогда не теряются/не смешиваются.
5. Успех доказывается устройством и журналом; PUBACK, synthetic EDGE-reference, найденный UART/CCCD и HTTP202 сами по себе не доказывают фискализацию.
6. Один protocol profile определяет vendor/model/fw/region/encoding/currency, command map, payload limits, status/errors и recovery queries.
7. Повтор sync после неоднозначного ответа сохраняет **точные bytes**; business ACK валидирует подписанный endpoint, batch, seq и hash.
8. UI/readme/traceability описывают именно запускаемый runtime. Host reference capabilities не выдаются за firmware implementation.

## Пакеты работ

| Пакет / ответственный профиль | Задания | Порядок и deliverables | Gate |
|---|---|---|---|
| W0 — Tech lead / QA | GAP-028/030/032/033/034 | Зафиксировать commit+dirty baseline, состав поддерживаемых profiles, SDK hashes, тестовую матрицу. Исправить failures после анализа fixtures. Внешний trust для HIL evidence | T-15/18/22/23; честный красный baseline до исправлений |
| W1 — backend + contract owner | GAP-001/002/024 | Решить adapter vs fiscal device identity; topic namespace/version, suffix и ACL; command/result mapping; internal/external/client sale IDs; offline import | T-01/13 на реальном broker и actual processors |
| W2 — embedded + mobile provisioning + factory | GAP-003…008/031 | Согласованный onboarding protocol; identity/trust/time/secrets/Wi-Fi; fragmented install; advertising/MTU/session reset; точный image layout | T-02/03/04/22; factory-reset boot на обоих mobile OS |
| W3 — device driver owner | GAP-009…012/015/021 | Сначала vendor golden vectors; затем единый runtime codec, stream parser/deadline/status/ErrorCode, session SEQ, profile text/payment/operator/reason mapping, typed fiscal result | T-05/06/07/10; исправление должно ломать старые неверные vectors |
| W4 — embedded payment owner | GAP-013/014/019 | Applicable BluePad SDK/spec; wake/discovery/completion queue; typed STAN/RRN; response correlation; durable per-tender evidence | T-08/09; approved/decline/unknown/late event и split |
| W5 — embedded storage + backend sync | GAP-016…020/022/023/025 | UNP authority snapshot; transaction/failpoints; all-tender recovery/readiness/ownership; canonical hashes; frozen batches/ACK bounds; capacity/retention | T-10/11/12; restart в каждой критической точке |
| W6 — POS web/native | GAP-026/027 | Durable route possible-send state, lookup taxonomy, session lifecycle, offline reconciliation и actual mobile I/O contract | T-04/14/17/21; no duplicate after reload/kill |
| W7 — release / SRE / QA | GAP-004/029/030/031/034 | Reproducible builds с явными flavors и IDF target; migration drill, broker mTLS/ACL, artifacts+trusted evidence и runtime coverage | T-16/19/20/22/23; clean CI и named firmware/APK/IPA |
| W8 — optional driver owner | GAP-032 | Только при расширении scope: Tremol physical profile, BG conformance и отдельный HIL | T-24; без этого capability не включать |
| W9 — embedded / device drivers / QA automation | GAP-009…023/030; T-25 | Расширяемый стенд эмуляции USB/COM/BLE устройств; сборка actual IoT runtime с transport mocks, проверка wire-команд и stateful device responses; обязательный профиль каждого нового устройства | T-25 + T-05…T-12/T-23; ни один включённый device profile не пропускается |

W0 стартует первым. W1 и protocol fixture часть W3 не зависят от железа; W2/W4 нуждаются в конкретных SDK/экземплярах. W5 использует согласованные wire/ID контракты W1 и profile W3/W4; POS recovery W6 использует semantics W5. W9 начинается после фиксации profiles в W0, предоставляет общий стенд для W3/W4/W5 и становится обязательной частью W7. W7 завершает состав release-кандидата. Для исполнения применяется разрешённое пользователем распараллеливание L1/L2/L3 + интегратор из ускоренной стратегии; зависимости пакетов сохраняются.

## W9 — Покрытие IoT сборки эмуляторами подключённых устройств

**Обязательное задание на доработку.** Реализовать программный стенд, запускающий код целевой IoT сборки с моками подключённых USB, COM/UART и BLE устройств. Эмуляторы должны воспроизводить документированный протокол кассового аппарата и POS-терминала, принимать реальные исходящие bytes и проверять корректность коммуникационных команд. Это расширение T-05…T-12 и T-23; интегральный gate — **T-25**.

### Состав и границы стенда

1. **Production firmware build.** Собирать выбранный ESP-IDF target с настоящими driver/executor/processor/storage/codec модулями. В manifest сохранять target, flags, зависимости, linked sources и hashes. Production artifact не содержит test emulators или test credentials.
2. **Host runtime test build.** Из тех же исходников собирать тестируемые runtime модули с подменой только нижних I/O API, часов и необходимых platform services. Использовать реальные codec, command payload builder, queue, saga, SQLite journal и recovery. Не подменять `purchase`, `fiscalize` или `execute` готовым успешным результатом.
3. **Transport mocks.** USB mock воспроизводит enumerate/descriptors/interface/attach/detach и CDC transfer callbacks; COM mock — UART configuration/read/write/partial I/O; BLE mock — scan/connect/discovery/GATT/CCCD/POWER/WAKE/MTU/write completion/notifications. Mock проверяет и конфигурацию канала, и последовательность API-вызовов.
4. **Stateful device emulators.** Модель устройства принимает byte stream, разбирает frames независимо от production codec, проверяет команды/параметры/SEQ/checksum и возвращает документированные responses. Хранит receipt/payment state и независимый счётчик фактических device actions. После host restart состояние устройства при необходимости сохраняется, чтобы воспроизвести «команда выполнена, ответ потерян».
5. **Scenario runner и wire oracle.** Сценарий задаёт intent, device profile, исходное состояние, clock/seed, ожидаемые TX/RX и fault points. Runner сопоставляет byte trace с vendor golden fixtures и допустимым автоматом команд; unexpected command/лишняя оплата/нарушенный порядок немедленно проваливают тест. Каждый ответ проверяется также по business result, journal и recovery effects.

Host test build не выдавать за исполнение полного ESP32 binary или CPU/RTOS эмуляцию. Сравнение linked runtime sources и отдельная production cross-build обязательны; transport mocks не подтверждают электрические уровни, USB PHY, radio interoperability, vendor firmware или acquirer acceptance. Эти свойства остаются в HIL.

### Начальные обязательные эмуляторы

| Профиль | Канал | Поведение, которое моделировать |
|---|---|---|
| Datecs DP-150MX, зафиксированная BG protocol/firmware revision | COM / UART через предусмотренный RS-232 transport boundary | Open/sale/discount/tender/close, X/Z, cash-in/out, storno, status, identity/time/EJ; LEN/BCC/SEQ, SYN/NAK, ErrorCode, receipt number/UNP |
| Daisy Compact S01, зафиксированная protocol/USB revision | USB CDC либо подтверждённый для экземпляра USB transport | USB descriptors/interface/line coding, CP1251, framing/status/retry, receipt lifecycle/recovery, attach/detach и short transfers |
| Datecs BluePad-50 Plus, зафиксированные SDK/payment app/acquirer profile | BLE central → peripheral terminal | Service/characteristics, POWER/WAKE, subscription completion, <=19byte writes по применимому SDK, payment TLV, approve/decline/cancel/unknown, STAN/RRN/auth, lookup/VOID/refund по capabilities |
| Комбинация S3 + Datecs COM + BluePad BLE | Два одновременно подключённых устройства | CASH/CARD/split, card-approved/fiscal-failed, per-tender compensation, reconnect и восстановление после потери ответа |
| Комбинация S3 + Daisy USB | USB + BLE управление MiniPOS → S3 | Cash receipt через MQTT и direct BLE; CARD включать только при явно поддержанном payment endpoint/profile, без фиктивного approve |

BLE peripheral S3 для MiniPOS и BLE central S3 для BluePad тестировать одновременно: inbound intent и terminal notifications не должны смешивать connection handles, UUID, buffers и operation correlation. К BlueCash Android использовать общие Datecs fixtures в его отдельном Kotlin контуре; успешная IoT suite не означает успешной проверки Android bridge.

### Что проверять в командах

- Точное бинарное framing, length/checksum, byte encoding, separators и limits по внешнему protocol fixture; тест не вычисляет ожидаемый BCC/LEN тем же production encoder.
- Полный порядок команд и допустимость перехода: configure/connect/probe → open → lines/discounts → tenders → close → result queries; запрос/ответ коррелированы по vendor sequence/command/reference.
- Содержимое: operator/password/till из test binding, tax/payment mapping, quantity/price/discount/total/currency, original receipt/UNP/reason для storno, amount/STAN/RRN для payment/reversal.
- Повторы transport frames с тем же SEQ/UUID, cached responses, отсутствие повторного физического действия, документированные query/recovery commands и запрет неподдержанных commands.
- Typed interpretation всех status/ErrorCode и receipt/payment evidence. Некорректный ответ эмулятора не должен приводить к ложному COMMITTED.

### Матрица сбоев

Для каждого применимого profile: arbitrary chunking, incomplete frame, corrupted LEN/BCC/TLV, wrong SEQ/CMD, noise, SYN/busy/NAK, delayed/duplicate/out-of-order/late reply, timeout, partial write, detach/disconnect, failed CCCD/wake и reconnect. Проверять fault до send, после части bytes, после device execution и до durable checkpoint. Добавить рестарт IoT process/SQLite reopen при сохранённом device state; cash/card/split recovery не создаёт второй чек/списание. Входные команды прогонять через actual MQTT/BLE ingress adapters и общую queue, а не только прямой вызов payload builder.

### Расширение новыми устройствами

Предлагаемая структура будущей реализации: `tests/iot-device-emulation/{transports,devices,fixtures,scenarios}` и versioned machine-readable device registry. Конкретное место можно уточнить при реализации, сохранив единый registry и contract schema.

Каждый registry profile содержит стабильный ID, vendor/model, protocol/version, firmware compatibility, region/currency/encoding, transport/config/UUID/descriptors, capabilities/unsupported commands, источники с version/hash, fixture/scenario IDs и coverage status. Разные несовместимые firmware/протоколы — разные profile versions.

**Добавление поддержки нового устройства обязательно включает:** независимые vendor vectors, stateful emulator либо обоснованно переиспользуемую protocol model с новыми fixtures, transport setup, command/result/error/retry/recovery scenarios, позитивный и негативный тест capabilities, регистрацию в общей матрице и обновлённую документацию. Общий runner обнаруживает profiles через registry; новая модель не требует переписывания orchestration harness. Unsupported operation должна явно отклоняться до I/O.

CI сравнивает enabled runtime device profiles и emulator registry: отсутствующий профиль, обязательный scenario, fixture/source revision либо невыполненный тест делает gate красным. Нельзя включить новую device capability в поддерживаемую сборку, оставив её только в README или пропустив tests. Существующие profiles всегда остаются в regression matrix.

### Критерии приёмки W9

- Production ESP-IDF build и host runtime build проходят на фиксированном toolchain; manifest доказывает использование тех же runtime sources, mocks отсутствуют в production linkage.
- Все начальные поддерживаемые profiles и их обязательные commands, setup, errors и recovery исполняются без unexpected SKIP. Для каждого case сохраняются profile/version, build/source hash, seed, timestamped TX/RX, device state/action counters, expected/actual result и journal assertions; секреты маскируются без потери проверяемости полей.
- Инъекция неправильных LEN/BCC/command mapping/SEQ, пропущенного wake/write completion либо повторной purchase ломает соответствующие тесты. Независимость oracle подтверждается mutation negative controls.
- Cash/card/split сценарии в обоих применимых ingress paths завершаются ожидаемо; при неоднозначном результате остаётся UNKNOWN/RECONCILE. Повторный submit/рестарт не увеличивает device action count и не теряет tender evidence.
- Пример добавления нового fixture/profile через registry прогоняется тем же runner. Включённая runtime модель без emulator coverage отклоняется CI.
- Статус evidence явно **EMULATED_RUNTIME**, отдельно от **PHYSICAL_HIL**; passing mocks не снимает аппаратные/банковские gates.

## Детальные задачи принятия архитектурных решений

- **ADR-A: transport identity.** Topic address = adapter или final device; подписанные fields для обоих, routing health и registry generation. Схема migration старых topics, временная совместимость без double delivery. Только один processor dedup domain.
- **ADR-B: authority/UNP.** Cloud-reserved и local offline range не создают разные UNP для одного intent. Persist counters/generation/UNP до I/O; отдельная нумерация diagnostic/report operations. Conflict не «исправлять» заменой UNP после печати.
- **ADR-C: canonical bytes.** Единый hash/signature preimage с recursive ordering, escaping, numeric representation; exact golden JSON/CBOR fixtures для всех языков. Отдельно protocol bytes и JSON representation; не менять signed payload при retry.
- **ADR-D: physical recovery.** Что доказывает own open/closed receipt; какие запросы безопасны; когда допускаются cancel/VOID/refund; как учитываются несколько CARD; никакого «cancel последний чек» без ownership proof.
- **ADR-E: BLE scope.** Provisioning и transaction services, OPEN_MVP limitations либо secure upgrade, minimum MTU/header, per-connection framing/flow, user-visible READY и expiration. Не обещать encrypted channel на основании Go reference.
- **ADR-F: release gates.** Debug-HIL APK и production binary отдельно; неизменяемые hashes, vendor SDK license/version, externally trusted reviewer, supported case set и срок evidence.

ADR должны дать конкретные schemas/fixtures, а не просто выбор названия. Согласованные изменения OpenAPI/AsyncAPI/generated types/versioned docs входить в тот же пакет, что runtime, когда начнётся кодирование.

## Предварительная приёмка по ступеням

G0–G4 описывают полную программу, включая hardware. Для результата **до подключения устройств** применяется отдельный gate из [A0–A11](07_AUTOMATION_EXECUTION.md): только обязательные no-hardware cases с явным HIL_PENDING.

**G0 — baseline.** Зафиксирован воспроизводимый срез, перечень модулей/profiles, все software suites зелёные, текущие три MiniPOS failures объяснены и закрыты. Не переписывать тесты так, чтобы скрыть operator/CAS регрессии. Обязательные тесты не skipped.

**G1 — protocol conformance.** T-05…T-08 выполняют actual runtime C++/Kotlin с внешними golden bytes. Corrupt length/checksum/sequence/typed error не возвращает успех; partial write классифицируется корректно. Для USB/COM готов descriptor/pinout/line-profile документ. T-25 подтверждает команды всех включённых IoT profiles на USB/COM/BLE эмуляторах и совпадение runtime composition с production build.

**G2 — software transaction/recovery.** T-01/09…T-17 проходят между реальными serializers, broker, PostgreSQL/SQLite и injectable physical transport. В каждом crash window нет повторного debit/receipt, потерянного approved tender или terminal outbox. UNKNOWN не превращается в success/decline без evidence.

**G3 — deployable artifacts и onboarding.** Конкретные IDF firmware, BlueCash demoDebug APK, MiniPOS Android APK и iOS development/ad-hoc build построены и устанавливаются. Из factory reset без ручных NVS правок воспроизводится provisioning → reboot → protocol probe. mTLS/ACL негативные проверки проходят. Build hash записан в evidence.

**G4 — разрешение основной HIL программы.** Все P0 закрыты для включённых profiles, P1 закрыты либо profile явно исключён из приёмки; T-21 physical smoke выполнен для обоих OS и обоих S3 fiscal profiles. Затем полная hardware стратегия из исходного плана, включая сбои питания/сети, длительные циклы и эквайерские сценарии. Неизвестные/внешние ограничения имеют owner и точный требуемый артефакт.

**Production acceptance отдельно.** Нужны manufacturer/acquirer/authorized service/регуляторное evidence и production security gates. Успех G0–G4 не является юридическим заключением или разрешением установки у клиента.

## Definition of Done одной доработки

- GAP ID и контрпример воспроизведены до исправления; бизнес-инвариант сформулирован.
- Изменение включено в фактический runtime, а не только в simulator/helper; новые тесты компилируют этот же путь. Новое устройство/протокол обязательно добавлены в registry эмуляторов W9, wire fixtures и regression matrix T-25.
- Unit + boundary/negative + relevant integration/fault tests проходят; race/concurrency проверки для изменённых coordinators, где применимо.
- Обновлены contract versions, generated artifacts, README/runbook и trace links. Старые неверные «готово» claims удалены/помечены историческими.
- Reviewer видит exact build ID, команды, exit codes, required cases и артефакты. Evidence проверяется внешним trust anchor.
- Если аппаратная часть не проверена, статус `SOFTWARE_VERIFIED_HIL_PENDING` только для конкретного закрытого profile, без blanket claim на всю систему.
