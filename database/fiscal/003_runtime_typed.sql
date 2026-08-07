-- Authoritative runtime projection for hot fiscal aggregates. Text identifiers
-- preserve the public API identifiers during migration from the compatibility store.
create table fiscal_runtime_sales(
  id text primary key, tenant_id text not null, external_id text not null,
  register_id text not null, operator_id text not null, unp text,
  state text not null, version bigint not null check(version>0),
  lines jsonb not null check(jsonb_typeof(lines)='array'),
  payments jsonb not null check(jsonb_typeof(payments)='array'),
  fiscal_operation_id text, receipt_artifact_id text,
  created_at timestamptz not null, updated_at timestamptz not null,
  unique(tenant_id,external_id), unique(tenant_id,unp)
);
create index fiscal_runtime_sales_tenant_state on fiscal_runtime_sales(tenant_id,state,updated_at);

create table fiscal_runtime_operations(
  id text primary key, tenant_id text not null, sale_id text,
  type text not null, state text not null, version bigint not null check(version>0),
  fiscal_reference text, simulated boolean not null, error_code text,
  created_at timestamptz not null, updated_at timestamptz not null
);
create index fiscal_runtime_operations_tenant_state on fiscal_runtime_operations(tenant_id,state,updated_at);

create or replace function app_tenant_key() returns text language sql stable parallel safe
as $$ select nullif(current_setting('app.tenant_id',true),'') $$;
alter table fiscal_runtime_sales enable row level security;
alter table fiscal_runtime_sales force row level security;
create policy tenant_boundary on fiscal_runtime_sales using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
alter table fiscal_runtime_operations enable row level security;
alter table fiscal_runtime_operations force row level security;
create policy tenant_boundary on fiscal_runtime_operations using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
grant select,insert,update,delete on fiscal_runtime_sales,fiscal_runtime_operations to beefiscal_tenant;
grant execute on function app_tenant_key() to beefiscal_tenant;
