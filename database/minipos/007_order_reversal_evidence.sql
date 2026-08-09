alter table minipos_runtime_orders
  add column if not exists reversal_operation_id text,
  add column if not exists reversal_fiscal_reference text,
  add column if not exists reversal_reason_code text;

alter table minipos_runtime_orders
  drop constraint if exists minipos_order_reversal_reason;
alter table minipos_runtime_orders
  add constraint minipos_order_reversal_reason check (
    reversal_reason_code is null or reversal_reason_code in (
      'OPERATOR_ERROR','CUSTOMER_RETURN','CUSTOMER_COMPLAINT','TAX_BASE_REDUCTION'
    )
  );

alter table minipos_runtime_orders
  drop constraint if exists minipos_reversed_order_has_evidence;
alter table minipos_runtime_orders
  add constraint minipos_reversed_order_has_evidence check (
    state <> 'REVERSED' or (
      reversal_operation_id is not null and
      reversal_fiscal_reference is not null and
      reversal_reason_code is not null
    )
  );

comment on column minipos_runtime_orders.reversal_operation_id is
  'Immutable Fiscal reversal operation linked to the original completed order.';
comment on column minipos_runtime_orders.reversal_fiscal_reference is
  'Fiscal storno document reference; required when state is REVERSED.';
comment on column minipos_runtime_orders.reversal_reason_code is
  'Closed Bulgarian reversal reason code submitted to Fiscal public API.';
