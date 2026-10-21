# MP026 — reliable GATT byte stream v1: accepted draft / Stage B candidate

2026-10-21. Contract draft accepted for implementation by root review; Stage B candidate implemented and tested in isolation. MP026 is NOT complete. Original repository read-only for this candidate, no production enablement. Implements a transport foundation for the standard TLS1.3 direction in doc46. No custom crypto handshake, exporter, application authorization, financial admission or OpenAPI change is proposed here. Device identity/private keys remain ESP32-S3/NVS; ATECC608A/608 is unused.

## Decisions requested in this review

Root reviewed and accepted contract b741ae8b75b1f3347c03f3b2cf514df1d24e5f95316f7a174ae38961675b5897 for Stage B implementation. This document preserves that contract with the explicit18-byte CLOSE clarification. Stage B code/evidence below awaits separate implementation review. The core must be exercised through the existing four actual runtime adapters (S3 and three client classes), first in test-only transport wiring. Passing an in-memory codec alone is insufficient. Production secure mode still requires the TLS engine, enrolled certificate/name/SPKI/pin verifier, protected client PoP, signed secure-policy enforcement and physical resource acceptance.

## Actual current constraints

| Runtime / real file | Observed behavior | Required adapter work |
|---|---|---|
| S3 `idf/main/ble_runtime.cpp` | Service suffix0000; provisioning control0001, command0002 WRITE/WRITE_NO_RSP, result0003 NOTIFY, flow0004 WRITE, info/status0005/0006 READ. `receive_command` assembles BFF1 only; `receive_flow` is a no-op MTU check. One active peripheral handle, local uint32 connection_generation. `gap` handles CONNECT/DISCONNECT only. `ble_publish_event` loops all notifications synchronously and returns on local submit error, with no receipt ACK, retry, subscribe tracking or NOTIFY_TX pacing. | Separate stream service and callbacks; preallocated bounded state; SUBSCRIBE/MTU/NOTIFY_TX/reset handling; worker/BIO boundary and generation fences; leave provisioning service intact. |
| RN Android/iOS `BeeMiniPOS/src/nativeBleTransport.ts` | Actual `react-native-ble-plx`3.5.1 in package manifest. Android requestMTU185, iOS uses connected.mtu; enforces61..517. Sequential write-with-response. Scan/connect/discover/MTU10s bounds, disconnect monitor and local generation fencing. Native notification callback decodes base64 and accepts only active BFF1 message ID. One fiscal operation at a time; timeout/reset preserves UNKNOWN-after-send. | New byte-stream lane before result framing; use real negotiated MTU only as a cap, fresh generation per connection, INFO read and notify monitoring before OPEN, serialized DATA/control writes. Preserve business operation ownership/UNKNOWN behavior above TLS. RN Hermes uses native C TLS client, not WASM. |
| Expo browser `BeeMiniPOS/src/webBle.ts` / `App.tsx` | `WebBleBootstrap` is active on Platform.OS web. startNotifications and event callback, BFF1 frameMessage default185; no MTU query, transport generation fence, disconnect event listener, listener removal or connect/write deadlines. `resetSession` does not reset assembler; pending asynchronous connect/old callbacks can regain ownership. `writeValue` fallback exists. | Add a dedicated stream adapter with local generation and abort ownership across every await/callback, strict writeValueWithResponse, removable event/disconnect listeners, deadlines and bounded queue. Do not reuse the existing stale callback shape. |
| PWA `miniposweb/src/fiscal/webBleRoute.ts` / `src/App.tsx` | `WebBleRoute` is active for configured BLE submit/reconciliation commands. Per-submit chooser/connect/discovery; BFF1 default185. Assembler Map is not bounded; no subscription/disconnect cleanup/finally, no generation fence or per-stage write deadline. Abort rejects result but does not stop write loop/late callbacks. Result timer120s; route lookup requires reconnect. | Give connection/stream an explicit abortable owner; replace raw assembler with shared stream adapter; stop writes and detach/close on abort, failure or timeout. Reconciliation must query existing business operation ID, never resubmit because stream retried. |

Current BFF1 has57-byte header, minATTMTU61, max complete message8192. BeeMiniPOS and PWA clients can send frames larger than the S3 publisher's128-byte body cap, and WebBluetooth has no public negotiated-MTU accessor. BFF1 SHA256 is unkeyed integrity metadata, not channel authentication. None of the current three client paths supplies this reliable TLS byte stream.

`command_queue.cpp` copies complete <=8192 command payloads into depth8 queue. It is not a TLS ciphertext RX queue and must never receive stream fragments. Existing `ble_publish_event` is likewise a message-result API, not a TLS BIO. `edge_runtime_port.h` has only CommandSink/Ingress::Ble, so add separate test-only stream hooks; later authenticated plaintext admission is a separate reviewed interface.

