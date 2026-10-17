# Реестр разрывов и заданий на доработку

[Навигация](README.md) · [Протоколы](03_DEVICE_PROTOCOLS.md) · [Тесты](05_TESTS_AND_ACCEPTANCE.md)

Все записи открыты. C/R/E и P0/P1/P2 определены в README. Пути ниже относительно корня репозитория; `IDF` означает `IoT/firmware/edge-agent-s3/idf/main`. Kotlin `...` означает `src/main/kotlin/com/beeloy/fiscal/bluecash`. Владелец указан в плане пакетов, фактического исполнителя назначить при старте реализации.

## GAP-001 — MQTT command namespace не совпадает

**P0 · C · OPEN** · Тесты: T-01.

**Основание:** `fiscal-backend/internal/mqttclient/client.go::Prepare; fiscal-backend/internal/domain/admin.go; IDF mqtt_runtime.cpp; BlueCashMqtt.kt`.

**Разрыв и последствия:** Backend публикует beeloy/v1/tenants/{t}/devices/{fiscal}/commands/{op}; signed binding и physical subscribers ожидают tenants/{t}/devices/{adapter}/commands. Exact topic match не доставляет эти сообщения в processor.

**Доработка:** Утвердить единый versioned topic binding; отдельно определить adapter identity, fiscal endpoint identity и operation correlation. Обновить publisher/subscriber/ACL/AsyncAPI вместе.

**Критерий закрытия:** Реальный backend publisher → S3 MQTT ingress и BlueCash ingress: каждый получает одну операцию; чужие tenant/device topics отвергаются.

## GAP-002 — S3 business ACK не доходит

**P0 · C · OPEN** · Тесты: T-01, T-12.

**Основание:** `admin.go::composite mqtt; IDF mqtt_runtime.cpp; mqttclient/client.go::ACK topic`.

**Разрыв и последствия:** Binding содержит sync/acks, backend добавляет /{batch_id}, S3 подписывается и сравнивает только base topic. PUBACK не заменяет business ACK.

**Доработка:** Согласовать wildcard subscription и строгий разбор suffix/body correlation; persist ack только после проверки подписи и journal hash.

**Критерий закрытия:** ACK с корректным suffix продвигает cursor; неверный batch/edge/hash и stale ACK не меняют его.

## GAP-003 — BeeFiscalApp → S3 onboarding отсутствует как совместимый цикл

**P0 · C · OPEN** · Тесты: T-02.

**Основание:** `BeeFiscalApp/src/smartDeviceBle.native.ts; src/smartDeviceBle.ts; IDF ble_runtime.cpp, app_main.cpp`.

**Разрыв и последствия:** App ищет Edge V2 service 7b6f2000…, device-info/AUTH_V2; IDF публикует 7b6f0000… и принимает BFPE. authenticate всегда бросает EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE; setWifi/bindStore не реализуют запись.

**Доработка:** Выбрать один протокол настройки с device proof, capability, fragmented transfer, atomic config и readback. Завершить клиент и firmware без скрытого успешного stub.

**Критерий закрытия:** Factory-reset S3 на Android и iOS проходит identity → auth → Wi-Fi → signed binding → reboot → READY без ручной записи NVS.

## GAP-004 — Нет замкнутого bootstrap сети, trust и secrets S3

**P0 · C/R · OPEN** · Тесты: T-02, T-20, T-22.

**Основание:** `IDF app_main.cpp; runtime_config.cpp; BindingTrustAnchor.h.example; device-manufacturing cli.py`.

**Разрыв и последствия:** Runtime читает mqtt-ca/cert/key, command-hmac и sync-ack-hmac из protected NVS; в просмотренном application/factory пути нет их provisioning writer. Не обнаружена инициализация Wi-Fi/netif/connect/SNTP в main runtime; без trust header радио отключается. Возможную внешнюю factory процедуру нужно предъявить.

**Доработка:** Описать и реализовать проверяемый factory+BLE bootstrap, доверенные anchors, time policy и ротацию. Не передавать private identity key на станцию.

**Критерий закрытия:** Чистая flashed плата достигает TLS broker без ручных недокументированных действий; отсутствие сети/времени/секрета даёт диагностику и 0 денежных I/O.

