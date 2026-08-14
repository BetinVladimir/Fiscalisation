# SOFTWARE_COMPLETE_HIL_PENDING — актуальный остаток работ

Дата пересчёта: 2026-08-13  
Область аудита: фактическая реализация `/Users/freelancer/Documents/Beeloy/Fiscalisation`
и требования `docs/MVP1/index.md`, `docs/MVP1/09_DUAL_ROUTE_FAILOVER_PROTOCOL.md`,
`docs/SUPTO/index.md`.

## 1. Итог

Текущий статус:

```text
SOFTWARE_COMPLETE_HIL_PENDING
```

### Финальная программная проверка — 2026-08-13

`SC-01`…`SC-08` закрыты. Два последовательных запуска
`scripts/run_mvp1_software_gate.sh` из clean ESP-IDF build завершились PASS.

Подтверждено:

- ESP-IDF 5.4 firmware дважды с нуля собран и слинкован в ELF/BIN для ESP32-S3;
- RAM 11,9%, Flash 45,8%; в исполняемом command path нет P0 placeholders;
- firmware contract tests 32/32 и native receipt fault matrix PASS;
- Datecs, DatecsPay, Daisy и protocol-abstraction suites PASS;
- fiscal-backend, MiniPOS backend и edge-agent Go suites PASS;
- MiniPOS TypeScript 42/42, BeeFiscalApp 8/8, UI matrix 42/42 PASS;
- OpenAPI/runtime/generated-contract/AsyncAPI surface gates PASS;
- recovery выполняет vendor lookup/cancel и восстанавливает canonical receipt,
  terminal reference, RRN и auth из SQLite после reboot;
- composite binding имеет atomic repository operation, typed PostgreSQL path,
  adapter-only applied-generation ACK и BeeFiscalApp disable/rebind/status UI;
- MiniPOS передаёт один агрегированный finalize с одним долговечным UUID и тем
  же receipt/payment plan через REST и BLE.

Открытых программных P0 для этого статуса нет. Остаются только HIL и отдельно
принятые production-hardening задачи, перечисленные в разделе 14.

### Усиленный повторный аудит — 2026-08-13

После независимого пересмотра прежний gate был расширен и повторён. Устранены
обнаруженные ложноположительные доказательства и программные разрывы:

- MiniPOS сохраняет aggregate checkout в AsyncStorage **до первого I/O**;
  restart, REST retry и REST→BLE continuation используют неизменные
  `client_operation_id`, `receipt_session_id` и payment UUID;
- journal удаляется только после доказанного `FISCALIZED`, `FAILED` или
  `CANCELLED`; transport/unknown outcome блокирует новый checkout и требует
  reconciliation;
- browser E2E использует ровно один `/sales/{id}:finalize` для cash/card/split,
  проверяет совпадение `Idempotency-Key` и `client_operation_id` и запрещает
  возврат к циклу отдельных payment-intents;
- native receipt fault matrix исполняет BlueCash, DP-150+BluePad и Daisy
  profiles: cash/card/split, decline, unknown/recovery, compensation и fiscal
  unknown/recovery; Daisy card без terminal fail-closed;
- BeeFiscal browser E2E исполняет composite create→PENDING→disable→rebind, а
  backend test — adapter exact-generation apply ACK→ACTIVE→route;
- исправлен runtime OpenAPI aggregate response: persisted
  `client_operation_id`/`receipt_session_id` объявлены в closed schema;
- исправлен MiniPOS storno payload: передаётся immutable
  `original_fiscal_reference`;
- PostgreSQL typed sale read сохраняет `fiscal_device.binding_version`, поэтому
  последующий storno не теряет immutable route generation.

Усиленный `scripts/run_mvp1_software_gate.sh` дополнительно запускает Android
SmartDevice release build/tests, fault/soak/security, browser interaction,
PostgreSQL integration, compose validation и полный two-compose E2E. Gate
завершён PASS и сохраняет toolchain manifest, ESP-IDF ELF/BIN и Android APK в
`MVP1_EVIDENCE_DIR`.

