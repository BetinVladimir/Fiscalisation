# HIL Test Runbook — MVP1

Hardware-in-the-Loop (HIL) tests validate fiscal unit behavior against physical devices.
HIL results count as `SOFTWARE_VERIFIED_HIL_PENDING` → `HIL_PASS` in `MVP_GATES.md`.
Never substitute emulator results for real HIL evidence.

---

## Prerequisites

### Hardware

| Device | Required firmware | Status |
|--------|------------------|--------|
| Datecs FP-700 (BlueCash BC-50) | PM v2.11.4+ | Required for HIL-BC-01..05 |
| Daisy Compact S/USB | Daisy v2.0-4+ | Required for HIL-DSY-01..03 |
| S3-compatible IoT module | edge-agent-s3 firmware | Required for HIL-S3-01..02 |

### Software

- `scripts/hil/run.sh` — scenario runner
- `contracts/hil-result.schema.json` — validates result.json
- USB serial adapter or Bluetooth dongle (for BlueCash)

### Environment variables

```bash
export HIL_DEVICE_HOST=192.168.1.100   # IP or serial port of device
export HIL_CONFIRM=yes                  # Safety gate; prevents accidental runs
```

---

## Test data (golden check)

Use the following golden fixture for all monetary assertions:

| Field | Value |
|-------|-------|
| Total | **1367 ¢** (= 1200 + 167) |
| Item 1 | 1200 ¢ (tax group А) |
| Item 2 | 167 ¢ (tax group В) |
| Payment — CASH | 500 ¢ |
| Payment — CARD | 867 ¢ |

Source: `docs/MVP1/fixtures/` — never derive values from the implementation.

---

## Running scenarios

```bash
# Dry-run (no hardware required — validates harness only)
bash scripts/hil/run.sh --dry-run HIL-BC-01

# Real run
bash scripts/hil/run.sh --device bc50 --scenario HIL-BC-01 \
  --out artifacts/hil/$(date +%Y-%m-%d)/HIL-BC-01
```

Artifacts land in `artifacts/hil/<date>/`.
Each run writes a `<SCENARIO>.result.json` that must validate against
`contracts/hil-result.schema.json`.

---

## Scenario checklist

### HIL-BC-01 — BlueCash BC-50 golden receipt

1. [ ] Device connected and powered on
2. [ ] Operator password set (see `docs/runbooks/SECRET_ROTATION.md`)
3. [ ] Fiscal shift open
4. [ ] Run golden receipt (1367 ¢, CASH 500 + CARD 867)
5. [ ] Verify fiscal document number returned from FU (not synthetic)
6. [ ] Verify receipt printout format (CP1251 text, correct tax group lines)
7. [ ] Verify UNKNOWN path: simulate timeout, confirm no second attempt
8. [ ] Record result in `artifacts/hil/<date>/HIL-BC-01.result.json`

### HIL-BC-02 — BlueCash BC-50 storno

1. [ ] Fiscal receipt from HIL-BC-01 completed
2. [ ] Issue storno via cmd43
3. [ ] Confirm storno documented before card refund (invariant 3)
4. [ ] Verify card refund only after confirmed fiscal storno

### HIL-DSY-01 — Daisy Compact USB golden receipt

Steps mirror HIL-BC-01, adapted for Daisy protocol (SEQ 0x20..0xFF, LEN=data+0x24).

---

## Result protocol template

```json
{
  "status": "PASS",
  "scenario": "HIL-BC-01",
  "timestamp": "2026-10-28T12:00:00Z",
  "device": "FP-700-SN-12345",
  "dry_run": false,
  "steps": [
    {"name": "golden-receipt", "status": "PASS"},
    {"name": "fiscal-doc-number-from-fu", "status": "PASS"},
    {"name": "unknown-path-no-retry", "status": "PASS"}
  ],
  "notes": "All steps passed; document number 000142 from FU"
}
```

---

## Gate transitions

| Gate state | Meaning |
|-----------|---------|
| `NOT_STARTED` | No software implementation |
| `SOFTWARE_VERIFIED_HIL_PENDING` | Software tests pass; awaiting HIL hardware run |
| `HIL_PASS` | Real hardware HIL completed with PASS result |
| `OUT_OF_MVP_SCOPE` | Feature excluded from MVP1 |

Gates are recorded in `MVP_GATES.md`. Only a real HIL run produces `HIL_PASS`.
