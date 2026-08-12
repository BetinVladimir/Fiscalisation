BEGIN;

CREATE TABLE IF NOT EXISTS fiscal_device_registry (
  device_id uuid PRIMARY KEY,
  serial varchar(64) NOT NULL UNIQUE,
  device_public_key_jwk jsonb NOT NULL,
  device_key_thumbprint varchar(64) NOT NULL UNIQUE,
  hardware_revision varchar(64) NOT NULL,
  firmware_version varchar(64) NOT NULL,
  bootloader_version varchar(64),
  manufacturing_batch varchar(64) NOT NULL,
  manufacturing_station_id varchar(128) NOT NULL,
  manufactured_at timestamptz NOT NULL,
  state varchar(16) NOT NULL CHECK (state IN ('MANUFACTURED','ASSIGNED','DEPLOYED','SUSPENDED','RETIRED')),
  tenant_id text,
  binding_version bigint NOT NULL DEFAULT 0 CHECK (binding_version >= 0),
  firmware_sha256 char(64) NOT NULL,
  registration_evidence_sha256 char(64) NOT NULL,
  last_seen_at timestamptz,
  suspended_at timestamptz,
  retired_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((state = 'MANUFACTURED' AND tenant_id IS NULL) OR state <> 'MANUFACTURED')
);

CREATE INDEX IF NOT EXISTS idx_fiscal_device_registry_tenant_state
  ON fiscal_device_registry(tenant_id, state);
CREATE INDEX IF NOT EXISTS idx_fiscal_device_registry_batch
  ON fiscal_device_registry(manufacturing_batch, serial);

CREATE TABLE IF NOT EXISTS fiscal_manufacturing_stations (
  station_id text PRIMARY KEY,
  display_name text NOT NULL,
  oidc_subject text NOT NULL UNIQUE,
  credential_thumbprint text,
  allowed_batches text[] NOT NULL DEFAULT '{}',
  status text NOT NULL CHECK (status IN ('ACTIVE','SUSPENDED','REVOKED')),
  last_used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE IF NOT EXISTS fiscal_device_bindings_v2 (
  binding_id uuid PRIMARY KEY,
  device_id uuid NOT NULL REFERENCES fiscal_device_registry(device_id),
  tenant_id text NOT NULL,
  location_id uuid,
  register_id uuid,
  roles text[] NOT NULL,
  binding_version bigint NOT NULL CHECK (binding_version > 0),
  state text NOT NULL CHECK (state IN ('PENDING','ACTIVE','REVOKED','SUPERSEDED')),
  activated_by text,
  active_from timestamptz,
  revoked_at timestamptz,
  revoke_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_device_active_binding_v2
  ON fiscal_device_bindings_v2(device_id) WHERE state IN ('PENDING','ACTIVE');
CREATE UNIQUE INDEX IF NOT EXISTS uq_register_active_fiscal_device_v2
  ON fiscal_device_bindings_v2(register_id)
  WHERE state = 'ACTIVE' AND roles @> ARRAY['FISCAL_DEVICE']::text[];

CREATE TABLE IF NOT EXISTS fiscal_actor_installations (
  installation_id uuid PRIMARY KEY,
  tenant_id text,
  actor_subject text NOT NULL,
  installation_type text NOT NULL CHECK (installation_type IN ('ADMIN','POS')),
  public_key_jwk jsonb NOT NULL,
  public_key_thumbprint varchar(64) NOT NULL UNIQUE,
  state text NOT NULL CHECK (state IN ('ACTIVE','REVOKED')),
  registered_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE IF NOT EXISTS fiscal_device_capabilities (
  capability_id uuid PRIMARY KEY,
  device_id uuid NOT NULL REFERENCES fiscal_device_registry(device_id),
  installation_id uuid NOT NULL REFERENCES fiscal_actor_installations(installation_id),
  tenant_id text NOT NULL,
  binding_version bigint NOT NULL,
  permissions text[] NOT NULL,
  signed_digest char(64) NOT NULL UNIQUE,
  state text NOT NULL CHECK (state IN ('ISSUED','ACTIVE','EXPIRED','REVOKED')),
  not_before timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  revoke_reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fiscal_device_auth_challenges (
  challenge_id uuid PRIMARY KEY,
  device_id uuid NOT NULL REFERENCES fiscal_device_registry(device_id),
  purpose text NOT NULL,
  nonce_hash char(64) NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fiscal_device_revocations (
  revocation_id uuid PRIMARY KEY,
  device_id uuid REFERENCES fiscal_device_registry(device_id),
  target_type text NOT NULL CHECK (target_type IN ('DEVICE','CAPABILITY','INSTALLATION','BINDING')),
  target_id text NOT NULL,
  revision bigint NOT NULL,
  reason text NOT NULL,
  actor_subject text NOT NULL,
  effective_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(target_type, target_id, revision)
);

ALTER TABLE fiscal_device_bindings_v2 ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal_actor_installations ENABLE ROW LEVEL SECURITY;
ALTER TABLE fiscal_device_capabilities ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS fiscal_device_bindings_tenant_isolation ON fiscal_device_bindings_v2;
CREATE POLICY fiscal_device_bindings_tenant_isolation ON fiscal_device_bindings_v2
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS fiscal_actor_installations_tenant_isolation ON fiscal_actor_installations;
CREATE POLICY fiscal_actor_installations_tenant_isolation ON fiscal_actor_installations
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS fiscal_device_capabilities_tenant_isolation ON fiscal_device_capabilities;
CREATE POLICY fiscal_device_capabilities_tenant_isolation ON fiscal_device_capabilities
  USING (tenant_id = current_setting('app.tenant_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

COMMIT;