## GAP-005 — Provisioning BFPE требует одного oversized GATT write

**P1 · C · OPEN** · Тесты: T-02, T-03.

**Основание:** `IDF ble_runtime.cpp::receive_provisioning; app_main.cpp::provision`.

**Разрыв и последствия:** BFPE envelope до 8192 bytes принимается целиком одним characteristic write; нет собственного сборщика фрагментов настройки. Нельзя рассчитывать на такой ATT value для обычного write.

**Доработка:** Ввести bounded fragmented onboarding: transfer ID/digest/length/TTL, ack/resume/abort, install только полного проверенного envelope.

**Критерий закрытия:** Binding более 512 bytes устанавливается на Android/iOS при поддерживаемом MTU; оборванный transfer не меняет прежний binding.

## GAP-006 — Legacy advertising S3 превышает budget

**P0 · C · OPEN** · Тесты: T-03.

**Основание:** `IDF ble_runtime.cpp::advertise; app_main.cpp setup name`.

**Разрыв и последствия:** Flags 3 bytes + 128-bit UUID AD 18 + full name BeeFiscal-setup 16 = 37 bytes при legacy budget 31. ble_gap_adv_set_fields error игнорируется возвратом; provisioning может быть невидим.

**Доработка:** Перенести имя в scan response/сократить AD; проверять ошибки start и публиковать диагностику.

**Критерий закрытия:** Расчёт AD <=31; код ошибки не подавляется; Android/iOS видят ожидаемый service и identity.

## GAP-007 — BLE MTU, framing и timeout расходятся

**P0 · C · OPEN** · Тесты: T-03, T-04.

**Основание:** `IDF ble_runtime.cpp::ble_publish_event; minipos/BeeMiniPOS/src/nativeBle.native.ts; bleFrames.ts`.

**Разрыв и последствия:** S3 event frame: header 57 + body 128 =185 value bytes, но ATT_MTU185 оставляет182. Native просит185 и не использует фактический результат; iOS negotiation не учитывается. Result timeout30s короче возможной оплаты120s.

**Доработка:** Размер chunks от negotiated MTU-3-header, поддерживаемый minimum MTU явный; flow/backpressure и согласованный timeout/reconcile. Для MTU23 header57 не помещается — нужен новый header/минимальный MTU и явный отказ до submit.

**Критерий закрытия:** На MTU185/247/517 каждый frame вмещается; MTU23 безопасно блокируется либо поддерживается новым форматом; длительная оплата не провоцирует повторный debit.

## GAP-008 — BLE flow/session isolation не замкнуты

**P1 · C/R · OPEN** · Тесты: T-03, T-04, T-18.

**Основание:** `IDF ble_runtime.cpp::receive_flow/gap; nativeBle.native.ts::resetSession; bleFrames.ts`.

**Разрыв и последствия:** S3 flow callback ничего не делает; assembler state общий и не сбрасывается явно на disconnect. В native resetSession не создаёт новый assembler. Нужны bounds/TTL для незавершённых сообщений и корреляция frame UUID с intent UUID. OPEN_MVP реально используется вместо secure reference handshake.

**Доработка:** Зафиксировать допустимый MVP security profile, реализовать flow или убрать ложные promises; per-connection state/TTL/global bounds и очищение при session change.

**Критерий закрытия:** Чужая/старая сессия не может продолжить frame; bounded memory; дубликаты byte-identical; UI и docs показывают фактический режим.

## GAP-009 — Daisy runtime codec не соответствует протоколу

**P0 · C · OPEN** · Тесты: T-05.

**Основание:** `IoT/protocol-abstraction/src/FrameCodec.cpp::DaisyCodec; IDF profile_executor.cpp`.

**Разрыв и последствия:** TX LEN=data+0x23 вместо+0x24, BCC обычный ASCII hex вместо nibble+0x30; RX dataLen вычитает10 вместо11. Независимый probe отвергает официальный golden RX как DAISY_LENGTH.

**Доработка:** Исправить/объединить codec в actual runtime; исключить независимые расходящиеся реализации. Golden fixtures получать из документа, а не генерировать тестируемым encoder.

