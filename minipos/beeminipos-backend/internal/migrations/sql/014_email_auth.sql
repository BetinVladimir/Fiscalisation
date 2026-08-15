alter table organizations add column if not exists address text not null default '';
alter table organizations add column if not exists tax_identifier text not null default '';

create table if not exists minipos_auth_challenges(
  id uuid primary key default gen_random_uuid(),
  email text not null,
  code_hash text not null,
  expires_at timestamptz not null,
  attempts integer not null default 0,
  consumed_at timestamptz,
  created_at timestamptz not null default now()
);
create index if not exists minipos_auth_challenges_email_created on minipos_auth_challenges(email,created_at desc);

create table if not exists minipos_auth_onboarding(
  token_hash text primary key,
  email text not null,
  expires_at timestamptz not null,
  consumed_at timestamptz
);

create table if not exists minipos_auth_accounts(
  email text primary key,
  organization_id uuid not null references organizations(id),
  employee_id uuid not null,
  roles text[] not null default array['ADMIN']::text[],
  created_at timestamptz not null default now()
);

create table if not exists minipos_auth_refresh_tokens(
  token_hash text primary key,
  email text not null references minipos_auth_accounts(email) on delete cascade,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);
