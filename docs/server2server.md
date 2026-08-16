# Server-to-server trust and synchronization plan

Status: implemented on 2026-08-16. This document is the normative architecture and rollout contract; the implementation lives in `fiscal-backend`, `beeminipos-backend`, `AdminApp`, `BeeFiscalApp`, and `docs/integration-kit`.

## 1. Objective

Build an explicit trust relationship between `fiscal-backend` and external source systems such as BeeMiniPOS without assuming shared JWT keys, shared databases, or implicit tenant identity.

The design must provide:

- platform-administered external systems;
- immediate rotation of a system bootstrap key;
- synchronous, email-confirmed tenant enrollment;
- a durable binding between a Fiscal tenant, the source system, and the source company's ID;
- a tenant-scoped credential for subsequent server-to-server API calls;
- asynchronous processing of company, location, register, and operator changes;
- status callbacks through signed webhooks;
- PostgreSQL outbox/inbox guarantees around RabbitMQ;
- bounded webhook retries with a terminal dead state after five failed attempts;
- complete security and operational audit trails without storing plaintext secrets.

## 2. Current-state findings

The repository already contains useful building blocks:

- `fiscal-backend` has platform OIDC authorization and `/platform/v1` endpoints used by `AdminApp`;
- tenant resources already exist for organizations, locations, registers, operators, devices, and bindings;
- webhook endpoint validation already includes HTTPS and SSRF controls;
- Fiscal domain operations already create durable outbox events atomically with business changes;
- the current webhook dispatcher retries direct HTTP delivery;
- RabbitMQ is currently used for OTP email delivery through a direct MiniPOS publication to `beeloy.email.otp`; this is a legacy cross-service path and is marked for removal;
- `beeminipos-backend` onboarding currently creates only a MiniPOS-local company and generates an unverified `fiscal_external_id`;
- the current MiniPOS-to-Fiscal OAuth client is global to the process, while Fiscal derives the tenant exclusively from token claims.

The new design should replace the implicit global OAuth tenant relationship for source-system integration. OIDC remains the authorization mechanism for platform administrators and may remain available for human/user flows.

### 2.1 Deprecated OTP transport

The current flow is deprecated:

```text
beeminipos-backend generates OTP
  -> publishes directly to RabbitMQ queue beeloy.email.otp
  -> fiscal-backend email worker consumes it
  -> SMTP
```

It must be removed after the enrollment API rollout. Specifically, remove:

- OTP generation for Fiscal tenant enrollment from `beeminipos-backend`;
- the MiniPOS direct RabbitMQ publisher for `beeloy.email.otp`;
- the shared ownership of the OTP queue between MiniPOS and Fiscal;
- the Fiscal consumer dedicated to the legacy `beeloy.email.otp` contract once no other login flow depends on it;
- related configuration and runbook references after the migration drain is complete.

The same legacy queue currently also carries BeeMiniPOS login codes. Removing it must not disable user login. Before deleting the queue, move the login-email path to either:

- a MiniPOS-owned transactional email outbox and MiniPOS email worker; or
- a separate notification service with its own authenticated API and durable inbox.

Fiscal tenant enrollment OTP remains owned and verified by Fiscal. BeeMiniPOS login OTP remains owned and verified by MiniPOS. They must not share challenges, tokens, tables, or queue contracts.

The replacement is synchronous API orchestration:

```text
beeminipos-backend
  -> POST Fiscal enrollment start
  -> Fiscal creates challenge and email-outbox row atomically
  -> Fiscal-owned email worker sends through SMTP
  -> MiniPOS submits temporary token + code to Fiscal verification API
```

RabbitMQ remains part of asynchronous resource synchronization and webhook delivery, but it is not the trust boundary and is not used for direct MiniPOS-to-Fiscal OTP submission or OTP verification.

## 3. Trust model

Use three separate credential classes. They must not share the same secret value.

### 3.1 System bootstrap credential

Purpose: authorize only enrollment initiation for a registered external system.

Format returned once by AdminApp:

```text
sys_live_<credential_id>.<random_secret>
```

Fiscal stores only:

- credential ID;
- keyed hash of the secret;
- key version/fingerprint;
- creation and revocation timestamps.

Recommended hash: HMAC-SHA-256 with a server-side pepper stored in the secret manager. Comparison must be constant-time. A plain SHA-256 hash is acceptable only if the secret has at least 256 bits of entropy, but a pepper is preferred.

Rotating this key is immediately destructive: the old credential becomes invalid in the same transaction that creates the new credential. There is no grace period for the bootstrap key.

### 3.2 Enrollment temporary token

Purpose: complete one specific email challenge. It cannot call business APIs.

Recommended format: opaque random 256-bit token. Fiscal stores only its hash. It is:

- bound to `external_system_id`;
- bound to normalized email;
- bound to `source_company_id`;
- bound to one enrollment request;
- valid for 10 minutes;
- single-use;
- invalid after five incorrect code attempts;
- invalidated when successfully consumed.

### 3.3 Tenant integration credential

Purpose: authorize future source-system API operations for exactly one Fiscal tenant and exactly one external system.

Format returned once after successful verification:

```text
tenant_live_<credential_id>.<random_secret>
```

The database stores only the credential ID and secret hash. Authentication resolves:

```text
credential -> external_system_id + tenant_id + source_company_id + scopes + status
```

Suggested initial scopes:

```text
organization.write
locations.write
registers.write
operators.write
operations.read
```

Do not reuse this credential to sign webhooks. Webhook signing material must be separately generated or derived with HKDF using a distinct context.

## 4. External-system secret storage boundary

The MiniPOS `.env` contains only the global BeeMiniPOS external-system bootstrap credential. This credential identifies the BeeMiniPOS source system and is used only to start company enrollment. It is not a tenant credential and cannot modify tenant resources.

The MiniPOS backend is multi-tenant. Tenant integration credentials are stored per company in the MiniPOS database using an encrypted credential store:

- ciphertext in MiniPOS PostgreSQL;
- envelope encryption through KMS/Vault/secret manager;
- encryption key never stored in the same database;
- credential never returned by read APIs or written to logs.

Required MiniPOS variables:

```text
FISCAL_SYSTEM_TOKEN=...          # BeeMiniPOS system bootstrap key only
FISCAL_CREDENTIAL_KEK_ID=...     # KMS/Vault key reference
FISCAL_PUBLIC_BASE_URL=...
FISCAL_SOURCE_SYSTEM_ID=...      # optional diagnostic assertion, not authority
```

Tenant credentials must never be placed in `.env`, including single-tenant deployments. This keeps one credential lifecycle and rotation model for every installation.

Every future MiniPOS-like external system follows the same boundary: one bootstrap credential in its deployment secret manager and encrypted per-company tenant credentials in its database or equivalent tenant-scoped secret store. Fiscal does not require or expose RabbitMQ credentials to external systems.

## 5. Fiscal database model

