-- Runtime resource identities are tenant-owned business keys. These indexes
-- mirror the case-insensitive domain rules and protect non-HTTP writers.
create unique index fiscal_runtime_one_organization_per_tenant
  on fiscal_runtime_resources(tenant_id)
  where kind='organization';

create unique index fiscal_runtime_resource_code_per_tenant_kind
  on fiscal_runtime_resources(tenant_id,kind,lower(data->>'code'))
  where kind in ('location','register','operator');

create unique index fiscal_runtime_device_serial_per_tenant
  on fiscal_runtime_resources(tenant_id,lower(data->>'serial'))
  where kind='device';
