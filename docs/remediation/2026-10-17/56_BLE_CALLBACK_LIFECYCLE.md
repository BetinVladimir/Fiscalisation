# MP026 Stage C — read-only inventory callbacks

2026-10-21. Код и режимы устройств не изменены; Stage C реализация остановлена по запросу root. Целевая база e56db144b26eb480f295525a98608e3b989e3209; текущий HEAD и hashes прочитанных файлов записаны отдельно. Здесь описано состояние исходников, а не результат новых runtime/HIL тестов. Ключи остаются ESP32-S3/NVS, ATECC608A/608 не используется.

## S3 peripheral: actual callbacks

Файл `IoT/firmware/edge-agent-s3/idf/main/ble_runtime.cpp`:

- `characteristics`/`services` (95/103): существующие0001 provisioning WRITE,0002 command WRITE/WRITE_NO_RSP,0003 event NOTIFY,0004 flow WRITE,0005/0006 provisioning READ. Для будущего stream нужны отдельные INFO/RX/TX UUID из doc52; текущие callbacks нельзя незаметно переиспользовать.
- `receive_command` (64): проверяет active connection и MTU>=61, копирует mbuf через malloc/flatten, собирает BFF1 до8192bytes, затем вызывает CommandSink. Точка будущего stream RX — **отдельный access callback** рядом с ним, передающий скопированные непрерывные кадры в `bs_feed`; не CommandSink/CommandQueue.
- `receive_provisioning` (42), `read_provisioning` (58): отдельный BFPF/BFRB setup path и JSON info/status. Не являются TLS transport и должны сохранять свой контракт.
- `receive_flow` (94): только startup/MTU check; ACK/credit сейчас не реализованы.
- `gap` (107): CONNECT/DISCONNECT увеличивают uint32 `connection_generation`, меняют `active_connection` и очищают сборку. Нет SUBSCRIBE, NOTIFY_TX и MTU-event веток; нет wire incarnation allocator или проверки переполнения local generation. Здесь будущие точки `bs_init`, subscription fence, close/reset и local TX settlement. Проверку точного peripheral handle/характеристики нельзя заменить общим событием другого BLE role.
- `ble_publish_event` (174): читает handle+generation, перед каждым BFF1 fragment повторно проверяет их; создаёт mbuf и вызывает `ble_gatts_notify_custom` в цикле. Нет peer ACK, subscription gate, backpressure/retry scheduler или NOTIFY_TX pacing. Stream TX должен быть отдельным bounded pump `bs_next`→SDK→`bs_settle`, с одним owned frame и owner/lease; результат local notify не является receipt ACK.
- `start_host` (145): `ble_hs_cfg.reset_cb` вызывает **только** `bluepad_ble_on_host_reset`; peripheral state здесь не инвалидируется. Общий reset/sync нужен обоим владельцам. SDK ownership mbuf на всех success/error ветках надо проверить в точном IDF5.5.6 NimBLE перед интеграцией: текущий fake всегда consumes notify mbuf, что не доказывает SDK behavior.

## BluePad central — другой протокол и отдельный owner

`IoT/firmware/edge-agent-s3/idf/main/bluepad_ble_central.cpp`:

- `scan` (92), `gap` (141): epoch token, discovery/connect; NOTIFY_RX проверяет central handle и копирует borrowed mbuf в ограниченный buffer; DISCONNECT обрабатывает только `s.connection`. Это vendor POS framing, **не** endpoint MP026 и не место для BFF stream framing.
- `service_cb`, `characteristic_cb`, `descriptor_cb`, `setup_write_cb`, `power_read_cb` (99–133): проверяют epoch/handle/phase; CCCD записи и power/wake transitions последовательны.
- `run` (77) и `send` (157): submission mutex отделён от state mutex; SDK вызовы выполняются без state mutex. `write_cb` (155) проверяет pending write ID/connection/attribute. Static semaphores живут дольше late callbacks; serial/epoch exhaustion fail closed.
- `bluepad_ble_on_host_reset/sync` (220/221): инвалидируют central owner и запускают повторный scan при допустимом состоянии. Будущий peripheral pump должен иметь собственные ресурсы, точную маршрутизацию событий и не блокировать эти callbacks. Совместный host/mbuf pool ещё не проверен stream нагрузкой.