The actually built IDF5.5.6 config has NimBLE max connections3, MSYS1 blocks12 x256 bytes. S3 also runs BluePad as central using the same host; do not consume all mbufs or treat BluePad-central callbacks as a MiniPOS peripheral link. Existing `bluepad_ble_central.cpp` filters its connection handle and receives shared host-reset notification. New host reset must invalidate both owners, not only BluePad. These are static config observations, not measurements of runtime available heap/radio capacity.

## GATT namespace and discovery

Allocate a new service, not an invisible reinterpretation of0001..0006. Proposed reserved UUIDs (verify uniqueness before implementation):

| Attribute | UUID | Properties / meaning |
|---|---|---|
| stream v1 service | `7b6f1000-7c6d-4c7a-9e4f-424545464953` | Dedicated reliable byte stream; does not itself authenticate device |
| INFO | `7b6f1001-7c6d-4c7a-9e4f-424545464953` | READ only, exactly20 bytes, per-connection capability/incarnation |
| RX | `7b6f1002-7c6d-4c7a-9e4f-424545464953` | WRITE with response only; central→S3 DATA/control |
| TX | `7b6f1003-7c6d-4c7a-9e4f-424545464953` | NOTIFY + CCCD; S3→central DATA/control |

These proposed UUIDs deliberately differ from existing service `7b6f0000-7c6d-4c7a-9e4f-424545464953`. A signed route/policy must later name v1 explicitly; discovery/name/INFO is never a trust anchor. Missing service, unsupported version or unusable throughput is an error, never permission to fall back to OPEN_MVP/raw BFF1. No changes to existing signed policy/schema in this foundation package. Do not advertise an enabled secure business channel before the complete security gate passes.

Legacy advertisement31bytes cannot contain both128-bit service UUIDs plus current flags. Existing service advertisement plus optionalServices/new service discovery can be used in test wiring. A later signed secure route may select the new service advertisement; switching advertising/discovery compatibility is an explicit rollout gate. Do not silently break native scan service filter or WebBluetooth chooser grants. Provisioning/setup remains independently authenticated and encrypted.

## Wire format, all integers unsigned big-endian

One ATT value is one complete stream frame. No ATT long writes/prepared-write assembly. GATT callbacks may deliver copied/late/repeated values; the adapter preserves frame boundaries. The negotiated maximum ATT-value size must20..182bytes; this is NOT a minimum size for every frame: CLOSE/CLOSE_ACK are18bytes and DATA may be17bytes. Do not derive trust from ATT write response or notification completion.

Common header16bytes:

| Byte | Field |
|---|---|
|0..1| magic `42 53` (BS) |
|2| version1 |
|3| type0 INFO,1 OPEN,2 OPEN_ACK,3 DATA,4 ACK,5 CLOSE,6 CLOSE_ACK |
|4..11| nonzero S3-issued transport incarnation, opaque64-bit bytes |
|12..15| offset; defined by type |

There are no flags; unknown types/version/magic, wrong exact control length or impossible field value reject the current stream. Generation mismatch is discarded before state mutation; log only bounded counters. On the server, access from a foreign/central connection handle is ATT-denied without touching the active stream. An unknown frame cannot cause unbounded allocation or parser-driven recursion.

| Type | Offset | Body / exact length | Direction |
|---|---|---|---|
| INFO0 |0| max_frame_value:u16, server_rx_capacity:u16; total20 | READ only |
| OPEN1 |0| client_max_frame_value:u16, client_rx_capacity:u16; total20 | central→S3 RX |
| OPEN_ACK2 |0| selected_frame_value:u16, server_rx_capacity:u16; total20 | S3→central TX |
| DATA3 | first byte offset in sender's independent directional stream |1..selected_frame_value-16 opaque bytes | either |
| ACK4 | next contiguous received byte, cumulative | receive_limit:u32 (exclusive absolute byte offset); total20 | either |
| CLOSE5 |0| reason:u16; total18 | either |
| CLOSE_ACK6 |0| same reason:u16; total18 | either |

INFO/OPEN/OPEN_ACK capacity range256..2048; values outside bounds fail, not clamp. Initial implementation S3/client use2048 each, tests include256. `selected_frame_value = min(INFO.max_frame_value, OPEN.client_max_frame_value)` and must20..182; OPEN_ACK must exactly match that choice and advertised server capacity. INFO.max_frame_value is min(182, current ATT_MTU-3), never below20. On native, client offer=min(182, actual known MTU-3); absent/invalid MTU information uses20. On both browser adapters v1 offer=20, independent of assumed MTU185. Therefore DATA can have4bytes at defaultMTU23. Larger browser frames require a separately tested profile/probe design, not optimistic silent upgrade. ACK/OPEN still fit20.

