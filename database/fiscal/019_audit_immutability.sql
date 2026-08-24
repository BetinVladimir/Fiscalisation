alter table fiscal_runtime_audit add column if not exists previous_hash text generated always as (coalesce(payload->>'prev_hash','')) stored;
create index if not exists fiscal_runtime_audit_tenant_chain on fiscal_runtime_audit(tenant_id,occurred_at,event_id,previous_hash,event_hash);

create or replace function fiscal_reject_audit_mutation() returns trigger language plpgsql as $$
begin
  raise exception using errcode='P0001',message='FISCAL_AUDIT_IS_IMMUTABLE';
end$$;
drop trigger if exists fiscal_runtime_audit_no_update on fiscal_runtime_audit;
create trigger fiscal_runtime_audit_no_update before update or delete on fiscal_runtime_audit for each row execute function fiscal_reject_audit_mutation();
