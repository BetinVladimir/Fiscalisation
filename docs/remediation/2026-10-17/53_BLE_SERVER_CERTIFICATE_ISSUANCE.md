# BLE server certificate issuance foundation

Status: service-only opt-in foundation. No public issuance route, startup default, BLE TLS channel, production trust-store adoption or release readiness is introduced. Stage B now has actual firmware-source/mbedTLS host interoperability below; production trusted time, public transport wiring and full ESP-IDF target/device acceptance remain OPEN. All production device private keys remain the enrolled DeviceIdentity key inside ESP32-S3 NVS; ATECC608A/608 is not used. Fresh PoP proves present possession, not historical entropy quality.

## API and authority inventory

`NewBLEServerCertificateIssuer(serverCAPEM, serverCAKeyPEM, mqttCAPEM)` requires a separate P256 CA/key, CertSign and exactly ServerAuth EKU. Reusing the MQTT CA SPKI, ClientAuth/Any/mixed/unrestricted CA profile or a mismatched private key is rejected. Existing X509DeviceCredentialIssuer.Issue remains ClientAuth-only without DNS SAN.

`Service.SetBLEServerCertificateIssuer` is explicit opt-in, with no configuration/router wiring in this foundation. `NewBLEServerCertificateChallenge(deviceID, activationID, bindingID, tenant)` resolves authority and returns a signed envelope only after durable challenge Save succeeds. `IssueBLEServerCertificate(challengeID, tenant, canonicalProofJSON, P1363Signature)` verifies fresh device proof and returns only a durable immutable result. No caller chooses a SAN, public key, certificate expiry, CA or arbitrary signing payload.

| Authority | Required association | Atomic postcheck |
|---|---|---|
| activation request | ACTIVE EDGE_AGENT_S3; device/tenant/location/register, existing credential and enrolled key | exact unchanged activation record |
| device resource | ACTIVE S3; serial/credential, transaction-signing SPKI and derived kid match activation/enrollment | tenant/id/version |
| platform_device | exactly one ASSIGNED/DEPLOYED manufacturing enrollment for serial+tenant; P256 key matches activation | platform scope/id/version |
| composite_binding | ACTIVE, exact adapter/register, positive exact generation | tenant/id/version |
| issued_binding | immutable canonical payload hash, schema3, same device/tenant/location/register/generation/signing kid | tenant/hash/version |
| register | ACTIVE, current composite_binding_id and adapter_device_id, same location | tenant/id/version |
| location | ACTIVE, same tenant/location | tenant/id/version |

CA signing happens outside the repository mutex. Challenge publication and certificate publication recheck all six resource versions, the exact activation and enrollment uniqueness under the existing mutex; expiry is rechecked using a fresh clock read after CA work. A non-durable in-memory repository is rejected. These methods reuse the reviewed plain/versioned/delta persistence barrier from doc51; no new generic Store semantics are added.

## Canonical v1 contract

Canonical JSON is sorted-key UTF-8 without whitespace. Typed integer decoding and existing UseNumber canonicalization avoid float rounding. Unknown/duplicate/noncanonical fields, fractional/exponential integer forms, nonfinite values, null required values and overflow are rejected. binding_generation is positive and bounded by int64; a floating source generation at/above 2^53 is rejected rather than guessed.

Challenge fields: kind=BLE_SERVER_CERT_CHALLENGE, version=1, purpose=ble-tls-server, challenge_id UUID, nonce (canonical unpadded base64url, 32 bytes), issued_at unix seconds, expires_at=issued_at+120, device_id UUID, device_spki_sha256 lowercase hex, tenant_id, location_id, register_id, binding_sha256 lowercase hex, binding_generation, server_name and certificate_not_after=issued_at+86400. The fixed name is `device-<UUID>.ble.beefiscal.invalid`; certificate DNS SAN is that name and its URI SAN is `spiffe://beefiscal/ble-server/device/<UUID>`.