**Критерий закрытия:** TX статуса совпадает с vendor bytes; golden RX принимается, checksum mutation отвергается, payload/6 status bytes точны.

## GAP-010 — Datecs runtime codecs ошибаются в LEN на четыре

**P0 · C · OPEN** · Тесты: T-05.

**Основание:** `FrameCodec.cpp::DatecsCodec; SmartDevices/bluecash-app/.../DatecsProtocol.kt::DatecsFrameCodec`.

**Разрыв и последствия:** TX использует data+0x26 вместо+0x2A; RX ожидает data+0x2F вместо+0x33. Исправный common Datecs driver не используется этими путями.

**Доработка:** Синхронно исправить C++/Kotlin runtime codecs и применить одинаковые внешние векторы; добавить отрицательные длины/max bounds.

**Критерий закрытия:** Независимые Datecs frames проходят C++ и Kotlin; empty TX LEN bytes 30 30 32 3A, malformed legacy vectors отвергаются.

## GAP-011 — Serial exchange не реализует полноценный NAK/SYN/error parser

**P0 · C · OPEN** · Тесты: T-06.

**Основание:** `IDF profile_executor.cpp::exchange; Kotlin DatecsProtocol.kt::DatecsSocketProtocol`.

**Разрыв и последствия:** S3 один раз пишет и копит bytes до03/фиксированного числа reads; нет streaming SYN/NAK. Проверяется только bit0 каждого status byte, command ErrorCode игнорируется. Kotlin blocking read без управляемого deadline может зависнуть.

**Доработка:** Parser конечного автомата, vendor-specific status masks и typed ErrorCode; timeout/retry policy с тем же seq для retransmit и UNKNOWN после возможного I/O.

**Критерий закрытия:** SYN не портит frame; NAK retransmit byte-identical; неверный password/invalid command/FM full не возвращают COMMITTED; stuck stream ограничен deadline.

## GAP-012 — Sequence reset и vendor mapping/encoding

**P0 · C/R · OPEN** · Тесты: T-06, T-07.

**Основание:** `IDF profile_executor.cpp::fiscalize/fiscal_command/recover; CommandPayload.cpp`.

**Разрыв и последствия:** Каждый вызов стартует seq0x20. Datecs кэширует последний ответ для повторённого SEQ; подряд single-command операции могут не исполниться. Daisy требует CP1251, runtime strings идут как UTF-8 bytes; storno reason mapping различен у производителей.

**Доработка:** Sequence state на port/session, безопасная startup synchronization; profile encoding, reason/payment/tax mapping с явными capabilities. Для Datecs кодировку подтвердить по точной модели/SDK.

**Критерий закрытия:** Два разных подряд X/Z/cash command не получают cached old result; Bulgarian text и каждый storno reason дают ожидаемые bytes по выбранному vendor.

## GAP-013 — BluePad central не исполняет vendor BLE wake/flow

**P0 · C/R · OPEN** · Тесты: T-08.

**Основание:** `IDF bluepad_ble_central.cpp; common-modules/datecspay/DatecsPayBleProfile.h/README.md`.

**Разрыв и последствия:** Runtime не использует POWER/WAKE, считает READY по найденному CCCD без подтверждения подписки; пишет chunks19 без ожидания каждого completion. Partial write failure возвращает NotSent даже после предыдущих принятых chunks.

**Доработка:** Включить vendor BLE startup state machine, completion-driven write queue и certainty; сверить exact service UUID для конкретного firmware, поддержать power transitions.

**Критерий закрытия:** READY только после успешной подписки и wake; failure после начала передачи = UNKNOWN; ни один неотправленный chunk не маскируется успехом.

## GAP-014 — BluePad correlation и reference types не доказаны

**P0 · R · OPEN** · Тесты: T-08, T-09.

**Основание:** `IDF bluepad_ble_central.cpp::consume/lookup_packet/exchange`.

**Разрыв и последствия:** Общий outcome/response semaphore без видимой проверки payment correlation. UUID преобразуется в STAN удалением недесятичных символов, purchase reference попадает в RRN tag; recovery/reversal смешивают reference types. Требуется сверка с точной payment app spec.