Prefer typed PostgreSQL tables rather than putting credentials into the generic JSON resource store.

### 5.1 `external_systems`

```text
id uuid primary key
code text unique not null
display_name text not null
status ACTIVE | SUSPENDED | REVOKED
webhook_url text not null
webhook_events text[] not null
webhook_signing_secret_ciphertext bytea not null
version bigint not null
created_at timestamptz not null
updated_at timestamptz not null
created_by text not null
updated_by text not null
```

The URL must reuse existing HTTPS/SSRF validation. In production, private, loopback, link-local, credential-bearing, and fragment URLs are rejected. DNS rebinding protection must remain in the HTTP transport.

### 5.2 `external_system_credentials`

```text
credential_id uuid primary key
external_system_id uuid not null
secret_hash bytea not null
key_fingerprint text not null
version bigint not null
status ACTIVE | REVOKED
created_at timestamptz not null
created_by text not null
revoked_at timestamptz
revoked_by text
revoke_reason text
last_used_at timestamptz
```

Constraint: one active bootstrap credential per external system. Rotation uses a transaction and a partial unique index.

### 5.3 `external_system_audit_log`

Append-only journal:

```text
id uuid primary key
external_system_id uuid not null
action CREATED | UPDATED | SUSPENDED | RESUMED | KEY_ROTATED | KEY_REVOKED
actor_subject text not null
request_id text not null
before_redacted jsonb
after_redacted jsonb
occurred_at timestamptz not null
```

Secrets, secret hashes, OTP codes, temporary tokens, and tenant access tokens must never appear in this table.

### 5.4 `external_enrollment_challenges`

```text
id uuid primary key
external_system_id uuid not null
source_company_id text not null
normalized_email text not null
temporary_token_hash bytea unique not null
otp_hash bytea not null
payload_hash bytea not null
legal_profile jsonb not null
attempts integer not null default 0
status PENDING | VERIFIED | EXPIRED | LOCKED | CANCELLED
expires_at timestamptz not null
created_at timestamptz not null
verified_at timestamptz
tenant_id text
```

Unique active constraint on `(external_system_id, source_company_id)` prevents parallel duplicate enrollment.

### 5.5 `tenant_source_bindings`

```text
id uuid primary key
tenant_id text not null
external_system_id uuid not null
source_company_id text not null
source_metadata jsonb
status ACTIVE | SUSPENDED | REVOKED
version bigint not null
created_at timestamptz not null
updated_at timestamptz not null
unique(external_system_id, source_company_id)
unique(tenant_id, external_system_id)
```

Every imported resource records source provenance:

```text
source_system_id
source_entity_id
source_version
```

Use a unique constraint per resource kind on `(tenant_id, source_system_id, source_entity_id)`.

### 5.6 `tenant_integration_credentials`

```text
credential_id uuid primary key
binding_id uuid not null
secret_hash bytea not null
scopes text[] not null
status ACTIVE | REVOKED
created_at timestamptz not null
expires_at timestamptz
last_used_at timestamptz
revoked_at timestamptz
```

Support rotation from the beginning even if the first UI does not expose it.

### 5.7 `integration_commands`

Authoritative inbox for asynchronous source changes:

```text
id uuid primary key
tenant_id text not null
external_system_id uuid not null
idempotency_key text not null
command_type text not null
aggregate_type text not null
aggregate_source_id text not null
source_version bigint not null
payload jsonb not null
payload_hash bytea not null
authenticated_system_id uuid not null
asserted_actor_type USER | SERVICE not null
asserted_actor_id text not null
asserted_actor_session_id text
status ACCEPTED | QUEUED | PROCESSING | SUCCEEDED | FAILED | DEAD
attempts integer not null default 0
last_error_code text
last_error_detail text
created_at timestamptz not null
updated_at timestamptz not null
unique(external_system_id, tenant_id, idempotency_key)
```

`authenticated_system_id` is established only from the validated tenant credential and must equal `external_system_id`. `asserted_actor_*` is supplied by that authenticated external system. It provides source-side accountability but is not represented as a user independently authenticated by Fiscal.

### 5.8 `integration_command_outbox`

Transactional RabbitMQ publishing table:

```text
id uuid primary key
command_id uuid unique not null
topic text not null
payload jsonb not null
status PENDING | LEASED | PUBLISHED | FAILED
attempts integer not null default 0
available_at timestamptz not null
lease_id uuid
lease_until timestamptz
published_at timestamptz
last_error text
created_at timestamptz not null
updated_at timestamptz not null
```

### 5.9 `webhook_deliveries`

One row per external-system delivery, not one mutable row shared by multiple destinations:

```text
id uuid primary key
event_id text not null
external_system_id uuid not null
tenant_id text not null
event_type text not null
payload jsonb not null
payload_hash bytea not null
status PENDING | LEASED | QUEUED | DELIVERING | DELIVERED | RETRY | DEAD
attempts integer not null default 0
next_attempt_at timestamptz not null
lease_id uuid
lease_until timestamptz
last_http_status integer
last_error_code text
last_error_detail text
delivered_at timestamptz
created_at timestamptz not null
updated_at timestamptz not null
unique(event_id, external_system_id)
```

### 5.10 `integration_change_journal`

Append-only attribution and data-change journal:

```text
id uuid primary key
tenant_id text not null
external_system_id uuid
authenticated_system_id uuid
asserted_actor_type USER | SERVICE
asserted_actor_id text
asserted_actor_session_id text
fiscal_platform_actor_subject text
operation_id uuid
idempotency_key text
resource_type text not null
source_entity_id text
action text not null
outcome ACCEPTED | APPLIED | REJECTED | SUPERSEDED | MANUAL_DECISION
before_redacted jsonb
after_redacted jsonb
change_diff_redacted jsonb
reason_code text
occurred_at timestamptz not null
```

For an integration mutation, `authenticated_system_id` and asserted actor fields are mandatory and `fiscal_platform_actor_subject` is null. For a direct AdminApp/platform action, `fiscal_platform_actor_subject` is mandatory; source assertion fields are null unless the action explicitly reviews an integration request. Database permissions deny application roles `UPDATE` and `DELETE` on this table. Corrections are appended as new linked events, never applied by overwriting history.

## 6. Platform Admin API and AdminApp

All management endpoints remain protected by platform OIDC and require `PLATFORM_SECURITY_ADMIN` or a new `PLATFORM_INTEGRATION_ADMIN` role.

Proposed endpoints:

```text
GET    /platform/v1/external-systems
POST   /platform/v1/external-systems
GET    /platform/v1/external-systems/{id}
PATCH  /platform/v1/external-systems/{id}
POST   /platform/v1/external-systems/{id}:rotate-key
POST   /platform/v1/external-systems/{id}:suspend
POST   /platform/v1/external-systems/{id}:resume
GET    /platform/v1/external-systems/{id}/audit-events
GET    /platform/v1/external-systems/{id}/tenant-bindings
GET    /platform/v1/external-systems/{id}/webhook-deliveries
POST   /platform/v1/webhook-deliveries/{id}:requeue
```

