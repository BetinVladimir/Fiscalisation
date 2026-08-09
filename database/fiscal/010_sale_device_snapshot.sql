alter table fiscal_runtime_sales
  add column if not exists fiscal_device_id text,
  add column if not exists fiscal_device_serial text,
  add column if not exists fiscal_device_number text,
  add column if not exists fiscal_memory_number text,
  add column if not exists fiscal_device_vendor text,
  add column if not exists fiscal_device_model text,
  add column if not exists fiscal_device_firmware text;

update fiscal_runtime_sales set
  fiscal_device_id = nullif(payload->'fiscal_device'->>'device_id',''),
  fiscal_device_serial = nullif(payload->'fiscal_device'->>'serial',''),
  fiscal_device_number = nullif(payload->'fiscal_device'->>'fiscal_device_number',''),
  fiscal_memory_number = nullif(payload->'fiscal_device'->>'fiscal_memory_number',''),
  fiscal_device_vendor = nullif(payload->'fiscal_device'->>'vendor',''),
  fiscal_device_model = nullif(payload->'fiscal_device'->>'model',''),
  fiscal_device_firmware = nullif(payload->'fiscal_device'->>'firmware','')
where payload ? 'fiscal_device';

create index if not exists fiscal_runtime_sales_tenant_device
  on fiscal_runtime_sales(tenant_id,fiscal_device_id,created_at);

comment on column fiscal_runtime_sales.fiscal_device_id is
  'Immutable end-device identity captured atomically when fiscal payment execution is reserved; never resolved from the current register binding for historical evidence.';
