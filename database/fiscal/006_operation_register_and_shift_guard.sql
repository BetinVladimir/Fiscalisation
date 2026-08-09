-- Durable operation/register identity and one unresolved shift per autonomous
-- tenant/register. Safe for existing DEV/PILOT databases initialized before
-- the runtime schema gained these constraints.
alter table fiscal_runtime_operations add column if not exists register_id text;
update fiscal_runtime_operations
set register_id = nullif(payload->>'register_id','')
where register_id is null and payload ? 'register_id';

drop index if exists fiscal_runtime_one_open_shift;
create unique index fiscal_runtime_one_open_shift
on fiscal_runtime_shifts(tenant_id,register_id)
where state in('OPEN','CLOSING','RECONCILIATION_REQUIRED');