No frame CRC/digest/crypto is added. TLS provides record integrity/authentication; transport corruption/replay may cause fatal TLS verification or availability loss, never plaintext acceptance. Frame offsets are not TLS record sequence numbers. DATA may split or combine any TLS record boundary; the transport does not parse TLS.

## Connection lifecycle and incarnation

1. Peripheral connect establishes a fresh local owner (connection handle + monotonically increasing local generation) and fresh nonzero wire incarnation from the approved S3 RNG. INFO is stable for that connection. RNG failure or detected reuse of the current/last incarnation fails closed; do not regenerate NVS identity. Numeric local generation overflow also closes/refuses instead of wrapping. Wire incarnation64 is a stale-frame discriminator, not cryptographic identity. An unknown historical random collision cannot be mathematically excluded with a finite field: TLS must always be freshly instantiated on reconnect even if wire bytes coincide. Do not claim this field supplies TLS anti-replay or authentication. Tests force detected nonce repetition and counter exhaustion.
2. Client starts notify monitoring and confirms it is active, reads INFO, then sends OPEN. S3 accepts OPEN only from active peripheral handle, while subscribed to TX. On success, it creates exactly one stream and queues OPEN_ACK. Local callbacks/timers retain their local owner independently of wire incarnation.
3. Central validates OPEN_ACK before feeding any TLS bytes. Server may accept DATA only after valid OPEN and after its OPEN_ACK is dispatched. TLS state remains separate from transport OPEN; canSendComplianceIntent stays false.
4. Lost OPEN_ACK: retransmit identical OPEN. Same incarnation+identical parameters returns the cached OPEN_ACK without resetting offsets/buffers/TLS. Changed parameters or a second attempt to initialize the stream closes. Lost OPEN write response does not create a second stream.
5. Once OPEN, changing MTU upward does not renegotiate frame size. A decrease below selected_frame_value+3, TX unsubscribe, host reset, adapter-off, auth-owner revocation, connection cancellation, local deadline, malformed current stream or failed native write invalidates owner and closes. No stream restart inside the same BLE connection in v1: reconnect, new INFO and new TLS instance required.
6. Every await callback, completion, queued task and timer checks captured local generation + exact device/connection owner before mutation or scheduling the next write. Remove listeners/subscriptions on close; invoke late callbacks in tests even after removal. S3 host reset must also fence stale TLS/worker output before re-advertise.

INFO/read and OPEN exchange are deliberately unauthenticated transport setup. Replayed INFO cannot establish a device pin; wrong service/INFO may at most fail setup/TLS. Certificate chain, ServerAuth/name/SPKI pin and protected client-authority proof must still succeed inside the independently trusted policy before any business data is admitted. Do not wire the dormant unsigned CHALLENGE/READY ECDHE path into this contract.

## Directional state, byte acceptance and backpressure

Each direction starts at offset0, independently. Track sender `tx_acked <= tx_dispatched <= tx_end`; receiver `rx_read <= rx_next <= rx_limit`. Store ciphertext only. Checked arithmetic uses at least64-bit intermediates in C; TS Number operations must stay within32-bit exact integer range and avoid signed bitwise offset conversion.