### Закрытие повторно найденных разрывов — 2026-08-13

- MiniPOS восстанавливает pending checkout из AsyncStorage после restart и
  использует сохранённый `client_operation_id` для lookup/reconcile даже если
  HTTP-ответ был потерян до появления operation ID в sale projection.
- Native profile matrix исполняет production `ProfileExecutor` для
  DP-150+BluePad и Daisy: cash/card/split, decline/unknown, compensation,
  unavailable endpoints, X/Z, cash in/out и fiscal storno. Vendor codecs и
  transport lifecycle дополнительно проверяются соответствующими protocol и
  ESP-IDF transport suites.
- PostgreSQL integration выполняет composite create→exact-generation apply
  ACK→restart→route→disable→rebind и проверяет сохранение applied generation.
- Software gate сам выполняет два последовательных запуска, сохраняет полный
  log каждого запуска, Go JSON test events, TAP, coverage, contract/toolchain
  manifests, firmware/APK и SHA-256 manifest. Итоговый `PASS` создаётся только
  после успеха обоих запусков.

### Строгая повторная верификация — 2026-08-13

- ESP-IDF native profile harness теперь непосредственно компилирует production
  `ProfileExecutor` вместе с общими Datecs/Daisy payload builders и frame
  decoders; fake строит корректный ответный byte frame, проверяет correlation и
  BCC, а corrupt-frame сценарий обязан завершиться ошибкой parser.
- BlueCash выделен в самостоятельный Android fault profile на production
  `BlueCashCommandProcessor`, `DatecsFrameCodec`, fiscal payloads и durable
  journal; проверяются success, decline, timeout после send, restart,
  idempotent duplicate, compensation и потеря business ACK.
- Добавлен единый cross-channel contract для всех трёх профилей: один
  operation/receipt/payment identity продолжается после lost cloud response в
  BLE journal и синхронизируется MQTT batch. Повтор committed batch вследствие
  потерянного ACK возвращает сохранённый business ACK без второй операции или
  webhook.
- Gate сохраняет SHA-256 manifest точного проверенного source tree, состояние
  Git, JUnit BlueCash и fail-closed проверяет обязательные evidence artifacts.
  Поэтому dirty working tree допустим только как явно хешированный audit input,
  а не ошибочно выдаётся за commit из `git rev-parse HEAD`.

## 2. Что уже закрыто и не должно реализовываться повторно

| Компонент | Фактическое состояние |
|---|---|
| ESP-IDF target | ESP-IDF 5.4, отдельный build target, managed USB CDC ACM component |
| SD/SQLite foundation | SDMMC mount, SQLite DDL, FULL synchronous journal, operation/receipt/payment/outbox/cursor tables |
| Durable invariants | reserve-before-I/O API, общий operation-id dedupe, payload conflict, recovery query, ACK cursor, retention floor 90 дней, запрет удаления unacknowledged events |
| MQTT foundation | `mqtts://`, client certificate/key, persistent session, QoS1 subscribe/publish |
| POS BLE peripheral foundation | открытый MVP GATT server с command/event characteristics |
| Daisy USB foundation | USB Host + CDC ACM discovery/read/write/reconnect primitive |
| MiniPOS checkout | один агрегированный `:finalize`, устойчивые UUID операции/receipt/payment, REST contract tests |
| MiniPOS failover foundation | lightweight HEAD ping, hysteresis controller, Web/Android BLE clients, `OPEN_MVP` route package |
| Backend fiscal domain | aggregate receipt command, route/outbox/MQTT bridge, sync/ACK primitives, webhook path |
| Protocol libraries | Datecs fiscal, DatecsPay и Daisy builders/parsers и protocol facade существуют вне IDF target |

Все перечисленные primitives подключены к рабочему runtime. `DurableStorage`
вызывается единым command processor до физического I/O для MQTT и BLE.

