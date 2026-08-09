begin;

alter table minipos_runtime_orders
  add column if not exists payments jsonb not null default '[]'::jsonb;

alter table minipos_runtime_orders
  drop constraint if exists minipos_runtime_orders_payments_array;
alter table minipos_runtime_orders
  add constraint minipos_runtime_orders_payments_array
  check (jsonb_typeof(payments) = 'array');

comment on column minipos_runtime_orders.payments is
  'Validated non-sensitive CASH/CARD tender type and exact amount evidence used for half-open commercial reports.';

commit;
