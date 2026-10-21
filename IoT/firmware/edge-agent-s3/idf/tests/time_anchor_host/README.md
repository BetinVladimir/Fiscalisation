# Pure time-anchor core — Phase2

This is an isolated core against Phase1 `2720a82`. It is **not** wired into app_main or firmware CMake, `TrustedBLECertificateTime`, BLE ingress or a public API. No cryptographic verifier/root request issuer, production nonce source, AuthorityClock health monitor or device clock calibration is implemented. Execution/local-token consumers remain unchanged. Default Stage B time stays unavailable. Its host oracle and counterfactual suites are mandatory through make time-anchor-test / make iot-test (including S3 CI).

## Ownership contract

`TimeAnchorCore` owns one boot identity, installed-scope snapshot, current request ticket and optional anchor. A trusted lifecycle caller supplies scope through `start_boot` / `change_binding`; production integration must atomically wire these to installed-authority invalidation. Core mutex serializes state transitions; injected clock/nonce sources must not reenter the core. Future integration lock order is installed-authority guard → core mutex. No networking/crypto runs under the core mutex.

`begin_request` retires only the old pending request, captures monotonic m0 before returning nonce bytes and issues a new checked int64 sequence plus private owner/ticket identity. Existing healthy anchor continues only to its original deadline. Request copies cannot forge that private identity. `accept` accepts only a private-constructed `VerifiedTimeAnchorPermit`; a future strict verifier must authenticate purpose/key/scope and exact canonical request hash before minting it. There is no production permit factory or generic settime. Explicit `BEEFISCAL_TIME_ANCHOR_HOST_TEST` enables synthetic permit/sequence injection only in host tests.

Root-signed samples require exact current scope/device key/SPKI, boot nonce, request nonce and sequence, plus the active private ticket. Stale/foreign packets cannot cancel a newer ticket, alter the old anchor, or refresh its deadline. A matching invalid/unhealthy sample invalidates authority. A matching excessive RTT consumes that ticket but preserves any still-healthy old anchor without refresh. Disjoint authenticated reanchors fail closed. Boot/binding changes and explicit invalidate retire pending+anchor; same-binding ABA cannot resurrect an old ticket. Clock failure/backwards jump quarantines authority. No NVS/SD/wallclock restore path exists.

## Nonce history and continuous operation

Injected entropy must provide fresh per-boot/request32-byte nonces; real entropy quality remains a future acceptance gate. Zero outputs and active boot/recent repeated nonces are rejected. Fixed128-slot storage, configurable recent ring≤128, detects stuck/recent RNG output. Only retired request history is evicted; begin_request retires the old pending before drawing another nonce. Boot nonce stays separately pinned. Private non-reused ticket and checked wire sequence fence old replay even after history eviction; the ring is diagnostic, not proof of lifetime nonce uniqueness. No workday request-capacity exhaustion exists. Tests perform10050 successful refreshes and then reuse an evicted old nonce to prove the old permit cannot refresh/cancel the newer ticket. Sequence exhaustion rejects new requests without wrap; an existing healthy anchor retains only its original deadline.

## Exact intervals and integers

Signed fields are checked int64. There is no global2^53−1 cap; opaque JS canonical_json relays must not parse/re-encode numbers. Existing installed legacy-binding generation limits are a separate unchanged contract. Internal µs conversion has an explicit checked int64 representation limit; it does not silently round large epoch milliseconds. Tests include exact generation and server milliseconds above2^53, int64 overflow and boundary failures.

At receipt m1 (sampled after the future verifier completed), let T be server microseconds, E its certified uncertainty, r=m1−m0, and p the configured drift fraction. Receipt interval is `[T−E, T+E+ceil(r/(1−p))]`. At elapsed d since receipt it advances outward by `floor(d/(1+p))` and `ceil(d/(1−p))`. Holdover is checked using `ceil((now−m0)/(1−p))`, so RTT and verification/body delay count. The signed `max_rtt_ms` bounds conservative real elapsed (`ceil(r/(1−p))`), not just raw local ticks; a raw RTT at the limit is rejected when drift can put real elapsed above it. Width and signed/device policy ceilings are checked on every read. Strict holdover expiry never refreshes on reads or replay. Bounds already published cannot decrease; overlapping reanchors retain conservative publication floors. Arithmetic uses checked quotient/remainder integer decomposition, no floating point/int128.

Policy values are engineering defaults/ceilings, not measured S3 accuracy: default RTT5s, holdover30s, uncertainty250ms, width6s, drift1000ppm. Production calibration, sleep/clock lifecycle and issuer health/error-source contract remain OPEN. Security consumers must validate both lower and upper bounds (Phase1 contract), never a midpoint.

## Tests

From repository root:

```
python3 IoT/firmware/edge-agent-s3/idf/tests/time_anchor_host/run_oracle.py
python3 IoT/firmware/edge-agent-s3/idf/tests/time_anchor_host/run_counterfactuals.py
```

The first builds actual core source with strict warnings, tests ownership/replay/nonce/boot/binding/health/disjoint/clock/concurrent requests, runs5229 vectors against independent Python `Fraction` arbitrary-precision arithmetic, and compiles without the host macro to prove raw permit construction/factory are unavailable. The second creates disposable copies and verifies RED for omitting RTT from holdover age, rounding upper bounds downward and refreshing holdover from stale replay. Source files are never mutated by either runner.

Native UBSan acceptance also covered all5229 vectors and lifecycle/ownership tests with no undefined-behavior diagnostics. Raw logs, exact source hashes and toolchain identity are supplied in the separate `/private/tmp/fiscal-time-core-20261021/evidence` review package. This is standalone native host acceptance, not a full ESP-IDF target build or HIL claim.
