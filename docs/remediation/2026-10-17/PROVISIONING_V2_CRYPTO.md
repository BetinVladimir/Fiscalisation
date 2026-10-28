# Secure provisioning transport foundation (BFP2/BFPF)

Status: crypto and bounded framing implemented and host-tested; onboarding remains OPEN. This package does not authorize devices, install configuration, or produce READY.

## Trust and subject

The issuer receives only a factory-enrolled P-256 SPKI public key from a trusted device registry. A key advertised over BLE must never enroll or replace this key. Device verifies the configured issuer public key, exact device/issuer key IDs, a fresh 32-byte setup nonce and generation strictly above its durable generation floor. The binding SHA-256 is authenticated; the installer must compare it with the exact signed schema3 binding before committing anything.

## Binary format

All integers are unsigned big endian. BFP2 fixed header is 159 bytes: magic BFP2 (4), generation (8), setup nonce (32), signed binding SHA-256 (32), ephemeral uncompressed P-256 SEC1 public point (65), GCM IV (12), device key ID length (1), issuer key ID length (1), ciphertext length including tag (4). ASCII key IDs follow, each 1..96 characters from letters/digits/period/underscore/hyphen. The entire header including key IDs is AEAD additional data.

ECDH uses the enrolled device key and fresh ephemeral issuer key. HKDF-SHA256 uses the setup nonce as salt and `BeeFiscalProvisioningV2` followed by a NUL byte and the entire additional-data header as info. AES-256-GCM uses a fresh 12-byte IV, appending its 16-byte tag to ciphertext. Finally append a 64-byte ES256 P1363 signature over SHA-256(header + ciphertext + tag); only canonical low-S signatures are accepted. Signature verification precedes ECDH. Plaintext is nonempty and at most 32768 bytes. The decoder rejects trailing bytes and clears plaintext on all failures.

## BLE framing

BFPF frame: magic (4), transfer ID (16), total package size (4), offset (4), package SHA-256 (32), final flag (1), nonempty body. Maximum frame size 512, maximum package 34000. Transfers are strictly ordered, noninterleaved, bounded, with a 10-second idle timeout. Disconnect must reset the per-connection assembler. Completion requires exact final length and digest. A completed transfer is not an installation acknowledgement.

## Verification

Run `IoT/firmware/edge-agent-s3/idf/tests/run_provisioning_host.sh`; set MBEDTLS_SOURCE when ESP-IDF is elsewhere. It builds actual ESP-IDF mbedcrypto, generates disposable Go-issued vectors and decrypts them through actual DeviceIdentity and firmware crypto. Fixtures are temporary test keys, not external trust or certification evidence. Tests include tampered signature/header/ciphertext, a correctly signed invalid GCM tag, foreign private key, nonce/generation/subject mismatch, invalid curve point, framing order/duplicates/disconnect/timeout/digest/early-final. Go issuer tests also reject invalid subjects and oversized plaintext.

## Remaining acceptance work

- Persist issuer schema3 binding snapshot before publish, with verified FMIN and immutable exact-retry semantics.
- Factory enrollment and authorized provisioning API must obtain recipient key from trusted registry; define ownership and signed binding authorization.
- Read-only GATT identity/setup nonce, unified service UUID, per-connection BFPF integration and authenticated installed ACK.
- Validate decrypted strict secret inventory, schema3 configuration and exact binding digest; atomically journal candidate, secrets, WiFi and binding with crash recovery and nonce/generation replay protection.
- Actual WiFi station startup and repair provisioning when secrets are absent; never expose plaintext secrets to mobile relay.
- BeeFiscalApp relay UX, reconnect/retry and explicit installed state; physical Android/iOS acceptance remains external.
