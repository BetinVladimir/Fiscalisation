alter table minipos_fiscal_enrollment_sessions add column if not exists tax_country text;
alter table minipos_fiscal_enrollment_sessions add column if not exists tax_type text;
alter table minipos_fiscal_enrollment_sessions add column if not exists tax_value text;

create table if not exists company_fiscal_binding_history(
  id uuid primary key default gen_random_uuid(), binding_id uuid not null references company_fiscal_bindings(id),
  actor_type text not null, actor_id text not null, reason text not null,
  before_redacted jsonb, after_redacted jsonb not null,
  created_at timestamptz not null default now()
);
create index if not exists company_fiscal_binding_history_binding on company_fiscal_binding_history(binding_id,created_at,id);