## Три активных MiniPOS client paths

| Файл / функции | Текущие ownership и SDK semantics | Конкретная точка подключения / отсутствующая защита |
|---|---|---|
| `minipos/BeeMiniPOS/src/nativeBleTransport.ts`: `resetSession`30, `connectFromUserGesture`44, notify callback107, `sendComplianceIntentAndWait`137, `write`179; factory `nativeBle.native.ts:createNativeBleBootstrap` | Local generation, connect-in-progress, 10s scan/connect/discovery/MTU deadlines; Android requestMTU185, iOS reported MTU; sequential write-with-response; monitor/disconnect subscriptions удаляются; stale notifications и late writes проверяют generation. Требует legacy MTU61. | Отдельный stream session рядом с legacy BFF1 callbacks: INFO read, new TX monitor, bounded pump/core lease, close per owner. Нет гарантии подтверждённого CCCD readiness от текущего monitor API; требуется проверить3.5.1/native behavior. Отдельно проверить ABA: late cancelDeviceConnection(device.id) от старого connect не должен убить replacement с тем же device ID. |
| `minipos/BeeMiniPOS/src/webBle.ts`: `resetSession`63, `connectFromUserGesture`72, notification callback103, `sendComplianceIntent`123, `write`151 | Active web factory в App.tsx. `startNotifications` вызывается **до** addEventListener. Нет local generation, disconnect listener/removal, step/write deadline; reset не очищает assembler. Старый pending connect может поздно установить ready. Write fallback до deprecated writeValue; default frame MTU185. | В connect/setup и callback нужен явный владелец; listener до subscription, INFO/OPEN только после успешной subscription, один serialized write queue. Для doc52 browser offer20; новый stream не должен унаследовать предположение185. |
| `minipos/miniposweb/src/fiscal/webBleRoute.ts`: `submit`115→`submitCommand`121, notify callback151, write loop182 | Реально используется в App.tsx для BLE submit/reconciliation. Новый connect на каждый submit; startNotifications до listener. Abort/timeouts отвергают result, но не останавливают write loop и не отключают GATT; нет finally/listener cleanup/generation/disconnect monitor. Assembler Map не ограничен; default185. | Connection object с AbortSignal/owner должен владеть всем setup/send/read/close. Stream pump вместо frame loop; result correlation/UNKNOWN и business lookup остаются над транспортом. Нельзя делать повтор submit из-за lost transport ACK. |

`IoT/common-modules/ble-stream/stream.h` и `stream.ts` дают serialized init/feed/write/read/take(or bs_next)/settle/tick/close API. Они **не** владеют SDK handle, subscription, RNG, внешним owner, worker/timer или native/WASM TLS. Никакой production callback сейчас их не импортирует. Перед будущим вызовом SDK требуется проверить owner, сохранить копию одного frame и captured incarnation/token; обработать поздний local completion независимо от peer ACK.

## BeeFiscal: активный provisioning и неактивный V2 scaffold

Активен `BeeFiscalApp/App.tsx` import `connectProvisioningDevice` и вызов `provisionDevice` (~596). REST создание BLE session (~628) само по себе не является SDK channel.

- `src/provisioningBle.native.ts:connectProvisioningDevice`24: singleton BleManager; Android permissions, scan15s, connect/discover, Android requestMTU185, проверка185; read INFO/STATUS, write-with-response CONTROL. Нет native disconnect monitor, generation/AbortSignal, read/write/connect deadlines. `discoverAllServicesAndCharacteristics` стоит **до try**, поэтому его failure не попадает в local disconnect catch.
- `src/provisioningBle.web.ts:connectProvisioningDevice`3: chooser→connect→service→read/write, catch disconnect. Нет notify path, disconnect events, per-stage deadline/abort owner. Это provisioning control/read protocol, не reliable command byte stream.
- `src/provisioningProtocol.ts:provisionDevice`29: `live()` до/после async шагов, finally disconnect; lost final write ACK восстанавливается reconnect+signed installed-status confirmation, без replay writes. Если connector завис до возврата link, верхний finally не может отменить его. Этот путь нельзя автоматически заменить stream протоколом.
- `src/smartDeviceBle{.native,.web}.ts:connectEdgeDeviceV2` — V2 scaffold; AUTH_V2 пишет control, затем всегда бросает `EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE`, setWifi/bindStore не открываются. В App нет импорта/вызова этого scaffold. Его UUID2000..2004 и имя SecureSmartDeviceConnection не доказывают действующий authenticated channel; Web decoder также игнорирует DataView offset. Не активировать как обход MP026.

