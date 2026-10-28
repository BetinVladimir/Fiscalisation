# Native BLE write deadline — 2026-10-21

Before fix, actual production NativeBleTransport with a non-settling fake adapter write produced an unhandled BLE_RESULT_TIMEOUT and stranded the command. Result rejection is now observed immediately alongside writes; an independent whole-write deadline remains active even after an early result. Failure invalidates the connection generation and reports UNKNOWN_AFTER_SEND after any write attempt. Late resolution cannot transmit subsequent fragments; reconnect/reconciliation is required. No automatic monetary retry is added.

Full npm test:64/64 PASS; npx tsc --noEmit exit0. Regressions cover non-settling write, early result plus hung write, late completion fencing and refusal before reconnect. These simulate adapter behavior and do not establish authenticated device responses or physical BLE acceptance.
