-- Typed-only runtime cutover. Exact domain payload is retained next to indexed,
-- constrained columns so restart does not depend on fiscal_state_rows.
alter table fiscal_runtime_sales add column if not exists payload jsonb;
alter table fiscal_runtime_operations add column if not exists payload jsonb;
alter table fiscal_runtime_operations add column if not exists original_fiscal_reference text;
alter table fiscal_runtime_operations add column if not exists reason_code text;
alter table fiscal_runtime_shifts add column if not exists payload jsonb;
alter table fiscal_runtime_resources add column if not exists payload jsonb;

alter table fiscal_state_meta add column if not exists storage_mode smallint not null default 1 check(storage_mode in (1,2));

comment on column fiscal_state_meta.storage_mode is
  '1=compatibility bootstrap, 2=typed tables are the sole durable runtime representation';