## 3. Принятые исключения MVP

Следующее **не блокирует** `SOFTWARE_COMPLETE_HIL_PENDING`:

- авторизация и шифрование POS→adapter BLE: для MVP действует `OPEN_MVP`;
- X25519/HKDF/AES-GCM transport handshake — production hardening;
- ESP32 Secure Boot и Flash Encryption — production gate;
- реальные сертификаты broker, Wi-Fi credentials и production secrets;
- vendor/acquirer approval, реальные merchant credentials;
- электрические, радиочастотные и реальные аппаратные испытания;
- производительность/ресурс SD-карты на реальном носителе.

Открытый BLE не отменяет binding validation, UUID/digest idempotency,
journal-before-I/O и fail-closed при недоступном конечном ФУ.

## 4. Порядок реализации

```text
SC-01 binding/config
   -> SC-02 common intent processor
      -> SC-03 BLE/MQTT reliable ingress+egress
         -> SC-04 DP-150+BluePad profile
         -> SC-05 Daisy profile
   -> SC-06 backend + BeeFiscalApp composite binding
      -> SC-07 cross-channel MiniPOS flow
         -> SC-08 regression/evidence gate
```

Нельзя начинать physical I/O из BLE/MQTT callback. Callback только собирает и
валидирует transport envelope и передаёт intent в одну bounded worker queue.

## 5. SC-01 — ESP-IDF provisioned binding и runtime configuration

**Цель:** убрать `BeeFiscal-unprovisioned`, нулевой USB binding и пустой MQTT
config из runtime path; запускать только разрешённый versioned composite profile.

**Основные пути:**

- `IoT/firmware/edge-agent-s3/idf/main/app_main.cpp`;
- новые `idf/main/provisioned_binding.{h,cpp}` и `runtime_config.{h,cpp}`;
- существующая семантика из `edge-agent-s3/src/ProvisionedBinding.cpp`;
- `fiscal-backend` activation/binding endpoints и OpenAPI;
- `docs/MVP1/05_BACKEND_DEVICE_ROUTE_SELECTION.md`.

**Сделать:**

1. Определить versioned binding: tenant/location/register/edge IDs,
   `binding_generation`, profile, fiscal endpoint, optional payment endpoint,
   MQTT topics/identity, BLE advertising identity и transport parameters.
2. Перенести decoder/validator в ESP-IDF без Arduino `String`; неизвестные поля
   допускаются только согласно версии schema, неизвестная версия fail closed.
3. Хранить active и previous generation в NVS; применять новую generation
   атомарно. Rollback generation и несовместимый profile запрещать.
4. Загружать Wi-Fi/MQTT CA/client credential references и endpoint parameters.
   Secrets не логировать и не помещать в SQLite event payload.
5. Проверять capability matrix:
   `DATECS_DP150_BLUEPAD50` = fiscal RS232 + optional/required-by-card BluePad BLE;
   `DAISY_COMPACT_S01` = Daisy USB, без BluePad в MVP.
6. Не поднимать command BLE/MQTT, пока SD/SQLite и binding не готовы. При
   unprovisioned состоянии разрешён только ограниченный activation path.

**Тесты:** version/schema golden files, corrupt/truncated NVS, rollback,
cross-tenant binding, illegal endpoint combination, atomic power-loss update,
secret-redaction test.

**PASS:** после reboot ровно один валидный profile воспроизводится из NVS;
transport получает ненулевую конфигурацию; неверный binding не открывает command
execution; тесты работают без hardware.

## 6. SC-02 — единый ComplianceIntent processor и durable receipt saga

**Цель:** заменить `app_main.cpp::command() -> ESP_ERR_NOT_FINISHED` общей
business state machine для MQTT и BLE.

**Основные пути:**

- новые `idf/main/intent_codec`, `intent_processor`, `receipt_saga`,
  `execution_worker`;
