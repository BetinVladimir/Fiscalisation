-- MiniPOS uses organization_id as its autonomous tenant boundary.
create role beeminipos_tenant noinherit nologin nosuperuser nocreatedb nocreaterole nobypassrls;

create or replace function app_organization_id() returns uuid
language sql stable parallel safe
as $$ select nullif(current_setting('app.organization_id', true), '')::uuid $$;

alter table webhook_inbox add column if not exists organization_id uuid references organizations(id);
alter table outbox add column if not exists organization_id uuid references organizations(id);

alter table organizations enable row level security;
alter table organizations force row level security;
create policy organization_boundary on organizations
using (id=app_organization_id()) with check (id=app_organization_id());

do $$
declare table_name text;
begin
  foreach table_name in array array['locations','employees','products','webhook_inbox','outbox'] loop
    execute format('alter table %I enable row level security', table_name);
    execute format('alter table %I force row level security', table_name);
    execute format('create policy organization_boundary on %I using (organization_id=app_organization_id()) with check (organization_id=app_organization_id())', table_name);
  end loop;
end $$;

alter table registers enable row level security;
alter table registers force row level security;
create policy organization_boundary on registers
using (exists(select 1 from locations l where l.id=location_id and l.organization_id=app_organization_id()))
with check (exists(select 1 from locations l where l.id=location_id and l.organization_id=app_organization_id()));

alter table shifts enable row level security;
alter table shifts force row level security;
create policy organization_boundary on shifts
using (exists(select 1 from registers r join locations l on l.id=r.location_id where r.id=register_id and l.organization_id=app_organization_id()))
with check (exists(select 1 from registers r join locations l on l.id=r.location_id where r.id=register_id and l.organization_id=app_organization_id()));

alter table orders enable row level security;
alter table orders force row level security;
create policy organization_boundary on orders
using (exists(select 1 from shifts s join registers r on r.id=s.register_id join locations l on l.id=r.location_id where s.id=shift_id and l.organization_id=app_organization_id()))
with check (exists(select 1 from shifts s join registers r on r.id=s.register_id join locations l on l.id=r.location_id where s.id=shift_id and l.organization_id=app_organization_id()));

alter table order_lines enable row level security;
alter table order_lines force row level security;
create policy organization_boundary on order_lines
using (exists(select 1 from orders o join shifts s on s.id=o.shift_id join registers r on r.id=s.register_id join locations l on l.id=r.location_id where o.id=order_id and l.organization_id=app_organization_id()))
with check (exists(select 1 from orders o join shifts s on s.id=o.shift_id join registers r on r.id=s.register_id join locations l on l.id=r.location_id where o.id=order_id and l.organization_id=app_organization_id()));

grant usage on schema public to beeminipos_tenant;
grant select,insert,update,delete on organizations,locations,registers,employees,products,shifts,orders,order_lines,webhook_inbox,outbox to beeminipos_tenant;
grant execute on function app_organization_id() to beeminipos_tenant;