Envelope: canonical_json, signature (unpadded base64url DER ECDSA), key_id. Root signature is ECDSA P256 over **one SHA256** of `BEEFISCAL_BLE_SERVER_CERT_CHALLENGE_V1\n` followed by canonical challenge bytes. The existing binding root key id is reused by a method which accepts only a validated BLE challenge type/purpose, never an unsigned generic signing endpoint.

Proof fields: kind=BLE_SERVER_CERT_PROOF, version=1, purpose=ble-tls-server, challenge_id, challenge_sha256 and device_spki_sha256. challenge_sha256 is SHA256 of the domain-prefixed root challenge preimage above. Device signature is unpadded base64url 64-byte P1363 ECDSA over **one SHA256** of `BEEFISCAL_BLE_SERVER_CERT_PROOF_V1\n` followed by canonical proof bytes. Legacy DeviceIdentity double-hashing must not be reused for this purpose. DER proof signatures and wrong-key/double-hashed proofs are rejected.

The 120-second challenge and 24-hour certificate are explicit bounded foundation policies, not a legal or production operating acceptance claim. Server CA must cover the full certificate lifetime. Before new issuance, time must lie inside challenge validity; replay may return the same durable result after challenge expiry but cannot return it after certificate expiry. Changing context/revoking authority rejects replay. The same proof can be re-signed nondeterministically without rotating its immutable result.

## Consumption, replay and failure behavior

The immutable ble_server_certificate record keyed by challenge_id is also its one-time consumption record. It contains the exact certificate result and challenge/proof digests. Both are persisted in one atomic snapshot/CAS; a concurrent same-proof issuance returns the first exact committed result. A mismatched digest conflicts. No optimistic certificate is returned after a failed Save.

If Save fails definitely, healthy retry may issue and commit once. If Save commits then returns an error, authoritative reload/recovery exposes that existing result; retry returns its exact bytes without another persisted issuance. If reload also fails, all authority reads/writes remain fenced and no certificate is returned until successful recovery and retry. Restart preserves the exact result. Stale generation or challenge expiry during CA work rejects publication and leaves the challenge unconsumed. Challenge creation has the same confirmed-save/no-publication rule; after an uncertain creation without a returned id, a later request creates a new random challenge rather than exposing an unconfirmed one.

## Evidence and remaining work

Go tests cover separate profile and enrolled key, ClientAuth regression, wrong purpose/CA/key/name, actual independent Go TLS1.3 client verification, fresh proof and exact replay/restart across plain/versioned/delta, committed-error/read-outage recovery, authority changes/deadlines after CA work, canonical/int64 boundaries, eight concurrent proofs producing one durable result, and default-disabled/non-durable rejection.

Mandatory explicit host verifier: `MBEDTLS_SOURCE_DIR=... MBEDTLS_BUILD_DIR=... scripts/verify-ble-server-certificate-profile.sh`. It fails closed for missing Go/C compiler/runtime or unreviewed mbedTLS/framework source revisions. Synthetic Go-produced certificates are checked by actual mbedTLS3.6.7 for CA, DNS, ServerAuth EKU and enrolled-SPKI pin; ClientAuth-only, wrong DNS, wrong issuer and wrong enrolled key must fail. The independent Go TLS client exercises real handshake verification. Existing doc46 portable TLS fixture separately proves the actual mbedTLS TLS client rejects an independently presented ClientAuth-only server certificate.

`BLE_LIFECYCLE_SYNTHETIC_FIXTURE_DIR=/absolute/disposable/path go test ./internal/domain -run '^TestBLEServerCanonicalContractFixtures$' -count=1` emits canonical signed challenge/proof/trusted context and synthetic keys for stage-B interoperability. Test-only private fixtures are never emitted by production paths. These fixtures do not prove an actual S3 signer yet.

Stage B core verification/signing/cache host coverage is described below. OPEN: fresh signed device-nonce trusted-time anchor and production integration; then certificate provisioning/install, trust-store purpose separation, expiry/renewal/revocation, mobile peer validation and actual reliable GATT/TLS adoption. No generic hash-signing command or unsigned CA oracle may be introduced to close those gaps.

