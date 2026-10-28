# TRACE_CRITERIA — fiscal-contour-review-20261021 criteria mapped to MVP1 work items

Each criterion from docs/MVP1/fixtures/review_criteria.json is mapped to a TST-*/BUG-*
work item, or out_of_mvp1 with a reason if not in scope.

| ID | Summary | Area | TST/BUG reference |
|---|---|---|---|
| PAY-001 | Sale with CASH payment records correct amount in cents | PAY | TST-CT-04 |
| PAY-002 | Sale with CARD payment records correct amount in cents | PAY | TST-DEEP-01 |
| PAY-003 | Split payment CASH+CARD sums equal total | PAY | TST-DEEP-02 |
| PAY-004 | Zero-quantity sale line is rejected before fiscal write | PAY | TST-DEEP-05 |
| PAY-005 | Payment amount mismatch with line total is rejected | PAY | TST-GO-03 |
| PAY-006 | Daisy Compact: CARD/split payment rejected before I/O | PAY | S3D-01 |
| PAY-007 | Payment slots BC-50: CASH=0, CARD=1, CHEQUE=3, VOUCHER=4, COUPON=5 | PAY | S3D-03 |
| PAY-008 | Unsupported payment type NHIF/DEFERRED/INTERNAL → PAYMENT_TYPE_UNSUPPORTED | PAY | TST-KT-01 |
| PAY-009 | Currency EUR gateway check (VG-09/VG-23) enforced | PAY | TST-CT-01 |
| PAY-010 | Golden receipt: total 1367¢ = 1200+167, CASH 500 + CARD 867 | PAY | VG-23 |
| REC-001 | Storno requires confirmed fiscal storno before card refund | REC | REFUND-01 |
| REC-002 | Durable refund state survives crash after storno | REC | TST-DEEP-06 |
| REC-003 | Durable refund state survives crash after refund send | REC | TST-DEEP-07 |
| REC-004 | Durable refund state survives crash before refund ACK | REC | BUG-071 |
| REC-005 | Lookup by payment_id prevents duplicate storno | REC | TST-KT-05 |
| REC-006 | Lookup by payment_id prevents duplicate successful refund | REC | TST-GO-07 |
| REC-007 | Smart S: no card-refund command → REFUND_MANUAL_REQUIRED | REC | TST-DEEP-08 |
| REC-008 | Multi-CARD refund handles each card separately | REC | TST-E2E-05 |
| REC-009 | BC-50 storno uses cmd43; external terminal | REC | BUG-066 |
| REC-010 | E-receipt: send rejected (feature disabled), no implementation | REC | TST-CT-04 |
| FW-001 | Firmware SEQ for Datecs: 0x20..0x7F wraps correctly | FW | S3D-01 |
| FW-002 | Firmware SEQ for Daisy: 0x20..0xFF wraps correctly | FW | S3D-03 |
| FW-003 | Datecs frame: LEN = data + 0x2A | FW | TST-S3-01 |
| FW-004 | Daisy frame: LEN = data + 0x24 | FW | TST-S3-02 |
| FW-005 | CP1251 text encoding for fiscal unit commands | FW | TST-S3-03 |
| FW-006 | Text limit 72 bytes per Datecs line (CP1251) | FW | TST-S3-04 |
| FW-007 | NAK/SYN handling in firmware protocol | FW | TST-S3-05 |
| FW-008 | Deadline/timeout handling in S3 edge agent | FW | EVIDENCE-01 |
| FW-009 | Profile executor maps payment slots from FiscalExecutionConfig | FW | TST-DEEP-13 |
| FW-010 | Factory manifest distinguishes app/merged image | FW | TST-DEEP-14 |
| TIME-001 | Business date read from BC-50 cmd62 | TIME | TST-DEEP-05 |
| TIME-002 | Clock drift WARN when drift > 5 minutes | TIME | TST-GO-10 |
| TIME-003 | Clock drift BLOCK when drift > 2 hours | TIME | TST-KT-08 |
| TIME-004 | Business date timezone: Europe/Sofia | TIME | BUG-064 |
| TIME-005 | Clock sync does not override fiscal device time | TIME | TST-E2E-01 |
| TIME-006 | Shift boundary uses business_date not wall clock | TIME | TST-S3-06 |
| TIME-007 | Clock read cmd62 returns valid timestamp | TIME | BUG-067 |
| TIME-008 | Time anchor validated on device binding | TIME | TST-DEEP-12 |
| TIME-009 | LAB profile does not enforce clock drift check | TIME | TST-GO-11 |
| TIME-010 | PROD profile enforces clock drift BLOCK > 2h | TIME | TST-DEEP-11 |
| WIRE-001 | Datecs 116 golden frames: Kotlin encode matches C++ encode | WIRE | S3D-01 |
| WIRE-002 | DatecsPay 72 golden frames parsed by IoT library | WIRE | TST-CT-03 |
| WIRE-003 | MQTT topic: tenants/{t}/devices/{d}/commands consistent publisher/subscriber | WIRE | TST-CT-05 |
| WIRE-004 | MQTT envelope finalize parsed by all three implementations | WIRE | TST-DEEP-14 |
| WIRE-005 | MQTT envelope reverse parsed by all three implementations | WIRE | TST-VENDOR-04 |
| WIRE-006 | MQTT envelope ack parsed by all three implementations | WIRE | TST-VENDOR-05 |
| WIRE-007 | MQTT result COMMITTED parsed by all three implementations | WIRE | VG-16 |
| WIRE-008 | MQTT result NOT_COMMITTED parsed by all three implementations | WIRE | VG-17 |
| WIRE-009 | MQTT result UNKNOWN parsed by all three implementations | WIRE | TST-KT-11 |
| WIRE-010 | Invalid MQTT signature → rejection with same error code all impls | WIRE | TG-006 |
| UI-001 | POS displays correct total in cents formatted as currency | UI | TST-POS-01 |
| UI-002 | MiniPOS displays correct total in cents formatted as currency | UI | TST-POS-02 |
| UI-003 | UNKNOWN state shown to operator without auto-refund | UI | TST-POS-03 |
| UI-004 | Error codes from catalog shown in UI, not raw exception | UI | TST-POS-04 |
| UI-005 | Shift open/close flows follow UI contract | UI | TST-POS-05 |
| UI-006 | Receipt display reflects fiscal document number from FU response | UI | TST-POS-06 |
| UI-007 | E-receipt send button disabled/absent when feature off | UI | TST-POS-07 |
| UI-008 | Payment confirmation shows split breakdown | UI | TST-POS-08 |
| UI-009 | REFUND_MANUAL_REQUIRED shown with actionable operator message | UI | TST-POS-09 |
| UI-010 | BLE pairing/provisioning UI flow completes without crash | UI | BF-01 |
| BE-001 | OpenAPI handler count matches lock count at runtime | BE | TST-CT-02 |
| BE-002 | Unknown field in request body → 400 Bad Request | BE | TST-INT-01 |
| BE-003 | Response body validates against OpenAPI schema | BE | TST-INT-02 |
| BE-004 | Error response codes from catalog, not arbitrary strings | BE | TST-INT-03 |
| BE-005 | Backend without database fails closed (no data-loss silent error) | BE | PERSIST-01 |
| BE-006 | Outbox delivers exactly once per durable record | BE | TST-GO-01 |
| BE-007 | Storage error stops monetary side effect (invariant 5) | BE | TST-GO-02 |
| BE-008 | SUPTO identifier included in fiscal operation record | BE | TST-GO-04 |
| BE-009 | Reporting export consistent with fiscal operations | BE | TST-GO-05 |
| BE-010 | Metrics endpoint reports operational counters | BE | TST-GO-06 |
| AUTH-001 | Device activation requires valid certificate chain | AUTH | TST-VENDOR-09 |
| AUTH-002 | BLE server TLS validated against CA | AUTH | TST-KT-02 |
| AUTH-003 | Operator password not logged or committed | AUTH | TST-KT-03 |
| AUTH-004 | Keystore credentials not in test fixtures or logs | AUTH | TST-CT-05 |
| AUTH-005 | HIL validator uses external trust store, not key from bundle | AUTH | EVIDENCE-01 |
| AUTH-006 | Device binding requires operator authorisation | AUTH | TST-DEEP-03 |
| AUTH-007 | ECDSA signatures: DER and P1363 formats accepted correctly | AUTH | BF-01 |
| AUTH-008 | Wrong DER↔P1363 format → rejection without panic | AUTH | TST-DEEP-04 |
| AUTH-009 | Corrupted signature byte → rejection without panic | AUTH | TST-KT-04 |
| AUTH-010 | Revoked device activation blocked at BLE server | AUTH | BUG-068 |
| CON-001 | MQTT broker reconnect does not duplicate operation | CON | EMU-01 |
| CON-002 | BLE connection loss handled with graceful retry | CON | EMU-02 |
| CON-003 | S3 edge agent queues operation on network outage | CON | EMU-03 |
| CON-004 | Queue delivery exactly once on network restore | CON | EMU-04 |
| CON-005 | Device readiness probe fails fast when transport unavailable | CON | EMU-05 |
| CON-006 | Edge sync v2 handles concurrent device updates | CON | EMU-06 |
| CON-007 | COM port timeout does not leave partial fiscal state | CON | TST-NAP-01 |
| CON-008 | USB timeout does not leave partial fiscal state | CON | TST-NAP-02 |
| CON-009 | HTTP timeout (Smart S) results in UNKNOWN, not FAILED | CON | TST-E2E-09 |
| CON-010 | Delegated dispatch does not retry on UNKNOWN outcome | CON | TST-E2E-10 |
| MONEY-001 | LineTotal: (unit¢ × qty_milli + 500) / 1000 (half-up) | MONEY | TST-CT-01 |
| MONEY-002 | No float/double in money computation | MONEY | TST-GO-08 |
| MONEY-003 | No parseFloat/Number(price) in TypeScript | MONEY | TST-KT-09 |
| MONEY-004 | Discount applied as integer cent reduction | MONEY | TST-DEEP-01 |
| MONEY-005 | All 34 golden rounding vectors pass Go+Kotlin+TS | MONEY | TST-CT-04 |
| MONEY-006 | Golden: 1200+167=1367¢; CASH 500+CARD 867=1367 | MONEY | BUG-065 |
| MONEY-007 | USD currency rejected (only BGN/EUR) | MONEY | BUG-069 |
| MONEY-008 | Fractional cents in payload rejected with error code | MONEY | TST-GO-09 |
| MONEY-009 | Sum of payments must equal total (validated before FU write) | MONEY | TST-KT-10 |
| MONEY-010 | Zero or negative line amount rejected before fiscal write | MONEY | BUG-070 |
| MONEY-011 | Receipt money test covers all payment-type combinations | MONEY | TST-DEEP-10 |
| IDEM-001 | Same idempotency key does not create second fiscal document | IDEM | TST-DEEP-09 |
| IDEM-002 | Duplicate finalize request returns existing result | IDEM | TST-GO-13 |
| IDEM-003 | UNKNOWN after write: only lookup, no retry write | IDEM | TST-KT-14 |
| IDEM-004 | UNKNOWN after write: no auto card refund | IDEM | BUG-072 |
| IDEM-005 | Concurrent finalize same key: exactly one document/operation/UNP | IDEM | TST-GATE-01 |
| IDEM-006 | Conflicting payload on second key → PAYLOAD_CONFLICT error | IDEM | TST-E2E-13 |
| IDEM-007 | 20 concurrent finalize same key: one winner, rest idempotent | IDEM | TST-E2E-14 |
| IDEM-008 | Crash after write followed by replay does not re-send to FU | IDEM | TST-INT-05 |
| IDEM-009 | Smart S LAB profile: COMMITTED_NO_REFERENCE (no synthetic number) | IDEM | PERSIST-01 |
| IDEM-010 | Smart S PROD profile: response without number → UNKNOWN | IDEM | TST-DEEP-08 |
| IDEM-011 | UNP allocated once per operation; counter not reused after restart | IDEM | TST-INT-06 |

Total: 112 criteria
