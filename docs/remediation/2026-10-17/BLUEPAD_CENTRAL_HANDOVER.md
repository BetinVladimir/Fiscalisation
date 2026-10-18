# BluePad central: documented purchase path and acceptance evidence

Date: 2026-10-18. Target: actual ESP32-S3 NimBLE central, DATECS / BLUEPAD-50 PLUS / BLE_GATT. Device and acquirer application compatibility still require real-device acceptance. Keys remain internal ESP32-S3/NVS; this change introduces no external secure element.

## Primary local vendor sources

Repository-adjacent vendor SDK: `examples/datecs/BC-50/datecspay-java-sdk_2025-02-28_v9.0.0Src`.

`DOC/DatecspayLowLevel.pdf` is **Datecs Pay Pinpad Commands v3**, Datecs software department, 2025. SHA256: `84a1d5156f17a322921734e6e97cedd86bcd40f5cc4040481110d0ab14d29b7c`. Page numbers below are the printed PDF page numbers.

| Evidence | Runtime contract |
|---|---|
| v3 p3 | 3E packet, command/status, BE length, XOR; 2s interbyte / 5s command response bounds |
| v3 p9–10 | Reference purchase START sub01 / type03, amount81 + merchant reference DF03; DF01 is not the merchant reference |
| v3 p33 | GET_RECEIPT_TAGS sub02 reads last performed transaction; request is concatenated tag IDs |
| v3 p34–35 | TX_END sub03 with two-byte confirmation 0001 success / 0000 failure |
| v3 p51–53 | Final event0E/sub01; sub02 concerns an earlier hung transaction and cannot approve the current purchase; STAN9F41 is BCD |
| v3 p63–66 | DF05 0 approved / 1 declined / 2 error / 3 other interface / 4 retry; transaction type reference purchase3; DF10 example is two-byte BE |
| `BluetoothLeConnector.java`35–43,405–438 | Exact service prefix and READ/WRITE/POWER/WAKE UUIDs; acknowledged READ then POWER CCCD writes before POWER read |
| same445–539 | POWER31 ->200ms settle; POWER30 ->notification31 ->WAKE write name ->write completion ->2200ms settle |
| same636–780 | 19-byte chunks, wait each characteristic-write callback |
| `DatecsPayWorker.java`419–435 | Purchase-reference constructs amount +DF03 then starts type03 |
| same820–830,969–1010,1199+ | Final callback wakes worker; result processing reads receipt tags, including DF03, amount, type, STAN and result/error |

Some packet examples contain inconsistent commentary about length/checksum; runtime follows the packet definition and tests the independently transcribed purchase-reference vector `3e3d0000110103810400000001df030631323334353649` (amount1, reference123456), not an encoder-generated expectation.

## Implemented behavior

Readiness requires discovered characteristics with required properties, bounded descriptors belonging to READ/POWER, both CCCD acknowledgments, POWER/WAKE workflow and settling. Setup has a finite30s bound. Financial writes use19-byte acknowledged chunks with one5s total send deadline per packet. An immediate error before any accepted write is NotSent; completion error, timeout or partial enqueue is Unknown. There is no automatic retry of a possibly transmitted payment.

A single operation mutex covers the whole purchase. Connection generation, monotonic write tokens and handle checks reject stale callbacks. Submission is serialized with disconnect/host reset; callbacks never wait for a write completion. State mutex is released before calling NimBLE. Static semaphores outlive late callbacks.

Purchase sends type03 with exact merchant operation reference DF03, requires empty successful START ACK, waits only final01, then reads receipt tags. Approval requires exact DF03, amount, reference-purchase type0003, matching valid BCD STAN and result/error. RRN/AUTH must exist for approval and agree with final-event values when supplied. Only verified result0/error0 is Approved; verified result1 is Declined. Other/error/retry results remain Unknown. It then sends TX_END and requires its empty ACK before releasing the session. END failure remains Unknown even when receipt evidence was available.

Unknown permanently quarantines this in-memory central session; subsequent monetary calls are NotSent. No automatic abort, VOID, lookup or retry is issued. Disconnect/reset during an active operation also quarantines. Normal inactive disconnect can rediscover with a fresh generation. Runtime RX limits are4096 payload bytes,514 bytes per notification,8 queued frames and2s between fragments; malformed XOR/BER/duplicate tags fail closed.

The configured BLE identity is limited to19 UTF-8 bytes because WAKE is one bounded default-MTU write in this adapter; a longer vendor name needs an explicitly tested long-write profile. Merchant reference is printable ASCII1..64; amount1..UINT32_MAX minor units. These are adapter acceptance bounds, not claims about all vendor/acquirer limits.

Shared `ble_hs_cfg.reset_cb` now calls `bluepad_ble_on_host_reset()` (native wiring regression included); the reset hook is required and a later sync calls the existing sync hook. This prevents stale readiness when NimBLE resets without an ordinary disconnect.

## Software acceptance

`python3 IoT/firmware/edge-agent-s3/idf/tests/bluepad_host/run.py` compiles **the actual production .cpp**, substitutes only NimBLE/FreeRTOS APIs, and runs independent packet vectors plus asynchronous callback scenarios. It exercises confirmed purchase/decline, fragmented/coalesced RX, POWER/WAKE, missing POWER and failed/absent CCCD/read/wake, generation reuse, wrong DF03/type/STAN, invalid BCD/missing/duplicate/invalid BER tags, intermediate02/empty ACK, missing final proof, bad XOR/oversize/queue/interbyte bounds, TX_END failure, serialized chunk completion, busy concurrent call, immediate/partial/completion failure, disconnect, host reset and late write tokens. Every uncertain path checks quarantine and absence of later sends. Baseline original runtime failed the readiness assertion before CCCD completion.

Host fake time is accelerated to keep timeout cases bounded; it is not a physical radio timing or bank certification test. Actual pinned ESP-IDF/ESP32-S3 compilation must also pass before integration.

## Remaining device/acquirer acceptance

Record actual terminal firmware, acquirer app version and SDK/spec hash. On sandbox/acquirer-approved terminal verify DF03 survives reference purchase and last-receipt retrieval, exact DF10 width/value, final/receipt STAN and RRN/AUTH, POWER workflow, TX_END confirmation semantics and repeated purchases. Test host reset, radio loss at every chunk, terminal reboot, and bank-approved but fiscal-failed handling. An unresolved payment must remain Unknown and enter operator/acquirer reconciliation; process restart is not proof of a safe new payment. Persistent quarantine/reconciliation authority remains a separate recovery requirement.

GET_REPORT_BY_STAN requires real STAN; a UUID cannot be converted into it. Generic reversal lacks the original AUTH required by documented VOID purchase. Verified query/reversal capabilities remain false. Do not enable them until the full vendor/acquirer contract, original payment evidence and physical test matrix are available. HIL and certification targets remain in scope.
