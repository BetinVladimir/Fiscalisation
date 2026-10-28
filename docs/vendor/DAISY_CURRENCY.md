# Daisy Compact FP — Currency Parameter Programming

## Command

To read or set the currency mode on Daisy Compact FP, use the programming command:

cmd=`<SOH><LEN><SEQ><CMD><DATA><ETX><BCC><EOT>`

Where:
- `<SOH>` = 0x01 (start of header)
- `<SEQ>` = 0x20..0xFF (sequence counter, wraps at 0xFF→0x20)
- `<CMD>` = 0xFF (parameter read/write)
- `<DATA>` = parameter name + separator + value
- `<ETX>` = 0x04 (end of text)

## Currency Mode Parameter

Parameter name: `CurrencyCode`

| Mode | Code | Description              |
|------|------|--------------------------|
| BGN  | 2    | Bulgarian Lev (default)  |
| EUR  | 3    | Euro                     |

## Reading the Currency Parameter

Send cmd=255 (0xFF) with parameter name `CurrencyCode` to read the active currency mode.
Response contains the current code (integer string: `"2"` for BGN, `"3"` for EUR).

## Activation Gate

At device activation, the system reads `CurrencyCode` and validates it:

- Mode 2 (BGN) → device is **NOT_READY** for fiscal operations (EUR required by NRA)
- Mode 3 (EUR) → device is **READY** for fiscal operations

Any mismatch between the device currency and the sale currency causes
`FISCAL_DEVICE_UNAVAILABLE` before any I/O.

## Notes

- Daisy Compact FP protocol: LEN = data_length + 0x24
- SEQ counter: 0x20..0xFF (wraps at 0xFF → 0x20); NOT 0x7F like Datecs
- Text encoding: CP1251 (Windows-1251)
- Source: Daisy Compact FP protocol documentation v2.0, §7 Parameter Table

## References

- DSY-08: Currency gate implementation task
- DSY-09: EUR-mode enforcement in binding activation
