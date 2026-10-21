# BLE server certificate issuance foundation

Status: service-only opt-in foundation. No public issuance route, startup default, BLE TLS channel, production trust-store adoption or release readiness is introduced. Device-side root verification and purpose-bound signing (stage B) remain OPEN pending contract review and actual ESP32-S3 implementation/interoperability evidence. All production device private keys remain the enrolled DeviceIdentity key inside ESP32-S3 NVS; ATECC608A/608 is not used. Fresh PoP proves present possession, not historical entropy quality.

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

OPEN: implement S3 strict exact-integer canonical parser and trusted-root challenge verifier, own immutable binding/SPKI/scope/time checks, purpose-separated same-NVS-key signer and one-time handling; prove Go↔actual C/mbedTLS device proof interoperability with hostile fixtures. Then review certificate provisioning/install, trust-store purpose separation, expiry/renewal/revocation, mobile peer validation and actual reliable GATT/TLS adoption. No generic hash-signing command or unsigned CA oracle may be introduced to close those gaps.

## Review follow-up: persisted result semantics and wire bounds

Before publication or exact replay, the stored certificate is revalidated rather than trusting parseable JSON metadata. Validation binds record id/tenant/version and challenge digest; exact device id, DNS, enrolled SPKI, DER SHA256, NotAfter and configured CA-chain metadata; one PEM certificate without extra material; actual leaf P256 key, DigitalSignature-only KU, ServerAuth-only EKU, exact DNS/URI/CN, non-CA/basic constraints, issuance-time lower bound and expiry; and actual x509 chain/name/time verification under the configured separate BLE CA. Unknown/trailing JSON and oversized stored certificate JSON are rejected. Semantic corruption tests inject damaged but structurally parseable stored results via test-only direct state mutation and restart; they do not claim protection against an arbitrarily dishonest database returning a valid old full state.

Proof signatures require exactly 86 canonical unpadded base64url characters decoding strictly to 64 P1363 bytes. Oversized, padded, nonzero trailing pad-bit and short signatures are rejected before decoding large input, reaching the CA/time operation or saving any result.

Atomicity scope: plain Store requires a single writer, as its interface has no cross-process CAS. In-process guards serialize that writer. Versioned/DeltaVersioned Store must implement authoritative Load and atomic expected-generation Save; the production PostgreSQL implementation supplies that contract. No cross-writer atomicity claim is made for plain Store, and synthetic plain-store tests are not PostgreSQL multi-instance evidence. Multi-instance certificate/activation writes and rotation remain a deployment acceptance gate until actual database/device integration is added.

## Root integration verification — 2026-10-21

Full Fiscal backend, domain race suite and actual mbedTLS profile verifier PASS in the integrated tree. [Raw logs and exact tested source hashes](evidence/ble-server-issuance-2026-10-21/README.md) preserve the acceptance scope; device signing, trusted time and runtime channel gates above remain OPEN.