Mutation requirements:

- `Idempotency-Key`;
- optimistic `version` or `If-Match`;
- platform actor subject in audit;
- key displayed exactly once after creation/rotation;
- explicit confirmation dialog for rotation;
- old key revoked in the same transaction;
- no key included in list/detail responses.

AdminApp screens:

1. External systems list with status and last activity.
2. Create/edit form: code, name, callback URL, subscribed event types.
3. Secret reveal-once modal with copy/download acknowledgement.
4. Immediate key rotation action.
5. Append-only system audit journal.
6. Tenant binding journal.
7. Webhook delivery journal with attempts, last status/error, next attempt, and manual requeue for `DEAD` rows.

## 7. Synchronous enrollment API

Use a dedicated API namespace which does not pass through the normal tenant JWT middleware.

### 7.1 Start enrollment

```http
POST /integration/v1/enrollments
Authorization: Bearer sys_live_<credential_id>.<secret>
Idempotency-Key: <source generated UUID>
Content-Type: application/json
```

Recommended request:

```json
{
  "email": "owner@example.com",
  "source_company_id": "minipos-company-uuid",
  "company": {
    "legal_name": "Example Ltd",
    "tax_identifier": {
      "country": "BG",
      "type": "EIK",
      "value": "123456789"
    },
    "address": "Sofia"
  }
}
```

`source_company_id` is mandatory. Email alone is not a stable company identity and cannot prevent duplicate tenants. Tax identity uses the universal tuple `(country, type, normalized_value)`, where `country` is ISO 3166-1 alpha-2 and `type` is a documented country-specific identifier type such as `EIK`, `VAT`, `TIN`, or another supported registry identifier. Fiscal owns a versioned normalization/validation policy per country and type. The normalized tuple is the Fiscal-wide legal-company identity used to prevent duplicate tenant activation; clients must send the original value and must not invent their own normalization rules.

Processing, synchronously and transactionally:

1. Parse credential ID and lookup active external system.
2. Constant-time validate system secret.
3. Apply rate limits per system, email, source company, and IP.
4. Normalize email and validate company payload.
5. Detect an existing active binding and check the normalized country + tax identifier against all existing tenants.
6. Generate six-digit OTP and opaque temporary token.
7. Store only hashes and bind them to system + source company + payload hash.
8. Insert a Fiscal-owned email outbox record in the same transaction.
9. Return the temporary token and expiry.

Response:

```json
{
  "temporary_token": "...",
  "expires_at": "...",
  "resend_after": "..."
}
```

The API response must not disclose whether the email already exists in another tenant. Logs must mask the email.

OTP delivery uses the Fiscal-owned transactional email outbox. A Fiscal worker claims email rows and sends them to SMTP. MiniPOS must not open RabbitMQ or publish an OTP message. The legacy `beeloy.email.otp` queue path is removed after migration.

### 7.2 Verify enrollment

```http
POST /integration/v1/enrollments:verify
Authorization: Bearer <temporary_token>
Idempotency-Key: <source generated UUID>
```

```json
{
  "code": "123456"
}
```

Processing in one serializable transaction or under advisory/row locks on both `(external_system_id, source_company_id)` and the normalized country + tax identifier:

1. Lock the challenge row.
2. Validate token, expiry, state, and attempt limit.
3. Constant-time validate OTP; increment attempts on failure.
4. Re-check that no source binding was created concurrently.
5. Revalidate the tax identifier under lock. If it is unused, allocate the canonical Fiscal tenant ID and allow automatic activation after successful OTP verification.
6. Create the Fiscal organization resource from the stored legal profile.
7. Create `tenant_source_bindings` with the source system provenance.
8. Create a tenant integration credential.
9. Mark the challenge consumed.
10. Commit and return the credential exactly once.

Response:

```json
{
  "tenant_id": "fiscal-tenant-uuid",
  "source_system_id": "external-system-uuid",
  "source_company_id": "minipos-company-uuid",
  "access_token": "tenant_live_<credential_id>.<secret>",
  "token_type": "Bearer",
  "scopes": ["organization.write", "locations.write", "registers.write", "operators.write"]
}
```

Every state-changing request contains a caller-generated idempotency key. Fiscal persists the key, request hash, HTTP status, and encrypted response atomically with the operation. Retrying with the same key and identical request returns the previous response without executing the operation again. Reuse of the key with a different request hash returns `409 Conflict`. Replay records containing credentials are encrypted with KMS/Vault, have a bounded retention period, and are never logged or stored as plaintext.

### 7.3 Proof limitation

Email OTP proves control of the mailbox. Tenant activation additionally requires a normalized tax identifier. If that identifier does not yet exist in Fiscal, successful OTP verification may automatically activate the tenant. The database must enforce uniqueness for the normalized country + tax identifier so concurrent enrollment cannot create duplicates.

If the tax identifier already exists, enrollment must not automatically create, merge, replace, or block either tenant. It creates a conflict case for manual review. An authorized Fiscal administrator decides which tenant/binding, if any, is blocked. The decision, actor, reason, before/after state, and related requests are written to the append-only audit journal.

## 8. MiniPOS changes after verification

MiniPOS stores:

- its current local company ID as `source_company_id`;
- returned Fiscal `tenant_id` in a dedicated `fiscal_tenant_id` column;
- `external_system_id`;
- tenant credential ID/fingerprint;
- encrypted tenant credential;
- integration status and last error;
- last successfully synchronized source versions.

Do not overload the existing local primary key or ambiguous `fiscal_external_id`. Use explicit columns and a unique constraint on `fiscal_tenant_id` when non-null.

Recommended MiniPOS-side typed tables:

### 8.1 `company_fiscal_bindings`

```text
id uuid primary key
company_id uuid not null references organizations(id)
external_system_id uuid not null
source_company_id uuid not null
fiscal_tenant_id text not null
status ENROLLMENT_PENDING | PROVISIONING | ACTIVE | DEGRADED | SUSPENDED
version bigint not null
last_synchronized_at timestamptz
last_error_code text
last_error_detail text
created_at timestamptz not null
updated_at timestamptz not null
unique(company_id)
unique(external_system_id, source_company_id)
unique(fiscal_tenant_id, external_system_id)
```

`company_id` remains the authoritative MiniPOS company identifier. `fiscal_tenant_id` is the authoritative external identifier assigned by Fiscal. `source_company_id` normally equals the MiniPOS company UUID, but it is mutable. Every change requires a transaction, uniqueness validation, binding-history entry, actor identity, reason, and before/after values; it must never be changed silently.

### 8.2 `company_fiscal_credentials`

