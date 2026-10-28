# DatecsPayBleProfile.h — Constant ↔ PDF Section Trace

Источник констант: `IoT/common-modules/datecspay/DatecsPayBleProfile.h`
Документация: `DatecspayLowLevel_v3_2025.pdf` (DOC-V3a), `DatecsPay_Pinpad_Commands_v1_9_2020.pdf` (DOC-V3b)
Статус: DOC-V3 получен 2026-10-23; BLE UUID отсутствуют в PDF (B-2); Q-B1 открыт.

## Таблица константа ↔ раздел PDF

| constant | value | type | source | pdf_section | spec_pending | note |
|---|---|---|---|---|---|---|
| ServiceUuidPrefix | "d839fc3c-84dd-4c36-9126-" | const char* | Android SDK example (vendor) | N/A — not in DOC-V3 PDF | true | UUID prefix not documented in PDF; Q-B1: pending written confirmation |
| PowerCharacteristic | "22ffc547-1bef-48e2-aa87-b87e23ac0bbd" | const char* | Android SDK example (vendor) | N/A — not in DOC-V3 PDF | true | BLE characteristic UUID not in PDF; Q-B1 pending |
| WakeCharacteristic | "f953144b-e33a-4079-b202-e3d7c1f3dbb0" | const char* | Android SDK example (vendor) | N/A — not in DOC-V3 PDF | true | BLE characteristic UUID not in PDF; Q-B1 pending |
| ReadCharacteristic | "1f6b14c9-97fa-4f1e-aaa6-7e152fdd04f4" | const char* | Android SDK example (vendor) | N/A — not in DOC-V3 PDF | true | BLE characteristic UUID not in PDF; Q-B1 pending |
| WriteCharacteristic | "b378db85-4ec3-4daa-828e-1b99607bd6a0" | const char* | Android SDK example (vendor) | N/A — not in DOC-V3 PDF | true | BLE characteristic UUID not in PDF; Q-B1 pending |
| VendorWriteChunk | 19 | uint16_t | Android SDK example (vendor) | N/A — not confirmed in DOC-V3 | true | BLE write chunk size (19 bytes); Q-B1: pending written confirmation of MTU |
| PowerOff | 0x30 | uint8_t | Android SDK example (vendor) | N/A — BLE power control not in DOC-V3 | true | Power characteristic OFF value; not described in DOC-V3 PDF sections |
| PowerOn | 0x31 | uint8_t | Android SDK example (vendor) | N/A — BLE power control not in DOC-V3 | true | Power characteristic ON value; not described in DOC-V3 PDF sections |

## Примечания

- B-2 (из DATECSPAY_PROTOCOL.md): BLE UUID профиля **отсутствуют в PDF** — взяты из Android SDK-примера вендора.
- Q-B1: письменное подтверждение UUID и размера записи ожидается от Datecs.
- Все 8 констант помечены `spec_pending: true` до получения подтверждения (Q-B1).
- HIL тесты BluePad допустимы только с `merchant credentials` (DOC-V6) и после закрытия Q-B1.
- Для профиля BLE используется `DatecsPay_Pinpad_Commands_v1_9_2020.pdf` (DOC-V3b) как дополнение к DOC-V3a: разделы транзакционного потока, строки ошибок; UUIDs ни в одном из документов нет.
