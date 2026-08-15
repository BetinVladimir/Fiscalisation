create table if not exists company_fiscal_bindings(
  id uuid primary key default gen_random_uuid(), company_id uuid not null references organizations(id),
  external_system_id uuid not null, source_company_id text not null, fiscal_tenant_id uuid not null,
  status text not null check(status in ('ENROLLMENT_PENDING','PROVISIONING','ACTIVE','DEGRADED','SUSPENDED')),
  version bigint not null default 1, last_synchronized_at timestamptz, last_error_code text, last_error_detail text,
  created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
  unique(company_id), unique(external_system_id,source_company_id), unique(fiscal_tenant_id,external_system_id)
);
create table if not exists company_fiscal_credentials(
  id uuid primary key default gen_random_uuid(), binding_id uuid not null references company_fiscal_bindings(id) on delete cascade,
  credential_id uuid not null, credential_fingerprint text not null, ciphertext bytea not null, encryption_key_id text not null,
  status text not null check(status in ('ACTIVE','ROTATING','REVOKED')), created_at timestamptz not null default now(),
  rotated_at timestamptz, revoked_at timestamptz
);
create unique index if not exists company_one_active_fiscal_credential on company_fiscal_credentials(binding_id) where status='ACTIVE';
create table if not exists company_fiscal_resource_links(
  id uuid primary key default gen_random_uuid(), binding_id uuid not null references company_fiscal_bindings(id) on delete cascade,
  resource_type text not null, source_entity_id text not null, fiscal_resource_id text,
  source_version bigint not null, fiscal_version bigint,
  sync_status text not null check(sync_status in ('PENDING','ACCEPTED','SUCCEEDED','FAILED','SUPERSEDED')),
  last_operation_id uuid, last_error_code text, created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
  unique(binding_id,resource_type,source_entity_id)
);
create table if not exists minipos_fiscal_enrollment_sessions(
  id uuid primary key default gen_random_uuid(), company_id uuid not null references organizations(id),
  idempotency_key uuid not null, temporary_token_ciphertext bytea not null, expires_at timestamptz not null,
  status text not null check(status in ('PENDING','VERIFIED','EXPIRED','FAILED')),
  created_at timestamptz not null default now(), updated_at timestamptz not null default now(), unique(company_id,idempotency_key)
);
