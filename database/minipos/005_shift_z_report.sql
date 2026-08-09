alter table minipos_runtime_shifts
  add column z_operation_id text,
  add column z_fiscal_reference text,
  add column close_error text;

drop index minipos_one_open_shift;
create unique index minipos_one_active_shift
  on minipos_runtime_shifts(organization_id, register_id)
  where state <> 'CLOSED';