- TX retained ring2048bytes; RX ring2048bytes. A write to TLS BIO returns a short accepted count or WANT_WRITE when TX ring is full, never silently drops/allocates an unbounded queue. TLS BIO read drains at most available contiguous RX bytes or WANT_READ; it never observes gaps/duplicates.
- Initial peer receive limit is its advertised rx_capacity. Receiver advertises `rx_limit=min(rx_read+rx_capacity, MAX_STREAM_BYTES)`. Receipt ACK advances `rx_next`, but credit advances only after TLS actually drains the RX ring (`rx_read`). Thus ACK does not overcommit ring storage.
- DATA is sent only if its entire end<=peer receive_limit, ring available, and <=16 distinct outstanding DATA frames (and <=2048 retained data bytes). Byte and frame caps both apply. Browser20 therefore has at most64 data bytes outstanding for full frames; allocation remains bounded despite many tiny fragments.
- Retain each dispatched frame boundary plus its exact bytes until cumulatively acknowledged. Maximum16 descriptors; no dynamic per-byte/per-frame vectors. On retransmission use identical header/body/boundaries. No re-encrypt, new TLS write or reconstructed business command.
- In-order DATA offset==rx_next is copied once if wholly inside rx_limit/capacity; then rx_next advances and a cumulative ACK becomes pending. Valid DATA length>0 only. RX callback copies into owned memory before return; never retain borrowed DataView/mbuf/base64 buffers.
- Fully duplicate DATA ending<=rx_next is discarded and triggers current cumulative ACK. It is not fed to TLS twice. Old duplicate bytes already consumed are not retained for comparison; no claim to detect changed duplicates after consumption. A partially overlapping frame or frame beyond advertised window is protocol-fatal. Offset>rx_next but within window is discarded (Go-Back-N), with immediate current ACK. No out-of-order allocation.
- ACK can release only fully dispatched frame boundaries: reject ack>tx_dispatched, non-boundary forward ACK, arithmetic overflow, receive_limit<ack, receive_limit>ack+advertised peer capacity or >MAX_STREAM_BYTES. Queued-but-not-dispatched bytes cannot be acknowledged. Old ACK with ack<tx_acked is ignored in full. Same/current ACK may grow peer limit monotonically; an older smaller window never shrinks the current accepted limit. ACK alone never allocates a buffer or completes a fiscal operation.
- ACK/control has reserved bounded storage independent of DATA window. Coalesce ACKs into one latest pending snapshot, rather than accumulating one object per fragment. Priority: CLOSE, OPEN_ACK, latest ACK, retransmission DATA, new DATA. Control is not charged DATA credit. Do not acknowledge ACKs. Control storms are rate-limited/coalesced, with a finite per-pump step budget.
- Duplicate ACKs do not cause immediate retransmit (avoid ACK storms); only retransmission timer drives Go-Back-N. A receiver gap indication does not reset retry budget. New cumulative ACK advancing tx_acked resets the progress timer/retry budget; duplicate/credit-only ACK cannot keep an unacknowledged range alive forever.
- While a replay batch is being sent, re-evaluate latest tx_acked before each frame. A delayed ACK may free earlier frames; never dereference freed slots. Only one SDK TX operation is in flight per connection. Local transport control/write completion and peer receipt ACK are independent; ACK may arrive before the SDK write promise settles.

Global MAX_STREAM_BYTES=`0xffff0000` per direction per connection. Neither offsets nor advertised credit wrap. Before accepting/enqueuing bytes past the cap, close with stream-exhausted; caller must establish fresh BLE/TLS connection. Clamp only computed receive_limit at this global cap; never clamp malformed incoming fields. Counters/timers use monotonic clock with tested wrap-safe duration arithmetic. No resumption/0RTT/replay of old TLS bytes on reconnect.

## Local I/O completion and bounded retry/deadlines

`dispatch(frame, token)` takes an owned stable copy and marks that exact frame as dispatched before invoking asynchronous SDK code, so a synchronous peer ACK can never acknowledge an allegedly unsent frame. A token includes local generation and frame lease. Its callback may arrive after peer ACK, timeout/reset/reconnect; it must not dereference discarded core state. Keep at most one local frame copy (182bytes) until the local operation settles or owner is invalidated. A late completion never schedules another frame.

Central writes use write-with-response only (no deprecated writeValue fallback and no parallel GATT operations). A successful write response proves local ATT acceptance, not TLS/fiscal completion. S3 sends only while subscribed and active owner matches. `ble_gatts_notify_custom` local return/TX callback are scheduling signals, not receipt proof. If mbuf creation fails or SDK returns an explicitly retryable no-memory/busy code, retain the frame without consuming a new descriptor and retry under the same absolute deadline. Check the pinned SDK API's mbuf ownership on all return paths before implementation. Never double-free a consumed mbuf. Other errors close; do not blindly retry disconnected/permission/malformed operations.

Proposed v1 bounded values (test constants, HIL tuning may require explicit reviewed revision):

| Timer / cap | Value | Required behavior |
|---|---|---|
| chooser | user-controlled; abortable ownership | no transport network work after owner invalidation; OS chooser may settle late |
| connect/discover/subscribe/INFO |10s each +30s setup absolute after device selected | any failure invalidates all pending steps |
| OPEN | RTO750ms, exponential cap3s, <=6 retransmits, setup absolute limit | identical OPEN, same state; no fallback |
| DATA ACK RTO |750ms→1.5s→3s max, <=6 retransmissions per oldest outstanding range | replay bounded retained flight; stop at15s no ACK progress regardless duplicate traffic |
| SDK write / NOTIFY_TX |5s | stuck promise/local credit invalidates generation; do not queue subsequent writes |
| pending TX with peer zero credit |15s no consumable credit | fail closed, no growth; timer does not run on truly idle stream |
| ACK dispatch | next available local slot; <=25ms coalescing delay when slot free | immediate on gap/duplicate; not repeatedly enqueued; ACK shares no DATA credit |
| TLS handshake |120s absolute from first TLS activity | browser20 may be slow; no automatic secure downgrade; validate actual throughput |
| graceful TLS close |3s | standard TLS close_notify via DATA, then transport close/disconnect; receipt ACK is not peer TLS close acknowledgment |
| transport CLOSE |best effort <=300ms then disconnect/free | no indefinite wait for peer, no business success implied |