## Review follow-up: persisted result semantics and wire bounds

Before publication or exact replay, the stored certificate is revalidated rather than trusting parseable JSON metadata. Validation binds record id/tenant/version and challenge digest; exact device id, DNS, enrolled SPKI, DER SHA256, NotAfter and configured CA-chain metadata; one PEM certificate without extra material; actual leaf P256 key, DigitalSignature-only KU, ServerAuth-only EKU, exact DNS/URI/CN, non-CA/basic constraints, issuance-time lower bound and expiry; and actual x509 chain/name/time verification under the configured separate BLE CA. Unknown/trailing JSON and oversized stored certificate JSON are rejected. Semantic corruption tests inject damaged but structurally parseable stored results via test-only direct state mutation and restart; they do not claim protection against an arbitrarily dishonest database returning a valid old full state.

Proof signatures require exactly 86 canonical unpadded base64url characters decoding strictly to 64 P1363 bytes. Oversized, padded, nonzero trailing pad-bit and short signatures are rejected before decoding large input, reaching the CA/time operation or saving any result.

Atomicity scope: plain Store requires a single writer, as its interface has no cross-process CAS. In-process guards serialize that writer. Versioned/DeltaVersioned Store must implement authoritative Load and atomic expected-generation Save; the production PostgreSQL implementation supplies that contract. No cross-writer atomicity claim is made for plain Store, and synthetic plain-store tests are not PostgreSQL multi-instance evidence. Multi-instance certificate/activation writes and rotation remain a deployment acceptance gate until actual database/device integration is added.

## Root integration verification — 2026-10-21

Full Fiscal backend, domain race suite and actual mbedTLS profile verifier PASS in the integrated tree. [Raw logs and exact tested source hashes](evidence/ble-server-issuance-2026-10-21/README.md) preserve the acceptance scope; device signing, trusted time and runtime channel gates above remain OPEN.

## Stage B: firmware proof foundation (disabled by default)

`BLEServerCertificateProofSigner::prove(envelope, out)` uses the actual DeviceIdentity/NVS path, current installed binding and protected compiled `BindingTrustAnchor.h` root. Production accepts no request-provided root. Explicit root injection exists only under the host-test compile macro. No public BLE/HTTP command, app_main enablement, certificate installation or channel is added.

Time is an explicit internal `TrustedBLECertificateTime` authority. Its production default always returns unavailable. Unauthenticated SNTP/network success and the signed certificate challenge are **not** cold-boot time anchors. The new proof path fails closed until a separately reviewed fresh signed device-nonce anchor supplies trusted time. That operational time gate remains OPEN. Host tests explicitly inject a synthetic trusted clock.

The existing lossless go_json parser retains numeric lexical tokens; the BLE verifier converts every typed integer through exact int64 parsing into Integer nodes before canonical encoding. This prevents generic float64 encoding from rounding 2^53+1. Duplicate/unknown fields, null/fractional/exponential/overflow integers and noncanonical bytes are rejected. Native parser tests cover 2^53−1, 2^53, 2^53+1 and int64 maximum independently of the existing legacy installed-binding INT32 generation cap, which is unchanged. Challenge and root signature sizes/base64url are bounded before crypto allocation.

The private friend-only DeviceIdentity method receives a verified permit, constructs the fixed proof domain itself, hashes exactly once and signs P1363 with the existing NVS P256 key/startup-seeded bounded DRBG. Legacy double-hashed sign_hash_hex is unchanged and unused. No digest-signing endpoint, new private key export, RF entropy toggle or silent rotation is introduced. The binding-only encrypted load validates the same authenticated installed package while suppressing private-key PEM reconstruction and wiping temporary secrets.

### Installed authority and lock inventory

| Source/path | Guard and authority |
|---|---|
| ProvisionedBindingStore open/close/load/install/validate/generation | shared installed_authority_mutex; signature-validated monotonic legacy installation |
| EncryptedProvisioning load/load_binding/load_internal/install | same recursive guard; authenticated encrypted blob first, legacy fallback only on NOT_FOUND |
| BLEServerCertificateProofSigner::prove | same guard held from initial load through signing, durable cache confirmation, final binding/time check and output publication |
| DeviceIdentity open/sign_hash_hex/ECDH/provisioning-install/private-PEM/new BLE proof | existing identity mutex; none calls an installer or acquires the authority mutex |

