alter table minipos_runtime_configurations
  add column if not exists location_id text,
  add column if not exists fiscal_adapter_id text,
  add column if not exists binding_generation bigint,
  add column if not exists adapter_base_url text,
  add column if not exists ble_advertising_identity text,
  add column if not exists ble_service_uuid text,
  add column if not exists ble_command_uuid text,
  add column if not exists ble_event_uuid text;

update minipos_runtime_configurations
set location_id = coalesce(location_id, payload->>'location_id'),
    fiscal_adapter_id = coalesce(fiscal_adapter_id, payload->>'fiscal_adapter_id'),
    binding_generation = coalesce(binding_generation, nullif(payload->>'binding_generation','')::bigint),
    adapter_base_url = coalesce(adapter_base_url, payload->>'adapter_base_url'),
    ble_advertising_identity = coalesce(ble_advertising_identity, payload->>'ble_advertising_identity'),
    ble_service_uuid = coalesce(ble_service_uuid, payload->>'ble_service_uuid'),
    ble_command_uuid = coalesce(ble_command_uuid, payload->>'ble_command_uuid'),
    ble_event_uuid = coalesce(ble_event_uuid, payload->>'ble_event_uuid');

alter table minipos_runtime_configurations
  drop constraint if exists minipos_runtime_configuration_local_adapter,
  add constraint minipos_runtime_configuration_local_adapter check (
    location_id is not null and location_id <> '' and
    fiscal_adapter_id is not null and fiscal_adapter_id <> '' and
    binding_generation is not null and binding_generation > 0 and
    adapter_base_url is not null and adapter_base_url ~ '^http://[^/]+/beeloy/local/v1/?$'
  ) not valid;

comment on column minipos_runtime_configurations.binding_generation is
  'Composite binding fencing generation used by HTTP and BLE local routes.';
