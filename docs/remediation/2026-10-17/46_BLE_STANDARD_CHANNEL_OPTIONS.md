# Standard authenticated BLE channel: options and recommendation

Date: 2026-10-21. DESIGN COMPARISON ONLY. No channel protocol implementation is accepted by this document. The earlier four-message signed-ECDHE sketch is an unapproved security-review candidate; do not implement it as a shortcut.

## Recommendation

Evaluate **TLS 1.3 over a bounded reliable GATT byte stream**, using mbedTLS server on S3 and the same small portable C TLS-client engine built natively for Android/iOS and as WebAssembly for both browser clients. Keep client OS/Browser signing-key PoP as a separately authenticated application step inside TLS. This reuses an established authenticated key exchange and record protocol while avoiding a bespoke handshake and asynchronous OS-key integration into TLS client-certificate signing.

This is a recommendation for a bounded feasibility spike, not a claim that the native/WASM bindings, reliable stream or device certificate lifecycle already exist. Do not enable secure mode until all surfaces pass the same vertical. If feasibility fails, evaluate a maintained Noise implementation and its required identity-key migration explicitly; do not automatically fall back to the custom sketch or OPEN_MVP.

## Verified starting inventory

- Current S3 framework: PlatformIO espressif32 6.10.0, ESP-IDF 5.4.0, installed mbedTLS **3.6.2** (local `include/mbedtls/build_info.h`). TLS server and client are enabled. TLS 1.3 is **disabled** in the checked sdkconfig. Existing input/output SSL buffer settings are 16 KiB/4 KiB. Enabling TLS 1.3 and its dependencies and measuring simultaneous BLE/MQTT/SQLite heap use are mandatory spike gates.
- C++/C compiler and Xcode are present. Android NDK installations include 23.1, 25.1, 26.1, 27.1 and 30.0; select the app's configured compatible version, not whichever is newest. Existing Expo execution-crypto native module has Kotlin/Swift bridges; its pod currently includes h/m/mm/swift, not a bundled portable TLS C library. New native build integration is needed.
- `emcc`, `cargo` and `rustc` were not found on this task shell PATH. This does not prove absence from the machine. Neither a TLS-WASM engine nor a Noise engine is declared in the inspected MiniPOS package manifests. Current noble primitives are not an authenticated-channel implementation.
- Both active browser transports use Web Bluetooth GATT, not a browser TCP socket. Web Crypto does not provide a TLS session engine. Browser HTTPS/WebSocket TLS is not a way to encrypt arbitrary GATT bytes. RN Hermes must not be assumed to execute the browser WASM build: use native C bindings, not a hidden WebView. Upstream Hermes discussion treats WASM as a separate integration concern. [Hermes upstream discussion](https://github.com/facebook/hermes/issues/429)

The pinned mbedTLS version is an implementation baseline, not a current-security approval. Its upstream 3.6 LTS branch has later security-fix releases. Check applicable advisories and an ESP-IDF-supported update/backport path before release; do not silently replace the ESP-patched submodule with an arbitrary upstream archive. [Official mbedTLS releases](https://github.com/Mbed-TLS/mbedtls/releases)

## Option comparison

| Option | Fit to current identity and clients | New integration / unresolved gates | Assessment |
|---|---|---|---|
| TLS 1.3, mbedTLS C core, native + browser WASM | Uses the enrolled P-256 S3 identity and an issuer-certified server key. Android/iOS compile C; browser uses Emscripten. OS signing key remains outside TLS and signs a fresh protected application challenge. | Reliable stream, TLS BIO/event pump, dedicated server certificate, TLS1.3 config/security updates, native C bridge, WASM build/loader/CSP and peak heap tests. | First choice for spike; greatest reuse of existing S3 crypto/trust infrastructure. |
| Standard Noise NK/25519 with a maintained common engine | Fits server-authenticated transport followed by client PoP. Requires a new dedicated static X25519 responder key, whose full public pin must be certified by existing P-256 enrollment/root policy. Existing P-256 identity is not automatically a Noise 25519 key. | New durable key/rotation/enrollment fields; audited library/build selection; S3/native/browser toolchains; independent vectors; authorization still required after handshake. | Viable alternative if a specific maintained engine is proven across all targets. Not already available. |
| Noise-C reference implementation | Portable C and documented standard 25519/448 support could suit native/WASM/S3. | Upstream calls it a reference implementation; current maintenance/security support and audited suitability have not been established here. Do not label it vetted merely from its name or license. [Noise-C documentation](https://rweather.github.io/noise-c/) | Not selected without maintenance/security review and exact revision approval. |
| Snow | Upstream Rust implementation supports no_std with alloc and selectable features. It also advertises P-256 as a nonstandard Noise extension. [Snow upstream](https://github.com/mcginty/snow) | Rust/ESP Xtensa, iOS/Android FFI and WASM build pipelines are new; an exact revision and security review are required. A P-256 extension is not standard Noise interoperability. | Consider standard 25519 suite first; extra toolchain and key-migration cost versus current mbedTLS. |
| ChainSafe libp2p Noise JS | Maintained project ecosystem and browser/JS-oriented Noise handshake implementation. [Upstream](https://github.com/ChainSafe/js-libp2p-noise) | Not a drop-in S3 C engine; libp2p identity/framing and cross-engine compatibility need review. No repository evidence establishes a vetted same-engine S3/native/browser deployment. | Useful implementation reference/interop candidate, not chosen as the embedded common core. |
| BLE bonding / link encryption alone | Can protect a radio link under its pairing model. | Does not bind the exact root-issued tenant/register/device pin and current app authorization; browser exposure/control and pairing policy differ across platforms. | Supplement only; does not close the application trust gap. |

Noise patterns and DH suites are specified separately; the standard framework describes 25519/448. Do not rename a P-256 custom composition as a standard Noise suite. [Noise specification](https://noiseprotocol.org/noise.html)

Exact new dependency revisions must be pinned after the spike and advisory review. No current installed Noise version or fully maintained cross-platform Noise deployment was verified; that uncertainty is a selection gate, not permission to write a replacement crypto protocol.

## TLS design boundary

### Shared engine and reliable stream

A small C wrapper owns `mbedtls_ssl_context`, bounded receive/transmit queues and lifecycle. Its public surface can expose create/feed/drain/handshake/read/write/close, with explicit WANT_READ/WANT_WRITE processing. Native RN bridges and browser WASM adapt bytes/events only. Cores stay per connection generation; avoid global TLS state shared between devices. mbedTLS already exposes custom BIO callbacks in the installed ssl.h, so no virtual TCP/IP stack or HTTP parser is required.

Build this C wrapper for ESP-IDF server, Android NDK, iOS static library and Emscripten WASM client. It is a common source engine, not an identical binary across architectures. Browser randomness must enter through a reviewed synchronous Web Crypto entropy callback; native RNG must use OS CSPRNGs. S3 uses its verified startup/runtime entropy policy, never a temporary SAR toggle while RF is running.

GATT notification success is not an end-to-end reliable byte-stream guarantee. Add bounded ordered fragments with connection generation, explicit sequence/offset, backpressure, cumulative receipt ACKs and duplicate suppression before feeding bytes into TLS. Retransmit only identical transport bytes; a business command must never be reconstructed as a new operation because of a lost ACK. Reconnect resets the stream and creates a fresh TLS session; do not resume old TLS records. Inject missing/duplicate/out-of-order/late fragments and host-queue overflow in tests.

Allocate a separate versioned secure characteristic/service contract or an explicitly versioned dispatch boundary. Current characteristic 0001 is provisioning control, 0002 command, 0003 events, 0004 flow, 0005/0006 provisioning information/status. Do not repurpose setup control invisibly. A signed binding policy must require secure business ingress and reject raw BFF1 commands/results; an attacker cannot force fallback by hiding the new characteristic. Setup provisioning remains its independently encrypted authenticated protocol.

Use TLS1.3-only policy, approved ECDHE/ECDSA/AEAD suites and no early data. Disable tickets/resumption initially. Let the library implement transcript signatures, Finished, record authentication, counters, key updates and nonce construction. Do not reimplement these in TS. Preserve operation-level idempotency/recovery above TLS.

### Certificate and pin bridge

The real S3 device CA issuer currently emits a leaf with **ClientAuth EKU only**, for MQTT-style credentials. It cannot simply be treated as a valid TLS server certificate. Add a purpose-constrained BLE-server certificate (or deliberately reviewed dual-purpose issuance) for the same enrolled NVS P-256 key, correct ServerAuth EKU and deterministic server-name/SAN identity. Prefer a separate BLE purpose/intermediate trust scope so a BLE device certificate cannot become a generic application backend certificate.

Root resolves the exact S3 adapter from the immutable issued composite binding and manufacturing record; reuse provisioning_delivery.go's strict enrolled-key/SPKI checks. Root-signed channel metadata binds full SPKI fingerprint, device ID, tenant/register/location, binding hash/generation, client signing-key fingerprint, scope, expiry and revocation generation. Advertising identity is not the pin. Neither legacy register+"-edge" identity nor a JWK announced by BLE is acceptable.

Client validates the server chain/purpose and exact expected pin/context from independently trusted root-signed metadata. Do not use a self-announced issuer key, TOFU or a bare decoded execution envelope as that anchor. Ship a constrained issuer/root key ring through trusted app/deployment updates and verify offline cached metadata before use. Browser WASM runs its own constrained trust policy; successful outer page HTTPS is not a certificate verification callback for the BLE peer.

The S3 private key stays on S3. Existing local TLS key material helpers are internal only; no BLE/API exposure of PEM/scalars. Keep TLS session secrets out of logs. Native and WASM wrappers must close/free/zeroize owned contexts, with honest limits: WASM memory is not a hardware boundary against compromised same-origin JS.

### Authenticate client authority inside TLS

Avoid forcing native nonexportable signing keys through a synchronous TLS client-certificate callback in the first implementation. After server-authenticated TLS completes, S3 sends a fresh random one-use application challenge bound to that connection/generation and exact installed binding. Client sends its short-lived issuer-signed channel credential and OS/browser-key signature over the canonical challenge, credential hash, peer pin and context. S3 verifies issuer, exact client key, context, expiry and PoP before opening business ingress. Replay into another connection fails on the fresh challenge.

Use a small reviewed application challenge schema, reusing existing verified signing providers. This is application authorization inside a standard channel, not another key-exchange protocol. The existing per-intent execution/payment authority and client PoP remain mandatory for every financial command.

The locally installed mbedTLS3.6.2 ssl.h exposes an export-keys callback but no simple public `mbedtls_ssl_export_keying_material` function was found. Do not derive an invented TLS1.3 exporter from private library structs or leak traffic secrets into JS just to bind app auth. A fresh protected application challenge avoids that dependency. If a future version supplies a supported standard exporter, review its use separately.

### Rotation, time and offline limits

Device key replacement requires enrollment PoP, root-approved CAS rebind at a higher generation, old-key revocation and live-channel invalidation. Never regenerate corrupt NVS state silently. CA/issuer rollover needs overlapping trusted keys, activation/retirement policy and anti-rollback metadata distributed through already trusted updates. Certificate-chain validity alone does not prove the current tenant/register binding or offline revocation state.

Offline use requires a finite verified credential window and trustworthy time. S3 cold boot without a secure clock cannot establish elapsed power-off time from its previous timestamp. Require a fresh nonce-bound signed time anchor or an independently justified clock policy; fail closed otherwise. Warm sessions can use a verified anchor plus monotonic elapsed time. No channel option makes a disconnected device immediately aware of a later server revocation.

## Fastest gated implementation sequence

1. **Feasibility only:** pin an ESP-IDF-supported patched mbedTLS baseline; enable TLS1.3 in an isolated S3 build. Build the same portable client core for native host and Emscripten, then Android/iOS bridges. Complete a virtual-memory-BIO handshake with a disposable correctly issued S3 server certificate. Measure peak S3 heap while MQTT/SQLite/BLE tasks remain active. No financial writes or OPEN_MVP fallback.
2. **Trusted identity:** implement root-issued purpose-constrained certificate/channel metadata, client verifier/pin cache, exact binding matching and rotation/time gates. Prove rejection of ClientAuth-only leaf, wrong pin/name/EKU/issuer, expired/old-generation credential and self-announced key.
3. **Transport and authorization:** integrate reliable GATT byte stream and protected app challenge into all three active TS/native transports and actual S3 callbacks. Require both TLS completion and authority success before command admission. Retain native deadline, disconnect generation and UNKNOWN semantics.
4. **Independent interop:** Go crypto/tls peer over net.Pipe/fake ordered transport ↔ actual S3 mbedTLS server ↔ common native/WASM client. Corrupt TLS records and stream fragments; replay old AUTH/challenge; test close/expiry/restart and no plaintext business ingress/results. No cryptographic implementation is duplicated in TS.
5. **Full financial vertical and HIL:** real Go issuer, real Android/iOS signing keys and browser WebCrypto, production S3 IntentProcessor/SQLite, emulator command verification and recovery; then physical S3/Datecs/Daisy/BluePad/BLE/MQTT trials. Host TLS success cannot certify radio scheduling or physical fiscal behavior.

Stop the spike and report the concrete blocker if TLS1.3/WASM/native builds or S3 resource constraints fail. The next reviewed option is a specific standard Noise engine plus its identity migration; the unapproved custom ECDHE draft remains unapproved.

## Bounded feasibility evidence, 2026-10-21

This section supersedes the initial toolchain inventory. Source and reproduction fixtures are staged under `/private/tmp/fiscal-tls-spike-20261021/portable-proof/`, without generated private keys. They are experimental test fixtures, not production channel code. Original repository sources were read-only. Production physical keys remain inside ESP32-S3 NVS; ATECC608A/608 is not used.

### Patched SDK selection

Actual official git tags, not search snippets, were checked. Latest observed5.4 maintenance release is5.4.4 (`296b6eab9445fd720e71aecab961e2d3fbca9944`), vendor mbedTLS `ffb280bb63c78bfec1e1ab55040671768c85c923` reports3.6.5. ESP-IDF5.5.5 (`b774170ff46c393eeb5e495ea37936038d3f4f4f`) vendor `9d669eadb1955d348986b9280156710aaaadf79f` reports3.6.6. New official5.5.6 (`880a203ae603d08a94d06aa01e0d6761958e5e9e`, released16October2026) pins vendor mbedTLS `2b96dd8eebe880f304c69976b3c2fa0c5100cbb6`,3.6.7. Candidate evaluation therefore uses the complete official5.5.6 framework and its submodules, not an upstream crypto-library swap. [Official release](https://github.com/espressif/esp-idf/releases/tag/v5.5.6), [ESP vendor fork documentation](https://docs.espressif.com/projects/esp-idf/en/v5.5.6/esp32s3/api-reference/protocols/mbedtls.html).

Security scope is configuration-specific:

| Advisory | Verified relevance and constraint |
| --- | --- |
| CVE2026-25832, TLS1.3 HRR group selection | Relevant to proposed portable TLS client.5.4.4/5.5.5 vendor source retains old mismatched-group condition;5.5.6 contains equality-first corrected logic. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2026-07-tls13-hrr-unadvertised-group/). |
| CVE2026-49300, X509 basicConstraints | Old vendor parser permits INTEGER→CA interpretation and lacks strict sequence boundary handling;5.5.6 has both fixes. Exposure depends on accepting untrusted trust-root certificates; use CSR→controlled CA issuance and compiled trusted roots, never user-provided arbitrary root certs. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2026-07-x509-ca-bit-forgery-invalid-basicconstraints/). |
| CVE2026-73064, TLS1.3 RNG error | Requires session-ticket issuance and application read/write before handshake completes. Fixture explicitly completes handshake and disables tickets;3.6.7 fixes library behavior. This workaround is not a justification to ship unreviewed old crypto. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2026-07-random-generator-fault-tls-integrity/). |
| CVE2026-50583, malformed zero-length ECC key |3.x exposure specifically driver-only PSA ECC with built-in ECC disabled; inventory this macro configuration before claiming current S3 exposure. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2026-07-zero-length-ecc-public-key-oob-read/). |
| CVE2026-54435, optimized ECC reduction | Local privileged tracing threat; NIST optimization/configuration and actual operation matter. Existing P256 public-key derivation is a reason to audit this beyond only BLE TLS. No claim that existing keys were recovered or predictable.3.6.7 fixes this family. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2026-07-ecc-optimized-modp-side-channel/). |
| CVE2025-27809, client server-auth configuration | Applies to application policy in all versions: trusted CA, required verification and expected hostname must all be configured. Pin exact enrolled SPKI/binding in addition. Version upgrade alone cannot establish correct device identity. [Primary advisory](https://mbed-tls.readthedocs.io/en/latest/security-advisories/mbedtls-security-advisory-2025-03-1/). |

The official daily security dashboard was inspected at commit `08f58c48259086dad8e5788961e943ffa5a2e237`;5.4.4 has mbedTLS scan findings;5.5.5/5.5.6 have no mbedTLS entries in that snapshot. Absence from a scanner is not proof of absence: the above source/advisory checks remain required.5.5.6 also has non-TLS component findings, requiring actual enabled-service applicability review (for example NimBLE BASS is not automatically equivalent to the custom GATT service). Do not declare the complete SDK security accepted based only on TLS. [Dashboard source](https://github.com/espressif/esp-idf-security-dashboard).

### Proven versus open

| Surface | Actual result | Limits |
| --- | --- | --- |
| Host common C | TLS1.3 fragmented memory-BIO client/server PASS; expected-name and expiry rejection PASS; ClientAuth-only server leaf selection rejects | Standard library cryptography only; no custom handshake/exporter; certificate pin/binding wrapper remains to implement |
| Independent Go client | Go crypto/tls→C mbedTLS memory-BIO server PASS, negotiated TLS1.3/AES128GCM | Host execution of server engine, not S3 hardware execution |
| Android arm64 | Full mbedTLS3.6.7 static libraries + common C engine compiled/linked into .so with NDK27.1.12297006/API24 | No JNI/JSI/Hermes app runtime, keystore PoP or real BLE transport test |
| iOS arm64 device/simulator | Full mbedTLS libraries + engine compile/link PASS, iOS15.1 minimum, Xcode26.5 SDK | No signed app, native bridge or simulator/device runtime |
| Browser WASM | emsdk4.0.20; Node and actual Chromium memory-BIO C↔C TLS1.3 runtime PASS | No WebBluetooth, JS/WASM exported state API or external-peer browser interop yet |
| S3 | Official5.5.6 candidate build evidence recorded separately below | No physical board, runtime heap/stack or simultaneous BLE/MQTT/SQLite measurements |

Host endpoint struct including two8KiB queues is19304bytes; wasm struct18120bytes. These are structure sizes, not total TLS heap. Browser wasm binary around732KiB is an unstripped feasibility artifact with broad default library configuration and assertions; mobile raw .so/.a sizes include debug/static archive contents and must not be sold as optimized app-size impact. No actual S3 runtime heap number is available.

### Required next gates and decision

**GO for standard TLS as the preferred implementation direction; NO-GO for enabling a production secure mode or upgrading the production SDK today.** Complete supported build integration and regression acceptance first. Then instrument target heap/stack and prove bounded reliable GATT stream under loss/reorder/duplicate/disconnect, full enrolled-SPKI/name/chain/EKU pinning, root issuer rotation, fresh protected application nonce with existing client-key PoP, and ownership/zeroization across all native/browser surfaces. Keep raw OPEN_MVP disallowed wherever signed policy requires secure mode. TLS server authentication is not client authorization, and command JWT/client PoP is not proof of an authenticated device channel.

No production private key is in this fixture bundle. Generated synthetic keys are local disposable test assets, recreated by gen.go and excluded from the deliverable. Existing NVS identities are preserved; no silent rekey.

### Candidate S3 build final disposition

The bounded full-app5.5.6 build and S3 TLS1.3 server fixture both reached actual target C/C++ compilation. The fixture's main.c object, vendor TLS/crypto libraries and full application sources compiled. Neither complete firmware image passed final build. Legacy PlatformIO6.10 first fails duplicate `esp_efuse_fields.c` object basenames. Direct generated CMake/Ninja proceeds further but bootloader rules select host `/usr/bin/ar`/`ranlib`, producing unsuitable/empty archives and final missing entry/end symbols with IRAM overflow. Setting the nested cache alone was overwritten on regeneration. No linker security/version checks were disabled and no firmware was flashed. This is an unresolved supported-build migration, not evidence that TLS cannot run on S3. Do not quote the attempted S3 build as PASS or infer binary size/runtime heap from it.

Exact terminal failure logs: `idf-candidate-final-build.log`, `s3-memory-bio-final-build.log` (PlatformIO); `idf-candidate-ninja-complete.log`, `s3-memory-bio-ninja-complete.log` (CMake/Ninja). Full candidate regressions were not claimed after failed image build. Production SDK remains5.4.0; its security acceptance is OPEN independently of the accepted startup-entropy software correction.

An initial local framework override accidentally began installing into the global PlatformIO core. It was interrupted, then exact official `platformio/framework-espidf@3.50400.0` restored. Actual prior isolated app rebuild succeeded12.17s and reproduced the preincident firmware SHA256 `9ae3d988a975ed1bd9664757991829a7acd0f54202c1c3eb3ee3a26e51c11051`. Restoration log is included. All later candidate builds used a separate `/tmp` core. This tooling incident did not edit original repository source or touch a device.

Final review strengthened negative fixtures: hostname rejection requires client `-0x2700` and verifyflag4; expiry requires client `-0x2700` and verifyflag1; ClientAuth-only fixture requires server certificate-selection `-0x6e00`. Any prohibited completed handshake fails with exit15. A separate counterfactual disabling client verification and normalizing hostname makes both negative tests RED (exit15), recorded in `counterfactual-final.log`. Initialization failure now frees all initialized TLS/certificate/key/DRBG/entropy contexts. These are fixture corrections and do not constitute production adoption.

## Root portable integration evidence

[Manifest/logs](evidence/tls-portable-2026-10-21/manifest.json) and [reproducible sources](../../../IoT/firmware/edge-agent-s3/idf/tests/tls_feasibility/reproduce-portable.md): root host C↔C/C↔Go, counterfactual negative guards, Android/iOS compile/link and actual Chromium four-case WASM PASS. Host checks pinned clean dependencies; clean iOS simulator configure and a Go fixture watchdog were added. No generated private assets/binaries were committed.

The earlier S3 compile failures above are a historical checkpoint. A subsequent pristine official full-app build with TLS1.3 enabled succeeded in a physical no-space isolated path; doc48 records that SDK candidate separately. Production SDK/channel integration, target dynamic heap, reliable GATT, client authority and all device/HIL gates remain OPEN. The two-endpoint browser test embeds disposable generated keys only and is never a production client artifact.

## Independent client-side EKU proof

The final host log `evidence/tls-portable-2026-10-21/root-host-eku-final.log.gz` adds an independent Go TLS1.3 server that forcibly presents a ClientAuth-only leaf. The actual common mbedTLS client rejects it with `-0x7a00` and `EXT_KEY_USAGE=0x1000`; a ServerAuth positive control exchanges data. Disabling verification in an isolated counterfactual makes the forbidden handshake succeed and the test fail. This closes the host client-side EKU test gap; it does not implement production certificate issuance/pinning or prove native/browser/device execution of this new host-only fixture.