Lock order is authority then identity. Recursive authority locking is required for existing nested encrypted/legacy validation and generation reads. There is no reverse identity-to-authority path in the inspected methods. Both actual encrypted and legacy concurrent-installer tests pause proof at its real NVS commit, require the installer to remain excluded, release proof, then verify a higher installed generation rejects the old challenge. This is in-process serialization; it is not a distributed authority claim or physical flash power-loss test.

### Bounded durable proof cache

Four entries occupy one fixed 728-byte versioned NVS blob. Only public challenge/context digests, UUID, generation, expiry and signature are stored. Validation precedes cache lookup. A matching cached signature is verified under the current DeviceIdentity; exact replay survives a new signer and a newly opened DeviceIdentity using the same NVS scalar. No unexpired entry is evicted; full cache rejects a fifth challenge. Expired entries may be reclaimed. Cached proof is never returned after local challenge expiry or loss of trusted time.

Publication follows sign → NVS set/commit → fresh exact-size/byte-for-byte confirmed read → current binding/time check. Save/read uncertainty returns no proof and requires an authoritative reload plus retry before further publication. Tests cover first empty cache and an acknowledged earlier proof under definite failure, committed-then-error and post-commit read outage, including preservation of the earlier proof. Missing cache after acknowledged state, malformed/truncated blobs, changed second-read size and invalid cached signatures fail closed. This relies on authoritative NVS read/commit semantics; it does not detect an arbitrary dishonest complete valid old storage image.

### Reproduction and acceptance boundary

Run `scripts/verify-ble-certificate-proof-host.sh` with MBEDTLS_SOURCE_DIR, MBEDTLS_BUILD_DIR and CJSON_SOURCE_DIR; CXX/CC may select strict GCC15. Missing tools/dependencies or unreviewed mbedTLS source revisions fail the runner. The explicit runner compiles actual firmware C++/DeviceIdentity code with real mbedTLS3.6.7 and a fault-injected NVS/entropy host model, installs an actual Go-sealed signed schema3 package, generates device proof and passes it to the actual Go IssueBLEServerCertificate method. Go checks the resulting enrolled-SPKI-bound certificate and exact replay. Generic Go tests skip this explicit native test when no binary is supplied and do not claim native coverage.

Host evidence covers hostile correctly root-signed contexts/canonical inputs, wrong issuer/key-id/signatures, exact integer boundaries, unknown/default time, expiry/future time, missing installation/foreign NVS identity, cache capacity/corruption/restart, NVS uncertainty with prior proof preserved and both concurrent installer paths. Strict GCC15 uses -Wall -Wextra -Werror without suppression. Existing actual provisioning/installer and startup entropy/DRBG exhaustion tests must also pass.

Mandatory remaining acceptance: complete ESP-IDF target build via the repository workflow after push, real S3 NVS/flash/reboot/power-loss and concurrent runtime tests, fresh signed trusted-time anchor and its lifecycle, purpose trust rotation/revocation, certificate install and authenticated mobile/channel integration. Host execution of actual source is not a full target-build or physical-device acceptance claim.

## Root Stage B integration verification

Actual Go→strict GCC15 C++/mbedTLS DeviceIdentity/NVS→Go issuer and root domain race PASS. All16 package source hashes match worker-tested sources; existing encrypted provisioning/installer and entropy/DRBG suites PASS with those same source changes. [Raw logs, source/dependency hashes and boundaries](evidence/ble-device-proof-2026-10-21/manifest.json). cJSON provenance is exact source hashes/header1.7.18; its dangling PlatformIO submodule pointer does not establish git-clean provenance. Default time remains unavailable, public command absent. Fresh complete official-IDF target build and physical acceptance remain OPEN until separately evidenced.
