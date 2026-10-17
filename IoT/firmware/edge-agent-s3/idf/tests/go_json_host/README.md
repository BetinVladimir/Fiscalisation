# Go-compatible JSON serializer host proof

Run `python3 tests/go_json_host/run.py` from the IDF folder. Host prerequisites:
C/C++17 compiler, ESP-IDF cJSON sources (`IDF_PATH` override), OpenSSL headers and
libcrypto (`OPENSSL_PREFIX` override). No network or Android toolchain needed.
The baseline executes the existing cJSON_PrintUnformatted payload path and must
fail the independent Go map oracle; then the actual new production module runs.

Golden oracle generation imports the actual backend `internal/domain` and
`internal/mqttclient` packages, including their DeviceEventHash, EdgeBatchHash and
Bridge.Prepare implementations. Copy fiscal-backend to a temporary directory,
copy generate.go to cmd/go-json-golden/main.go there, and run
`go run ./cmd/go-json-golden > golden-full.tsv`. For a compact checked-in corpus
retain the first 2048 lines plus every line whose type is not F. Random IEEE64
seed22 is fixed; command timestamps follow actual Prepare and change on regen.
No C++ encoder is used to produce expected JSON or hashes.

F records compare arbitrary double bits directly against encoding/json. M checks
recursive maps; E/B check actual domain hashes with INT64_MAX typed fields; P/C
check a real backend command payload digest and complete command body hash.

## Integration contract

`Value::object(fields, true)` represents a Go struct with fields in declaration
order. Default objects represent Go map[string]any and recursively sort UTF-8
keys. Payloads decoded via from_cjson are generic maps; event/batch envelopes
must be constructed as ordered objects. Keep journal_seq/first_seq/last_seq and
other typed int64 fields as Value::integer_value(storage_value), never cJSON
numbers (double precision is insufficient above 2^53). A null slice/map remains
Value::null, and allocated empty slices/maps remain array/object respectively.

Use Value strings for byte-exact strings including embedded NUL. cJSON strings
lose embedded NUL and cannot recover original spelling; from_cjson is only an
adapter for already validated NUL-free cJSON values. Raw ingress requiring the
full Go string domain needs a lossless parser; do not silently certify escaped
NUL interoperability from this adapter. Duplicate keys are rejected rather than
ambiguous signed-message interpretation. Nonfinite floats/depth>128 fail without
changing output. Valid UTF-8, HTML, U+2028/U+2029, controls, invalid UTF-8 and -0
follow Go encoding/json. Map payload numeric values model Go float64 decoding.

Only go_json.cpp and its three fmt headers need firmware source inclusion.
CMake and sync/intent callsites are deliberately outside this isolated packet.
Frozen existing batches signed with a different canonical representation need
an explicit repair/migration strategy before rollout; changing serialization
cannot repair their already persisted signatures or event hash chain.
