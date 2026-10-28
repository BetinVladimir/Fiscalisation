# Журнал реализации и проверок

Дата: 2026-10-17. Исходный аудит: `cfff038` + пользовательские изменения; общий сохранённый baseline: `f5a6d1ff43ef9c01a0c8aa588319097e1c024c49`. Последующие изменения пока находятся в рабочем дереве. **SOFTWARE_GAPS_OPEN**. Этот журнал не заменяет final-snapshot manifest, HIL или регуляторное подтверждение.

## Принятые изменения

| GAP | Реализовано | Реальная проверка | Что ещё не доказано |
|---|---|---|---|
| 001, 024 | MQTT topic exact `tenants/{tenant}/devices/{adapter}/commands`; adapter ID и applied generation сохраняются в sale snapshot отдельно от fiscal endpoint и register version; HMAC использует credential адаптера; server_sale_id содержит внутренний sale.ID | Go publisher tests с разными fiscal/adapter IDs, generation7/registerVersion99, derived HMAC, internal/external sale IDs; domain composite snapshot после изменения register version | Broker+S3/BlueCash сквозная доставка, AsyncAPI/ACL согласование, исторические composite snapshots без adapter fields, полная SALE_FINALIZE/reversal/offline materialization |
| 002 | Подписка `ackBase/+`, один непустой suffix; reset всех assembly flags; проверки total/offset/topic/null/size и disconnect | Actual mqtt_runtime.cpp с fake ESP transport: корректные/чужие/bare/nested topics, fragmented ACK, ошибки фрагментов, bounds8192 | End-to-end подпись ACK/hash/cursor/frozen batch и stale ACK |
| 009, 010 | C++ Daisy LEN/BCC nibble+30/RX; Datecs LEN TX+2a/RX+33; границы Daisy DATA200, Datecs TX496/RX480 | Независимые literal vendor TX/RX, truncation, повреждение каждого байта, границы; actual FrameCodec; native response mock приведён к вендорному layout | Serial command semantics, status/deadline/retry и физические профили |
| 010 | Kotlin Datecs LEN+2a/+33, RX480; найденная дополнительная ошибка BCC исправлена: PST05 входит в сумму | Actual DatecsProtocol.kt на JVM, те же vendor bytes, socket command correlation, bounds/noise | Android production linkage/APK/HIL и остальные transport gaps |
| 011, 012 | SerialSession связан с actual EspIdfDeviceIo: session SEQ, bounded SYN/NAK/timeout retries тем же packet/SEQ, partial write без resend, профильные status/error checks | Actual serial FSM + FrameCodec + injected clock/transport: lost reply сохраняет actions1, NAK, SYN deadline, fragmentation/correlation, bounds, status/error, SEQ wrap | Durable SEQ после reboot, полная device command/recovery matrix, command-specific timing HIL |
| 017, 018, 019 | Fail-closed EXECUTING checkpoint; SQLite PRAGMA выполняется до transaction; execute и recovery фиксируют operation+receipt+outbox атомарно; CARD references не перезаписываются общим результатом; recovery errors возвращаются; event IDs содержат result hash | Actual IntentProcessor + DurableStorage + настоящий SQLite: 16 сценариев, triggers отказов checkpoint/receipt/payment/outbox, event collision, replay, split references, recovery UNKNOWN→COMMITTED, reopen | Полноценное multi-tender recovery, payment checkpoints внутри executor, readiness/owner checks при recovery, fsync/power-cut/VFS failures |
| 029, 030 | `make iot-test` включает actual MQTT и SQLite tests; устаревшая durability token-check удалена; SmartDevices gate использует prod/demo debug tasks, Kotlin host test отдельно | Общий IoT gate PASS; `smart-device-host-test` PASS; Android unit/build PASS:4 debug APK prod/demo | Полный required case inventory, final manifest/mutations и cross-build всех целевых платформ |

CARD references сохраняются как individual executor evidence. Это не доказательство корректного восстановления двух карточных tender. Safe блокировка неопределённой операции также не заменяет реализованный lookup/reconciliation.

## Выполненные команды на объединённом рабочем коде

- Из корня: `make iot-test` — PASS: 32 прежние Python structural checks, native saga/profile, actual MQTT runtime, 16 actual SQLite scenarios, Daisy/Datecs/DatecsPay common drivers, facade и runtime vendor codecs. Одна прежняя проверка наличия строк заменена исполняемыми SQLite regressions.
- Из `fiscal-backend`: `GOCACHE=/private/tmp/fiscal-routing-go-cache go test ./...` — PASS всех пакетов. Локальные HTTP test listeners требуют запуска за пределами sandbox.
- Из корня: `python3 SmartDevices/bluecash-app/tests/host/run-datecs-codec.py` — PASS. Runner требует кешированные pinned Kotlin2.0.21 JVM dependencies; Android SDK не подменяется этим тестом.
- Из корня: `JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ANDROID_HOME=/Users/freelancer/Library/Android/sdk make smart-device-test` — PASS:185 Gradle tasks,4 debug APK. Daisy debug APK остаётся stub; это не реальная Daisy integration и не release-ready приложение.
- Из корня: `bash IoT/firmware/edge-agent-s3/idf/tests/run_native_serial.sh` — PASS. ESP32-S3 cross-build выполняется отдельно.
- `git diff --check` — PASS для текущих tracked изменений.