- `idf/main/durable_storage.{h,cpp}`;
- `IoT/protocol-abstraction` и `IoT/common-modules/{datecs,datecspay,daisy}`;
- canonical contracts из `docs/MVP1/01_MVP_REQUIREMENTS_AND_CODEGEN.md` и
  `09_DUAL_ROUTE_FAILOVER_PROTOCOL.md`.

**Сделать:**

1. Декодировать versioned canonical intent: route snapshot, UUIDv4
   `client_operation_id`, receipt/payment IDs, payload digest, ordered items и
   payments, EUR, command kind и expected binding generation.
2. Пересчитать canonical SHA-256 на устройстве; не доверять переданному digest.
3. До любого device I/O атомарно вызвать `reserve_with_received_event()`.
   Duplicate+same digest возвращает сохранённый результат; same UUID+different
   digest возвращает `IDEMPOTENCY_PAYLOAD_CONFLICT`.
4. Реализовать состояния receipt/payment/operation и transitions:
   RESERVED, EXECUTING, CARD_UNKNOWN, FISCAL_OPEN, COMMITTED, COMPENSATING,
   COMPENSATED, RECOVERY_REQUIRED, REJECTED.
5. Разложить агрегированный чек в последовательность: readiness → card
   authorization/lookup → fiscal open → items → ordered payments → close. При
   доказанном failure выполнить cancel/reversal; unknown никогда не повторять
   вслепую.
6. Сохранять STAN/RRN/auth/reference до fiscal close; append result/event в
   outbox до ответа клиенту.
7. На старте обработать `recovery()`: lookup/reconcile незавершённых terminal и
   fiscal steps, сохраняя immutable route snapshot.
8. BLE и MQTT возвращают один canonical result/error vocabulary.

**Тесты:** fake clock + fake drivers; cash, card approve/decline/timeout/unknown,
split tender, duplicate через другой transport, digest conflict, crash на каждой
границе, restart recovery, compensation failure, unavailable final device.

**PASS:** ни один fake-driver call не происходит до durable reservation; один
UUID создаёт не более одного physical side effect через BLE+MQTT; после restart
каждое состояние приходит к COMMITTED/COMPENSATED/RECOVERY_REQUIRED.

## 7. SC-03 — надёжные BLE и MQTT ingress/egress, sync и backend ACK

**Цель:** обеспечить перенос полных intents/results больше ATT/MQTT fragment и
гарантированную асинхронную синхронизацию outbox.

**Основные пути:**

- `idf/main/ble_runtime.cpp`, `mqtt_runtime.cpp`, `edge_runtime_port.h`;
- `minipos/BeeMiniPOS/src/{webBle,nativeBle.native}.ts`;
- `fiscal-backend/internal/mqttclient` и sync API/domain;
- AsyncAPI/CBOR schemas в `contracts`.

**Сделать:**

1. Добавить BLE application framing: protocol version, message UUID, total
   length, offset/sequence, payload digest, final flag. Поддержать negotiated MTU,
   bounded reassembly, timeout, duplicate/out-of-order fragments и backpressure.
2. Фрагментировать event notifications; реализовать result correlation и flow
   control. Обновить Web и Android MiniPOS одним golden-vector codec.
3. MQTT собирать `MQTT_EVENT_DATA` по `current_data_offset/total_data_len`, а не
   принимать только single-fragment messages. Проверять exact topic и bounds.
4. Реализовать Wi-Fi lifecycle, reconnect и bounded command worker queue.
5. Создать sync task: читать `pending()` строго по sequence, публиковать QoS1,
   записывать attempt, повторять с bounded exponential backoff+jitter.
6. Не считать MQTT PUBACK business ACK. Обрабатывать подписанный/tenant-bound
   backend ACK с cursor/ack ID, после проверки вызывать `acknowledge_through()`.
7. Периодически выполнять `prune_acknowledged(now, >=90)`; unacknowledged не
   удалять даже при нехватке места, а блокировать новые операции понятной ошибкой.
