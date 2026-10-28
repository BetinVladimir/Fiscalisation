# Secret Rotation Runbook

## Overview

This runbook covers rotating secrets used by the BeeFiscal fiscal backend.
Never commit actual secret values; reference environment variable names only.

---

## 1. HMAC Signing Key Rotation

The fiscal backend signs API responses and webhook payloads with an HMAC-SHA256 key
stored in `AUTH_HMAC_KEY` (configured in `fiscal-backend/internal/config/config.go`).

### Steps

1. **Generate a new key** (32 bytes, base64-encoded):
   ```bash
   openssl rand -base64 32
   ```

2. **Deploy with dual-key support** — add the new key as `AUTH_HMAC_KEY_NEW` while keeping
   the old key in `AUTH_HMAC_KEY`. The middleware accepts both during the transition window.

3. **Roll all clients** to present tokens signed with the new key.

4. **Promote the new key**: set `AUTH_HMAC_KEY` to the new value, remove `AUTH_HMAC_KEY_NEW`.

5. **Verify** by checking the `/readyz` endpoint returns HTTP 200 and confirming
   MQTT broker reconnects successfully.

6. **Revoke the old key** after 24 h with zero errors in the logs.

### Rollback

Restore the previous value of `AUTH_HMAC_KEY` from the secret manager and redeploy.

---

## 2. Fiscal Unit (ФУ) Operator Password Rotation

The fiscal unit password is never stored in the repository. It is injected at runtime
as `FU_OPERATOR_PASSWORD` (see `docs/MVP1/decisions/ADR-BLUEPAD-001.md`).

### Steps

1. **Pause fiscal operations** via the operator console or set the device to maintenance mode.

2. **Change the password on the device** using the vendor-specific procedure:
   - Datecs: command 0x61 (decimal 97) — see Datecs PM §4.3.
   - Daisy: command 0x61 — see Daisy integration manual §3.2.

3. **Update the secret** in the secret manager (e.g., Vault, AWS Secrets Manager):
   ```
   secret set fiscal/fu_operator_password = <new_value>
   ```

4. **Redeploy** the affected service so it picks up the new value from the environment.

5. **Resume fiscal operations** and verify the first sale succeeds (golden check: 1367 ¢).

6. **Confirm** no `FISCAL_OPERATOR_AUTH_FAILED` errors appear in logs within 5 minutes.

### Important invariants

- The password MUST NOT appear in logs, test output, or git history (invariant 8).
- The UNKNOWN state after a mid-rotation operation must be treated as UNKNOWN, not failed
  (invariant 2). Do not replay or issue refunds until the outcome is confirmed.

---

## 3. EAS / MQTT TLS Certificate Rotation

1. Generate a new mTLS client certificate for the edge agent.
2. Upload to the EMQX ACL table alongside the expiring certificate.
3. Redeploy edge agents one by one.
4. Revoke the old certificate after all agents reconnect.

---

## See Also

- `scripts/check-no-secrets.sh` — CI check that no secrets are committed.
- `scripts/check-mqtt-tls.sh` — CI check that MQTT is TLS-only.
- `docs/MVP1/decisions/ADR-BLUEPAD-001.md` — BluePad secret handling decision.
