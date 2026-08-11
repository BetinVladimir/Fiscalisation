# Daisy fiscal-device protocol

This module implements Daisy Tech PC-to-FD Communication Protocol V.2.0-4
(2026) over an Arduino `Stream`.

The protocol layer provides all command codes from the specification and a raw
`execute()` API. `DaisyPrinter` provides validated typed builders for the MVP
receipt, payment, refund, report, cash and device-information operations.

Important wire rules implemented here:

- `LEN` includes the `LEN` byte itself and has the `0x20` offset;
- `SEQ` cycles through `0x20..0xFF`;
- BCC is the arithmetic sum from `LEN` through `PST1`; each checksum nibble is
  encoded by adding `0x30` (`A..F` become bytes `0x3A..0x3F`, not ASCII letters);
- `NAK`, timeout, malformed response and bad BCC retry the identical SEQ/CMD;
- every `SYN` extends the response timeout;
- responses require exact length, `PST2`, six status bytes, `PST1`, BCC and ETX;
- report lines (`0x1A`) and structured EJT blocks (`0x1B LEN DATA`) use
  `executeStreaming()`, which sends `DC1` after each accepted block.

All textual fields supplied to the module must already be encoded as CP1251.
Tax groups may be supplied as Latin `A..H` or raw CP1251 `0xC0..0xC7`.

Run tests:

```sh
./run-tests.sh
```
