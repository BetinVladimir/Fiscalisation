# Device time request/verifier actual-crypto profile

Scoped implementation of the approved17/22field contract. Request uses single SHA256 of `BEEFISCAL_DEVICE_TIME_ANCHOR_REQUEST_V1\n` + canonical bytes and low-S rawP1363 actual DeviceIdentity signature. Response uses `BEEFISCAL_DEVICE_TIME_ANCHOR_V1\n`, strict fully consumed DER P256 signature under compiled BindingTrustAnchor; root DER high/low-S are both accepted. Request hash includes its domain and canonical bytes, never signature encoding.

ExpectedTimeAnchorExchange has private immutable request/canonical/hash/trust ownership. RootTrust fingerprint derives compiled SPKI DER+kid; only explicit crypto-host builds have a root override. Signer never auto-starts/rebinds core. It compares core request against actual installed scope/SPKI/kid, releases core mutex before identity signing and rechecks current ticket before returning prepared bytes. Returning a packet is not network publication: a later replacement makes it stale and verifier cannot adopt it. Relay transport is not implemented.

Lifecycle locks are installed-authority guard → coordinator mutex → core. Explicit trusted boot/binding/invalidate operations and retiring a prepared ticket follow this order. Retiring an old handle cannot cancel its replacement or clear healthy time. Response parsing/crypto happens outside coordinator/core locks; final installed scope/trust/current ticket checks precede private permit construction/core.accept. Invalid unsigned packets preserve pending/healthy anchor; verified matching samples are subject to core error policy. No raw caller time setter, mutable expected context or public permit is exposed.

Strict canonical flat schemas promote each typed int64 before re-encoding (no binary64 roundoff). Request17fields/response22fields, unknown/duplicate/trailing fields, NUL/UTF8/surrogate defects, exponent/fraction/−0/overflow, base64 padding and DER garbage fail closed. Actual signed root parser-only golden vectors include2^53−1,2^53,2^53+1,INT64_MAX independently of installed legacy binding limits; they mint no permit. Core representation/policy math remains independently covered by Phase2.

Run from repository root with reviewed native dependencies:

```
MBEDTLS_SOURCE_DIR=/private/tmp/fiscal-tls-spike-20261021/mbedtls-3.6.7 \
MBEDTLS_BUILD_DIR=/private/tmp/fiscal-tls-spike-20261021/build-host \
CJSON_SOURCE_DIR=/private/tmp/fiscal-ble-crypto-ci-20261021/upstream-cjson \
CXX=g++-15 sh scripts/verify-device-time-anchor-crypto-host.sh
```

Go synthetic fixture verifies actual request scope/PoP under enrolled device SPKI, then signs independent root responses. Actual C++ verifier/core is compiled **without BEEFISCAL_TIME_ANCHOR_HOST_TEST**; no synthetic core permit factory can bypass crypto. Crypto-host-only private identity fault access exhausts the real DRBG budget to exercise actual ECDSA failure; it is absent from production. The source-including native fixture mirrors the existing BLE proof host pattern to inspect exact-int64 parsing; production parser has no test hook.

Acceptance includes signed hostile scope/nonce/hash/purpose/integer fields, root high/low-S and corrupted signatures/DER, returned-packet replacement, simultaneous actual signing, retirement ownership, verification/body real-RTT budget, replay/expiry/boot/binding and actual signing failure. Production compilation rejects root override/raw permit/prepared-proof construction. Existing Phase1 and Phase2 regressions are recorded separately. Double-hash device signing and root-signature bypass counterfactuals are RED.

No production issuer/routes, SNTP changes, time provider, app_main/bootstrap wiring, CI/Make/CMake enablement or health/entropy/HIL claims. Stage B default remains unavailable. A valid V1 signed profile asserts bounded healthy issuer time; it does not implement an AuthorityClock monitor. Server health/error-source contract and real nonce entropy remain OPEN before actual issuance/provider enablement. Future live binding/identity/trust lifecycle must invoke the coordinator invalidations; existing installers are not automatically wired in this package.