**Доработка:** Typed client UUID/STAN/RRN/auth mapping; durable STAN allocation/mapping; привязка response к операции, amount и terminal. Нельзя переносить raw UUID в vendor RRN без спецификации.

**Критерий закрытия:** Late notification старого payment не утверждает новый; lookup/refund использует реально записанный vendor reference; bank receipt соответствует tender.

## GAP-015 — Неполная проверка S3 intent до оплаты

**P0 · C/R · OPEN** · Тесты: T-07, T-10.

**Основание:** `IDF intent_processor.cpp::decode; profile_executor.cpp::finish_receipt`.

**Разрыв и последствия:** Проверяются не все totals/line/profile инварианты; часть payload validation выполняется при построении fiscal line уже после card purchase. Discount читается, но не передаётся SaleItemPayload в finish_receipt; operator/password/till и cash/card slots захардкожены.

**Доработка:** До reservation/card I/O построить и проверить полный vendor plan, exact minor-unit totals/discounts/quantity/tax/currency, credential/payment mapping из binding.

**Критерий закрытия:** Невалидная line/tender сумма отвергается до первого device byte; discounted total совпадает во всех слоях и на чеке; конфигурация оператора реально применяется.

## GAP-016 — S3 UNP allocation и recovery snapshot расходятся

**P0 · C/R · OPEN** · Тесты: T-10, T-11.

**Основание:** `IDF durable_storage.cpp::reserve; intent_processor.cpp::execute`.

**Разрыв и последствия:** UNP собирается prefix+to_string(counter), без padding7; common payload validator ожидает7digits. p.unp присваивается после сохранения canonical payload; recovery читает $.unp. Нужно согласовать cloud UNP с локальной authority, включая команды без sale.

**Доработка:** Единый владелец UNP; typed persisted canonical_unp и authority sequence в reservation transaction; padding/range/generation; не изобретать новое UNP для уже закреплённого sale.

**Критерий закрытия:** Counter1 даёт ровно7digit suffix; restart восстанавливает тот же UNP; cloud и local result совпадают; reports не расходуют sale range без явного контракта.

## GAP-017 — Storage errors не останавливают денежный side effect

**P0 · C · OPEN** · Тесты: T-11.

**Основание:** `IDF intent_processor.cpp::execute; ProfileExecutor payment checkpoint calls`.

**Разрыв и последствия:** Результат set_operation_state EXECUTING и части upsert игнорируется; execute вызывается даже после ошибки checkpoint.

**Доработка:** Каждый pre-I/O checkpoint должен успешно durable commit; explicit storage-failure path, запрет нового fiscal/card I/O, сохранение неопределённости уже начатого действия.

**Критерий закрытия:** Failpoint на каждом checkpoint даёт 0 последующих vendor calls; restart восстанавливает достоверное предыдущее состояние.

## GAP-018 — Terminal state и outbox не атомарны

**P0 · C · OPEN** · Тесты: T-11.

**Основание:** `IDF intent_processor.cpp::execute/recover_pending; durable_storage.cpp`.

**Разрыв и последствия:** Operation/receipt/payment updates и append_event отдельные commits. Crash после terminal update до event может оставить terminal operation без sync события; обычный pending recovery её не подберёт. INSERT OR IGNORE не проверяет одинаковость payload.

**Доработка:** Одна transaction для terminal projection+outbox; journal uniqueness с conflict detection и repair scan для старых разрывов.

**Критерий закрытия:** Crash в каждой SQL точке не теряет terminal event; divergent повтор event_id отвергается; UI success только после durable result+event.

## GAP-019 — Split-card evidence и compensation не индивидуальны

**P0 · C/R · OPEN** · Тесты: T-09, T-11.

**Основание:** `IDF intent_processor.cpp::execute/recover_pending; profile_executor.cpp; durable_storage recovery SQL`.

**Разрыв и последствия:** После исполнения все CARD rows получают общий terminal_reference/rrn/auth из одного результата; recovery выбирает один payment. При нескольких CARD и позднем decline нельзя доказывать корректную компенсацию всех ранних approves.

**Доработка:** Отдельная state machine/checkpoint на tender с amount/STAN/RRN; recovery всех tender rows в порядке, idempotent compensation ledger.