```text
id uuid primary key
binding_id uuid not null references company_fiscal_bindings(id)
credential_id uuid not null
credential_fingerprint text not null
ciphertext bytea not null
encryption_key_id text not null
status ACTIVE | ROTATING | REVOKED
created_at timestamptz not null
rotated_at timestamptz
revoked_at timestamptz
unique(binding_id) where status = 'ACTIVE'
```

The access token plaintext exists only in memory during enrollment or an outbound request. It is encrypted before the enrollment transaction is considered complete and is never returned by MiniPOS read APIs.

If MiniPOS loses a tenant credential, it starts a credential-recovery flow using the same normalized email and tax identifier. Fiscal sends and verifies a new OTP, validates that the email is authorized for the existing tenant and that the tax identifier matches, issues a new tenant credential, revokes the lost credential atomically, and records the actor, reason, old credential ID, and new credential ID in the audit journal. Recovery never creates another tenant.

### 8.3 `company_fiscal_resource_links`

```text
id uuid primary key
binding_id uuid not null references company_fiscal_bindings(id)
resource_type ORGANIZATION | LOCATION | REGISTER | OPERATOR
source_entity_id text not null
fiscal_resource_id text
source_version bigint not null
fiscal_version bigint
sync_status PENDING | ACCEPTED | SUCCEEDED | FAILED | SUPERSEDED
last_operation_id uuid
last_error_code text
created_at timestamptz not null
updated_at timestamptz not null
unique(binding_id, resource_type, source_entity_id)
```

This table is the explicit source-to-Fiscal mapping and synchronization journal for company data, points, registers, and operators.

Suggested company integration states:

```text
NOT_LINKED
ENROLLMENT_PENDING
PROVISIONING
ACTIVE
DEGRADED
SUSPENDED
```

MiniPOS must not open a fiscal shift until the binding is `ACTIVE` and required Fiscal resources are confirmed.

## 8A. BeeFiscalApp email OTP authentication

`BeeFiscalApp` currently uses OIDC Authorization Code + PKCE through `src/adminOidc.ts`. Replace that application login flow with a Fiscal-owned email OTP flow.

This change applies to `BeeFiscalApp`, not `AdminApp`:

- `AdminApp` remains protected by platform OIDC and platform administration roles;
- `BeeFiscalApp` uses email OTP for registered tenant users;
- no public company onboarding, tenant creation, invitation acceptance, or self-registration endpoint is exposed through `BeeFiscalApp`.

### 8A.1 Registered users only

An email can authenticate only when it already has at least one active membership in a registered Fiscal tenant. The OTP flow must never create:

- a tenant;
- an organization;
- a user membership;
- an operator;
- an external-system binding.

Tenant and membership creation remains an authenticated administrative/integration process outside BeeFiscalApp login.

Before OTP verification the API must not disclose whether the email exists. The challenge-start response is generic for registered and unknown emails. For an unknown email, Fiscal may store a short-lived non-deliverable challenge/audit record but does not send a code and does not create an account.

### 8A.2 Membership model

Add a typed Fiscal table such as `tenant_user_memberships`:

```text
id uuid primary key
user_id uuid not null
tenant_id text not null
normalized_email text not null
display_name text
roles text[] not null
status ACTIVE | SUSPENDED | REVOKED
version bigint not null
created_at timestamptz not null
updated_at timestamptz not null
revoked_at timestamptz
unique(tenant_id, normalized_email)
unique(tenant_id, user_id)
```

One logical user may therefore have multiple active tenant memberships and different roles in each tenant.

Add `app_auth_challenges`:

```text
id uuid primary key
normalized_email text not null
temporary_token_hash bytea unique not null
otp_hash bytea
attempts integer not null default 0
status PENDING | VERIFIED | EXPIRED | LOCKED | CANCELLED | UNKNOWN_EMAIL
expires_at timestamptz not null
created_at timestamptz not null
verified_at timestamptz
request_ip_hash bytea
app_instance_id uuid
```

Add `app_auth_sessions` and hashed refresh credentials:

```text
id uuid primary key
user_id uuid not null
selected_tenant_id text
refresh_token_hash bytea unique not null
app_instance_id uuid not null
status ACTIVE | REVOKED | EXPIRED
created_at timestamptz not null
expires_at timestamptz not null
last_used_at timestamptz
revoked_at timestamptz
```

Add a server-side issued-token registry (or equivalent session-token-version rows) containing the token `jti`, session ID, selected tenant, issued/expiry timestamps, and revocation state. Every BeeFiscalApp access-token authorization checks this state through the database or a coherently invalidated cache. Logout, tenant switch, membership revocation, and administrative revocation invalidate the affected access-token IDs immediately.

### 8A.3 OTP challenge start

```http
POST /public/v1/app-auth/challenges
Content-Type: application/json
```

```json
{
  "email": "user@example.com",
  "language": "bg",
  "app_instance_id": "uuid"
}
```

Behavior:

1. Normalize and validate email.
2. Rate-limit by email, IP, and app instance.
3. Check active memberships internally without exposing the result.
4. Generate an opaque temporary token for every accepted request.
5. Generate and store an OTP hash only when at least one active membership exists.
6. Insert a Fiscal-owned transactional email-outbox row only for a registered email.
7. Return the same `202` response shape in both cases.

```json
{
  "temporary_token": "opaque-token",
  "expires_at": "...",
  "resend_after": "..."
}
```

BeeFiscalApp does not publish OTP messages directly to RabbitMQ. OTP email delivery uses the Fiscal-owned email outbox/worker described earlier.

### 8A.4 OTP verification and tenant discovery

```http
POST /public/v1/app-auth/challenges:verify
Authorization: Bearer <temporary_token>
Content-Type: application/json
```

```json
{
  "code": "123456",
  "app_instance_id": "uuid"
}
```

After successful verification, Fiscal resolves all active memberships for the normalized email.

If exactly one tenant is available, Fiscal may immediately create a session and return tenant-scoped access and refresh tokens.

If several tenants are available, Fiscal returns a short-lived, one-time tenant-selection token and a minimal tenant list:

```json
{
  "selection_required": true,
  "tenant_selection_token": "opaque-one-time-token",
  "expires_at": "...",
  "tenants": [
    {
      "tenant_id": "...",
      "display_name": "Company A",
      "roles": ["ADMIN"]
    },
    {
      "tenant_id": "...",
      "display_name": "Company B",
      "roles": ["AUDITOR"]
    }
  ]
}
```

The tenant list is disclosed only after successful OTP verification. It must not contain tax identifiers, addresses, credentials, or other unnecessary company data.

### 8A.5 Initial tenant selection

```http
POST /public/v1/app-auth/tenant-session
Authorization: Bearer <tenant_selection_token>
```

```json
{
  "tenant_id": "selected-tenant-id",
  "app_instance_id": "uuid"
}
```

Fiscal verifies that the selected tenant belongs to the verified email and creates a session. Response:

```json
{
  "access_token": "short-lived-token",
  "refresh_token": "opaque-refresh-token",
  "expires_in": 900,
  "tenant": {
    "tenant_id": "...",
    "display_name": "Company A",
    "roles": ["ADMIN"]
  }
}
```