These timer proposals do not redefine existing operation-level30s/120s deadlines. Handshake occurs before admitting an operation; a command owner remains separately bounded once sent. Android/iOS application backgrounding/browser page freeze cannot pause an unbounded retry indefinitely: on resume detect monotonic absolute deadline expiry before any write. No background delivery guarantee is claimed.

CLOSE reason codes0 normal,1 protocol,2 timeout,3 cancel,4 resource,5 local-I/O,6 TLS failure,7 stream-exhausted; generic bounded diagnostics only. Both directions stop new DATA, best-effort CLOSE_ACK echoes reason, then clear stream. Simultaneous CLOSE is idempotent. Generation-mismatched CLOSE is ignored. For TLS-driven graceful close, TLS engine owns close_notify; transport CLOSE must not be treated as a verified TLS close or proof of fiscal outcome. Any partial/outstanding operation remains UNKNOWN until existing idempotent recovery proves state.

## Mapping into real callbacks (implementation plan, not enabled now)

### S3

- `ble_runtime.cpp`: add separate characteristic table/service and read_stream_info/receive_stream callbacks; do not route through `receive_command`, `receive_provisioning`, `receive_flow` or CommandQueue. Validate active peripheral conn+generation, exact mbuf size and copy bounded bytes before callback returns. Prefer core ingest/short lock into preallocated RX ring to an unconstrained external queue.
- `gap`: CONNECT creates state; SUBSCRIBE for exact stream TX handle controls subscription; MTU verifies selected size; NOTIFY_TX for exact peripheral handle/TX value handle releases one local TX slot; DISCONNECT clears owner before scheduling advertisement. Event for BluePad central handle must not alter MiniPOS stream. Reset callback fans out invalidation to stream and existing BluePad owner.
- A single serialized core owns rings, offsets, pending ACK and timers. An adapter lock may protect bounded ring operations, but never invoke TLS, malloc, callback, ble_gatts_notify_custom, wait or application sink under it. Schedule BLE sends on host context with fresh owner check; TLS worker read/write/tick use the same short core lock and bounded nonblocking API. Do not run a TLS handshake in NimBLE host callback.
- TLS BIO maps to core read/write WANT states. A bounded worker pump wakes on RX, peer ACK/credit, local TX completion and timers. Core event queue must be coalesced (one wake), not one heap task per fragment. Cap steps per pass (e.g.32) and yield; exact timing is target measurement.
- Future TLS plaintext result publishing needs a secure-channel handle tied to exact authenticated connection generation. Current `ble_publish_event(operation_id,...)` cannot be silently redirected. Future plaintext admission requires both authenticated device channel and client authority, then normal per-intent checks/CommandQueue. Foundation tests assert zero calls to current CommandSink from stream bytes.

### RN native

- Keep scanner/device selection in `NativeBleTransport`; add stream-specific connection object behind explicit test-only factory until secure route schema exists. Read INFO via manager/device characteristic read; subscribe through monitorCharacteristicForService new TX. Bound base64 input before decode (max182 decoded, canonical length cap), copy and call compatible framing/core adapter. Promise continuations all check generation.
- RequestMTU on Android remains best effort connection-stage policy; choose20 if no valid known cap. iOS uses negotiated manager-reported MTU, without requestMTU assumption. Base frame contract works at23 instead of enforcing legacy61. Separate legacy BFF1 minimum remains unchanged.
- One writeCharacteristicWithResponseForDevice at a time, one timer and one lease. Control priority includes peer ACK generated while a DATA write is in progress. Monitor error/disconnect/cancel resets worker/TLS/session authority and removes listeners; old callbacks/late write/late connect must remain inert.
- Native C TLS binding reads/writes ciphertext through byte adapter only. Do not export TLS traffic keys into JS, use WASM in Hermes or expose S3 NVS private key.

### Both actual browser clients

