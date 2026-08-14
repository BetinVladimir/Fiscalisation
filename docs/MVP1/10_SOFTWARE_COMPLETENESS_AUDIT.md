# Повторный аудит полноты software MVP1

Дата повторной проверки: 2026-08-13.  
Проверено рабочее дерево с aggregate finalize, P1363, ping/failover и durable BlueCash payment journal.  
Решение: **утверждение «реализовано и протестировано всё, кроме реального
аппаратного тестирования» НЕ ПОДТВЕРЖДЕНО**.

## 1. Сводка

| Область | Статус software | Доказательство |
|---|---|---|
| Domain/API/SUPTO foundation | PARTIAL/PASS существующего surface | Go tests, contract tests, SUPTO/BG verifier |
| Platform device API | PASS после исправления verifier | 73 canonical + 47 runtime contract surface |
| Lightweight ping | PASS базовый | handler/unit tests, без DB/business service, отдельный limiter |
| MiniPOS failover controller | PARTIAL | unit state machine есть; нет фонового scheduler, durable transport epochs и полного cross-channel continuation |
| Client UUID / aggregate checkout | PARTIAL | finalize UUID и receipt UUID durable создаются MiniPOS до I/O; UUID остальных mutations и cross-channel webhook proof ещё неполны |
| BlueCash physical software adapter | PASS software / HIL pending | release composition использует `DatecsAndroidFiscalPort`, `DatecsAndroidPaymentPort` и `BoricaPinpadCodec`; vendor/acquirer HIL отсутствует |
| Aggregate receipt/payment saga | PASS software / HIL pending | `SALE_FINALIZE`, DDL, state machine, one-command MiniPOS path, ordered payments, device journal и compensation реализованы и проходят restart/fault regression |
| Edge-agent-s3 runtime | FAIL | `main.cpp` имеет hardcoded Daisy/UART DEV profile и `loop(delay(1000))`; MQTT/GATT/orchestrator отсутствуют |
| DP-150 COM profile | FAIL | protocol library есть, но firmware runtime/config/saga отсутствуют |
| BluePad BLE profile | FAIL | pairing/runtime/payment journal/composite saga отсутствуют |
| Daisy Compact USB profile | FAIL | USB host runtime/enumeration/recovery отсутствуют |
| BeeFiscal composite configuration | PARTIAL | базовый device UI есть; нет завершённой compatibility/composite endpoint workflow |
| Full stub E2E all profiles | PARTIAL | общие driver/protocol stubs и full two-compose checkout/fault suite проходят; нет firmware-level three-profile transport harness |

## 2. Статус обязательных P0

| P0 | Статус | Фактический разрыв |
|---|---|---|
| 001 ECDSA wire | PARTIAL | DER преобразуется в P1363 и JVM vectors проходят; cross-language Android Keystore instrumentation/evidence ещё требуется |
| 002 physical readiness | FAIL | MQTT bridge probe подтверждает broker, не FU/FMIN/pinpad |
| 003 SALE_FINALIZE | PASS software | MiniPOS вызывает один finalize; backend, MQTT, BlueCash processor и durable receipt saga покрыты contract/restart/fault tests |
| 004 payment journal | PASS software / HIL pending | Android SQLite хранит PREPARED/APPROVED/REVERSED, amount/RRN/auth и restart compensation; vendor lookup semantics требуют HIL |
| 005 lossless payload | PASS software | discount/unit/department и ordered payments проходят MQTT parser; HIL print comparison остаётся отдельно |
| 006 BlueCash bridge | PASS software / HIL pending | release composition gate доказывает выбор Android Fiscal/Pinpad IPC и BORICA codec; реальные SDK/firmware/acquirer ответы требуют HIL |
| 007 dispatcher catalog | FAIL | reports/cash/clock/probe не проходят единый async MQTT dispatcher |
| 008 route resolver | FAIL | backend всё ещё выбирает global driver; composite route resolver отсутствует |
| 009 operator/FMIN/time | FAIL | vendor operator/password local config; physical FMIN/time end-to-end не замкнуты |
| 010 MQTT stack/ACL | PARTIAL | TLS URI checks есть; воспроизводимый broker/ACL stack и tests отсутствуют |
| 011 saga materialization | FAIL | sync materializer не хранит receipt/payment/step/compensation projection |
| 012 contracts/DDL/trace | FAIL | нет required aggregate/composite/message schemas и таблиц нового scope |
| 013 Platform API router | PASS | операции реализованы; verifier исправлен для `/platform/v1` |
| 014 automatic failover | PARTIAL | ping/controller tests есть; checkout делает inline probes, нет полного 23-case protocol и continuation |
| 015 client UUID | PARTIAL | HTTP key UUID появился; нет atomic end-to-end client ID contract/index/device journal/webhook proof |