The access token is tenant-scoped and includes only the selected tenant and that membership's roles. It must not authorize access to every company associated with the email.

### 8A.6 Tenant switch inside BeeFiscalApp

BeeFiscalApp displays a tenant selector in its authenticated shell when the user has more than one active membership.

Suggested endpoints:

```text
GET  /public/v1/app-auth/tenants
POST /public/v1/app-auth/sessions:switch-tenant
POST /public/v1/app-auth/refresh
POST /public/v1/app-auth/logout
```

Switch request uses the active session/refresh credential:

```json
{
  "tenant_id": "new-tenant-id",
  "app_instance_id": "uuid"
}
```

Fiscal revalidates the membership on every switch and returns a new tenant-scoped access token. The previous access-token ID is revoked immediately in the server-side token registry before the switch transaction commits.

BeeFiscalApp must clear tenant-specific cached state before loading the new tenant. Queries, selected register/device, reports, pending forms, and pagination cursors from the previous tenant must not survive the switch.

### 8A.7 Membership and session changes

- Suspended/revoked membership disappears from tenant selection immediately.
- If the currently selected membership is revoked, API requests return `401/403`, the app clears tenant data, and the session must select another valid tenant or log out.
- Roles are read from the selected membership when issuing/refreshing a token; they are not copied permanently into the user account.
- Refresh and tenant switch rotate refresh tokens to prevent replay.
- Logout revokes the server-side session and refresh credential.
- OTP verification is not repeated for every tenant switch while the authenticated session remains valid.

### 8A.8 BeeFiscalApp migration

Mark the following BeeFiscalApp path for replacement/removal:

- `src/adminOidc.ts` login orchestration;
- `expo-auth-session` dependency if no other BeeFiscalApp feature uses it;
- `EXPO_PUBLIC_FISCAL_OIDC_ISSUER` and `EXPO_PUBLIC_FISCAL_OIDC_CLIENT_ID` application configuration;
- OIDC callback/deep-link configuration used only for BeeFiscalApp login;
- UI text and tests that require OIDC Authorization Code + PKCE.

Do not remove platform OIDC support from `fiscal-backend` or `AdminApp`; that is a separate privileged platform identity boundary.

## 9. Asynchronous REST resource synchronization

All company, location, register, and operator mutations use tenant credentials.

The public integration contract is resource-oriented REST. External systems do not submit generic `command_type` envelopes. Fiscal maps every accepted REST mutation to an internal `integration_commands` record and RabbitMQ message.

Required endpoints:

```text
PUT    /integration/v1/organization
PUT    /integration/v1/locations/{source_location_id}
DELETE /integration/v1/locations/{source_location_id}
PUT    /integration/v1/registers/{source_register_id}
DELETE /integration/v1/registers/{source_register_id}
PUT    /integration/v1/operators/{source_operator_id}
DELETE /integration/v1/operators/{source_operator_id}
GET    /integration/v1/operations/{operation_id}
```

Example:

```http
PUT /integration/v1/operators/minipos-employee-uuid
Authorization: Bearer tenant_live_<credential_id>.<secret>
Idempotency-Key: <stable UUID>
Source-Version: 7
BeeFiscal-Source-Actor-Type: USER
BeeFiscal-Source-Actor-Id: minipos-user-uuid
BeeFiscal-Source-Actor-Session-Id: optional-source-session-id
Content-Type: application/json
```

```json
{
  "display_name": "Operator Name",
  "email": "operator@example.com",
  "status": "ACTIVE"
}
```

Path identifiers are opaque source-system identifiers. Fiscal authenticates the credential and derives `tenant_id`, `external_system_id`, and `source_company_id` from its binding. These authority fields are not accepted from the body. Every endpoint has an explicit request schema; unknown fields are rejected unless the versioned schema explicitly allows extensions.

Every mutation must also include source actor attribution. `BeeFiscal-Source-Actor-Type` is `USER` for an end-user action and `SERVICE` for an automatic/background action. `BeeFiscal-Source-Actor-Id` is the stable actor identifier inside the external system; an optional source session ID supports investigation. Fiscal validates syntax and bounded length, but it does not claim to have authenticated that user. The audit model always distinguishes:

```text
authenticated_system_id   # verified by Fiscal tenant credential
asserted_source_actor      # asserted by the authenticated external system
fiscal_platform_actor      # populated only for direct Fiscal/AdminApp actions
```

Human-readable names and email addresses are not accepted in attribution headers. They may change and may expose personal data in infrastructure logs. Any required display data is resolved through separately synchronized operator/user records.

The HTTP transaction:

1. validates authentication, resource scope, OpenAPI schema, idempotency, and source version;
2. translates the REST operation into an internal command and inserts `integration_commands` as `ACCEPTED`;
3. inserts `integration_command_outbox` in the same transaction;
4. returns `202 Accepted` with `operation_id`, `status=ACCEPTED`, and a status URL.

Do not publish directly to RabbitMQ before or after an unrelated database commit. The DB row is the durability boundary.

All REST mutations are asynchronous and return `202 Accepted`; `DELETE` means a source-authoritative deactivate/delete command, not immediate physical deletion inside Fiscal. The final result is available through the operation resource and the signed webhook. HTTP status, stable machine-readable error code, retry semantics, and webhook result must be identical across all resource endpoints.

### 9.1 Ordering and conflicts

- The external system is authoritative for all synchronized company, location, register, and operator fields. Fiscal is a black-box processor and must not silently alter source-owned values.
- Every accepted, rejected, superseded, or applied mutation creates an append-only change-journal entry linked to the authenticated external system, tenant, source entity, request/idempotency ID, asserted source user/service actor, and Fiscal platform actor when applicable. Store before/after values or a durable diff according to the retention policy. Never collapse the authenticated system and asserted actor into one field.
- Apply each mutation in a database transaction that locks the target aggregate/entity. Validation, resource update, source-version update, audit entry, command status, and webhook outbox insertion commit atomically.
- Partition RabbitMQ routing by `tenant_id + aggregate_type + source_entity_id`.
- Consumers must be idempotent by `command_id`.
- `source_version` must be monotonic per source entity.
- Duplicate version with identical payload is success/no-op.
- Duplicate version with different payload is a conflict.
- Older versions become `SUPERSEDED` or a successful no-op; they must never overwrite newer state.
- Referential dependencies must be explicit: location before register, operator before active workstation session.

### 9.2 Consumer result

The consumer applies the Fiscal change and atomically writes:

- command terminal/intermediate status;
- resulting Fiscal resource ID and version;
- audit event;
- webhook delivery row/event.

The resulting webhook tells MiniPOS whether its source version was applied:

```json
{
  "event_id": "...",
  "event_type": "integration.command.updated",
  "source_system_id": "...",
  "tenant_id": "...",
  "operation_id": "...",
  "command_type": "operator.upsert",
  "source_entity_id": "...",
  "source_version": 7,
  "status": "SUCCEEDED",
  "fiscal_resource_id": "...",
  "fiscal_resource_version": 3,
  "error": null,
  "occurred_at": "..."
}
```

MiniPOS webhook inbox must deduplicate by `event_id`, verify source system and tenant binding, and persist the status before returning `2xx`.

## 10. RabbitMQ publisher pattern

### 10.1 Command outbox publisher

Claim a batch atomically:

```sql
WITH picked AS (
  SELECT id
  FROM integration_command_outbox
  WHERE status IN ('PENDING','FAILED')
    AND available_at <= now()
    AND (lease_until IS NULL OR lease_until < now())
  ORDER BY available_at, id
  FOR UPDATE SKIP LOCKED
  LIMIT $1
)
UPDATE integration_command_outbox o
SET status = 'LEASED',
    lease_id = $2,
    lease_until = now() + interval '30 seconds',
    updated_at = now()
FROM picked
WHERE o.id = picked.id
RETURNING o.*;
```

For each item:

1. publish a persistent RabbitMQ message;
2. require publisher confirms;
3. after confirm, mark `PUBLISHED` only when `lease_id` still matches;
4. on failure, clear lease, increment publish attempts, and set `available_at` using backoff.

A crash after broker confirm but before the DB update creates a duplicate. Consumers must therefore deduplicate by immutable `command_id`. Exactly-once transport is not assumed.

Use manual consumer acknowledgements and a dead-letter exchange for poison messages.

## 11. Webhook queue and delivery pattern

Webhook delivery must always use RabbitMQ. The database remains the authoritative durable state, and RabbitMQ is the mandatory delivery transport:

```text
business transaction
  -> webhook_deliveries(PENDING)
  -> DB relay leases rows
  -> persistent RabbitMQ publish + confirm
  -> webhook sender consumes
  -> signed HTTP POST
  -> DB terminal/retry update
  -> Rabbit ACK
```

RabbitMQ is a deliberate scalability boundary for an ultra-high-load Fiscal deployment. It absorbs bursts, isolates HTTP request latency from consumers and webhook destinations, permits horizontal consumer scaling, and applies backpressure outside business transactions. PostgreSQL retains compact authoritative command/outbox/delivery state, but workers must not poll business tables for the full processing workload. Outbox relays use bounded indexed batches, and retention/partitioning must prevent completed command and delivery history from becoming an unbounded hot dataset.

### 11.1 DB relay

Claim rows with `FOR UPDATE SKIP LOCKED`, a unique `lease_id`, and future `lease_until`. Set status to `LEASED`, not delivered. After Rabbit publisher confirm, set `QUEUED`.

If the relay crashes after publish, the message may be published again after lease expiry. The sender uses `delivery_id` and the destination uses `event_id` for deduplication.

### 11.2 Webhook sender

For every message:

1. lock the `webhook_deliveries` row by `delivery_id`;
2. return/ACK immediately if already `DELIVERED` or `DEAD`;
3. mark `DELIVERING` under a short lease;
4. sign the exact raw body;
5. send with strict timeouts and existing SSRF-safe transport;
6. on `2xx`, set `DELIVERED` and `delivered_at`;
7. on failure, atomically increment attempts and set `RETRY` plus `next_attempt_at`;
8. after attempt 5, set `DEAD`; normal workers no longer select it;
9. ACK Rabbit only after the DB update commits.

Recommended retry schedule for five delivery attempts:

```text
attempt 1: immediate
attempt 2: +30 seconds
attempt 3: +5 minutes
attempt 4: +30 minutes
attempt 5: +6 hours
then DEAD
```

HTTP policy:

- `2xx`: delivered;
- `408`, `425`, `429`, `5xx`, network/timeout: retry;
- other `4xx`: normally terminal `DEAD` immediately, except policy-approved cases;
- respect bounded `Retry-After` for `429/503`;
- response body capped before logging;
- redirects disabled or revalidated against SSRF policy at every hop.

AdminApp manual requeue creates a new audit event, resets delivery to `RETRY`, and must not silently reset the historical attempt journal.

### 11.3 Webhook signature

Headers:

```text
BeeFiscal-Event-Id
BeeFiscal-Source-System-Id
BeeFiscal-Delivery-Id
BeeFiscal-Signature: t=<unix>,kid=<key-id>,v1=<hex-hmac>
```

Signature input:

```text
timestamp + "." + exact_raw_body
```

The external system validates timestamp tolerance, active `kid`, HMAC, expected source system, expected tenant binding, and event deduplication.

## 12. Relationship to the existing webhook implementation

The existing per-tenant `webhook_endpoint` feature and direct HTTP dispatcher overlap with the proposed system-level callback.

Recommended direction:

- use `external_systems.webhook_url` as the destination for integration-status events;
- retain tenant webhook endpoints for customer-configured business notifications if still required;
- route both through the new `webhook_deliveries -> RabbitMQ -> sender` transport;
- remove the legacy process-level `WEBHOOK_TARGET_URL` after migration;
- do not deliver every Fiscal event to every external system; filter by binding and subscribed event types.

System bootstrap key rotation differs from current webhook-secret rotation. The user requirement says the previous system key burns immediately. Existing 24-hour webhook secret overlap may remain only for separately managed webhook signing keys if zero-downtime callback rotation is required.

## 13. Failure scenarios and required behavior

### Enrollment

- Repeated start with same idempotency key: same challenge metadata, no second tenant.
- Repeated verify after success: same tenant result, no second credential unless explicit recovery flow.
- Wrong OTP: increment attempt atomically; lock at five.
- Expired temporary token: no tenant creation.
- Two concurrent verifies: one tenant and one binding only.
- System key rotated between start and verify: temporary token may finish because it was already issued, unless the system is suspended/revoked. Document this policy.
- System suspended/revoked: start is denied; pending verification and tenant credentials should be blocked according to explicit policy.

### Commands

- RabbitMQ unavailable: HTTP still returns `202` only after durable DB commit; relay retries later.
- Duplicate Rabbit message: consumer returns existing command result.
- Out-of-order versions: older version cannot overwrite newer data.
- Permanent validation error: command becomes `FAILED`, webhook emitted once.
- Consumer crash after resource write: resource write, command status, audit, and webhook outbox must share one transaction.

### Webhooks

- Relay crash after publish: duplicate allowed and deduplicated.
- Sender crash after HTTP success before DB commit: destination sees duplicate event and must return `2xx` idempotently.
- Callback URL changed: pending deliveries must define whether they use the captured historical URL or current system URL. Recommended: capture destination URL and signing key ID when the delivery row is created.
- Five failures: delivery becomes `DEAD`; alert and AdminApp action required.

## 14. Security controls