8. Backend должен дедуплицировать sync events и повторно выдавать тот же ACK.

**Тесты:** BLE fragmentation golden vectors TypeScript↔C++; malformed/oversize;
MQTT multi-fragment; disconnect before/after publish/PUBACK/business ACK; reboot;
duplicate sync; cursor jump; broker reconnect; full-storage fail closed.

**PASS:** 8 KiB intent/result проходит оба transport; outbox очищается только
после backend business ACK; повтор/переключение transport не создаёт side effect.

## 8. SC-04 — `DATECS_DP150_BLUEPAD50` ESP-IDF profile

**Цель:** подключить DP-150 MX по UART/RS-232 и BluePad-50 Plus как ESP-IDF BLE
central к одной receipt saga.

**Основные пути:**

- новые `idf/main/transports/uart_stream` и `bluepad_ble_central`;
- новые ESP-IDF-compatible adapters под `IoT/protocol-abstraction`;
- `IoT/common-modules/datecs`, `IoT/common-modules/datecspay`;
- `docs/MVP1/04_EDGE_AGENT_S3_EQUIVALENT_TRACK.md`.

**Сделать:**

1. Реализовать настраиваемый UART port/pins/baud/data/parity/stop/timeout и
   RS-232 transceiver profile; incremental Datecs frames, status bytes, retry и
   reconnect без Arduino `Stream`.
2. Перенести/обернуть fiscal adapter так, чтобы собирался текущим IDF CMake и
   выполнял обязательные readiness, receipt, cancel/storno, X/Z, cash in/out,
   lookup команды.
3. Реализовать NimBLE central отдельно от POS GATT server: discovery по
   provisioned identity, service/characteristics, connection state, MTU,
   DatecsPay framing, timeout, reconnect.
4. Реализовать payment purchase/status/lookup/reversal и сохранять terminal
   references через `DurableStorage`.
5. Связать оба endpoint в одной saga: CARD без ready terminal запрещён; CASH
   допускается при исправном ФУ; approved card + fiscal failure запускает
   reversal; unknown требует lookup.

**Тесты:** UART byte-stream fake с partial frames/status errors; NimBLE central
fake approve/decline/timeout/unknown/reconnect; combined receipt fault matrix;
compile/link test, доказывающий использование реальных protocol builders/parsers,
а не дублирующего stub grammar.

**PASS:** полный cash/card/split/storno/X/Z сценарий проходит на driver fakes;
faults дают детерминированную compensation/recovery; реальное устройство не
требуется и evidence помечено `SIMULATED`.

## 9. SC-05 — `DAISY_COMPACT_S01` ESP-IDF USB profile

**Цель:** довести существующий USB CDC primitive до Daisy fiscal adapter.

**Основные пути:**

- `idf/main/usb_cdc_runtime.cpp`;
- новый `idf/main/adapters/daisy_fiscal_adapter`;
- `IoT/common-modules/daisy` и `IoT/protocol-abstraction`.

**Сделать:**

1. Загружать VID/PID/interface и serial identity из binding; при нескольких
   совпадениях выбирать только точное provisioned устройство, иначе fail closed.
2. Исправить lifecycle resource ownership: open/close/reopen, RX overflow,
   disconnect during TX/RX, task shutdown и повторную enumeration.
3. Адаптировать USB byte stream к Daisy frame encoder/parser, sequence/status и
   timeout policy из protocol library.
4. Подключить readiness, serial/FMIN/time, receipt/items/payments/close,
   cancel/storno, X/Z, cash in/out и lookup/recovery к общей saga.
5. USB disconnect после send переводить в UNKNOWN/RECOVERY_REQUIRED до lookup,
   а не выполнять автоматический повтор.

**Тесты:** fake CDC enumeration, partial/corrupt frames, wrong VID/PID/serial,
disconnect на каждой I/O границе, reconnect и restart recovery; protocol golden
frames из Daisy tests.

