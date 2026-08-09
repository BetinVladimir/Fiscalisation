alter table minipos_runtime_products add column barcode text;
create unique index minipos_runtime_products_tenant_barcode
  on minipos_runtime_products(organization_id, barcode)
  where barcode is not null;