Исходные локальные логи: `/private/tmp/fiscal-integrated-iot-20261017.log`, `/private/tmp/fiscal-integrated-backend-20261017.log`; baseline-контрпримеры и mutation evidence хранятся в рабочих каталогах пакетов `/private/tmp/fiscal-l*-20261017/`. Временные пути не являются долговременным release evidence: перед final gate сохранить manifest, source/artifact hashes и логи в согласованном evidence archive.

## Дополнительные исправления после baseline

- **GAP035 — MQTT outcome isolation:** результат проверяется по immutable tenant/device исходного outbox до terminal replay. Чужие tenant/device и небезопасные legacy physical snapshots отклоняются; composite credential/generation обязательны. Полный fiscal-backend Go suite PASS после изменений. Broker/ACL E2E остаётся открытым.
- **GAP036 — JSON результата:** ручной буфер1024 заменён cJSON; длинные ссылки, кавычки, переносы и backslash сохраняются без повреждения. Два actual SQLite regression сценария проверяют execute/recovery и reopen, всего16 сценариев. Контрпример на baseline воспроизводился.
- **GAP026 — browser outbox:** atomic prepare/claim в настоящих IndexedDB transactions; UNKNOWN восстанавливается только lookup исходного route, без повторного submit. Scope/digest/semantic identity и terminal evidence защищены от overwrite. MiniPOS Web:99 unit tests и10 Chromium tests на actual IndexedDB PASS; production web build PASS. Настоящие CLOUD/BLE lookup ещё требуют завершения; safe-block не означает recovery.
- **GAP028 — MiniPOS API fixtures:** тесты используют публичную employee identity и cashier workstation session; негативные проверки impersonation сохранены. Полный MiniPOS backend Go suite PASS.
- **GAP006/007 — BLE:** устранён duplicate HCI init для pinned NimBLE; startup ownership защищён CAS; connection generation защищает повторно использованный handle. Assembled ATT writes ограничены512 bytes, provisioning допускается приMTU23, monetary BFF требуетMTU61. Actual runtime host tests, включая deterministic concurrent start/reconnect cases, PASS в пакете; ASan/UBSan иTSan PASS. RTOS timing и radio/HIL ещё не доказаны.
- **GAP031 — factory:** manifest/hash/partition validation до subprocess; staged private bytes исключают замену файлов между проверкой и flash. Добавлен `make factory-test`; generator release manifest ещё требуется. Физическая заливка не выполнялась.
- Добавлены gate targets `minipos-outbox-test` и `factory-test`. Финальная проверка IoT+factory и повторная S3 cross-build на последнем BLE коде выполняются отдельно; предыдущие успешные сборки не являются доказательством последующего snapshot.

## Ближайшая приёмка

1. Завершить настоящий CLOUD lookup с company-specific credential и строгим сопоставлением operation/session/sale/register; для BLE определить read-only lookup protocol и reconnect lifecycle.
2. Заморозить MQTT batch до публикации, обеспечить exact ACK и атомарное подтверждение с restart/fault tests; отдельно закрыть GAP022 canonicalization.
3. Проверить последний объединённый IoT/factory код и S3 cross-build; сохранить source/artifact fingerprints и логи в долговременном evidence archive.
4. Продолжить bootstrap, multi-tender recovery, SEQ persistence и остальные исходные GAP. Полная готовность к HIL и НАП не заявляется.

## Wave2 integration

Долговременные текстовые evidence: [evidence/wave2/README.md](evidence/wave2/README.md). Это промежуточный snapshot с открытыми GAP, не final gate.

- MQTT frozen batch интегрирован: exact signed bytes/UUID переживают retry/restart; ACK подтверждает только frozen sequence/hash и неизменённые events; cursor transaction не удаляет поздние rows.10 actual sync scenarios PASS, включая duplicate/conflict/concurrency/fault/int64. Duplicate event replay больше не расходует journal sequence.
- Последний общий IoT/factory gate и S3 cross-build после sync PASS. GAP022 воспроизведён независимо: порядок nested maps, Unicode/HTML escaping и float rendering отличаются от Go. Serializer отдельно добавлен, callsite integration ещё требуется.
- CLOUD lookup интегрирован: авторизованный GET с company credential, strict operation/session/sale/register/tenant checks;113 web tests, полный MiniPOS backend и production web build PASS. Missing-sale recovery/projection и proof-aware UI ещё проверяются отдельно.
- Factory bundle generator интегрирован;16 factory tests PASS. Actual S3 binary bundle сформирован и прошёл тот же validator, что flash. Никаких обращений к физическому устройству не было.
- Datecs CARD mapping уточнён по вендору:1=credit,2=debit. Универсальное изменение1→2 было бы необоснованным; нужен явный профиль device payment configuration, включая Daisy programmed slots.
