# GAP022 canonical JSON integration and reconciliation

Firmware now serializes actual event/batch envelope structs in Go declaration
order, recursively sorts generic payload maps, preserves exact int64 envelope
sequences, and uses the pinned portable shortest-float64 serializer. Actual
command payload digest and command HMAC use generic recursively sorted maps,
matching fiscal-backend/internal/mqttclient.Bridge.Prepare. ACK authentication
reconstructs actual Go SyncAck/SyncOperationResult typed declaration order,
including int64 version fields and generic rejected payloads.

Strict raw JSON ingress rejects duplicate keys, trailing junk, invalid grammar,
nonfinite overflow and nesting beyond16. UTF-8 decoding follows Go Unmarshal
replacement rules. Numbers retain lexical tokens so typed int64 fields never
round through cJSON. Commands reject embedded NUL strings/keys and numeric
magnitudes beyond the safe cJSON integer boundary before business decoding;
BLE CBOR rejects embedded NUL text before cJSON truncation. Reversal document
number/reason additionally reject fractional or clamped cJSON valueint values.
Opaque ACK rejection payloads retain NUL/Unicode/float semantics; critical ACK
IDs, hash and signature strings reject embedded NUL before C-string consumers.

## Existing frozen batches

Before any retransmission firmware recomputes canonical Go event/batch hashes
and validates the frozen sequence metadata. A mismatch returns
ESP_ERR_INVALID_STATE, logs FROZEN_SYNC_CANONICAL_MISMATCH and persists that
diagnostic on the relevant outbox rows. The frozen UUID, signed body, members,
cursor and monetary operation records remain unchanged. Firmware never silently
rewrites/resigns old incompatible batches or re-executes their money operations.

Operator reconciliation is a release prerequisite for an affected deployed DB:

1. Preserve a database copy, original frozen signed bytes/UUID, event membership,
   device identity/key ID and fiscal/POS references before taking action.
2. Compare with the backend authoritative last committed journal sequence/hash.
   If the exact batch was already committed, recover its authenticated backend
   ACK and apply through the normal verified ACK/storage path.
3. If it was not committed, reconcile actual fiscal/POS outcomes and backend
   journal state. Any journal migration/import requires a separately reviewed
   tool and evidence preserving operation/receipt identities and hash-chain
   continuity. Do not erase the queue, reset the cursor, fabricate ACKs, resign
   entries in place or replay physical money commands as a repair.
4. Keep hardware acceptance blocked until the incompatible frozen batch has a
   documented authoritative resolution. This packet supplies detection and
   retention, not an automatic migration policy.

## Acceptance evidence

Run `python3 IoT/firmware/edge-agent-s3/idf/tests/go_wire_host/run.py`. It stages
actual backend domain/config/mqttclient sources into a temporary Go module and
compiles the actual firmware sync_runtime, intent_processor, durable_storage,
canonical_cbor and serializers. CJSON_SOURCE_DIR, OPENSSL_PREFIX and
FISCAL_REPO_ROOT can override installed dependencies. SANITIZE=1 enables
ASan/UBSan. No network, brokers or Android tooling is required when Go module
dependencies are installed. DeviceIdentity signing is a declared host fixture;
the test validates cross-language event/batch hash preimages and real Go HMAC
command/ACK signatures, not the separate hardware ECDSA signing implementation.

- Real backend Prepare(SALE_FINALIZE) command with nested typed items, money,
  Cyrillic/HTML name passes actual IntentProcessor, including reordered keys.
- Valid command fixtures modified with duplicate keys, embedded NUL, trailing
  junk or an unsafe integer fail before any additional executor call.
- Firmware nested payloads with float thresholds, HTML, Unicode and NUL are
  accepted by actual domain.DeviceEventHash and domain.EdgeBatchHash.
- Actual Go SyncAck with int64-max operation version, nested rejected maps,
  HTML/Unicode/NUL and float thresholds authenticates after root/nested field
  reordering. Tampering fails; duplicate original ACK remains safe.
- Separate processes exercise restart with journal sequence1 and
  9223372036854775806, retaining exact values in SQLite.
- A legacy cJSON-rendered incompatible frozen body is blocked without rewrite.
- Existing SQLite recovery and sync fault/interleaving suites remain required.

The actual backend reversal contract remains a separate discovered gap:
Prepare emits command_type REVERSAL with original_document.reason, whereas
IntentProcessor currently expects SALE_REVERSE and payload.reason. This packet
does not claim reversal interoperability or change that contract implicitly.
