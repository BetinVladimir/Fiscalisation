alter table fiscal_runtime_sales add column if not exists location_id text;

update fiscal_runtime_sales
set location_id = nullif(payload->>'location_id','')
where location_id is null and payload ? 'location_id';

create index if not exists fiscal_runtime_sales_tenant_location
  on fiscal_runtime_sales(tenant_id,location_id,created_at);

comment on column fiscal_runtime_sales.location_id is
  'Immutable location identity captured when the sale is created; historical exports never resolve it through the current register configuration.';
