# External HIL evidence verification (GAP-034)

`make supto-hil-verifier-test` tests the verifier with ephemeral cryptographic fixtures. It does not run hardware, manufacture physical evidence, approve production, or establish SUPTO/NAP legal acceptance.

Actual evidence verification requires independently maintained inputs, outside the evidence directory and resolved symlink tree:

```sh
HIL_TRUST_REGISTRY=/controlled/hil/reviewers.json \
HIL_REQUIREMENTS=/controlled/hil/approved-plan.json \
make supto-hil-verify EVIDENCE_DIR=/evidence/physical-run HIL_RUBY=/opt/homebrew/opt/ruby/bin/ruby
```

Use Ruby with Ed25519 OpenSSL support and JSON gem >= 2.18 (duplicate-key rejection). The full acceptance subprocess honors HIL_RUBY, falling back to its own RbConfig.ruby interpreter; this avoids silently selecting a different system Ruby. The macOS system Ruby is insufficient; `HIL_RUBY` selects the provisioned runtime. No dependency is downloaded by the gate. Missing inputs, incompatible runtime, unknown fields, malformed JSON, or any mismatch fail closed. `supto-software-acceptance` includes verifier unit tests; the real HIL gate remains separate and requires actual external inputs.

Security authority must provision reviewer public keys and the approved plan through a controlled channel. Whoever can replace both environment-selected files can replace the trust decision: protect CI variables, paths and write permissions independently of evidence producers. No real reviewer key or approved device matrix is provided by this change.

## Schema version 1

All objects have exactly the listed keys. All inventories and string arrays are nonempty and unique. IDs and metadata are nonempty strings; SHA256 is 64 lowercase hexadecimal characters; source commit is a full 40-character lowercase Git commit. UTC timestamps use `YYYY-MM-DDTHH:MM:SSZ`. Arrays are order-independent when compared with the approved plan.

External registry structure:

```json
{
  "schema_version": 1,
  "reviewers": [{
    "reviewer_id": "independently-approved-reviewer",
    "public_key_path": "keys/reviewer-public.pem",
    "public_key_sha256": "REPLACE_WITH_SHA256_OF_ED25519_PUBLIC_SPKI_DER",
    "revoked": false,
    "valid_from": "2026-10-01T00:00:00Z",
    "valid_until": "2027-01-01T00:00:00Z"
  }]
}
```

Key paths resolve relative to the registry file, or may be absolute. Their real paths must remain outside the bundle. Registry, plan and public-key files must have exactly one hard link; hard-linked evidence-tree aliases are rejected. Symlink aliases resolve before containment checks. Only public Ed25519 PEM keys are accepted. The fingerprint hashes public SPKI DER bytes, not PEM formatting. Revoked reviewers and evidence extending beyond reviewer validity are rejected.

External approved plan structure (placeholders intentionally cannot pass):

```json
{
  "schema_version": 1,
  "source_commit": "REPLACE_WITH_FULL_COMMIT",
  "execution_valid_from": "2026-10-17T00:00:00Z",
  "execution_valid_until": "2026-10-18T00:00:00Z",
  "max_evidence_age_seconds": 86400,
  "device_profiles": [
    {"profile_id": "s3", "manufacturer": "APPROVED_MANUFACTURER", "serial_numbers": ["APPROVED_PHYSICAL_SERIAL"], "model": "REPLACE_WITH_BOARD_MODEL", "firmware_version": "REPLACE_WITH_FIRMWARE"},
    {"profile_id": "fiscal", "manufacturer": "APPROVED_MANUFACTURER", "serial_numbers": ["APPROVED_PHYSICAL_SERIAL"], "model": "REPLACE_WITH_VENDOR_MODEL", "firmware_version": "REPLACE_WITH_VENDOR_FIRMWARE"}
  ],
  "build_artifacts": [{"artifact_id": "iot-firmware", "sha256": "REPLACE_WITH_APPROVED_BINARY_SHA256"}],
  "required_cases": [{
    "case_id": "iot-datecs-lost-response",
    "profile": "minipos-iot-datecs",
    "os": "ANDROID_APPROVED_VERSION",
    "transports": ["BLE", "COM"],
    "device_profile_ids": ["s3", "fiscal"],
    "build_artifact_ids": ["iot-firmware"],
    "evidence_roles": [{"role": "COM_CAPTURE", "kind": "CAPTURE"}, {"role": "BLE_CAPTURE", "kind": "CAPTURE"}]
  }]
}
```