**PASS:** Daisy profile проходит тот же fiscal scenario contract на fake CDC;
ни один unknown result не повторяется; IDF target использует общий Daisy parser.

## 10. SC-06 — composite binding API, persistence и BeeFiscalApp UI

**Цель:** администратор создаёт и меняет один versioned edge binding с fiscal и
optional payment endpoints; backend маршрутизирует чек по immutable snapshot.

**Основные пути:**

- `fiscal-backend/contracts/openapi-runtime-v1.yaml` и generated contracts;
- `fiscal-backend/internal/{api,domain,persistence,mqttclient}`;
- `BeeFiscalApp/App.tsx`, `BeeFiscalApp/src/deviceProfiles.ts`.

**Сделать:**

1. Добавить/завершить OpenAPI resource `CompositeDeviceBinding` вместо хранения
   только независимых `fiscal_device_id`/`payment_terminal_id`: profile,
   adapter ID, endpoint IDs/transports/config references, generation, status,
   activation timestamps, concurrency version.
2. Сделать PostgreSQL DDL/repository, unique active fiscal binding per register,
   transactional activate/deactivate/rebind, audit и optimistic locking.
3. Route resolver создаёт immutable snapshot с обоими final-device IDs и
   generation; MQTT topic выбирается по adapter, не по payment device.
4. Binding delivery в edge-agent должен соответствовать SC-01 и поддерживать
   ACK applied generation; до ACK новый binding не ACTIVE.
5. В BeeFiscalApp реализовать выбор location/register/profile, совместимые
   endpoints, discovery/probe results, activation, disable/rebind и состояние
   applied generation. Tenant берётся только из access token.
6. Обновить generated OpenAPI contracts и API integration guide.

**Тесты:** domain/repository/OpenAPI; cross-tenant; two active fiscal bindings;
invalid combinations; generation race; activation ACK timeout; UI interaction
для трёх profiles; route resolver DP+BluePad и Daisy.

**PASS:** после UI activation backend, DB, MQTT command и edge binding имеют одну
generation и endpoint IDs; старый snapshot продолжает старый чек, новый чек
получает новую generation.

## 11. SC-07 — сквозной REST↔BLE failover и продолжение чека

**Цель:** доказать, что MiniPOS использует одну операцию и одну device state
machine независимо от канала.

**Основные пути:**

- `minipos/BeeMiniPOS/src/connectivity.ts`, BLE clients и UI orchestration;
- `minipos/beeminipos-backend/internal/domain/service.go`;
- fiscal backend ping, BLE route package, MQTT/sync/webhook handlers;
- BlueCash app и edge-agent adapter fakes.

**Сделать:**

1. Использовать один долговечный `client_operation_id` и canonical payload для
   REST и BLE attempts. Route switch не пересоздаёт receipt/payment IDs.
2. Подключить `ConnectivityController` к реальному checkout orchestration, а не
   только к isolated helper; не запускать два physical execution одновременно.
3. При REST unknown сначала lookup/reconcile. BLE разрешать только с актуальным
   route package, совпадающим binding и живым physical readiness.
4. После cloud recovery завершить уже исполняемый BLE step, синхронизировать
   outbox/authority и только затем отправлять следующий intent через REST.
5. Webhook и MQTT sync одного события должны дедуплицироваться в MiniPOS backend.
6. Прогнать одинаковые сценарии для BlueCash fake, DP+BluePad fake и Daisy fake.

**Тесты:** REST→BLE и BLE→REST между каждым шагом; ping flapping; lost HTTP
response; BLE disconnect; backend/broker restart; duplicated webhook+sync;
cross-channel digest conflict; open receipt recovery.

**PASS:** каждый scenario имеет ровно один card charge и один fiscal receipt либо
доказанную compensation/RECOVERY_REQUIRED; чек может продолжиться другим каналом
без нового surrogate/operation ID.

## 12. SC-08 — software regression и evidence gate

**Цель:** получить воспроизводимое машинное доказательство software completeness.

