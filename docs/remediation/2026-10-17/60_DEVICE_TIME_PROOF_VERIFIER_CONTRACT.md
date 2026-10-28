# Device time request proof + verifier contract V1 (accepted implementation contract)

Read-only proposal against HEAD851168ad06df3681487ad0d69f16ac3d07ecd142. No implementation, issuer, production provider/enable, CI or original files changed. Sources and hashes: SOURCE_INVENTORY.json. Stage B interval API and Phase2 core remain as integrated. Server AuthorityClock health/real entropy/HIL remain OPEN.

## Bounded decision

Implement only: (1) purpose-constrained request signer using actual ESP32 DeviceIdentity; (2) immutable expected-exchange record; (3) actual root-response verifier that alone mints VerifiedTimeAnchorPermit and immediately passes it to TimeAnchorCore. Tests use real mbedTLS and an independent synthetic Go root fixture. No actual root issuer/service route or live time provider in this package. Device private key stays inside ESP32-S3; ATECC608 is unused.

Existing BLE proof is the template: private VerifiedBLECertificateProofPermit, DeviceIdentity friend signer, single SHA256 of purpose-domain+canonical bytes, low-S rawP1363 device signature, compiled BindingTrustAnchor root and strict purpose/context validation. Reuse those boundaries, not public sign_hash_hex, certificate challenge payload, mutable caller JSON or a wallclock dependency.

## Exact envelope and canonical bytes

Both envelopes have exactly4keys: `algorithm`, `canonical_json`, `key_id`, `signature`. Reject duplicate/unknown keys, wrong types and total envelope length>4096bytes before allocations. Outer JSON need not be canonical: opaque relay reserialization is allowed only if canonical_json string bytes remain unchanged. The **inner** JSON is the sole signed canonical payload. No client-provided CA/PEM/SPKI is accepted as a trust anchor.

Inner JSON is a flat Go-map canonical object: recursively sorted keys, Go encoding/json string escaping, no whitespace/trailing data/duplicate/unknown fields; not RFC8785/JCS or float64 JSON. Strict UTF8/no NUL, canonical re-encode equality before signature verification. C++ must follow BLE proof's typed integer helper: parse lexical token losslessly, extract int64, **replace parsed numeric Value with integer_value BEFORE re-encoding**. Generic go_json Number encode would otherwise use binary64. Never use cJSON valuedouble or an untyped float64 Go map. Go uses typed int64 fields and edgeCanonical/UseNumber. Relay JS must not parse inner canonical_json.

Integer lexical form: exact signed-int64 decimal token; no +, exponent, fraction, leading zeros or −0; booleans/strings/null do not coerce. Positive fields must be1..INT64_MAX; uncertainty_ms may be0. No global2^53 limit. Explicit representation/math/policy checks remain in core; no silent truncation or guessed year2100 cap. Existing installed legacy binding generation limit is a separate unchanged contract. Provisional design57 sequence2^31 cap and residual generation2^53 cap are superseded: both use positive checked int64, matching Phase2.

Text: existing repository ID validation, nonempty≤128 UTF8bytes/no NUL for opaque tenant/location/register/key IDs; canonical device UUID36; hashes lowercase hex64. Nonces canonical unpadded base64url43chars decoding to32bytes; reject allzero, boot=request and wrong sizes. Signatures canonical unpadded base64url; decoder must re-encode/compare.

### Request: exactly17 inner fields

| Fields | Exact semantics |
|---|---|
| kind,version,purpose,scope,policy_version | `DEVICE_TIME_ANCHOR_REQUEST`,1,`trusted-device-time`,`device.time.anchor`,1 |
| device_id,device_kid,device_spki_sha256 | Installed device ID, actual DeviceIdentity kid and SHA256(SPKI DER), matching installed transaction_signing_kid |
| tenant_id,location_id,register_id,binding_sha256,binding_generation | Exact current installed signed scope, no labels/fallback IDs |
| boot_nonce,request_nonce,request_sequence | Copied only from core-owned current TimeAnchorRequest |
| signer_key_id | **Requested pinned ROOT issuer key ID**, not the device signer ID |

Outer request `algorithm=ES256_P1363`, `key_id=device_kid`; raw signature is exactly64bytes r||s,32-byte big-endian each,1≤r,s<N and low-S. DeviceIdentity hashes once and signs SHA256(`BEEFISCAL_DEVICE_TIME_ANCHOR_REQUEST_V1` + LF0x0a + canonical_request). No trailing NUL. Request hash is lowercase SHA256 of the **same domain-separated transcript**, not raw canonical alone, signature bytes, outer envelope or a caller-declared hash. Signing once must not call legacy double-hash sign_hash_hex. Root verifies PoP under enrolled SPKI/current device key; never trusts the key from request content.