Extend device profiles and cases for each Datecs COM, Daisy USB, BlueCash Android app, BluePay BLE, S3 firmware, MiniPOS Android/iOS and MQTT/BLE route combination in the approved test matrix. A case supports multiple devices, binaries and transports. Transport vocabulary is `USB`, `COM`, `BLE`, `MQTT`, `HTTP`; OS includes the approved version. Distinct OS/device/route variants need distinct case IDs. All required cases must be present; unknown and duplicate case IDs fail.

The evidence bundle contains `hil-manifest.json`, `hil-manifest.sig` and referenced artifacts. Signature is exactly 64 raw Ed25519 bytes over the exact manifest file bytes (no JSON normalization). No `trusted-reviewer-public.pem` is accepted inside the bundle.

Manifest structure:

```json
{
  "schema_version": 1,
  "evidence_type": "PHYSICAL_HIL",
  "result": "PASS",
  "reviewer_id": "independently-approved-reviewer",
  "source_commit": "REPLACE_WITH_FULL_COMMIT",
  "issued_at": "2026-10-17T12:00:00Z",
  "valid_until": "2026-10-18T12:00:00Z",
  "devices": [{"device_id": "physical-instance", "profile_id": "fiscal", "manufacturer": "VENDOR", "model": "APPROVED_MODEL", "serial_number": "PHYSICAL_SERIAL", "firmware_version": "APPROVED_FIRMWARE"}],
  "artifacts": [
    {"artifact_id": "iot-firmware", "path": "build/firmware.bin", "sha256": "APPROVED_BINARY_SHA256", "kind": "BUILD", "role": "BUILD"},
    {"artifact_id": "serial-capture", "path": "logs/serial.bin", "sha256": "CAPTURE_SHA256", "kind": "CAPTURE", "role": "COM_CAPTURE"}
  ],
  "cases": [{"case_id": "iot-datecs-lost-response", "profile": "minipos-iot-datecs", "os": "ANDROID_APPROVED_VERSION", "transports": ["BLE", "COM"], "device_ids": ["physical-instance"], "build_artifact_ids": ["iot-firmware"], "result": "PASS", "started_at": "2026-10-17T11:00:00Z", "completed_at": "2026-10-17T11:05:00Z", "artifact_ids": ["serial-capture"]}]
}
```

This abbreviated manifest is structural guidance, not a matching or valid run: include every approved device profile instance per case, including S3 and BLE_CAPTURE in the illustrated plan. Artifact kinds are BUILD, LOG, CAPTURE, REPORT. Each case must reference non-build execution evidence. Device manufacturer, serial (approved inventory), model and firmware must match the independent plan; each physical instance has manufacturer and serial metadata. Simulator/STUB/emulator metadata is rejected. Reviewer attestation remains necessary: file signatures alone cannot prove a device was physically connected.

Artifacts must be relative paths without empty/dot/parent components or backslashes. Realpath must remain inside the evidence directory, including symlinks. Every referenced artifact is hashed, duplicate physical artifact paths are rejected, and approved build hashes must match. Cases must finish before issuance and execute entirely within reviewer authority and the external plan execution window. The plan maximum age bounds the earliest case start relative to verification time. Invalid calendar dates are rejected without normalization. Each case must have exactly the approved evidence role/kind pairs (for example COM_CAPTURE/CAPTURE and BLE_CAPTURE/CAPTURE); a generic LOG cannot replace a required transport capture. Manifest and reviewer must be unexpired at verification time.

## Remaining independent gates

HIL verification is not legal acceptance or activation permission. `verify_legal_evidence.rb` still trusts a key inside its own bundle and an asserted organization field; that trust gap requires separate external legal authority and scope/validity binding. Production activation must consume independently verified HIL/release/legal proofs and bind them to the activated build and device; a successful standalone HIL command does not establish that integration. Keep production/SUPTO approval blocked until those gates and real physical matrix runs are accepted.