- Minimum 256-bit random secrets from a CSPRNG.
- Store credential hashes, never plaintext credentials.
- Encrypt webhook signing secrets and MiniPOS tenant credentials with KMS/Vault.
- Do not place access tokens in query parameters.
- Never log OTP, temporary token, system token, tenant token, webhook secret, or their hashes.
- Mask email and sensitive company fields in operational logs.
- Rate-limit enrollment by credential, system, IP, email, and source company.
- Require API version and idempotency keys on mutations.
- Use constant-time secret and OTP comparisons.
- Use short HTTP timeouts, capped bodies, no implicit redirects, and SSRF-safe DNS resolution.
- Bind every business credential to both tenant and source system.
- Derive tenant from authenticated credential, never from request JSON or headers.
- Derive `authenticated_system_id` from the tenant credential. Treat source actor headers as assertions made by that system, validate their format, and never describe them as identities independently authenticated by Fiscal.
- Audit all key creation, rotation, suspension, enrollment, credential issuance/revocation, manual webhook requeue, and binding changes.
- Add security metrics and alerts for authentication failures, OTP abuse, dead deliveries, queue age, and repeated source-version conflicts.

## 15. API/OpenAPI contract requirements

OpenAPI is the authoritative public integration contract, not supplementary documentation. Add contracts and generated request/response validation for:

- platform external-system administration;
- enrollment start/verify;
- organization, location, register, and operator REST resources;
- asynchronous operation status lookup;
- tenant credential rotation/revocation;
- integration webhook event schema.

The specification must define security schemes, source actor attribution headers, every request/response schema, country/type tax-identifier rules, idempotency behavior, source-version conflicts, pagination where applicable, stable error codes, examples, webhook signatures, and retry semantics. CI must fail on breaking contract changes unless a new API version is introduced. Generate the published reference documentation and supported client models from the same specification.

### 15.1 Integration kit

Deliver an integration kit before onboarding a second production external system:

- versioned OpenAPI document and rendered reference portal;
- JSON Schemas and example payloads for REST resources and webhook events;
- downloadable Postman or Bruno collection;
- sandbox external-system registration and test tenants;
- webhook signature verifier and local webhook receiver example;
- conformance tests for enrollment, idempotency, source versions, operation polling, signatures, retries, and deduplication;
- at least a TypeScript client package generated from OpenAPI, with thin maintained helpers for credentials, idempotency, polling, and webhook verification;
- integration readiness checklist and compatibility/version policy.

The kit must not expose RabbitMQ. External systems integrate only through HTTPS REST and signed HTTPS webhooks.

Every async acceptance response includes:

```text
operation_id
status
status_url
accepted_at
```

Errors use stable machine-readable codes, for example:

```text
SYSTEM_CREDENTIAL_INVALID
SYSTEM_SUSPENDED
ENROLLMENT_RATE_LIMITED
ENROLLMENT_EXPIRED
ENROLLMENT_CODE_INVALID
ENROLLMENT_LOCKED
SOURCE_COMPANY_ALREADY_BOUND
TENANT_CREDENTIAL_INVALID
SOURCE_VERSION_CONFLICT
COMMAND_PAYLOAD_CONFLICT
WEBHOOK_DELIVERY_DEAD
```

## 16. Implementation phases

### Phase 0: ADR and threat model

- Approve trust boundaries and credential classes.
- Record the activation rule: OTP plus a previously unused normalized `(country, tax identifier type, tax identifier value)` may activate; duplicates require manual review.
- Record the fixed storage decision: only the BeeMiniPOS system bootstrap key is stored in `.env`; tenant credentials are encrypted per company in MiniPOS PostgreSQL.
- Define and version country/type-specific tax identifier normalization and validation rules.
- Define source event schemas and ordering rules.
- Define Rabbit exchanges, queues, routing keys, DLQs, retention, and ownership.

### Phase 1: Fiscal platform registry

- Add typed migrations for external systems, credentials, audit log, challenges, bindings, integration credentials, commands, outbox, and webhook deliveries.
- Add platform service/repository APIs.
- Add platform endpoints and authorization roles.
- Add AdminApp list/detail/create/edit/rotate/audit screens.
- Add secret reveal-once behavior.

### Phase 2: Synchronous enrollment

- Implement system credential middleware.
- Implement enrollment start and verify endpoints.
- Add the Fiscal-owned transactional email outbox and worker for enrollment OTP delivery.
- Mark the MiniPOS `beeloy.email.otp` publisher and the matching legacy Fiscal consumer as deprecated.
- Remove the direct MiniPOS -> RabbitMQ OTP path after all active challenges have expired and the legacy queue is drained.
- Implement concurrency, idempotency, expiry, attempt limits, and tenant bootstrap transaction.
- Add security and integration tests.

### Phase 3: MiniPOS binding

- Add explicit `fiscal_tenant_id`, `external_system_id`, binding status, and encrypted credential fields.
- Add enrollment orchestration and recovery UI/API.
- Store bootstrap system token only in deployment secrets.
- Block fiscal operations until binding is active.

### Phase 3A: BeeFiscalApp OTP authentication

- Add tenant user membership, OTP challenge, session, and refresh credential tables.
- Add Fiscal-owned BeeFiscalApp challenge start/verify endpoints.
- Add tenant discovery, initial selection, refresh, tenant switch, and logout endpoints.
- Reuse the Fiscal transactional email outbox without exposing RabbitMQ to BeeFiscalApp.
- Replace BeeFiscalApp OIDC login UI with email, code, and tenant-selection screens.
- Add an authenticated tenant switcher and strict cache/state reset.
- Remove BeeFiscalApp-only OIDC/PKCE code and configuration after migration.
- Keep AdminApp and platform APIs on OIDC.

### Phase 4: Async source synchronization

- Add tenant credential middleware and scopes.
- Implement the versioned organization, location, register, and operator REST endpoints plus operation-status endpoint from the OpenAPI contract.
- Generate server request/response validation from OpenAPI and reject undocumented authority or extension fields.
- Add transactional command outbox relay.
- Add RabbitMQ consumer with source-version/idempotency enforcement.
- Emit integration result events atomically.
- Convert company, location, register, and operator mutations.

### Phase 5: Rabbit-backed webhook delivery

- Add delivery materialization by source binding and subscriptions.
- Add batch leasing with `SKIP LOCKED` and leases.
- Add publisher confirms and persistent messages.
- Add idempotent HTTP sender and five-attempt terminal policy.
- Add AdminApp delivery journal, dead alerts, and manual requeue.
- Migrate existing direct dispatcher and legacy target.

### Phase 5A: Integration kit

- Publish the versioned OpenAPI document and rendered reference documentation.
- Publish schemas, examples, Postman/Bruno collection, webhook verifier, and local receiver example.
- Provide sandbox registration/test tenants and automated conformance tests.
- Generate the TypeScript client and maintain thin helpers for idempotency, operation polling, and webhook verification.
- Validate BeeMiniPOS itself with the conformance suite before accepting a second production external system.

### Phase 6: rollout