**Сделать:**

1. Добавить три firmware fault profiles: BlueCash, DP-150+BluePad, Daisy. Для
   каждого: success, decline, timeout before send, disconnect after send,
   restart, duplicate other channel, compensation и backend ACK loss.
2. Создать один non-interactive gate script, который выполняет:
   ESP-IDF clean build; native firmware unit/integration tests; protocol tests;
   Go backend tests; MiniPOS Go/TypeScript tests; BeeFiscalApp tests; Android
   BlueCash unit tests; OpenAPI/AsyncAPI/CBOR validation; stub E2E.
3. Запускать минимум один clean build без старого Arduino target в include/link
   path, чтобы не скрывать IDF portability defects.
4. Сохранить JUnit/log/coverage/contract artifacts и exact toolchain versions.
5. Обновить `contracts/roadmap-stage-acceptance.json`: обязательные MVP profiles
   не могут оставаться `FORMALLY_EXCLUDED_EXTERNAL`. До HIL их корректный статус
   — `SOFTWARE_PASS_HIL_PENDING`, с `SIMULATED` evidence.
6. Обновить устаревший `10_SOFTWARE_COMPLETENESS_AUDIT.md` или пометить этот
   документ его актуальной заменой.

**PASS:** gate зелёный два последовательных запуска из clean worktree; нет skip,
stub или excluded для software P0; нет `ESP_ERR_NOT_FINISHED` в command path;
HIL tests перечислены отдельно и не выданы за software evidence.

## 13. Минимальная матрица финальной приёмки

| Проверка | Требуемый результат до статуса |
|---|---|
| BlueCash REST/MQTT fiscal+card fake E2E | PASS |
| BlueCash direct BLE fiscal+card fake E2E | PASS |
| DP-150 UART fiscal fake E2E | PASS |
| BluePad BLE central payment fake E2E | PASS |
| DP-150+BluePad composite compensation fake E2E | PASS |
| Daisy USB CDC fiscal fake E2E | PASS |
| REST→BLE→MQTT sync continuation для 3 profiles | PASS |
| journal crash/restart/dedupe/retention/backend ACK | PASS |
| composite binding API/DB/UI/apply-generation | PASS |
| OpenAPI/AsyncAPI/CBOR contract validation | PASS |
| clean ESP-IDF build и полный regression | PASS |
| реальные device/acquirer tests | HIL_PENDING |

## 14. Условия присвоения статуса

Статус можно изменить на:

```text
SOFTWARE_COMPLETE_HIL_PENDING
```

только если одновременно выполнены условия:

```text
SC-01..SC-08 = PASS
AND all three MVP profiles execute on protocol-faithful fakes
AND REST and BLE use one durable processor and idempotency domain
AND composite binding is persisted, applied and routed end-to-end
AND clean regression gate = PASS
AND remaining blockers require physical device, vendor/acquirer access,
    electrical verification or production security deployment only
```

После этого допустимыми открытыми пунктами остаются только:

- BlueCash-50 fiscal/card HIL;
- DP-150 MX COM/electrical/fiscal HIL;
- BluePad-50 Plus BLE/acquirer/payment HIL;
- combined DP-150+BluePad compensation HIL;
- Daisy Compact S 01 USB/electrical/fiscal HIL;
- production Secure Boot, Flash Encryption и защищённый BLE;
- юридическая/vendor/acquirer acceptance evidence.

Production-path composite binding доставляется MQTT bridge как подписанный BFPE
envelope. ESP-IDF подтверждает применённую generation, и только exact ACK
активирует register route. Выданные при activation MQTT/UNP authority сохраняются
для последующего provisioning; неполная UART/USB/BLE/MQTT-конфигурация
отклоняется до перехода binding в `PENDING`.

`SC-01`…`SC-08` закрыты двумя clean gate; дальнейшее подтверждение заявленных
профилей требует реального аппаратного и эквайрингового HIL.