- Put shared pure framing/stream state in a workspace-neutral source consumed by BeeMiniPOS web and miniposweb (not a fragile runtime relative import outside Expo root); compile/test identical source content for both. Native may use the same TS transport framing while TLS stays native C. Core C is the independent S3 implementation.
- Adapt WebBleBootstrap and WebBleRoute to the same explicit connection API: connect/select→notifications/listener→INFO→OPEN→opaque send/read→close. RequestDevice grants the new UUID via optionalServices. Each has its own owner; no global shared device/stream state.
- Use gattserverdisconnected listener and AbortSignal cleanup; add characteristicvaluechanged listener before startNotifications to avoid losing immediate values, then perform INFO/OPEN only after startNotifications resolves. Remove listeners and stopNotifications best effort; invalidate owner before awaiting cleanup. A late stop/disconnect callback cannot affect a newly created connection.
- Strict20-byte client frame offer; no automatic retry of a failed oversized GATT write at smaller size. No unbounded chain of Promises from RX events; bounded synchronous parser/core ingest and one scheduled worker wake. Serialize GATT writes through one queue including ACK, not one queue per caller. Copy DataView byteOffset/byteLength correctly.
- PWA submit cancellation stops all writes/notifications and disconnects even if result timeout/abort fires before discovery completes. Operation recovery queries the stable business operation ID through supported route; transport retransmission never calls submit again.

## Resource envelope (design caps, not S3 heap measurement)

Per connection core: TX2048+RX2048 ciphertext rings; <=16 descriptors x16bytes budget256; pending control slots3 x20bytes=60; one owned local frame182bytes; bounded counters/state budget<=512bytes. Target core static size<=5120bytes with compile-time assertions. Exact struct size is a test result to collect, not yet measured. No separate out-of-order buffers, unbounded promise chains or complete8192-byte command allocation in this layer. One active MiniPOS stream only; BluePad has separate resource ownership.

TLS heap, certificate parse buffers, existing BIO queues, native bridge/WASM copies, OS BLE stack and NimBLE mbufs are additional costs. Avoid accidentally adding a second RX/TX8KiB queue pair in the portable TLS wrapper; reuse stream ring or account for both explicitly.12 x256MSYS blocks are shared; one notification local slot and no busy loop must preserve BluePad progress. Dynamic peak heap/largest block/stack high-water/latency and foreground/background measurements on actual S3+phones remain OPEN.

At browser20 only4 DATA bytes/frame and one response write at a time: throughput can be poor even with16 flight frames. No estimated host speed proves120s TLS feasibility. Measure certificate-size handshake on actual supported browser/phone/adapter combination and simultaneous BluePad activity; if unacceptable, review an explicit tested larger-value browser profile or revise architecture, never bypass TLS/authentication.

## Acceptance tests required before any production adoption

1. Independent codec vectors: manually fixed hex for INFO/OPEN/DATA/ACK/CLOSE at offset0,255,65536,nearMAX; C emits→TS parses and TS emits→C parses. Malformed every length/type/version/integer and nonzero DataView offset; no shared codec generator as sole oracle. All JS imports must run under both actual app toolchains.
2. Deterministic duplex fault runner: C stream peer vs TS peer in both roles, simultaneous bidirectional random bytes/TLS-sized transcripts, MTU/value20/61/182, RXwindow256/2048. Independently inject drop, duplicate, reorder, stale ACK/credit, notification lost, delayed ATT response, delayed NOTIFY_TX, lost OPEN_ACK and local transient no-memory. Compare exactly-once ordered bytes to independent expected originals and actual memory/queue caps. No financial commands.
3. ACK abuse: future/unsent/nonboundary ACK, ACK for merely queued bytes, overflow/limit beyond capacity, stale ACK that would reopen consumed credit, repeated gap/dup causing retry storm; assert fail/ignore per specification and no allocation growth. Distinguish ACK arrival before SDK promise completion. Zero-credit then drain must resume without deadlock; peer never drains must close.
4. Generation: disconnect at every async stage; same numeric handle reused; late old notifications, INFO/read, OPEN_ACK, writes, timers, worker output, NOTIFY_TX, removed listeners and rejected promises. Forced nonce repeat/RNG error/local generation wrap/offset exhaustion/clock wrap. No old byte enters new TLS instance and no old callback closes new owner.
5. Actual S3 callbacks: extend `tests/ble_runtime_native.cpp` fake headers with SUBSCRIBE/MTU/NOTIFY_TX/reset and invoke actual service access callbacks, mbuf ownership/failure, write-before-OPEN, foreign handle, unsubscribe mid-flight, reset while worker pending, no-notify-before-subscribe, BluePad-central disconnect ignored. Original BFF1/provisioning tests stay passing. Build full actual IDF image with reviewed lock/gates.
6. Actual clients: invoke NativeBleTransport real methods with fake BLE manager; invoke WebBleBootstrap and WebBleRoute through fake browser GATT containing real event targets, abort and late native promises. Assertions inspect the new actual UUID writes, lifecycle, queue high-water and exact ordered byte delivery, not only a standalone helper. All existing native/BFF1/operation ownership tests preserved. Both browser bundle/type checks and native compile gates are required.
7. TLS integration test-only: actual pinned mbedTLS server BIO over C core vs common native/WASM TLS client driven by each adapter, with independent Go TLS peer for stream+TLS cross-check. Loss/dup/reorder must preserve TLS or terminate boundedly; corruption must never yield plaintext. Verify client ServerAuth/name/pin and protected PoP in their later packages; transport OPEN alone must not enable canSendComplianceIntent. Never bypass TLS verification to get radio tests green.
8. Counterfactual regressions: disable duplicate suppression, future-ACK check, generation guard, byte/frame bound, retry deadline and subscription gate in isolated variants; the corresponding tests must fail. Sanitizers/bounded randomized seeds on C core, deterministic fake monotonic clock and no test sleeps for protocol timers. Persist seeds/limits/tool versions/source hashes.
9. Hardware OPEN: Android/iOS actual native bridge, supported WebBluetooth browser, S3+BluePad simultaneous role traffic, MTU23/negotiated larger, throttled/frozen JS, app background/resume, disconnect/reboot/low memory, usable handshake latency and minimum heap/stack. Host pass is not physical BLE/HIL certification.