**Критерий закрытия:** CARD1 approved → CARD2 declined/unknown/crash: каждое списание и возврат отражено отдельно; нет перезаписи первого RRN вторым; итог не ложно COMMITTED.

## GAP-020 — Recovery стартует раньше readiness и может отменить чужой чек

**P0 · C/R · OPEN** · Тесты: T-09, T-11.

**Основание:** `IDF app_main.cpp::recover_pending; profile_executor.cpp::recover`.

**Разрыв и последствия:** Recovery вызван до usb_cdc_runtime_start и до BluePad host readiness. При open receipt recovery отменяет его без доказанной связи с canonical UNP операции. Повторная обработка после readiness нуждается в явном scheduler.

**Доработка:** Readiness-gated retry recovery; ownership proof по UNP/receipt state/terminal evidence. При недоказанной принадлежности оставить RECONCILE и блокировать конфликтный порт.

**Критерий закрытия:** Cold boot с delayed USB/BLE восстанавливается после readiness; чужой открытый чек не отменяется и чужой payment не реверсируется.

## GAP-021 — Synthetic fiscal_reference вместо device evidence

**P0 · C · OPEN** · Тесты: T-07, T-13.

**Основание:** `IDF profile_executor.cpp::reference/finish_receipt/fiscal_command`.

**Разрыв и последствия:** Normal success возвращает EDGE-hash(operation_id); response close receipt не разбирается. DEVICE_IDENTITY/TIME выполняются, но data отбрасывается и возвращается generic committed reference.

**Доработка:** Typed results из vendor response: документ/ФМ/time/UNP и профильные поля; synthetic operation correlation хранить отдельно, не как доказательство фискального документа.

**Критерий закрытия:** Чек и API имеют одинаковый реальный номер/ФМ; identity/time возвращают значения устройства; отсутствие доказательства не делает sale COMPLETED.

## GAP-022 — S3 sync hash preimage несовместим с Go map serialization

**P0 · C/R · OPEN** · Тесты: T-12.

**Основание:** `IDF sync_runtime.cpp::event/worker; fiscal-backend/domain/edge_sync.go::DeviceEventHash/EdgeBatchHash`.

**Разрыв и последствия:** S3 cJSON печатает insertion order вложенных payload; Go json.Marshal(map) сортирует keys и иначе экранирует HTML. ACCEPTED payload строится operation_sequence,unp_sequence,command, что отличается от key order Go. Подпись верна только для одних и тех же bytes.

**Доработка:** Утвердить canonical wire/hash algorithm и golden preimages для C++/Kotlin/Go/TS; отделить stored payload digest от transport event hash; не сортировать лишь верхний объект.

**Критерий закрытия:** События actual S3 builder проверяются настоящими Go hash/signature functions, включая nested objects, null/empty, Cyrillic, <>&, decimals.

## GAP-023 — S3 не фиксирует неизменяемый in-flight sync batch

**P0 · C/R · OPEN** · Тесты: T-12.

**Основание:** `IDF sync_runtime.cpp::worker; durable_storage.cpp::acknowledge_through`.

**Разрыв и последствия:** Каждый цикл строит новый UUID и набор всех текущих pending, заново подписывает events. После lost business ACK и новых events batch расширится вопреки backend replay. ACK persistence не сверяет hash с сохранённым ожидаемым event и может переписать hash устаревшим ACK при monotonic sequence.

**Доработка:** Freeze exact bytes/event signatures/batch ID до publish, persist retry state; ACK сравнить с frozen batch/hash, запретить regression; durable clear только атомарно с cursor.

**Критерий закрытия:** После server commit+lost ACK+append+reboot уходит тот же batch body; stale ACK не меняет hash/cursor; после valid ACK следующий batch продолжает chain.

## GAP-024 — Backend/S3 command types, result events и sale ID расходятся

**P0 · C · OPEN** · Тесты: T-01, T-13.

**Основание:** `mqttclient/client.go::Prepare; IDF intent_processor.cpp/sync_runtime.cpp; domain/edge_sync.go::materializeEdgeResult`.

**Разрыв и последствия:** Backend reversal type REVERSAL, S3 принимает SALE_REVERSE; COMMITTED reversal сериализуется как FISCALIZED, backend ждёт REVERSED. Normal server_sale_id равен sale.ExternalID, materializer читает repo.Sale(internalID). SALE_FINALIZE offline не попадает в legacy FISCAL_SALE offline importer.

