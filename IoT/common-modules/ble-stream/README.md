# Reliable GATT stream — Stage B

Experimental shared portable C and dependency-free TypeScript state machines for MP026. They transport opaque bytes; OPEN is not TLS or client authority. No existing runtime imports this module. Device keys remain ESP32-S3/NVS, no ATECC608A/608.

`python3 IoT/common-modules/ble-stream/reproduce.py --evidence /private/tmp/mp026-stageb-evidence` builds C with AddressSanitizer/UndefinedBehaviorSanitizer, requires the independent SIZE_MAX C API boundary test to reject before accessing payload and runs actual process-to-process C↔TS fault tests in both roles, and requires six isolated mutations to fail assertions. No installations/device access/flash. Installed app TypeScript compilers are checked when available and explicitly OPEN otherwise. Node22.14+ with strip-types is required.

The shared package location/exports prepares future file-dependency consumption by both app bundlers. Metro/PWA package-resolution and actual callback integration are Stage C; no fragile external relative runtime import is introduced here.

Core calls are serialized by the future adapter. `bs_next`/`take` reserves one local dispatch lease; call the platform API immediately and settle only the captured incarnation/token. Do not queue multiple taken frames. ACK can arrive before local settle. Any reported local I/O failure closes and clears queued bytes; late token/generation completions are inert. All active API time samples use monotonic millisecond time, reject backwards/invalid or deadline-addition-overflow samples; the adapter must schedule it on every wake/resume and before dispatch. Byte/offset exhaustion fails closed. `bs_read` is opaque received ciphertext, never an API for authorized plaintext.

`bs_init`/constructor receives the adapter-owned fresh incarnation, validates nonzero server incarnation and frame/window bounds. Approved RNG, detected incarnation reuse, local connection generation overflow, listener/subscription ownership and physical handle validation belong to Stage C. Reinitializing live memory without invalidating old owner is forbidden. A finite random incarnation is not a trust anchor and never licenses TLS resumption/replay.

CLOSE/CLOSE_ACK are exactly18bytes.20bytes is the minimum negotiated maximum ATT-value size, not a minimum size for every frame; DATA may be17bytes. Control frames INFO/OPEN/OPEN_ACK/ACK are20bytes. Service UUID reservation is in doc52. Transport constants/descriptors/rings are bounded and independent of fiscal operation idempotency.

No secure mode, financial sink, private key provider, BLE SDK integration or TLS implementation is enabled. Full doc52 contract and outstanding gates govern adoption. Test-only driver `O`/`LIMIT` fast-forward public test state to prove32-bit arithmetic boundaries without transmitting4GiB; those commands do not exist in the library API.
