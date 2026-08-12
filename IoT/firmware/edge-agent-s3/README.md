# BeeFiscal edge-agent-s3

PlatformIO firmware skeleton for an ESP32-S3 camera-class module acting as a
local fiscal edge adapter. It creates fiscal/payment drivers through
`IoT/protocol-abstraction` and maintains a durable SQLite transaction journal on
the built-in SD card.

## Build

```bash
cd IoT/firmware/edge-agent-s3
pio run
pio run -t upload
pio device monitor
```

Pinned dependencies:

- PlatformIO `espressif32@6.10.0`;
- Arduino framework supplied by that platform;
- `siara-cc/Sqlite3Esp32@2.5` from the
  [PlatformIO registry](https://registry.platformio.org/libraries/siara-cc/Sqlite3Esp32).

## Board and pins

The default compile target is `esp32-s3-devkitc-1` with 8 MB flash and OPI
PSRAM. “ESP32-S3 Camera Module” is not one unique board/pinout. Before hardware
deployment verify the exact module schematic and override:

```ini
-DEDGE_SD_CLK=<gpio>
-DEDGE_SD_CMD=<gpio>
-DEDGE_SD_D0=<gpio>
-DEDGE_FISCAL_UART_RX=<gpio>
-DEDGE_FISCAL_UART_TX=<gpio>
```

When all `EDGE_SD_*` values are `-1`, Arduino `SD_MMC` board defaults are used.
The storage runs in 1-bit SD mode to reserve pins for camera, UART, I²C secure
element and other peripherals. GPIO 17/18 in the example are placeholders and
must also be checked against camera/PSRAM wiring.

## Protocol integration

`DeviceProtocolProvider` is a thin firmware-facing entry point for:

- `IFiscalDevice`: Daisy, Datecs or Tremol over serial-derived channels;
- `IPaymentTerminal`: DatecsPay over a BLE GATT `Stream` such as
  `DatecsPayBleStream`.

The two objects are intentionally independent. `main.cpp` creates a Daisy UART
profile only as a non-operating development default. It never sends fiscal
commands during boot. Production must load vendor, channel and payment code from
signed provisioning data.

## SD and SQLite

`EdgeStorage` mounts SD_MMC at `/sdcard`, creates
`/sdcard/beefiscal/edge-agent.db`, applies idempotent schema migration and
provides:

- idempotent enqueue by unique `transaction_id`;
- ordered iteration over unsent transactions;
- attempt/error accounting;
- acknowledgement (`markSynced`);
- deletion only for acknowledged records and only after at least 90 days;
- explicit optimization/checkpoint operation.

SQLite uses `journal_mode=DELETE` and `synchronous=FULL`. WAL is intentionally
not selected for removable FAT media. Payloads are expected to be canonical JSON
and signatures are persisted separately; signing and verification belong to the
secure identity/orchestration layer.

The database is not encrypted by SQLite. Do not place tokens or private keys in
it. Each device generates its own P-256 identity keypair on ESP32-S3 during
first initialization. The private key is stored only in encrypted NVS and is
never exposed by the firmware API; production Flash Encryption and NVS
Encryption protect its at-rest representation. No ATECC608A or external crypto
provider is part of this architecture.

## Remaining hardware decisions

- exact S3 camera board name, revision and pin map;
- SD interface voltage/pull-ups and whether it supports SD_MMC or only SPI;
- fiscal UART electrical layer (native TTL versus MAX3232 RS-232);
- BLE transport instance and BluePad pairing policy;
- secure boot, flash/NVS encryption and ESP32-generated identity provisioning.
