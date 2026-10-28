# MVP Gates — BeeFiscal MVP1

Status values: `NOT_STARTED` | `SOFTWARE_VERIFIED_HIL_PENDING` | `HIL_PASS` | `OUT_OF_MVP_SCOPE`

Only a real HIL run against physical hardware produces `HIL_PASS`.
Never set `HIL_PASS` based on emulator or simulator results.

---

## §5 Paths from ARCH_17 DELIVERY_PIPELINE

### Path 1 — BlueCash BC-50 (Datecs FP-700) fiscal sale

| Scenario | Gate | Notes |
|----------|------|-------|
| HIL-BC-01: golden receipt 1367¢ | `SOFTWARE_VERIFIED_HIL_PENDING` | VG-11, VG-12 complete |
| HIL-BC-02: storno after sale | `SOFTWARE_VERIFIED_HIL_PENDING` | VG-04 complete |
| HIL-BC-03: shift open/close | `SOFTWARE_VERIFIED_HIL_PENDING` | VG-12 complete |
| HIL-BC-04: clock drift WARN/BLOCK | `SOFTWARE_VERIFIED_HIL_PENDING` | VG-11 complete |
| HIL-BC-05: UNKNOWN path (no retry) | `SOFTWARE_VERIFIED_HIL_PENDING` | TG-010 complete |

### Path 2 — Daisy Compact USB fiscal sale

| Scenario | Gate | Notes |
|----------|------|-------|
| HIL-DSY-01: golden receipt 1367¢ | `SOFTWARE_VERIFIED_HIL_PENDING` | DSY-04..08 complete |
| HIL-DSY-02: storno | `SOFTWARE_VERIFIED_HIL_PENDING` | DSY-05 complete |
| HIL-DSY-03: UNKNOWN path | `SOFTWARE_VERIFIED_HIL_PENDING` | |

### Path 3 — IoT S3 edge agent

| Scenario | Gate | Notes |
|----------|------|-------|
| HIL-S3-01: profile execute | `SOFTWARE_VERIFIED_HIL_PENDING` | S3FIX-01..07 pending |
| HIL-S3-02: durable storage | `SOFTWARE_VERIFIED_HIL_PENDING` | S3FIX-01 complete |

### Path 4 — Daisy Smart S (Android/HTTP)

| Scenario | Gate | Notes |
|----------|------|-------|
| HIL-SMS-01: golden receipt | `NOT_STARTED` | SMS-02..09 pending |

### Path 5 — Electronic receipts

| Scenario | Gate | Notes |
|----------|------|-------|
| E-receipt | `OUT_OF_MVP_SCOPE` | Disabled by owner decision (TG-008); delivery not implemented |

### Path 6 — Time synchronisation (Annex 29)

| Scenario | Gate | Notes |
|----------|------|-------|
| HIL-CLOCK-01: FU clock read, drift check | `SOFTWARE_VERIFIED_HIL_PENDING` | VG-11 complete |

---

## Cross-cutting requirements

| Requirement | Gate |
|-------------|------|
| HMAC key rotation runbook | `SOFTWARE_VERIFIED_HIL_PENDING` |
| MQTT TLS-only enforced | `SOFTWARE_VERIFIED_HIL_PENDING` |
| Secrets not in repo (`check-no-secrets.sh`) | `SOFTWARE_VERIFIED_HIL_PENDING` |
| Contract lock for all components | `SOFTWARE_VERIFIED_HIL_PENDING` |