**Доработка:** Один versioned command/result mapping и explicit internal/external/client IDs; отдельная поддержка offline aggregate materialization и reversal states.

**Критерий закрытия:** external_id != internal_id сквозной sale/reversal работает; offline SALE_FINALIZE импортирует ровно один sale; order cancellation/webhook верны.

## GAP-025 — S3 command expiry и bounded storage не закрыты

**P1 · C/R · OPEN** · Тесты: T-10, T-11, T-20.

**Основание:** `IDF intent_processor.cpp::decode; durable_storage.cpp::prune_acknowledged`.

**Разрыв и последствия:** Для MQTT не найдена полноценная issued/expires validation, в отличие от BlueCash. Prune очищает outbox, но operations/receipts/payments продолжают расти; нужен capacity/archival policy с сохранением dedup history.

**Доработка:** Проверка времени и generation до I/O, trusted clock policy; signed retention/archival и capacity alarms без удаления unacked history или разрешения старого дубля.

**Критерий закрытия:** Expired signed command исполняется0раз; full SD безопасно блокирует; после retention replay не печатает второй чек; chain anchors проверяемы.

## GAP-026 — MiniPOS Web UNKNOWN может повторно submit

**P0 · C · OPEN** · Тесты: T-14.

**Основание:** `minipos/miniposweb/src/fiscal/routeController.ts::execute`.

**Разрыв и последствия:** old UNKNOWN допускает новый route loop; любой lookup error превращается в undefined и вызывает submit; submitted хранится только в текущем вызове. После reload/нового execute можно повторить uncertain side effect.

**Доработка:** Persist send_started/route/receipt authority, различать authoritative NOT_FOUND и transport/auth failure. UNKNOWN → lookup/reconcile only до доказанного NOT_SENT; альтернативные routes не начинают новую продажу.

**Критерий закрытия:** Submit response lost → reload → lookup timeout/401/503 → submit count остаётся1 на всех routes.

## GAP-027 — Native checkout и outbox нужно доказать на реальном lifecycle

**P1 · R · OPEN** · Тесты: T-04, T-14, T-17, T-21.

**Основание:** `minipos/BeeMiniPOS/src/nativeBle.native.ts; checkout/session/bleResult modules`.

**Разрыв и последствия:** 47 module tests не проверяют OS kill, actual native library disconnect, background/resume, MTU и service cache. Native bundle не равен APK/IPA на устройстве.

**Доработка:** Добавить adapter-level tests и Android/iOS lifecycle instrumentation; persist uncertain operation и safe session recovery.

**Критерий закрытия:** Kill во время оплаты не теряет UUID и не повторяет submit; denied/revoked BLE permission не даёт false READY; polling закрывает потерянную notification.

## GAP-028 — MiniPOS backend regression suite красный

**P1 · C · OPEN** · Тесты: T-15.

**Основание:** `minipos/beeminipos-backend/internal/api/handler_test.go:479/499/637`.

**Разрыв и последствия:** Три текущих теста падают: два operator binding403 и mandatory version. По одним failures нельзя решить, ошибочен код или fixtures.

**Доработка:** Сопоставить OpenAPI/роль/operator subject с fixtures; исправить контрактный дефект либо устаревший тест без ослабления tenant/operator/CAS checks. Добавить explicit negative tests.

**Критерий закрытия:** go test -count=1 ./... проходит; wrong operator остаётся403; missing/stale version отклоняются ожидаемым canonical error.

## GAP-029 — BlueCash build/readme не соответствуют composition

**P1 · C · OPEN** · Тесты: T-05, T-22.

**Основание:** `SmartDevices/bluecash-app/build.gradle.kts; SmartDevices/README.md; Makefile::smart-device-test`.

**Разрыв и последствия:** JAR dependencies существуют, README всё ещё утверждает отсутствие bridge; demo/prod flavors и unconditional STORE_BLOCKED на Release конфликтуют со старой командой testDebugUnitTest/assembleRelease и claims Release PASS.

