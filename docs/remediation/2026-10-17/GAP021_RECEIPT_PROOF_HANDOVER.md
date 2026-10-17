# GAP021: vendor-issued receipt proof

Scope is actual ESP-IDF S3 execution; historical Go edge-agent is out of scope.

## Changes

Successful fiscal close no longer produces an EDGE hash as fiscal reference.
Datecs command56 response must parse as exactly ErrorCode0 and SlipNumber1..9999999.
Command125 then selects that exact global document, verifies fiscal-sale type1
or fiscal-storno type6 and valid calendar/time, and searches at most96 EJ text
lines for the exact whole UNP token. Command123 serial must match the configured
signed authority prefix. Any missing, mismatched or malformed evidence yields
UNKNOWN with empty fiscal_reference. Success returns the actual global document
number as a decimal string.

Daisy command56 contains per-Z all/fiscal counters, not a document number.
It must parse, but is never used as fiscal_reference. Commands113 and119 retrieve
and verify the issued global document number, exact UNP, fiscal-sale description
with fiscal/EJT flags, valid date/time and operation type. Storno additionally
matches its mapped reason type, original document number and original FM number.
Command90 verifies device serial against the configured authority prefix.

Datecs EJ scan stops after96 lines or when its8second between-command budget
expires. Each individual serial exchange separately retains its existing6second
maximum and identical-packet/sameSEQ retransmission policy. Monetary commands
are never replayed by the receipt-proof layer. It does not promise an8second
whole-operation wall-clock bound.

Daisy manual v2.0-4 p42 specifies HH.mm.ss for command119; the shared issued-
document parser now accepts that documented format and existing HH:mm:ss,
rejecting mixed delimiters. An independent literal vendor vector covers this.
Calendar validity is checked at the fiscal-proof boundary.

Other commands no longer label synthetic EDGE hashes as fiscal references.
Their successful transport completion may return an empty fiscal reference;
this does not assert that a report/cash-movement receipt identity was read back.

## Read-only restart proof

The DeviceIo API `fiscal_receipt_proof(const RecoveryItem&)` is supplied by the
parallel recovery/header package. EspIdfDeviceIo implements it without printing,
cancellation, payment lookup or payment reversal. Legacy recover delegates to it.

SALE_FINALIZE needs durable route command_type, closed transaction status and
exact document/type/time/UNP/serial proof. Historical snapshots without type
metadata remain UNKNOWN. REVERSAL recovery remains UNKNOWN without device I/O:
a reversal repeats the original sale UNP, so an old matching storno is not proof
of this operation. A future durable pre-print document watermark and exact
storno-linkage query contract are required before enabling that recovery.
The patch never treats absence from the latest-document query as proof of no
fiscal document and never converts that uncertainty to rejection/compensation.

## Evidence and tests

Primary local audit sources:
- Datecs programmers manual v2.11.4 section4.18 close56, section4.63 identity123,
  section4.65 EJ125 (global number, document types1/6, date and bounded text lines).
- Daisy protocol v2.0-4 sections56,90,113,119, pp23,32,40,42–43. Section119
  defines document flags, sale/storno types, UNP and original document/FM links.
  Extracted PDFs/text live in `/tmp/fiscal-deep-audit-20261017`.

Baseline actual original runtime: successful Datecs sale returns a synthetic
reference instead of470 (assertion RED), and malformed Daisy close still yields
Committed (assertion RED). Fixed actual runtime + real FrameCodec/SerialSession
passes53 processes under ASan/UBSan: sale/reversal/recovery-sale both profiles,
invalid close/number/UNP/substrings/type/date/serial, Daisy EJT flags and refund
linkage,96-line bound, and no-I/O uncertain reversal recovery. Tests assert no
money command is sent twice and no monetary command is sent during recovery.

`tests/receipt_host/run.py` is included in make iot-test. Existing money host
fixtures now supply independently specified issued-document responses and keep
all monetary/preflight byte assertions. The actual Go Prepare→firmware→backend
reversal runner likewise includes the new readout exchanges and real reference.
Protocol native vectors and actual vendor-codec tests remain green.

## Remaining acceptance limits

- Restart reversal identity needs a persisted watermark; unsupported recovery
  remains fail-closed, with no automatic reprint/cancel/refund.
- Returned fiscal_reference is now a real document number, but current result
  schema does not separately expose/store the queried issue time and device FM
  number. A typed durable receipt-evidence object is a follow-up schema migration.
- A separately controlled device or external app can change the last document;
  mismatched readout yields UNKNOWN. Physical port ownership must be exclusive.
- Certified firmware, printer-specific EJ layout/UNP print configuration and
  actual hardware signing must be confirmed on the physical HIL matrix.
