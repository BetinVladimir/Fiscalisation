# Prepared native harness evidence

`make native-harness-test`:30 tests PASS, no skips; source syntax check:26 mjs modules PASS. The30 total include eight actual localhost HTTP tests for malformed JSON, byte limits, stalled bodies and invalid UTF8 across both products. Other tests cover fixture DTO/auth ledgers, snapshot admission, path/entrypoint/env restrictions, process deadlines, device leases and evidence redaction. No Android emulator, native build, APK installation, iOS app or physical USB/COM/BLE device was started in this check.

Logs are preserved byte-for-byte in deterministic gzip. Manifest hashes exact harness sources, selected native YAML scenarios and Makefile. These hashes are test-harness inputs, not hashes of a newly built APK/IPA/firmware. No signing key or live JWT is included. Local fixtures use disposable synthetic authority and do not replace real issuer/S3 interoperability evidence.