### Response: exactly22 inner fields

Same17field names, with `kind=DEVICE_TIME_ANCHOR`; all fixed purpose/scope/version/policy and device/binding/nonce/sequence/root-key values must equal the immutable expected request. Add exactly:

- request_sha256: locally recomputed domain-separated request hash.
- server_unix_ms: positive int64 authoritative sample; core enforces≥1700000000000 and checked µs conversion/arithmetic.
- uncertainty_ms: nonnegative int64, root-asserted bounded error; within signed/device policy.
- max_rtt_ms: positive int64 **upper bound on real elapsed**, checked as ceil(raw_monotonic_rtt/(1−drift)), includes request signing/relay/body/response verification.
- max_holdover_ms: positive int64 upper age from sample, including RTT; may not increase core ceilings.

Outer response `algorithm=ES256_DER`, `key_id=inner.signer_key_id=expected requested compiled root key ID`. Root signature is SHA256(`BEEFISCAL_DEVICE_TIME_ANCHOR_V1` + LF0x0a + canonical_response), signed P256 DER ASN.1. Require strict minimally encoded DER with full input consumed,1≤r,s<N,8..72decoded bytes and no trailing garbage. Do not require low-S for root DER: existing Go ecdsa.SignASN1 can produce either S; request raw signature remains low-S. Cache/identity/request matching is canonical transcript based, independent of DER malleability.

`DEVICE_TIME_ANCHOR` V1 is a root assertion of **healthy bounded time** under the future reviewed AuthorityClock policy. No caller boolean clock_healthy field is accepted. Verifier sets internal claims.clock_healthy=true only after this exact authenticated profile succeeds. Signature proves an assertion, not measurement: real root must not issue this profile until health/error-source contract is accepted. Signed unhealthy-denial profile is out of scope; no HTTP error/unsigned status is converted into authority. Unknown/malformed profiles reject without erasing a healthy existing anchor. Existing core unhealthy-permit tests remain valid internal fault coverage, not an implemented root health monitor.

## Immutable expected exchange and nonce ownership API

Proposed internal surfaces (signatures illustrative; no code yet):

```
class ExpectedTimeAnchorExchange; // private request snapshot, canonical bytes/hash,
                                // root trust epoch, device/binding identity.
class PreparedDeviceTimeRequestPermit; // private constructor; signer+identity friends.
DeviceTimeAnchorRequestSigner::begin(RequestEnvelope&, ExpectedTimeAnchorExchange&);
DeviceTimeAnchorVerifier::verify_and_accept(const ExpectedTimeAnchorExchange&,
                                          const ResponseEnvelope&);
DeviceIdentity::sign_time_anchor_request(const PreparedDeviceTimeRequestPermit&,
                                        uint8_t signature[64]); // private.
```

Expected exchange is created only by request signer from `core.begin_request`, never supplied as mutable raw TimeAnchorRequest/canonical JSON/hash by BLE callers. It carries a copy of private owner/ticket provenance and exact published request canonical bytes/hash. Copying a handle for an in-flight task is allowed; editing public fields is not. The verifier never returns a raw permit or generic successful time struct to arbitrary callers; construct the existing private VerifiedTimeAnchorPermit and call core.accept while holding current installed authority. No production factory/root-key override is exposed.

Request signer admission: installed-authority guard → exchange mutex → core.begin_request (core locks/releases internally). Verify identity.ready, actual SPKI/kid and installed signed binding; build canonical17fields from the returned core request and pinned root key, construct private signing permit, call DeviceIdentity private signer. Before publishing envelope/expected handle, recheck pending ticket and authority/trust epoch. No caller-specified nonce/scope/time/key/hash signing API. Request signature cost is inside measured RTT because m0 precedes begin_request publication/signature. If signing fails, retire only its own current ticket; never a replacement ticket or healthy anchor.

Response path: validate bounded packet framing, capture immutable expected handle/trust snapshot, do strict parse/canonical/domain/root-signature/request-hash checks outside core mutex. Then installed-authority guard → exchange mutex → recheck current installed device/SPKI/key/scope/trust epoch and exact active ticket → construct permit → core.accept. Core captures m1 after real verification, includes verification/body delay in drift-adjusted RTT/age, and rejects stale ownership. No reverse core→installed/exchange lock order, no network while locked. Identity signing takes its mutex only after core lock was released. Callback admission/late completion must not reacquire a replacement exchange.

Narrow core API additions likely required for a race-free signer: friend-only `is_current_request(request)` and `retire_request_if_current(request)` (compare owner+ticket+scope+boot+nonce+seq under core mutex). No raw setter or generic invalidate fallback. Use these only for signer failure/explicit timeout and final publication fencing; existing begin_request/accept remain authoritative. Final check must be linearized with actual request publication under exchange mutex shared by all begins/timeouts/response completions. Calling `is_current` and then publishing without that shared ownership guard would race.

