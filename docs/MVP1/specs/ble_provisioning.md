# BLE Provisioning Protocol v2

Source documents (authoritative, in priority order):
1. `IoT/firmware/edge-agent-s3/idf/BINDING_PROVISIONING.md`
2. `docs/remediation/2026-10-17/PROVISIONING_V2_CRYPTO.md`
3. `docs/remediation/2026-10-17/52_RELIABLE_GATT_STREAM.md`

## GATT Service and Characteristics

Service: Edge Device v2 Provisioning Service

| Attribute | UUID | Properties |
|-----------|------|------------|
| Edge v2 Service | `7b6f2000-7c6d-4c7a-9e4f-424545464953` | Service |
| Device Info | `7b6f2001-7c6d-4c7a-9e4f-424545464953` | READ |
| Session Control | `7b6f2002-7c6d-4c7a-9e4f-424545464953` | WRITE with response |
| Session RX | `7b6f2003-7c6d-4c7a-9e4f-424545464953` | NOTIFY |
| Session TX | `7b6f2004-7c6d-4c7a-9e4f-424545464953` | WRITE with response |

Source: `BeeFiscalApp/src/smartDeviceBle.ts` (UUID constants; firmware counterpart required for full verification — not independently confirmed in firmware source reviewed here — BLOCKED_SPEC: firmware UUID source file not in scope).

## Device Info Characteristic

READ returns JSON (base64-encoded):

```json
{
  "protocol_version": 2,
  "device_id": "<string>",
  "serial": "<string>",
  "firmware_version": "<string>",
  "nonce": "<string>"
}
```

`protocol_version` must be exactly `2`; mismatch → `EDGE_PROTOCOL_V2_REQUIRED`.

## Authentication Command Table

| Command | Direction | Characteristic | Payload | Response (via Notify SESSION_RX) |
|---------|-----------|---------------|---------|----------------------------------|
| `AUTH_V2` | App → Device | SESSION_CONTROL | `{type:"AUTH_V2", capability: DeploymentCapability}` | `{type:"CHALLENGE", nonce:"<base64>"}` |
| `AUTH_RESPONSE` | App → Device | SESSION_CONTROL | `{type:"AUTH_RESPONSE", signed_nonce:"<base64>"}` | `{type:"AUTH_OK"}` or `{type:"AUTH_FAIL"}` |

`DeploymentCapability` structure:
```
{
  version: 2,
  device_id: string,
  capability_id: string,
  permissions: string[],
  expires_at: number,
  signature: string
}
```

Validation: `capability.version === 2` AND `capability.device_id === deviceInfo.device_id`. Mismatch → `CAPABILITY_DEVICE_MISMATCH`.

### Challenge signing

BLOCKED_SPEC: The exact mechanism for computing `signed_nonce` from the challenge nonce and capability is not described in the reviewed firmware protocol documents. The firmware uses BFP2 (ECDH + HKDF-SHA256 + AES-256-GCM, ES256 P1363 signature) for binding payload transport (see PROVISIONING_V2_CRYPTO.md), but the mobile app's side of the AUTH_V2 challenge-response signing is not independently specified in the authoritative sources.

## Provisioning Commands (post-authentication)

| Command | Direction | Characteristic | Payload | Response |
|---------|-----------|---------------|---------|----------|
| `SET_WIFI` | App → Device | SESSION_TX | Encrypted + fragmented `{type:"SET_WIFI", ssid:"<string>", password:"<string>"}` | `{type:"WIFI_OK"}` or `{type:"WIFI_FAIL"}` via SESSION_RX NOTIFY |
| `BIND_STORE` | App → Device | SESSION_TX | Encrypted `{type:"BIND_STORE", location_id:"<string>", register_id:"<string>", roles:[...]}` | `{type:"BIND_OK"}` or `{type:"BIND_FAIL"}` via SESSION_RX NOTIFY |

BLOCKED_SPEC: Session key derivation after AUTH_OK is not described in the reviewed firmware documents.

## Fragmentation

Source: `PROVISIONING_V2_CRYPTO.md` — BFPF frame format:
```
magic          4 bytes
transfer_id   16 bytes
total_size     4 bytes (uint32 big-endian)
offset         4 bytes (uint32 big-endian)
sha256        32 bytes (SHA-256 of complete package)
final_flag     1 byte  (0x01 = last frame)
body          nonempty
```
- Maximum frame size: 512 bytes
- Maximum package size: 34 000 bytes
- Idle timeout: 10 seconds (transfer level)

Body per frame ≤ `negotiated_MTU - 3` bytes (ATT overhead); `SET_WIFI` payload must be split into frames each ≤ `MTU - 3` bytes.

## Timeouts

| Operation | Timeout | Error code |
|-----------|---------|-----------|
| Device scan | 15 s | `EDGE_SCAN_TIMEOUT` |
| connect() | 15 s | `EDGE_CONNECT_TIMEOUT` |
| Authentication (AUTH_OK after AUTH_V2) | 10 s | `EDGE_AUTH_TIMEOUT` |
| connect not completed | 15 s | `EDGE_CONNECT_TIMEOUT` |

After `EDGE_CONNECT_TIMEOUT`, `cancelDeviceConnection` must be called.

## BleManager lifecycle

`BleManager` must be initialized lazily (not at module import time) to avoid creating Bluetooth state on platforms where BLE is unavailable.

## Error codes

| Code | Meaning |
|------|---------|
| `BLE_PERMISSION_DENIED` | Android BLE permissions not granted |
| `EDGE_SCAN_TIMEOUT` | No device found within 15 s |
| `EDGE_DEVICE_INFO_MISSING` | Device Info characteristic returned empty value |
| `EDGE_PROTOCOL_V2_REQUIRED` | Device reports protocol_version ≠ 2 |
| `CAPABILITY_DEVICE_MISMATCH` | capability.version ≠ 2 or capability.device_id ≠ device_id |
| `EDGE_AUTH_TIMEOUT` | No AUTH_OK/AUTH_FAIL within 10 s of AUTH_V2 |
| `EDGE_AUTH_REJECTED` | Device responded AUTH_FAIL |
| `BLE_SESSION_UNAUTHORIZED` | setWifi/bindStore called before authenticate |
| `EDGE_CONNECT_TIMEOUT` | connect()/discoverAllServicesAndCharacteristics did not complete within 15 s |

## Binding Envelope (BFPE/1)

Source: `BINDING_PROVISIONING.md`:
```
magic "BFPE"       4 bytes
version            uint8 = 1
key_id_length      uint8
signature_length   uint16 big-endian
json_length        uint32 big-endian
key_id             UTF-8
signature          base64url over SHA-256(canonical JSON)
canonical JSON     UTF-8, max 8192 bytes
```
Business transports start only when:
- Firmware contains a pinned binding signing public key
- Envelope verifies with that key and exact `key_id`
- Schema/profile is valid and generation is monotonic
- Referenced MQTT secrets exist in protected NVS
- SD/SQLite is ready

BLOCKED_SPEC: Mobile app interaction for binding envelope provisioning is not described in the reviewed documents; the firmware receives the envelope but the mobile-side API for sending it is not defined.
