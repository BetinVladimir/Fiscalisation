create table if not exists minipos_runtime_identity_bindings(
  binding_key text primary key,
  organization_id text not null,
  employee_id text not null,
  subject_hash char(64) not null check(length(subject_hash)=64),
  identity_issuer text not null,
  bound_at timestamptz not null,
  payload jsonb not null,
  unique(organization_id,identity_issuer,subject_hash),
  unique(organization_id,employee_id)
);

alter table minipos_runtime_identity_bindings enable row level security;
alter table minipos_runtime_identity_bindings force row level security;
create policy organization_boundary on minipos_runtime_identity_bindings
  using(organization_id=app_organization_key())
  with check(organization_id=app_organization_key());
grant select,insert,update,delete on minipos_runtime_identity_bindings to beeminipos_tenant;

-- Stores only the base64url SHA-256 fingerprint of an access token, never the
-- bearer credential itself. This ledger makes logout/revocation restart-safe.
create table if not exists minipos_runtime_operator_sessions(
  session_key text primary key,
  organization_id text not null,
  employee_id text not null,
  app_instance_id text not null,
  credential_fingerprint text not null,
  state text not null check(state in ('ACTIVE','REVOKED')),
  first_seen timestamptz not null,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  payload jsonb not null,
  unique(organization_id,credential_fingerprint)
);
alter table minipos_runtime_operator_sessions enable row level security;
alter table minipos_runtime_operator_sessions force row level security;
create policy organization_boundary on minipos_runtime_operator_sessions
  using(organization_id=app_organization_key())
  with check(organization_id=app_organization_key());
grant select,insert,update,delete on minipos_runtime_operator_sessions to beeminipos_tenant;
