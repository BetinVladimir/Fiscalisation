alter table fiscal_runtime_ble_sessions
  add column if not exists location_id text,
  add column if not exists fiscal_device_id text;

update fiscal_runtime_ble_sessions set
  location_id = nullif(payload->>'location_id',''),
  fiscal_device_id = nullif(payload->>'fiscal_device_id','')
where location_id is null or fiscal_device_id is null;

create index if not exists fiscal_runtime_ble_sessions_tenant_binding
  on fiscal_runtime_ble_sessions(tenant_id,location_id,register_id,fiscal_device_id,expires_at);

comment on column fiscal_runtime_ble_sessions.device_id is
  'Edge advertising/runtime identity; distinct from the final fiscal device.';
comment on column fiscal_runtime_ble_sessions.fiscal_device_id is
  'Final fiscal device frozen into the signed BLE authority package.';