## Lifecycle coverage: что есть и чего нет

- `idf/tests/ble_runtime_native.cpp` реально включает ble_runtime.cpp: startup failures/timeouts, MTU/length/framing, foreign/disconnected command handle, reconnect generation fence при publish. **Нет** SUBSCRIBE/NOTIFY_TX/MTU/reset-invalidates-peripheral тестов, stream callbacks→core, incarnation failure/reuse/overflow, paused pump late completion, native mbuf ownership/error accounting или mixed-role BluePad+peripheral сценария.
- `idf/tests/bluepad_host/runtime.cpp` + `run.py`: actual vendor callbacks, CCCD/power/wake failures, stale epoch, fragmentation/overflow/timeouts/disconnect/host reset/held completion. Это уже полезное покрытие; **нет** совместного server stream/client POS scheduling и shared-pool starvation/reset fan-out теста.
- `BeeMiniPOS/src/nativeBleTransport.test.ts`: actual transport fake manager покрывает MTU, identity/correlation, partial/hung write, disconnect/reconnect, malformed notify, synchronous disconnect при registration, hung connect/discovery/MTU. **Нет** stream UUID/INFO/OPEN/credit/timer tests, CCCD readiness, owner ABA same device with old cancellation, adapter off/background/resume, реальные Android/iOS callbacks.
- `BeeMiniPOS/src/bleHandshake.test.ts` и `e2e/ble.spec.ts`: protocol/mock/browser UI проверки; не доказательство нового stream либо native lifecycle. Для двух browser classes нет найденного прямого теста late connect/subscribe/notify/write после abort/disconnect, listener removal, DataView-copy ownership и bounded queues. В `miniposweb/tests/fiscal-operation-app-checkout-current.test.tsx` WebBleRoute замокан целиком.
- `BeeFiscalApp/src/provisioningProtocol.test.ts` покрывает orchestration с fake ProvisioningLink (wire, lost ACK, stale proof, live invalidation); dialog tests покрывают UI-owner/password cleanup. **Нет** прямых tests реальных `.native/.web` connectors на failed discovery, stuck SDK stage, disconnect, competing connects/scan cancellation и late callbacks. V2 scaffold тоже не имеет найденного actual SDK lifecycle покрытия.

Новый stream требует Stage C callback/owner tests до TLS/PoP/pin vertical; Stage B in-memory/process tests не заменяют их. Реальный radio scheduling, foreground/background поведение, S3 heap/stack/mbuf запас и HIL остаются OPEN. В этом read-only inventory новые тесты не запускались и код не создавался.

## Принятый ограниченный fix — native provisioning discovery cleanup

После inventory исправлен `BeeFiscalApp/src/provisioningBle.native.ts`: service discovery перенесён под существующий try/catch, чтобы уже подключённое устройство закрывалось при ошибке. Actual connector/fake SDK RED: discovery rejects без cancel; GREEN: тот же error возвращается после cancel. Дополнительно small-MTU cleanup и успешный setup→owner disconnect. Полный configured Fiscal UI26suites/115tests и TypeScript PASS. [Evidence и точные source hashes](evidence/provisioning-discovery-cleanup-2026-10-21/manifest.json).

Inventory source hashes описывают предыдущий срез; native connector после fix имеет отдельный новый hash. Ни generation/deadline/abort safety, ни физическая утечка/закрытие на Android/iOS этим тестом не приняты. Эти требования из inventory остаются OPEN.
