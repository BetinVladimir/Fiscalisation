alter table minipos_runtime_orders
  add column if not exists fiscal_reference text;

alter table minipos_runtime_checkout_results
  add column if not exists fiscal_reference text;

alter table minipos_runtime_orders
  drop constraint if exists minipos_final_order_has_fiscal_reference;
alter table minipos_runtime_orders
  add constraint minipos_final_order_has_fiscal_reference check (
    state not in ('COMPLETED','REVERSED') or fiscal_reference is not null
  ) not valid;

comment on column minipos_runtime_orders.fiscal_reference is
  'Original fiscal receipt reference. Required for every newly completed or reversed order.';
comment on column minipos_runtime_checkout_results.fiscal_reference is
  'Original fiscal receipt reference captured with the idempotent checkout result.';