Unauthenticated malformed/foreign packets do **not** consume a pending ticket: nonce is public and permitting unsigned cancellation creates trivial DoS. This explicitly tightens design57's provisional matching-auth-failure retirement sentence. Current pending can still accept a valid response before its unchanged deadline. Explicit timeout retires only matching ticket. Authenticated structurally valid matching sample reaches core; core consumes matching ticket on accept/error, preserves old deadline for RTT/width expiry failures, and fails closed on authenticated unhealthy/arithmetic/disjoint faults as integrated. Replacement, boot, binding and trust epoch changes retire old expected handles. Replays cannot update receive instant/deadline or cancel a new ticket, including same-scope ABA and evicted recent nonce.

Trust epoch change must invalidate core anchor/pending and cached expected request together. Existing core.change_binding handles scope changes; trust-only change needs the same trusted invalidation path even when binding tuple is byte-identical. No NVS time/cache/epoch rollback is allowed to resurrect authority. Relay/protocol framing remains a future separate bounded control path; this package adds no BLE characteristic, route or automatic bootstrap.

## Actual signature interoperability acceptance (future bounded implementation)

Compile real DeviceIdentity/request signer/verifier/core with actual mbedTLS and installed encrypted/legacy binding fixtures. Use explicit crypto-test trust injection guard `BEEFISCAL_TIME_ANCHOR_CRYPTO_HOST_TEST` only; do **not** define core `BEEFISCAL_TIME_ANCHOR_HOST_TEST` in the real-crypto profile. That prevents accepting through the synthetic core permit factory.

1. C++ builds17field request using actual DeviceIdentity NVS scalar and signs low-S rawP1363. Independent Go verifies exact canonical bytes, one SHA256/domain, SPKI/kid/PoP and low-S. Test double-hash/wrong-domain/signature-format negatives.
2. Independent Go synthetic fixture creates22field response and signs with ecdsa.SignASN1; actual C++ verifier checks strict DER/pinned root and privately calls core.accept. Both high-S and low-S root DER positive vectors; malformed DER/trailing/zero/out-of-range r/s reject. No production issuer method changed to produce these fixtures.
3. Canonical golden vectors at2^53−1,2^53,2^53+1,INT64_MAX for generation/sequence and representable epoch milliseconds, with explicit core arithmetic rejection where µs conversion cannot fit;0 uncertainty allowed. Fraction/exponent/−0/duplicate/unknown/NUL/UTF8/trailing/mismatched re-encoding reject even when signed by the actual synthetic root.
4. Every field mutation signed under the correct root but wrong expected request: nonce/boot/seq/device/SPKI/kid/root key/tenant/location/register/binding/generation/request hash/purpose/version/policy; reject without touching newer pending/healthy anchor. Cross-protocol BLE certificate signature/time request/time response cannot substitute.
5. Pause actual request signature/root verification at deterministic gates: pendingA→B, binding/trust/identity/boot ABA, timeout, signing error, concurrent begin/late response. Only current handle publishes/adopts; failure cannot retire replacement. End-to-end body/signature verification delay past real RTT/holdover budget rejects.
6. Production compile lacks host root overrides, synthetic permit factory and raw signing/time setters. Actual Xtensa compile and inherited Phase1/Phase2 mandatory gates remain required; this contract itself is no runtime/target/HIL acceptance.

## Root review points

Approve exact17/22field schema and asymmetric signature representations/domains; int64 supersedes old provisional caps; immutable expected-exchange coordinator plus friend-only pending check/retire APIs; unsigned failures preserve pending; compiled BindingTrustAnchor initially authorizes only the new strict time purpose. Server health/real issuer, production entropy/provider, consumer migration and bootstrap transport remain explicitly OPEN after verifier implementation.

## Accepted integration refinements

The exact17/22-field schema, purpose domains, signature formats and private permit boundary are accepted for the isolated implementation. This document alone proves no runtime acceptance. Compiled root trust/fingerprint may not be replaced by a caller; only explicit host-test overrides are allowed. Request scope must equal the actual guarded installed binding and DeviceIdentity SPKI/kid; the signer cannot automatically initialize a boot/provider. Trusted boot/binding/trust invalidation must use coordinator→core lock order; no core mutex across identity signing. Returning an immutable prepared exchange is not network publication: replacement after return can make that packet stale; later relay admission must preserve ownership and stale responses must never adopt authority. Add that post-return replacement case to acceptance. Actual server health, root issuer, bootstrap route, entropy source, consumer migration and production enable remain OPEN.
