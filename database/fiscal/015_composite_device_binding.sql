CREATE TABLE IF NOT EXISTS composite_device_bindings (
  binding_id uuid PRIMARY KEY,
  tenant_id uuid NOT NULL REFERENCES tenants(id),
  location_id uuid NOT NULL REFERENCES locations(id),
  register_id uuid NOT NULL REFERENCES registers(id),
  profile text NOT NULL CHECK (profile IN ('DATECS_BLUECASH50_EMBEDDED','DATECS_DP150_BLUEPAD50','DAISY_COMPACT_S01')),
  adapter_device_id uuid NOT NULL REFERENCES devices(id),
  fiscal_device_id uuid NOT NULL REFERENCES devices(id),
  payment_device_id uuid REFERENCES devices(id),
  generation bigint NOT NULL CHECK (generation > 0),
  expected_register_version bigint NOT NULL CHECK (expected_register_version > 0),
  applied_generation bigint,
  status text NOT NULL CHECK (status IN ('PENDING','ACTIVE','REVOKED','SUPERSEDED')),
  concurrency_version bigint NOT NULL DEFAULT 1 CHECK (concurrency_version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  applied_at timestamptz,
  revoked_at timestamptz,
  CHECK ((profile='DATECS_DP150_BLUEPAD50')=(payment_device_id IS NOT NULL)),
  CHECK (applied_generation IS NULL OR applied_generation=generation)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_composite_binding_register_live
  ON composite_device_bindings(tenant_id,register_id) WHERE status IN ('PENDING','ACTIVE');
CREATE UNIQUE INDEX IF NOT EXISTS uq_composite_binding_fiscal_active
  ON composite_device_bindings(tenant_id,fiscal_device_id) WHERE status='ACTIVE';
ALTER TABLE composite_device_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE composite_device_bindings FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS composite_device_bindings_tenant_isolation ON composite_device_bindings;
CREATE POLICY composite_device_bindings_tenant_isolation ON composite_device_bindings
  USING (tenant_id=current_setting('app.tenant_id',true)::uuid)
  WITH CHECK (tenant_id=current_setting('app.tenant_id',true)::uuid);
REVOKE UPDATE, DELETE, TRUNCATE ON composite_device_bindings FROM PUBLIC;
