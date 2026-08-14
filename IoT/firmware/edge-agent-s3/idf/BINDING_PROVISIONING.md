# ESP-IDF composite binding provisioning

Business transports start only when the firmware contains a pinned binding
signing public key, an envelope verifies with that key and exact `key_id`, the
schema/profile is valid, the generation is monotonic, referenced MQTT secrets
exist in protected NVS, and SD/SQLite is ready.

A development build without `main/BindingTrustAnchor.h` is intentionally
non-provisionable and exposes no radio ingress.

## Envelope `BFPE/1`

```text
magic "BFPE"       4 bytes
version            uint8 = 1
key_id_length      uint8
signature_length   uint16 big endian
json_length        uint32 big endian
key_id             UTF-8
signature          base64url over SHA-256(canonical JSON)
canonical JSON     UTF-8, maximum 8192 bytes
```

For MVP setup accepts one bounded envelope. Shared application-level BLE
fragmentation is SC-03. Installation retains the old active payload as
`previous`, commits active payload and generation at one NVS commit boundary,
and prohibits rollback. Secret values never enter binding JSON or SQLite; JSON
contains only keys referencing records in the `edge-secrets` NVS namespace.
