-- Align typed projections with the domain model before enforcing the rule.
-- Product/employee payload is mutable administration data, not fiscal history.
update minipos_runtime_products
set active=(status='ACTIVE'),
    payload=case when payload is null then null else jsonb_set(payload,'{active}',to_jsonb(status='ACTIVE'),true) end
where active is distinct from (status='ACTIVE')
   or status not in ('ACTIVE','INACTIVE');

update minipos_runtime_employees
set active=(status='ACTIVE'),
    payload=case when payload is null then null else jsonb_set(payload,'{active}',to_jsonb(status='ACTIVE'),true) end
where active is distinct from (status='ACTIVE')
   or status not in ('ACTIVE','INACTIVE');

alter table minipos_runtime_products
  add constraint minipos_product_status_valid check(status in ('ACTIVE','INACTIVE')),
  add constraint minipos_product_active_matches_status check(active=(status='ACTIVE'));

alter table minipos_runtime_employees
  add constraint minipos_employee_status_valid check(status in ('ACTIVE','INACTIVE')),
  add constraint minipos_employee_active_matches_status check(active=(status='ACTIVE'));

create unique index minipos_runtime_product_sku_ci
  on minipos_runtime_products(organization_id,lower(sku));

create unique index minipos_runtime_employee_operator_code_ci
  on minipos_runtime_employees(organization_id,lower(operator_code));
