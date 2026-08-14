CREATE TABLE IF NOT EXISTS fiscal_runtime_composite_bindings (
  binding_id text PRIMARY KEY,
  tenant_id text NOT NULL,
  register_id text NOT NULL,
  profile text NOT NULL,
  adapter_device_id text NOT NULL,
  fiscal_device_id text NOT NULL,
  payment_device_id text,
  generation bigint NOT NULL CHECK(generation>0),
  applied_generation bigint,
  status text NOT NULL CHECK(status IN('PENDING','ACTIVE','REVOKED','SUPERSEDED')),
  concurrency_version bigint NOT NULL CHECK(concurrency_version>0),
  payload jsonb NOT NULL,
  updated_at timestamptz NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_runtime_composite_register_live ON fiscal_runtime_composite_bindings(tenant_id,register_id) WHERE status IN('PENDING','ACTIVE');
CREATE INDEX IF NOT EXISTS ix_runtime_composite_tenant_register ON fiscal_runtime_composite_bindings(tenant_id,register_id,updated_at);
ALTER TABLE fiscal_runtime_composite_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal_runtime_composite_bindings FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_boundary ON fiscal_runtime_composite_bindings;
CREATE POLICY tenant_boundary ON fiscal_runtime_composite_bindings USING(tenant_id=app_tenant_key()) WITH CHECK(tenant_id=app_tenant_key());
GRANT SELECT,INSERT,UPDATE,DELETE ON fiscal_runtime_composite_bindings TO beefiscal_tenant;
