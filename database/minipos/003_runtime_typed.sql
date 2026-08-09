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

create table minipos_runtime_configurations(
  id text primary key, organization_id text not null unique,
  location_name text not null, location_address text not null,
  workstation_name text not null, fiscal_register_id text not null,
  version bigint not null check(version>0), created_at timestamptz not null,
  updated_at timestamptz not null
);

create table minipos_runtime_api_replays(
  replay_key text primary key, organization_id text not null,
  method text not null, path text not null, request_hash text not null,
  status integer not null check(status between 100 and 599),
  pending boolean not null default false,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index minipos_runtime_api_replays_tenant on minipos_runtime_api_replays(organization_id,path);

create table minipos_runtime_webhook_inbox(
  event_id text primary key, organization_id text not null,
  request_hash text not null, received_at timestamptz not null,
  processed_at timestamptz, error text,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index minipos_runtime_webhook_tenant_received on minipos_runtime_webhook_inbox(organization_id,received_at);

create table minipos_runtime_checkout_results(
  replay_key text primary key, organization_id text not null,
  order_id text not null, state text not null, version bigint not null check(version>0),
  fiscal_sale_id text, fiscal_operation_id text,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index minipos_runtime_checkout_results_tenant on minipos_runtime_checkout_results(organization_id,order_id);

create table minipos_runtime_checkout_hashes(
  replay_key text primary key, organization_id text not null,
  request_hash text not null check(length(request_hash)=64)
);

create or replace function app_organization_key() returns text language sql stable parallel safe
as $$ select nullif(current_setting('app.organization_id',true),'') $$;
do $$ declare t text; begin foreach t in array array['minipos_runtime_products','minipos_runtime_employees','minipos_runtime_shifts','minipos_runtime_orders','minipos_runtime_configurations','minipos_runtime_api_replays','minipos_runtime_webhook_inbox','minipos_runtime_checkout_results','minipos_runtime_checkout_hashes'] loop execute format('alter table %I enable row level security',t);execute format('alter table %I force row level security',t);execute format('create policy organization_boundary on %I using(organization_id=app_organization_key()) with check(organization_id=app_organization_key())',t);end loop;end $$;
grant select,insert,update,delete on minipos_runtime_products,minipos_runtime_employees,minipos_runtime_shifts,minipos_runtime_orders,minipos_runtime_configurations,minipos_runtime_api_replays,minipos_runtime_webhook_inbox,minipos_runtime_checkout_results,minipos_runtime_checkout_hashes to beeminipos_tenant;
grant execute on function app_organization_key() to beeminipos_tenant;
