-- Typed legal/reporting model tenant boundary. Runtime compatibility rows are
-- intentionally excluded until their payloads have been replaced by typed writes.
create role beefiscal_tenant noinherit nologin nosuperuser nocreatedb nocreaterole nobypassrls;
grant beefiscal_tenant to current_user;

create or replace function app_tenant_id() returns uuid
language sql stable parallel safe
as $$ select nullif(current_setting('app.tenant_id', true), '')::uuid $$;

alter table edge_sync_acks add column if not exists tenant_id uuid references tenants(id);

do $$
declare table_name text;
begin
  foreach table_name in array array[
    'tenants','locations','registers','operators','shifts','devices','sales',
    'fiscal_operations','idempotency_records','audit_events','artifacts','edge_sync_acks'
  ] loop
    execute format('alter table %I enable row level security', table_name);
    execute format('alter table %I force row level security', table_name);
    execute format('drop policy if exists tenant_boundary on %I', table_name);
    if table_name = 'tenants' then
      execute format('create policy tenant_boundary on %I using (id = app_tenant_id()) with check (id = app_tenant_id())', table_name);
    else
      execute format('create policy tenant_boundary on %I using (tenant_id = app_tenant_id()) with check (tenant_id = app_tenant_id())', table_name);
    end if;
  end loop;
end $$;

alter table sale_lines enable row level security;
alter table sale_lines force row level security;
create policy tenant_boundary on sale_lines
using (exists(select 1 from sales s where s.id=sale_id and s.tenant_id=app_tenant_id()))
with check (exists(select 1 from sales s where s.id=sale_id and s.tenant_id=app_tenant_id()));

alter table operation_events enable row level security;
alter table operation_events force row level security;
create policy tenant_boundary on operation_events
using (exists(select 1 from fiscal_operations o where o.id=operation_id and o.tenant_id=app_tenant_id()))
with check (exists(select 1 from fiscal_operations o where o.id=operation_id and o.tenant_id=app_tenant_id()));

grant usage on schema public to beefiscal_tenant;
grant select,insert,update,delete on tenants,locations,registers,operators,shifts,devices,sales,sale_lines,
  fiscal_operations,operation_events,idempotency_records,audit_events,artifacts,edge_sync_acks to beefiscal_tenant;
grant execute on function app_tenant_id() to beefiscal_tenant;
