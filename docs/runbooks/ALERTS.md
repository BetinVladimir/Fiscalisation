# Alert Runbook — BeeFiscal Fiscal Backend

## Overview

Alerts fire when `/metrics` counters cross defined thresholds.
Each alert has a runbook entry below.

---

## Alert: `FiscalUnknownRateCritical`

**Condition**: `rate(fiscal_unknown_total[5m]) > 0.1`
**Severity**: critical
**Meaning**: More than 6 UNKNOWN fiscal operations per minute — likely a device timeout loop or network partition.

### Response

1. Check `/readyz` — is the repository reachable?
2. Check MQTT connectivity to the fiscal device.
3. Inspect the last 5 operations: `GET /public/v1/operations?limit=5`.
4. **Do NOT retry automatically** — UNKNOWN ≠ FAILED; the operation may have been committed by the device. See invariant 2 in AGENTS.md.
5. If all operations are stuck in EXECUTING, the reconciler (TG-010) should have triggered; check its logs.
6. Escalate to on-call if unresolved after 10 min.

---

## Alert: `FiscalOperationFailureRateHigh`

**Condition**: `rate(fiscal_operations_total{state="FAILED"}[5m]) > 0.5`
**Severity**: warning
**Meaning**: More than 30 FAILED fiscal operations per minute.

### Response

1. Check device connectivity.
2. Review error codes in recent operations.
3. If `FISCAL_DEVICE_UNAVAILABLE`: check device power and USB/BT connection.
4. If `OPERATOR_AUTH_FAILED`: rotate the FU operator password (see `SECRET_ROTATION.md`).

---

## Alert: `MQTTCommandLatencyHigh`

**Condition**: `mqtt_command_latency_seconds > 5`
**Severity**: warning
**Meaning**: MQTT command round-trips taking more than 5 seconds.

### Response

1. Check MQTT broker metrics (EMQX console).
2. Check edge agent logs for retries.
3. If latency is 6–9 s: normal retry behaviour; monitor.
4. If latency is ≥ 10 s: check serial device responsiveness and restart edge agent.

---

## Alert: `RepositoryUnavailable`

**Condition**: `/readyz` returns non-200 for > 30 s
**Severity**: critical
**Meaning**: Storage is unavailable — fiscal operations cannot proceed.

### Response

1. `GET /readyz` — note which storage backend fails.
2. Check database/Redis health.
3. **All in-flight operations will be stuck in EXECUTING** — do not replay them until storage recovers.
4. After recovery, the reconciler will resume pending operations.
5. Page the on-call DBA if storage is unreachable for > 2 min.

---

## Metrics endpoint

`GET /metrics` — returns Prometheus text format.

Exposed metrics:
- `fiscal_operations_total{state}` — total operations by final state
- `fiscal_unknown_total` — total UNKNOWN-outcome operations
- `mqtt_command_latency_seconds` — average MQTT round-trip latency in seconds