## 3. Прямые свидетельства отсутствующего кода

1. `edge-agent-s3/src/main.cpp` выбирает hardcoded Daisy UART и не запускает
   MQTT, BLE GATT, USB host или command orchestration.
2. В firmware отсутствуют runtime references для BluePad/DP-150/Daisy USB
   profiles, кроме документации/protocol abstractions.
3. `contracts/roadmap-stage-acceptance.json` всё ещё обозначает DP-150 track как
   `FORMALLY_EXCLUDED_EXTERNAL`, что конфликтует с новым обязательным `index.md`.
4. Полный regression и отдельная PlatformIO release-сборка проходят, но сборка
   подтверждает только компилируемость текущего firmware skeleton, а не наличие
   обязательного transport/orchestrator runtime.

## 4. Значение зелёных тестов

Зелёные unit/contract/SUPTO tests подтверждают корректность **реализованного
среза**, но не полноту требований. Тест не может доказать отсутствующий runtime.
До добавления machine trace `MVP1-P0 → implementation symbol → test → stub E2E`
общий regression не должен формировать `SOFTWARE_MVP_COMPLETE`.

## 5. Что требуется до статуса «остался только HIL»

1. Реализовать firmware MQTT command/sync runtime, BLE GATT direct route и один
   общий command orchestrator поверх durable journal.
2. Реализовать composite DP-150 UART + BluePad BLE runtime profile.
3. Реализовать Daisy USB host profile с enumeration/reconnect.
4. Завершить route-per-binding resolver и BeeFiscal composite workflow.
5. Добавить firmware-level deterministic three-profile transport/fault harness.
6. Обновить machine trace и acceptance statuses после прохождения этих gates.

Два последовательных `make full-regression` и отдельный `pio run` прошли
2026-08-13. Это устраняет прежние contract/protocol/BlueCash build-дефекты, но
не заменяет отсутствующий ESP32 transport runtime.

## 6. Реализация следующего software-среза

После аудита добавлены и отдельно проверены:

- `EdgeRuntime` — единый journal-before-I/O coordinator для MQTT/BLE ingress,
  ordered card tenders, fiscal receipt и compensation/RECOVERY_REQUIRED;
- строгая проверка двух edge profiles: DP-150/RS-232 + BluePad/BLE и
  Daisy Compact/USB без несовместимого terminal;
- `BindingDriverResolver` с ключом tenant/register/device/binding generation и
  тестами отсутствия cross-tenant, cross-register и stale-generation fallback;
- versioned BeeFiscalApp compatibility matrix и fail-closed tests;
- успешная PlatformIO release-сборка нового coordinator.

Provisioning envelope теперь реализован в `ProvisionedBindingStore`: trust anchor
закрепляется build pipeline в `PinnedDeviceCA.h`, подпись проверяется до записи,
а generation может только увеличиваться. Сборка без pinned CA fail-closed.
BlueCash Android также проверяет `binding_signature` до сохранения credential.

Backend повторно сверяет immutable device snapshot непосредственно перед
finalize/payment/reversal, включая device ID, generation, serial, FMIN, vendor и
model. Отдельный `BindingDriverResolver` покрывает тестами отсутствие fallback
между tenant/register/device/generation.

Остающиеся software-разрывы после этого среза: реализация ESP32 open MVP GATT
с payload binding validation и durable deduplication, MQTT TLS client/task, sync ACK
task, BluePad BLE central link, Daisy USB-host CDC link и их подключение к
firmware boot/runtime.

Дополнительная проверка официальных Espressif APIs установила:

- `SecureBleSession` и X25519/AES-GCM не входят в MVP gate согласно явному
  исключению в `index.md`; golden vectors сохраняются для production gate;
- Arduino `USB/USBCDC` в используемом framework работает в USB **device** mode,
  а Daisy требует USB **host** CDC-ACM/VCP. Поддержанный путь — ESP-IDF USB Host
  Library + managed `cdc_acm_host` component и отдельный daemon/client task;
- до миграции firmware build на этот component path Daisy USB runtime не может
  считаться программно реализованным.

Только после этого допустим статус `SOFTWARE_COMPLETE_HIL_PENDING`. Сейчас
корректный статус: `SOFTWARE_INCOMPLETE_AND_HIL_PENDING`.
