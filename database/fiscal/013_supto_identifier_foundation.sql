CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS country_fiscal_profiles (
  tenant_id uuid NOT NULL,
  country_code char(2) NOT NULL,
  profile_version text NOT NULL,
  identifier_scheme text NOT NULL,
  currency char(3) NOT NULL CHECK (currency = 'EUR'),
  effective_from timestamptz NOT NULL,
  effective_to timestamptz,
  manifest jsonb NOT NULL,
  manifest_sha256 char(64) NOT NULL CHECK (manifest_sha256 ~ '^[0-9a-f]{64}$'),
  signature text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, country_code, profile_version),
  CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE IF NOT EXISTS fiscal_device_identities (
  tenant_id uuid NOT NULL,
  device_id uuid NOT NULL,
  fiscal_device_number varchar(8) NOT NULL CHECK (fiscal_device_number ~ '^[A-Za-z0-9]{8}$'),
  source text NOT NULL CHECK (source IN ('DEVICE_READ','SERVICE_REGISTRY')),
  verified_at timestamptz NOT NULL,
  valid_from timestamptz NOT NULL,
  valid_to timestamptz,
  evidence jsonb NOT NULL,
  PRIMARY KEY (tenant_id, device_id, valid_from),
  CHECK (valid_to IS NULL OR valid_to > valid_from)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_active_fiscal_device_number
  ON fiscal_device_identities (tenant_id, fiscal_device_number) WHERE valid_to IS NULL;

CREATE TABLE IF NOT EXISTS unp_range_leases (
  tenant_id uuid NOT NULL,
  lease_id uuid NOT NULL,
  fiscal_device_number varchar(8) NOT NULL CHECK (fiscal_device_number ~ '^[A-Za-z0-9]{8}$'),
  range_start integer NOT NULL CHECK (range_start BETWEEN 1 AND 9999999),
  range_end integer NOT NULL CHECK (range_end BETWEEN 1 AND 9999999),
  fencing_token bigint NOT NULL CHECK (fencing_token > 0),
  authority_instance_id uuid NOT NULL,
  state text NOT NULL CHECK (state IN ('ACTIVE','EXHAUSTED','REVOKED','EXPIRED')),
  valid_from timestamptz NOT NULL,
  valid_until timestamptz NOT NULL,
  signature text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, lease_id),
  CHECK (range_end >= range_start),
  CHECK (valid_until > valid_from),
  EXCLUDE USING gist (tenant_id WITH =, fiscal_device_number WITH =, int4range(range_start, range_end, '[]') WITH &&) WHERE (state = 'ACTIVE')
);

CREATE TABLE IF NOT EXISTS unp_allocations (
  tenant_id uuid NOT NULL,
  allocation_id uuid NOT NULL,
  lease_id uuid,
  fiscal_device_number varchar(8) NOT NULL CHECK (fiscal_device_number ~ '^[A-Za-z0-9]{8}$'),
  operator_code varchar(4) NOT NULL CHECK (operator_code ~ '^[A-Za-z0-9]{4}$'),
  unp_sequence integer NOT NULL CHECK (unp_sequence BETWEEN 1 AND 9999999),
  unp varchar(21) NOT NULL CHECK (unp ~ '^[A-Za-z0-9]{8}-[A-Za-z0-9]{4}-[0-9]{7}$'),
  status text NOT NULL CHECK (status IN ('ALLOCATED','BOUND','ABORTED')),
  allocated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, allocation_id),
  UNIQUE (tenant_id, unp),
  UNIQUE (tenant_id, fiscal_device_number, unp_sequence),
  FOREIGN KEY (tenant_id, lease_id) REFERENCES unp_range_leases (tenant_id, lease_id)
);

CREATE TABLE IF NOT EXISTS regulatory_identifier_bindings (
  tenant_id uuid NOT NULL,
  binding_id uuid NOT NULL,
  source_system text NOT NULL,
  source_app_instance_id uuid NOT NULL,
  surrogate_type text NOT NULL,
  surrogate_id uuid NOT NULL,
  resource_type text NOT NULL,
  resource_id uuid NOT NULL,
  regulatory_identifier_type text NOT NULL,
  scheme text NOT NULL,
  value text NOT NULL,
  country_code char(2) NOT NULL,
  profile_version text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, binding_id),
  UNIQUE (tenant_id, source_system, source_app_instance_id, surrogate_type, surrogate_id, regulatory_identifier_type),
  UNIQUE (tenant_id, regulatory_identifier_type, scheme, value)
);

CREATE TABLE IF NOT EXISTS readiness_leases (
  tenant_id uuid NOT NULL, lease_id uuid NOT NULL, workstation_id uuid NOT NULL,
  fiscal_device_id uuid NOT NULL, fiscal_device_number varchar(8) NOT NULL CHECK (fiscal_device_number ~ '^[A-Za-z0-9]{8}$'),
  profile_version text NOT NULL, checked_at timestamptz NOT NULL, valid_until timestamptz NOT NULL,
  signature text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, lease_id), CHECK (valid_until > checked_at), CHECK (valid_until <= checked_at + interval '2 hours')
);
CREATE TABLE IF NOT EXISTS device_clock_sync_events (
  tenant_id uuid NOT NULL, event_id uuid NOT NULL, workstation_id uuid NOT NULL, device_id uuid NOT NULL,
  business_date date NOT NULL, trusted_time timestamptz NOT NULL, device_time timestamptz NOT NULL,
  drift_seconds bigint NOT NULL, set_performed boolean NOT NULL, verified boolean NOT NULL,
  occurred_at timestamptz NOT NULL, PRIMARY KEY (tenant_id, event_id)
);

ALTER TABLE country_fiscal_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal_device_identities ENABLE ROW LEVEL SECURITY;
ALTER TABLE unp_range_leases ENABLE ROW LEVEL SECURITY;
ALTER TABLE unp_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE regulatory_identifier_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE readiness_leases ENABLE ROW LEVEL SECURITY;
ALTER TABLE device_clock_sync_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE country_fiscal_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE fiscal_device_identities FORCE ROW LEVEL SECURITY;
ALTER TABLE unp_range_leases FORCE ROW LEVEL SECURITY;
ALTER TABLE unp_allocations FORCE ROW LEVEL SECURITY;
ALTER TABLE regulatory_identifier_bindings FORCE ROW LEVEL SECURITY;
ALTER TABLE readiness_leases FORCE ROW LEVEL SECURITY;
ALTER TABLE device_clock_sync_events FORCE ROW LEVEL SECURITY;

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['country_fiscal_profiles','fiscal_device_identities','unp_range_leases','unp_allocations','regulatory_identifier_bindings','readiness_leases','device_clock_sync_events'] LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = current_setting(''app.tenant_id'', true)::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'', true)::uuid)', t);
  END LOOP;
END $$;

REVOKE UPDATE, DELETE, TRUNCATE ON unp_allocations, regulatory_identifier_bindings FROM PUBLIC;
REVOKE UPDATE, DELETE, TRUNCATE ON readiness_leases, device_clock_sync_events FROM PUBLIC;
