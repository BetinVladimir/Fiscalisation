create table if not exists minipos_runtime_tax_groups (
  id text primary key,
  organization_id text not null,
  code text not null check (code in ('A','B','C','D','E','F','G','H')),
  name text not null check (btrim(name) <> ''),
  rate numeric(5,2) not null check (rate between 0 and 100),
  status text not null check (status in ('ACTIVE','INACTIVE')),
  version bigint not null check (version > 0),
  created_at timestamptz not null,
  updated_at timestamptz not null,
  payload jsonb not null,
  unique (organization_id, code)
);

alter table minipos_runtime_tax_groups enable row level security;
alter table minipos_runtime_tax_groups force row level security;

drop policy if exists minipos_runtime_tax_groups_tenant on minipos_runtime_tax_groups;
create policy minipos_runtime_tax_groups_tenant on minipos_runtime_tax_groups
  using (organization_id = current_setting('app.organization_id', true))
  with check (organization_id = current_setting('app.organization_id', true));

grant select, insert, update, delete on minipos_runtime_tax_groups to beeminipos_tenant;