**Доработка:** Отдельные debug-HIL и production release gates; exact flavored Gradle tasks, vendor JAR/version/hash manifest. Обновить README/application IDs; release guard снимать только по реальному evidence.

**Критерий закрытия:** Clean checkout строит конкретный demoDebug APK и unit tests; sha известен, install/logcat успешны; release block честно отражён.

## GAP-030 — Software PASS не доказывает physical runtime path

**P1 · C · OPEN** · Тесты: T-23.

**Основание:** `Makefile; scripts/verify_driver_coverage.rb; IoT Python/native tests; docs/MVP1/README.md`.

**Разрыв и последствия:** Coverage во многом по symbols/source strings; native saga использует иной orchestrator, тесты facade не ловят ошибки actual FrameCodec. Go edge encrypted BLE/frozen sync ошибочно воспринимаются как S3 capabilities.

**Доработка:** Runtime composition manifest и behavioral tests actual translation units; separate model/unit/runtime/HIL statuses, CI fail при подмене physical implementation fake-объектом.

**Критерий закрытия:** Удаление/порча runtime LEN/topic/checkpoint ломает gate; traceability ссылается на выполняемый test и конкретный binary target.

## GAP-031 — Factory flash не различает app и merged image

**P1 · C/R · OPEN** · Тесты: T-22.

**Основание:** `IoT/device-manufacturing/src/beefiscal_factory/cli.py::flash`.

**Разрыв и последствия:** CLI всегда write_flash0x0 одного firmware file. Это корректно лишь для подходящего merged image, не для произвольного IDF application firmware.bin. Expected SHA не заменяет signed release manifest и layout.

**Доработка:** Typed image manifest с chip/partition offsets/image kind/hashes, verify signature до esptool, explicit merged-vs-segmented flashing и boot verification.

**Критерий закрытия:** App-only image не принимается за merged; wrong chip/layout/hash/signature блокируются до flash; factory-reset boot подтверждается serial log и build ID.

## GAP-032 — Tremol не подтверждён отдельным conformance contour

**P2 · R · OPEN** · Тесты: T-24.

**Основание:** `IoT/common-modules/termol/TremolProtocol.cpp, TremolPrinter.cpp; official ZFPLAB2507141400`.

**Разрыв и последствия:** Driver имеет собственные framing/ACK/XOR/NBL. Нет основания переносить Daisy/Datecs PASS на Tremol; в S3 runtime нет Tremol physical profile.

**Доработка:** Либо явно exclude из текущего MVP, либо отдельный profile+capability+golden fixtures по BG2507141400 и конкретной модели.

**Критерий закрытия:** README/registry не обещают неподдерживаемый Tremol runtime; при включении проходят все T-24 до HIL.

## GAP-033 — Daisy Smart app и placeholders не являются готовым продуктом

**P1 · C/E · OPEN** · Тесты: T-18, T-22.

**Основание:** `SmartDevices/daisy-smart-app; apps/cashbox-a/b; shared-sdk`.

**Разрыв и последствия:** Daisy smart app явно STUB; generic cashbox modules placeholders. Наличие Daisy USB driver не реализует Android SMART payment integration.

**Доработка:** Разделить product scope. Для SMART получить SDK/API/лицензию и отдельное ТЗ; до этого блокировать production capability, не показывать как tested supported profile.

**Критерий закрытия:** S3+Daisy USB доступен как отдельный профиль после своих gates; SMART недоступен для реальных денежных операций до реализации/evidence.

## GAP-034 — HIL verifier доверяет ключу из самого evidence bundle

**P1 · C · OPEN** · Тесты: T-22, T-23.

**Основание:** `scripts/verify_hil_evidence.rb`.

**Разрыв и последствия:** trusted-reviewer-public.pem берётся из того же переданного каталога. Проверяются type/result/valid_until/signature, но нет внешнего anchor, обязательной матрицы cases и сверки artifact hashes/model/build.

**Доработка:** Trust root из CI/управляемого registry, manifest schema с required case IDs/build hashes/device model/fw/operator/date; verify referenced artifact bytes и revoked reviewer keys.

**Критерий закрытия:** Самоподписанный пакет с произвольным новым ключом/без cases/с подменённым логом не проходит; валидный полный пакет проходит.