Stage A: review contract only. Stage B: portable C core + TS codec/state + independent fault tests; no SDK callbacks enabled. Stage C: integrate existing real S3/three-client callbacks behind explicit test-only harness, compile/type/browser adapter lifecycle tests and full software regressions. Stage D: TLS/pin/PoP/secure-policy vertical then physical gates. Each stage records exactly which gates remain OPEN; do not report MP026 complete from a codec test alone.

## Primary references checked for API constraints

- [NimBLE GATT server reference](https://mynewt.apache.org/latest/network/ble_hs/ble_gatts.html): local notify API and mbuf ownership must be checked against pinned SDK implementation when coding.
- [Web Bluetooth API draft](https://bluetooth.spec.whatwg.org/): GATT write-with-response/notifications and connection lifecycle; no negotiated MTU accessor is provided by the exposed characteristic API.
- [react-native-ble-plx documentation](https://dotintent.github.io/react-native-ble-plx/): iOS MTU exchange is automatic and requestMTU is Android-specific; published site labels3.3.0, so implementation must also inspect installed3.5.1 native adapter before relying on version-specific behavior.

Repository observations are pinned separately in SOURCE_OBSERVATIONS_SHA256SUMS; do not substitute public generic API docs for current source/runtime tests.


## Stage B implementation candidate and evidence

Isolated original source observation HEAD: `4e7b9712d3520eccc7b9bc7b0e11b369fe3e07c4` at final packaging; observations initially captured by SOURCE_OBSERVATIONS_SHA256SUMS. Original source files were never edited by this worker. Proposed new module: `IoT/common-modules/ble-stream`, portable C11 core and dependency-free shared TS framing/state, package export prepared for later file-dependency use by all three TS transports. No existing app/runtime imports or IDF CMake changes are included. Thus no claim of actual SDK callback/TLS integration or production activation.

Actual software evidence: **35 PASS** under Clang AddressSanitizer+UndefinedBehaviorSanitizer C peer and Node22.14 strip-types TS peer, including both C server/client roles, ATT-value20/61/182 × receive window256/2048, simultaneous duplex data with deterministic drop/duplicate/reorder/delayed ACK/local SDK completion scheduling; independently fixed hex vectors, offsets255/65536/nearMAX, exact retransmission,16 descriptors/2048-byte rings, unsent/nonboundary/excess-credit ACK rejection, stale generation/token, failed I/O preventing further reads, stalled I/O and no-progress termination, zero-window drain/resume or bounded abort, backwards/invalid time, deadline-addition overflow, invalid TS read count, CLOSE18 and exhaustion. An additional independent C boundary test confirms SIZE_MAX/near-limit write rejection before any payload access, rejects oversized/null feed before generation peek, and safely clamps SIZE_MAX reads. The pre-fix write test was actual ASan RED (terminal134); final boundary sanitizer run is PASS. The reproducer independently restores only that former unsigned-addition expression in a disposable mutant and requires ASan stack-buffer-overflow RED (observed signal6); semantic35 and the original six mutation checks remain unchanged. Six isolated counterfactuals each produce expected assertion RED: duplicate suppression, future ACK rejection, generation fence, SDK deadline, TX ring bound and descriptor cap. This uses deterministic fault scheduling, not a claim of exhaustive state-space/model checking.

Installed BeeMiniPOS and miniposweb TypeScript compilers both strict-check this exact shared TS source: PASS. This is a module type check, not an app bundle or Metro dependency-resolution test. Android NDK27.1.12297006/API24, iOS arm64 device and arm64 simulator15.1 compile the exact C source: PASS compile-only. No Android/iOS app runtime or actual GATT operation was executed. Host sizeof(bs_stream)=4368bytes; target compile-time static assertion<=5120 passes. This is the C state struct only, not total TLS/native/JS/S3 heap. JS object/GC overhead remains unmeasured even though ring/descriptor counts are bounded.

[Evidence manifest](evidence/gatt-stream-stage-b-2026-10-21/manifest.json) preserves source hashes, sanitizer/cross/TypeScript logs and expected-RED records; no binaries or private keys. Reproduce with `python3 IoT/common-modules/ble-stream/reproduce.py --evidence /private/tmp/mp026-stageb-evidence`. The script copies sources into a disposable temporary test directory and installs nothing. Its optional `--repo` locates already-installed app compilers during isolated review; unavailable compilers are reported OPEN. Optional `cross_compile.py --ndk <installed NDK27.1.12297006> --log <path>` repeats compile-only portability checks without altering SDKs.

Boundary notes for Stage C: the serialized adapter must provide current monotonic timestamps and tick monotonic time on every wake/resume before dispatch, enforce actual peripheral handle/subscription/local connection-owner generation, generate/reuse-check the incarnation using the approved RNG and invalidate old owners before reinitializing. Core receives caller-owned incarnation and validates nonzero; it does not itself generate random values or persist an incarnation history. Pending local frame lease includes incarnation+token, but outer generation remains mandatory against object/memory reuse. All nonzero SDK settlement errors conservatively abort; no SDK busy/NOMEM retry optimization is claimed. Hardware and actual callback adapters must implement ownership/mbuf handling before any such optimization. C test-driver O/LIMIT commands only fast-forward arithmetic state and do not exist in production library API.

**OPEN:** Stage C actual four callback integrations/subscription and BluePad co-role/resource guards; approved random-incarnation lifecycle; full S3 build including this module; native bridges/WASM-TLS attachment; signed route UUID/version and fail-closed downgrade policy; server identity/name/chain/SPKI pin; protected client PoP and time/rotation policy; plaintext admission and financial outcome gates; real target heap/stack/latency/HIL. Existing BFF1/provisioning/financial behavior has not been rerouted. Do not mark MP026 or the authenticated channel complete from these Stage B results.

## Root integrated verification — 2026-10-21

Integrated C↔TS reproduction PASS with ASan/UBSan, 35 fault cases, six semantic counterfactuals and the SIZE_MAX unsafe-expression counterfactual. Android arm64, iOS arm64 device and simulator C compile checks PASS; these are compilation checks, not mobile runtime tests. [Root evidence and tested source hashes](evidence/gatt-stream-stage-b-2026-10-21/root/manifest.json). Actual GATT callbacks, packaged clients, TLS, authenticated admission and physical HIL remain OPEN.

## Regression and CI entrypoint

`make ble-stream-test` is now a root regression prerequisite and passed locally. Dedicated `.github/workflows/ble-stream.yml` runs mandatory sanitizer/C↔TS fault tests on Node22.14.0 and retains evidence on failure. Workflow YAML and Make expansion checked locally; remote execution remains OPEN. Optional app compiler checks do not imply app bundler acceptance. [Entrypoint log and source hashes](evidence/gatt-stream-stage-b-2026-10-21/entrypoint/manifest.json).

### GCC CI portability follow-up

First actual Ubuntu CI run39544110964 rejected two adjacent C guards under `-Werror=misleading-indentation`; this is a failed run, not acceptance. Split those statements without changing semantics or disabling warnings. GCC15 strict compile and full local Make sanitizer/fault reproduction PASS afterward. [Before/after evidence](evidence/gatt-stream-stage-b-2026-10-21/gcc-portability/manifest.json). Remote rerun required.

Actual Ubuntu rerun [39544238159](https://github.com/BetinVladimir/Fiscalisation/actions/runs/39544238159) at e81a3f3 PASS:35 fault cases, sanitizer boundary and expected-RED mutations. [Downloaded raw evidence hashes](evidence/gatt-stream-stage-b-2026-10-21/remote-success/manifest.json). App TypeScript compiler checks are explicitly OPEN in this dependency-free CI; earlier local app-compiler checks remain separate. No runtime/HIL acceptance is inferred.
