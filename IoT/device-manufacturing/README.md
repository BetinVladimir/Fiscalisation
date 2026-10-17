# BeeFiscal device manufacturing

Guarded station CLI for ESP32-S3 release hash verification, flashing and OIDC
workload-authenticated registration. The token is read only from
`BEEFISCAL_FACTORY_OIDC_TOKEN`; it is never accepted as a command-line argument.

```bash
python -m pip install -e .
beefiscal-factory flash --port /dev/ttyUSB0 --manifest flash-manifest.json --expected-manifest-sha256 <manifest-sha256>
beefiscal-factory register --backend https://fiscal.example --evidence evidence.json --proof <device-proof>
```

`device-proof` is produced by the device itself: unpadded base64url of the
64-byte IEEE P1363 ECDSA P-256 signature (`r || s`) over SHA-256 of the
canonical manufacturing proof. The station must never generate, receive or
persist the device private key.

The production station must obtain the OIDC token from workload identity and
must verify a signed release manifest before invoking `flash`.

## Flash manifest v1

The old firmware-only invocation is rejected: an application binary must never
be inferred to be a merged image and written at offset 0. The expected manifest
SHA-256 must come from independently verified build/release evidence. A hash pins
bytes; it does not authenticate the release publisher or replace release approval.

```json
{
  "schema_version": 1,
  "chip": "esp32s3",
  "mode": "segments",
  "flash_size_bytes": 8388608,
  "partition_layout": {
    "path": "partition-table.bin",
    "sha256": "<64 lowercase hex characters>",
    "table_offset": 32768,
    "app_label": "factory"
  },
  "segments": [
    {"kind": "bootloader", "offset": 0, "path": "bootloader.bin", "sha256": "<sha256>"},
    {"kind": "partition_table", "offset": 32768, "path": "partition-table.bin", "sha256": "<sha256>"},
    {"kind": "application", "offset": 65536, "path": "firmware.bin", "sha256": "<sha256>"}
  ]
}
```

Offsets above are examples, not defaults. Populate addresses, flash size and
partition label from the exact build's `flasher_args.json` / partition-table
artifacts. A manifest generator is not included in this package. Paths resolve
relative to the manifest. Offsets/sizes are JSON integers, never booleans/strings.
Every nonempty artifact has a mandatory SHA-256. Duplicate kinds, overlapping
segments, invalid offsets and segments outside flash are rejected before esptool.

Modes:

- `segments`: exactly bootloader + partition_table + application; ESP32-S3
  bootloader at explicit zero and application at the selected partition offset.
- `app_only`: exactly application, with verified binary partition layout and
  application fitting its selected partition. This does not inspect the layout
  already installed on a physical board; the station must establish that match.
- `merged`: exactly one merged artifact at explicit zero, bootloader/application
  image headers and embedded table matching the independently hashed layout.
  This intentionally narrow format rejects merged images extending beyond the
  selected app partition; arbitrary multi-app/data preloads need a future schema.

The table parser validates entries, partition overlaps/flash bounds, optional ESP
MD5 checksum record and selected app label/capacity. It reserves the full 4 KiB
partition-table sector, requires app offsets aligned to 64 KiB, other partition
offsets aligned to 4 KiB, and app capacities aligned to 4 KiB (current nonsecure
profile). Every entry has a unique nonempty label; unsupported predefined
types/subtypes/flags and invalid or duplicate otadata partitions fail closed.
Custom partition types 0x40–0xFE are permitted with 4 KiB offset alignment. ESP image magic is checked;
full image/signature/chip-revision validation remains the release toolchain and
esptool's responsibility. Verified bytes are copied into private temporary files
before invocation, avoiding rereading changed source artifacts. Cleanup also runs
when esptool fails. No device flash is performed by automated unit tests.

```bash
PYTHONPATH=src python3 -m unittest discover -s tests -v
```

Flash validation tests mock only the subprocess boundary and inspect exact
addresses and staged bytes. They cover segmented/app-only/merged layouts, hash
mismatch, range/overlap/duplicate failures, malformed layout and image headers.

## Воспроизводимый offline flash bundle

Команда `bundle` копирует проверенные bytes в новый каталог, формирует manifest и SHA-256. Она не вызывает esptool. App offset берётся из выбранного раздела binary partition table; существующий каталог не перезаписывается. Flash size, table offset и app label должны соответствовать проверенной конфигурации сборки.

```sh
PYTHONPATH=src python3 -m beefiscal_factory.cli bundle \
  --output /absolute/path/release-bundle \
  --bootloader /absolute/path/build/bootloader.bin \
  --partition-table /absolute/path/build/partitions.bin \
  --application /absolute/path/build/firmware.bin \
  --flash-size-bytes 8388608 --table-offset 0x8000 --app-label factory
```

Возвращённый `manifest_sha256` передаётся в `flash --expected-manifest-sha256`. Хеш подтверждает целостность bundle, но сам по себе не подтверждает доверенное происхождение: approval/release evidence должны фиксировать source snapshot, toolchain и этот digest. Изменение исходных build files после генерации не меняет bundle. При ошибке публикации частичный каталог сохраняется для диагностики; flash отклоняет отсутствующие или повреждённые файлы.
