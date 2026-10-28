# Vendor Documentation

This directory contains vendor protocol documents and their integrity manifest.

## Files

| ID | File | Status | Covers |
|---|---|---|---|
| V-02 / VG-17 | `93PC_FD_CProtocol_ver_2_0-4_2026_EN.pdf` | OBTAINED | Daisy Compact S 01 binary protocol v2.0-4 2026 |
| DOC-V3a | `DatecspayLowLevel_v3_2025.pdf` | OBTAINED | Datecs Pay LowLevel BLE protocol v3 2025 |
| DOC-V3b | `DatecsPay_Pinpad_Commands_v1_9_2020.pdf` | OBTAINED | Datecs Pay Pinpad Commands v1.9 2020 |
| DOC-V4a | `Protocol_Smart_Pro_v2_2025.pdf` | OBTAINED | Daisy Smart Pro fiscal JSON protocol v2.2025 |
| DOC-V4b | `Protocol_Daisy_Smart_Payment_v1_2026.pdf` | OBTAINED | Daisy Smart S Payment Bridge v1.2026 |

## Manifest

`manifest.json` — array of all known vendor documents, including those not yet obtained
(status: `OBTAINED`, `REQUESTED`, `PARTIALLY_OBTAINED`, `EXTERNAL`).

Each entry has: `id`, `doc`, `name`, `file`, `sha256`, `version`, `date_obtained`,
`status`, `url`, `license`, `covers`, `blocks`, `note`.

## Integrity Check

```bash
bash scripts/check-vendor-docs.sh
```

Verifies SHA256 of all OBTAINED entries. Returns exit 0 on success, non-zero on mismatch.

## Analysis Notes

- `DATECSPAY_PROTOCOL.md` — analysis of DatecspayLowLevel_v3_2025 for VG-16
- `DAISY_SMART_PROTOCOLS.md` — analysis of Smart Pro + Payment Bridge protocols for VG-17
- `DAISY_CURRENCY.md` — Daisy currency protocol notes
- `LETTER_DATECS.md` — vendor correspondence notes (Datecs)
- `LETTER_DAISY.md` — vendor correspondence notes (Daisy)
- `probe_results.json` — HIL probe results
