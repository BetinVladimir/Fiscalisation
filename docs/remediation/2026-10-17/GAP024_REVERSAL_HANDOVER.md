# GAP024: signed REVERSAL interoperability

## Contract fixed

The backend `mqttclient.Bridge.Prepare` publishes `command_type=REVERSAL` and
`payload.original_document.reason`. Firmware authenticates the original Go
canonical envelope before normalizing REVERSAL to the internal SALE_REVERSE
executor operation. Signed payload bytes and payload SHA256 are not rewritten.
Accepted journal snapshots expose REVERSAL to the backend materializer; committed
results include operation_type=REVERSAL and publish event_type=REVERSED.
The durable route snapshot stores this type for post-restart result classification.
Historical snapshots without type metadata are not guessed or retroactively changed.

Original document UNP is retained for the vendor storno open command, including
when the authority allocator reserves a different local sequence. Both supplied
UNP aliases must agree. The current allocator still consumes one authority number
for a reversal; deciding whether to remove that allocation is a separate authority
policy migration, not part of this wire-contract fix.

The backend canonical reason enum is 0 operator error, 1 return/complaint,
2 tax base reduction. Datecs uses these numeric codes. Daisy v2.0-4 uses
0 return/complaint, 1 operator error, 2 tax base reduction; firmware translates
only at the selected device-profile boundary. Optional reason_code must agree
with the numeric reason. Conflicting legacy top-level reason is rejected.
Document number is constrained to 1..9999999, and original UNP is ASCII validated.

## Protocol evidence

- Datecs *Fiscal Printer Programming Manual v2.11.4*, section 4.6 (command 43):
  Storno reasons 0/1/2, document-number bound, original document date/time and
  fiscal-memory number, and NSale original unique sale number for a PC sale.
  Local audit extraction: `/tmp/fiscal-deep-audit-20261017/datecs.txt`.
- Daisy *Communication protocol v2.0-4*, section 48, pp17–19:
  `Operator,Password,UNP\tRReason,DocLinkNum,DocLinkDT\tDocLinkFMNo`, reason
  0 return/complaint, 1 operator error, 2 tax reduction. Section 51 specifies
  subtotal and eight tax totals. Local extraction is in the same audit directory.
- Existing native Kotlin Datecs implementation independently supplies originalUnp
  to command 43; the original UNP must not become the newly reserved UNP.

## Reproducible acceptance

Run `python3 IoT/firmware/edge-agent-s3/idf/tests/reversal_host/run.py` (also in
`make iot-test`). This compiles actual IntentProcessor, SQLite storage, sync,
ProfileExecutor, profile orchestrator, serial exchange and FrameCodec.
Only physical UART/USB, signing hardware and MQTT transport are faked.
Independent literal vendor frames validate physical command bytes and sequence.

Actual Go Prepare creates signed commands for four reasons and both profiles.
Actual Go domain hashing verifies the resulting accepted/result batch. The real
backend SyncBatchForTenant then projects the original internal sale to CANCELLED,
operation type REVERSAL/state FISCALIZED, and one success webhook. External sale
ID deliberately differs from internal sale ID. Duplicate delivery sends no second
vendor operation. Five freshly re-signed invalid variants per profile (reason
conflict, reason-code mismatch, UNP conflict/null, document overflow) fail before
physical I/O.

Baseline: unmodified firmware rejects real Go REVERSAL with ESP_ERR_NOT_SUPPORTED
(261), before any vendor commands. Fixed isolated runtime: eight positive cases
and ten negative cases pass. The test supports both existing profile constructor
and the separately developed explicit execution-config/password constructor;
with that profile package its expected sequence includes mandatory subtotal 51.

## Still open; not claimed by this package

- Offline SALE_FINALIZE creation: backend materialization requires an already
  existing server sale; legacy FISCAL_SALE importer is a different path. An offline
  missing-sale test and explicit authoritative create/merge contract remain needed.
- Actual fiscal document readout: current executor exposes a synthetic EDGE hash
  reference, not the validated vendor close-response slip number. The interop test
  proves materialization of the returned reference, not correct real receipt ID.
- CARD storno requires original processor reference; payment UUID is not an RRN.
  The separate payment/profile package must keep unsupported reversal fail-closed.
- Hardware signatures and physical device acceptance remain separate gates. The
  host test uses a signing fixture and re-signs the unchanged batch hash with the
  backend legacy test HMAC solely to exercise actual backend projection.
- Historical pending reversals without stored type metadata need reconciliation;
  this patch neither guesses their type nor reprints a monetary operation.
