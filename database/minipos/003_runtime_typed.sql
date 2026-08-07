create table minipos_runtime_products(
  id text primary key, organization_id text not null, sku text not null,
  name text not null, unit text not null, amount numeric(18,2) not null check(amount>=0),
  currency char(3) not null check(currency='EUR'), tax_group char(1) not null,
  active boolean not null, status text not null, version bigint not null check(version>0),
  created_at timestamptz not null, updated_at timestamptz not null,
  unique(organization_id,sku)
);
create table minipos_runtime_employees(
  id text primary key, organization_id text not null, first_name text not null,
  last_name text not null, operator_code char(4) not null, roles jsonb not null check(jsonb_typeof(roles)='array'),
  active boolean not null, status text not null, version bigint not null check(version>0),
  created_at timestamptz not null, updated_at timestamptz not null,
  unique(organization_id,operator_code)
);
create table minipos_runtime_shifts(
  id text primary key, organization_id text not null, register_id text not null,
  employee_id text not null, state text not null, version bigint not null check(version>0),
  opened_at timestamptz not null, closed_at timestamptz,
  created_at timestamptz not null, updated_at timestamptz not null
);
create unique index minipos_one_open_shift on minipos_runtime_shifts(organization_id,register_id) where state='OPEN';
create table minipos_runtime_orders(
  id text primary key, organization_id text not null, external_id text not null,
  shift_id text not null, register_id text not null, operator_code char(4) not null,
  state text not null, total numeric(18,2) not null check(total>=0),
  currency char(3) not null check(currency='EUR'), lines jsonb not null check(jsonb_typeof(lines)='array'),
  fiscal_sale_id text, fiscal_operation_id text, fiscal_version bigint,
  version bigint not null check(version>0), created_at timestamptz not null, updated_at timestamptz not null,
  unique(organization_id,external_id)
);
create index minipos_runtime_orders_tenant_state on minipos_runtime_orders(organization_id,state,updated_at);

create or replace function app_organization_key() returns text language sql stable parallel safe
as $$ select nullif(current_setting('app.organization_id',true),'') $$;
do $$ declare t text; begin foreach t in array array['minipos_runtime_products','minipos_runtime_employees','minipos_runtime_shifts','minipos_runtime_orders'] loop execute format('alter table %I enable row level security',t);execute format('alter table %I force row level security',t);execute format('create policy organization_boundary on %I using(organization_id=app_organization_key()) with check(organization_id=app_organization_key())',t);end loop;end $$;
grant select,insert,update,delete on minipos_runtime_products,minipos_runtime_employees,minipos_runtime_shifts,minipos_runtime_orders to beeminipos_tenant;
grant execute on function app_organization_key() to beeminipos_tenant;
