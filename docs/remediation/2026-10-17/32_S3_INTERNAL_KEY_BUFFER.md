# ESP32-S3 internal key handling — 2026-10-21

The current device identity private key remains in internal ESP32-S3 NVS (edge-identity/p256). ATECC608A/608 is unused. This change does not migrate keys or add an I2C dependency.

Three temporary private-scalar buffer cleanup sites in DeviceIdentity::open used ordinary memset, which is not a guaranteed secret-erasure primitive under optimization. They now use mbedtls_platform_zeroize, already available in this production source. The key storage format and public identity remain unchanged.

The actual-mbedTLS provisioning host suite additionally exercises zero and all-FF stored scalars: open fails, ready stays false, signing fails and clears a stale output, and mock durable/pending NVS data is not replaced. Reloading the original valid scalar produces the same public key. These test production identity/crypto code with an NVS substitute; they do not inspect optimized target RAM or prove encrypted physical flash.

Production Flash Encryption, NVS Encryption, secure boot and physical recovery remain separate release/HIL requirements. Development build success does not establish them. Validation logs are recorded separately after terminal completion.

Acceptance: provisioning host runner exit0 and S3 build exit0 (73.86s). Logs and tested source hashes: evidence/internal-key-2026-10-21.