- Register BeeMiniPOS as an external system through AdminApp.
- Distribute the bootstrap token through the secret manager.
- Enroll a canary company.
- Shadow existing update paths and compare results.
- Enable async writes per resource type behind feature flags.
- Migrate existing tenant mappings explicitly.
- Disable the global MiniPOS OAuth credential for integration traffic.
- Remove legacy `WEBHOOK_TARGET_URL` after the outbox is drained.
- First migrate BeeMiniPOS login email delivery to a MiniPOS-owned outbox/worker or notification service. Then stop new publications to `beeloy.email.otp`, wait at least the maximum OTP/challenge lifetime, drain or explicitly expire remaining messages, and remove the legacy producer, Fiscal consumer, queue declaration, credentials, metrics, and configuration.

## 17. Test plan

Minimum automated coverage:

- system token creation, constant-time validation, immediate rotation, suspension, and audit;
- no plaintext secret persistence or logging;
- enrollment idempotency, expiry, resend policy, wrong-code lockout, and concurrent verify;
- unique source-company binding and provenance;
- tenant credential scope and cross-tenant/cross-system rejection;
- verified external-system attribution, mandatory asserted actor attribution, and rejection of malformed or missing actor headers;
- audit queries preserve the distinction between authenticated system, asserted source actor, and Fiscal platform actor;
- command DB/outbox atomicity;
- Rabbit publish-confirm failure and lease recovery;
- duplicate and out-of-order command consumption;
- consumer crash transaction rollback;
- webhook target filtering by source binding;
- webhook signature and SSRF protections;
- duplicate delivery handling;
- five-attempt `DEAD` transition;
- manual requeue audit;
- RLS and platform/tenant authorization separation;
- full MiniPOS enrollment -> resource update -> Rabbit consumer -> webhook status end-to-end test.
- OpenAPI request/response conformance for every REST resource and stable error response;
- tax identifier normalization, cross-country/type uniqueness, and concurrent duplicate enrollment;
- integration-kit conformance run against the sandbox and BeeMiniPOS adapter;
- BeeFiscalApp unknown-email non-enumeration and no-account-creation behavior;
- BeeFiscalApp OTP expiry, resend throttling, attempt lockout, one-time token consumption, and session revocation;
- single-membership automatic selection and multi-membership tenant chooser;
- cross-tenant access rejection after selection and after switching;
- per-tenant role changes and membership revocation during an active session;
- tenant switch clears application state and rotates/revokes credentials;

## 18. Acceptance criteria

The design is complete when:

1. No MiniPOS/Fiscal shared signing secret or implicit trust is required for tenant integration.
2. AdminApp can create, edit, suspend, audit, and rotate an external system key.
3. The old system key fails immediately after rotation.
4. Enrollment creates exactly one tenant binding under retries and concurrency.
5. Fiscal tenant identity always comes from a validated tenant credential.
6. MiniPOS stores the explicit Fiscal tenant ID and an encrypted tenant credential.
7. Source mutations return durable `202` operations and survive broker outages.
8. Consumers are idempotent and enforce monotonic source versions.
9. Status webhooks are source/tenant-bound, signed, durable, and deduplicated.
10. A delivery is no longer automatically selected after five failures and is visible/requeueable in AdminApp.
11. Every key and binding transition is auditable without exposing secrets.
12. Existing fiscal sale invariants, tenant isolation, and device authorization tests remain green.
13. MiniPOS no longer generates Fiscal enrollment OTPs or publishes them directly to RabbitMQ, and the legacy `beeloy.email.otp` path is removed after a controlled drain.
14. BeeFiscalApp authenticates registered users through Fiscal-owned email OTP without offering tenant or account onboarding.
15. A verified email with multiple memberships can select and switch tenants, while every access token remains scoped to exactly one tenant.
16. External systems use versioned resource-oriented REST endpoints documented and validated by the authoritative OpenAPI specification; generic command envelopes are internal only.
17. A second external system can integrate through the published sandbox and integration kit without RabbitMQ or Fiscal database access.
18. RabbitMQ absorbs asynchronous processing and webhook bursts, while bounded indexed outbox relays and retention/partitioning protect PostgreSQL under sustained high load.

## 19. Resolved decisions and remaining decisions

The credential storage decision is resolved: `.env` contains only the BeeMiniPOS system bootstrap key; tenant credentials and company-to-tenant bindings are stored in MiniPOS PostgreSQL, with credentials encrypted through KMS/Vault.

Resolved:

1. OTP automatically activates a tenant only after receiving a normalized tax identifier that does not already exist. A duplicate tax identifier requires manual review; no tenant is automatically blocked or replaced.
2. `source_company_id` is mutable, but every change is validated, atomic, actor-linked, and journaled with before/after values.
3. The external system owns all synchronized fields. Fiscal acts as a black box, and every modification is retained in an append-only, user/service-attributed journal.
4. Concurrent changes are serialized by a database transaction locking the affected entity; state, version, audit, command status, and outbox change atomically.
5. Suspending an external system prevents new enrollment but does not revoke already issued tenant credentials. Tenant credentials have an independent lifecycle and must be revoked explicitly.
6. Every mutation is idempotent by caller-generated request ID. The same ID and request hash return the stored previous response without re-execution.
7. Lost tenant credentials are recovered using the existing email + tax identifier and a new OTP verification; recovery rotates the credential and does not create a tenant.
8. BeeFiscalApp access tokens are immediately revocable through a server-side registry of issued token IDs.
9. Webhook delivery always passes through RabbitMQ, with the database as the authoritative delivery state.
10. Public synchronization uses resource-oriented REST with OpenAPI as the authoritative contract. Generic commands remain an internal Fiscal implementation detail.
11. Tax identity is represented universally as `(country, identifier type, original value)` and compared by Fiscal-owned versioned normalization rules.
12. An integration kit is required before onboarding the second production external system.
13. Bulk/mass OTP onboarding is deferred to a later phase and is not required for the initial MiniPOS rollout.
14. Fiscal authenticates the external system through the tenant credential and stores the source user/service separately as an assertion by that system. Audit records never present an asserted source actor as independently authenticated by Fiscal.

Resolved webhook subscription for the first release: BeeMiniPOS subscribes to
`integration.command.updated`. New event families are opt-in and require a
backward-compatible OpenAPI/webhook-schema revision before subscription.
2. Is the five-attempt limit applied only to HTTP delivery attempts, or also to DB-to-Rabbit publish attempts? Recommended: five only for HTTP delivery; broker publication retries remain durable with alerting.
3. What retention periods apply to challenges, idempotent response records, commands, change-journal payloads, delivery payloads, and security audit records?
4. Who is allowed to create and revoke BeeFiscalApp tenant memberships: tenant ADMIN, platform administrator, source-system synchronization, or a combination with explicit ownership rules?
5. What maximum BeeFiscalApp session and refresh-token lifetimes are required for administrative users?
