# ADR-FISCAL-CARD-PATH-001 — Card Payment Path: Datecs Pay SDK (internal-terminal)

**Status:** DECIDED  
**Date:** 2026-10-28  
**Decision ID:** D-8  

## Context

BlueCash BC-50 supports two paths for card payment:

1. **device-pinpad** — BluePad-50 (Plus) connected directly to the BC-50's COM/USB port; the BC-50 drives the pinpad via cmd 53 (`PaidMode=2, Type=1|12`) and cmd 55; error −111560 means "pinpad success, fiscal unit failure".
2. **internal-terminal** — Datecs Pay SDK runs on the Android POS device and communicates with the BluePad-50 over BLE/SPP directly; the SDK returns an authorisation reference which is then recorded in the BC-50 via cmd 48/53 as a CARD tender; a compensation saga reverses the card if the fiscal step fails.

The current implementation uses path 2 (`card.purchaseEur(…)` in `BlueCashEngine.kt`).

## Decision

**DECIDED: internal-terminal** — keep the existing Datecs Pay SDK path.

Rationale:
- The device-pinpad path requires DOC-V6 (merchant credentials, bank certification) which is an external blocker; it also requires new BLOCKED_SPEC cards.
- The internal-terminal path is already implemented and tested.
- The compensation saga (storno-first, invariant 3) provides equivalent safety: card refund only after confirmed fiscal storno.
- The −111560 error code is only relevant for the device-pinpad path; it does not occur in the internal-terminal path.

## Consequences

- cmd 53 `Type=1|12` and cmd 55 are **not implemented** in MVP1.
- `BlueCashEngine.purchaseEur` continues to be the production card-payment entry point.
- HIL card tests require Datecs Pay SDK terminal + merchant credentials (DOC-V6).
- If the device-pinpad path is needed in a future release, VG-14 sub-cards should be created with status BLOCKED_SPEC until DOC-V6 is obtained.

## References

- ARCH_09 §3 VG-14
- MASTER_PLAN.md D-8
- BlueCash BC-50 PM v2.11.4 §4.15.2 (cmd 53), §4.17 (cmd 55)
