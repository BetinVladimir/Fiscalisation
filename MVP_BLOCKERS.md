# MVP implementation blockers and formal exclusions

This document records decisions that cannot be implemented safely from the locked contracts or without external vendor/hardware evidence. A blocker is not counted as implemented and must not be replaced with a simulator PASS.

## P0-AUTH-001 — MiniPOS passwordless operator session contract

**Status:** software enforcement implemented; external IdP/passwordless deployment evidence remains a production blocker.

**Requirement:** passwordless login, a cashier session uniquely bound to one employee, no shared cashier account, and auditable login/logout.

**Missing authoritative decisions:**

1. Locked `openapi-public-v1.yaml` still lacks these operations; the implemented additive surface is therefore explicitly versioned in `contracts/openapi-runtime-v1.yaml` rather than hidden from OpenAPI.
2. Runtime OpenAPI now defines immutable ADMIN-only issuer+subject binding, current operator session and durable logout/revoke. The subject is SHA-256-bound with tenant+issuer and never returned; only a non-secret access-token fingerprint is persisted.
3. PROD UI uses OIDC Authorization Code + PKCE and ignores `EXPO_PUBLIC_MINIPOS_AUTH_TOKEN`; the backend requires issuer/audience/JWKS validation, one active employee binding, active non-expired session and matching `X-App-Instance-Id` before employee actions.
4. PostgreSQL typed/RLS tables preserve binding, first session observation and revocation across restart. Executable API/domain/PostgreSQL/browser tests prove cross-employee, cross-app-instance, anonymous PROD and post-logout denial.

**Remaining external resolution:** select and configure the organization IdP to enforce passwordless/passkey authentication, register the Android/iOS/Web redirect URIs and client, set the accepted issuer/audience/JWKS, and provide real login/expiry/IdP-revoke evidence. The local software does not invent passwords, OTP delivery or passkey credentials and cannot prove the policy of an unselected IdP.

**Implemented acceptance evidence:** generated clients for all runtime operations; ADMIN-only/shared-account rejection; cross-employee/cross-tenant/app-instance rejection; expiry and durable logout/restart tests; typed PostgreSQL/RLS evidence; Android/iOS/Web builds; real browser PROD fail-closed test. Still required externally: a real passwordless IdP login and IdP-initiated revoke/expiry run on target devices.

**Forbidden workaround:** a local PIN, hard-coded OTP, employee picker presented as login, shared bearer token, or DEV response that exposes an OTP in PROD.
## P0-BLUECASH-043 — Vendor conflict for physical storno

The supplied BlueCash fiscal demo executes `open_StornoReceiptAsync`, but `PM_XXXXXX-BUL_CommunicationProtocol_v2.11.4 (7).pdf` states below command 43 that it is not used on BC-50. Obtain Datecs confirmation of the supported in-device Android SDK/wire path and run physical CASH/CARD storno HIL (success, decline, timeout, crash after card reversal, crash after fiscal close) before PROD. The implemented software contour remains fail-closed and must not waive this gate.
