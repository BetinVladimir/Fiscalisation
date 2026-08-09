-- Typed-only runtime cutover. Exact domain payload is retained next to indexed,
-- constrained columns so restart does not depend on minipos_state_rows.
alter table minipos_runtime_products add column if not exists payload jsonb;
alter table minipos_runtime_employees add column if not exists payload jsonb;
alter table minipos_runtime_shifts add column if not exists payload jsonb;
alter table minipos_runtime_orders add column if not exists payload jsonb;
alter table minipos_runtime_configurations add column if not exists payload jsonb;

alter table minipos_state_meta add column if not exists sequence bigint not null default 0 check(sequence>=0);
alter table minipos_state_meta add column if not exists storage_mode smallint not null default 1 check(storage_mode in (1,2));

comment on column minipos_state_meta.storage_mode is
  '1=compatibility bootstrap, 2=typed tables are the sole durable runtime representation';
