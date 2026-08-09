# Support and escalation guide

## Severity

- `S0`: duplicate/lost fiscalization, cross-tenant exposure, corrupted immutable evidence or uncontrolled card/fiscal mismatch. Stop affected execution immediately.
- `S1`: sales blocked for a site/device, unresolved UNKNOWN without safe reconciliation, restore or security control failure.
- `S2`: degraded non-critical reporting/editor/diagnostic function with a safe workaround.
- `S3`: cosmetic/documentation issue with no fiscal, payment, security or retention impact.

## Minimum ticket evidence

Collect UTC time, tenant/register/device IDs, operation/payment/idempotency IDs, app/build/API versions, selected route, final-device and terminal readiness, Edge journal/cursor/storage state, HTTP status/trace ID and redacted logs. Never collect PAN, PIN, CVV or magnetic-track data.

## Common decisions

- `FISCAL_RESULT_UNKNOWN`: do not create a replacement transaction; use GET/reconciliation and vendor lookup.
- `FISCAL_DEVICE_UNREACHABLE`: sale stays blocked even when BLE transport itself is connected.
- `PAYMENT_TERMINAL_UNAVAILABLE`: CARD stays failed; cash is a separate explicit attempt.
- Sync gap/signature/fencing error: quarantine the batch, preserve the journal and compare last signed ACK/cursor.
- Closed shift without valid Z reference is a defect; do not force it closed in SQL.
- STUB/simulator observed in PROD is `S0` configuration compromise and must hard-fail.

## Escalation ownership

Route API/state/data defects to Fiscal or MiniPOS engineering, BLE/journal/OTA to Edge/IoT, terminal outcomes to the approved vendor/acquirer, identity failures to the organization IdP owner, and regulatory interpretation/evidence to Bulgarian compliance/legal. Hardware/vendor/legal gates cannot be waived by software support.
