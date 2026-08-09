# Operations runbook

## Scope and entry points

Fiscal and MiniPOS are independent products. Their only HTTP(S) entry points are their respective Caddy services. MiniPOS may call Fiscal only through `FISCAL_PUBLIC_BASE_URL`; database sharing and internal-service calls are prohibited.

## DEV startup

```sh
cp .env.example .env
docker compose -p beefiscal-dev -f compose.fiscalisation.yaml -f compose.fiscalisation.dev.yaml up -d --build
docker compose -p beeminipos-dev -f compose.minipos.yaml -f compose.minipos.dev.yaml up -d --build
```

Verify `/health/live`, `/health/ready` and `/public/v1/version` through each Caddy endpoint. A healthy cloud route does not imply a reachable final fiscal device; use the public readiness/connectivity APIs.

## PROD prerequisites

- HTTPS site names and Caddy certificate storage are durable.
- `FISCAL_PUBLIC_BASE_URL`, `FISCAL_CORS_ALLOWED_ORIGINS` and `MINIPOS_CORS_ALLOWED_ORIGINS` are explicit HTTPS values routed through the corresponding Caddy entry points; wildcard or URL-with-path CORS entries fail startup.
- OIDC issuer/audience/JWKS and MiniPOS OAuth client credentials are supplied from a secret manager.
- BeeMiniPOS and BeeFiscalApp have separate registered OIDC clients/redirect URIs (`beeminipos://oauth/callback`, `beefiscalapp://oauth/callback`); both production bundles ignore `EXPO_PUBLIC_*_AUTH_TOKEN` static credentials.
- Both public clients request refresh authority. Access and rotating refresh tokens remain memory-only; refresh is scheduled before provider expiry. If no refresh token is issued, the token expires at its declared deadline; if refresh fails or the provider omits/malforms lifetime metadata, authority is cleared immediately and the UI requires a new personal login. Configure the IdP access-token lifetime and refresh-token rotation/reuse policy accordingly; never add browser/mobile client secrets.
- Any production `401` from either public API invalidates the interactive session instead of allowing repeated calls with stale authority. In MiniPOS, personal-authority loss clears BLE READY immediately and performs a best-effort revoke using the previous in-memory bearer before discarding it; the cashier must log in and explicitly activate a new BLE session.
- Writer and FORCE-RLS reader database identities are different.
- BLE and webhook keys contain at least 32 bytes and are supplied from the secret manager; DEV HMAC/static tokens and all simulator flags are absent.
- Both public backends retain bounded header/read/write/idle HTTP timeouts behind Caddy; do not replace the configured server with an unbounded default server.
- A signed, independently trusted release package and approved vulnerability report are verified.
- Hardware/vendor/legal gates in `MVP_GATES.md` are closed for the selected track.

Fail startup or deployment when any prerequisite is missing. Do not temporarily enable a STUB in PROD.

## Backup and restore

Back up Fiscal and MiniPOS PostgreSQL independently with custom-format `pg_dump`. Record image digests, migration versions, backup SHA-256 and UTC timestamps. Test restore into new databases with `pg_restore --exit-on-error`; verify typed runtime row counts, RLS identities and representative immutable artifact hashes before switching traffic.

Pilot target RTO is under 120 seconds for the tested dataset. An untested or partial restore is not a recovery.

## Incident actions

- Final fiscal device unreachable: block sales; do not reinterpret the condition as a cloud-only outage.
- Cloud route lost but valid BLE authority and live final device remain: freeze the route for that command, execute locally once, journal before device I/O and sync after recovery.
- Timeout after send or lost response: mark `UNKNOWN`; reconcile by operation/payment references and never blind-retry.
- Payment terminal unavailable: reject CARD without cash fallback. A cashier must explicitly choose a new CASH attempt with a new payment ID.
- Edge storage at 95% or above: block new fiscal execution, preserve unacknowledged/young records and restore capacity.
- Suspected credential/key compromise: revoke sessions/keys, rotate through the documented overlap window and preserve audit evidence.

## Routine checks

Monitor cloud transport and final-device reachability as separate metrics, outbox age/retries, UNKNOWN operations, Edge cursor/gaps/storage, open shifts, failed Z reports, certificate/key expiry, backup freshness and release evidence state.

For `BLOCKED_RECONCILIATION` shift close, do not submit another close/Z command. The operator-owned shift exposes only `READ` and `RECONCILE`; use `POST /minipos/shifts/{shift_id}/reconcile`. MiniPOS performs GET-only lookup of the persisted Fiscal operation. Only when no operation ID was received may it replay the original report request with the exact deterministic idempotency key. Sales and opening a replacement shift remain blocked until a final `FISCALIZED` operation with a fiscal reference closes the shift.

Cashier access is deliberately narrower than tenant membership. `CASHIER` may read catalog/configuration and only orders/shifts belonging to the employee resolved from the active OIDC app session. Employee directory and tenant-wide sales reports require `SUPERVISOR`, `ADMIN` or read-only `AUDITOR`; object editors require `SUPERVISOR`/`ADMIN`. A cashier UI must not request `/employees` or render management controls.
