-- BG SUPTO full-profile append-only evidence foundation.
CREATE TABLE IF NOT EXISTS country_identifier_schemes (
  country_code char(2) NOT NULL,
  scheme text NOT NULL,
  version text NOT NULL,
  authority_owner text NOT NULL CHECK (authority_owner IN ('FISCAL_CORE','EDGE','FISCAL_DEVICE','EXTERNAL_SERVICE')),
  definition jsonb NOT NULL,
  definition_sha256 char(64) NOT NULL CHECK (definition_sha256 ~ '^[0-9a-f]{64}$'),
  effective_from timestamptz NOT NULL,
  effective_to timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (country_code, scheme, version),
  CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE IF NOT EXISTS sale_events (
  tenant_id uuid NOT NULL,
  event_id uuid NOT NULL,
  sale_id uuid NOT NULL,
  event_type text NOT NULL,
  sale_version bigint NOT NULL CHECK (sale_version > 0),
  actor_subject text NOT NULL,
  operator_id text,
  operator_code varchar(4),
  workstation_id uuid,
  fiscal_device_id uuid,
  fiscal_device_number varchar(8),
  unp text,
  client_surrogate_id uuid,
  operation_id uuid,
  idempotency_key text,
  source_component text NOT NULL,
  source_version text NOT NULL,
  country_code char(2) NOT NULL DEFAULT 'BG',
  country_profile_version text NOT NULL,
  payload jsonb NOT NULL,
  occurred_at timestamptz NOT NULL,
  previous_hash char(64),
  event_hash char(64) NOT NULL CHECK (event_hash ~ '^[0-9a-f]{64}$'),
  signature text,
  PRIMARY KEY (tenant_id, event_id),
  UNIQUE (tenant_id, sale_id, sale_version, event_type, event_id),
  UNIQUE (tenant_id, event_hash),
  CHECK (operator_code IS NULL OR operator_code ~ '^[A-Za-z0-9]{4}$'),
  CHECK (fiscal_device_number IS NULL OR fiscal_device_number ~ '^[A-Za-z0-9]{8}$')
);
CREATE INDEX IF NOT EXISTS sale_events_sale_order_idx ON sale_events (tenant_id, sale_id, occurred_at, event_id);
CREATE INDEX IF NOT EXISTS sale_events_unp_idx ON sale_events (tenant_id, unp) WHERE unp IS NOT NULL;

CREATE TABLE IF NOT EXISTS operator_security_events (
  tenant_id uuid NOT NULL,
  event_id uuid NOT NULL,
  actor_subject text NOT NULL,
  operator_id text,
  event_type text NOT NULL CHECK (event_type IN ('LOGIN_SUCCEEDED','LOGIN_FAILED','LOGOUT','OPERATOR_CREATED','OPERATOR_CHANGED','OPERATOR_ROLE_CHANGED','OPERATOR_DEACTIVATED')),
  source_app_instance_id text,
  workstation_id uuid,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL,
  previous_hash char(64),
  event_hash char(64) NOT NULL CHECK (event_hash ~ '^[0-9a-f]{64}$'),
  signature text,
  PRIMARY KEY (tenant_id, event_id),
  UNIQUE (tenant_id, event_hash)
);

CREATE TABLE IF NOT EXISTS supto_evidence_manifests (
  manifest_id uuid PRIMARY KEY,
  profile_id text NOT NULL,
  profile_version text NOT NULL,
  evidence_class text NOT NULL CHECK (evidence_class IN ('SOFTWARE','HIL','RELEASE','LEGAL','DECLARATION')),
  status text NOT NULL CHECK (status IN ('VALID','INVALID','EXPIRED','REVOKED')),
  payload jsonb NOT NULL,
  payload_sha256 char(64) NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
  signer_key_id text NOT NULL,
  signature text NOT NULL,
  issued_at timestamptz NOT NULL,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (profile_id, profile_version, evidence_class, payload_sha256),
  CHECK (expires_at IS NULL OR expires_at > issued_at)
);

CREATE TABLE IF NOT EXISTS device_compliance_profiles (
  profile_id uuid PRIMARY KEY,
  vendor text NOT NULL,
  model text NOT NULL,
  firmware_range text NOT NULL,
  fiscal_protocol text,
  payment_protocol text,
  required_capabilities jsonb NOT NULL,
  capability_evidence jsonb NOT NULL,
  status text NOT NULL CHECK (status IN ('DRAFT','SOFTWARE_VERIFIED','APPROVED_FOR_PROD','REVOKED')),
  evidence_manifest_id uuid REFERENCES supto_evidence_manifests(manifest_id),
  effective_from timestamptz NOT NULL,
  effective_to timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (vendor, model, firmware_range),
  CHECK (effective_to IS NULL OR effective_to > effective_from),
  CHECK (status <> 'APPROVED_FOR_PROD' OR evidence_manifest_id IS NOT NULL)
);

ALTER TABLE sale_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE sale_events FORCE ROW LEVEL SECURITY;
ALTER TABLE operator_security_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE operator_security_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sale_events;
CREATE POLICY tenant_isolation ON sale_events
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);
DROP POLICY IF EXISTS tenant_isolation ON operator_security_events;
CREATE POLICY tenant_isolation ON operator_security_events
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

REVOKE UPDATE, DELETE, TRUNCATE ON sale_events, operator_security_events,
  supto_evidence_manifests, country_identifier_schemes FROM PUBLIC;

CREATE OR REPLACE FUNCTION reject_append_only_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '% is append-only', TG_TABLE_NAME; END $$;

DROP TRIGGER IF EXISTS sale_events_append_only ON sale_events;
CREATE TRIGGER sale_events_append_only BEFORE UPDATE OR DELETE ON sale_events
FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();
DROP TRIGGER IF EXISTS operator_security_events_append_only ON operator_security_events;
CREATE TRIGGER operator_security_events_append_only BEFORE UPDATE OR DELETE ON operator_security_events
FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();
DROP TRIGGER IF EXISTS regulatory_identifier_bindings_append_only ON regulatory_identifier_bindings;
CREATE TRIGGER regulatory_identifier_bindings_append_only BEFORE UPDATE OR DELETE ON regulatory_identifier_bindings
FOR EACH ROW EXECUTE FUNCTION reject_append_only_mutation();

-- The runtime repository remains the transactional writer during the staged
-- migration. These triggers provide the required typed dual-write without a
-- second application transaction and make replay idempotent by event_id.
CREATE OR REPLACE FUNCTION project_runtime_audit_to_supto_events() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  p jsonb := NEW.payload;
  sale_uuid uuid;
  tenant_uuid uuid;
  event_uuid uuid;
  version_value bigint;
BEGIN
  IF NEW.tenant_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
     OR NEW.event_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' THEN
    RETURN NEW;
  END IF;
  tenant_uuid := NEW.tenant_id::uuid;
  event_uuid := NEW.event_id::uuid;
  IF NEW.object_type = 'sale'
     AND NEW.object_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' THEN
    sale_uuid := NEW.object_id::uuid;
    version_value := greatest(1, coalesce(nullif(p#>>'{after,version}','')::bigint,
                                           nullif(p#>>'{before,version}','')::bigint, 1));
    INSERT INTO sale_events(
      tenant_id,event_id,sale_id,event_type,sale_version,actor_subject,
      operator_id,operator_code,workstation_id,fiscal_device_id,
      fiscal_device_number,unp,client_surrogate_id,operation_id,idempotency_key,
      source_component,source_version,country_code,country_profile_version,
      payload,occurred_at,previous_hash,event_hash,signature)
    VALUES(
      tenant_uuid,event_uuid,sale_uuid,NEW.action,version_value,NEW.actor_id,
      nullif(p#>>'{after,operator_id}',''),nullif(p#>>'{after,operator_snapshot,code}',''),
      CASE WHEN coalesce(p#>>'{after,register_id}','') ~* '^[0-9a-f-]{36}$' THEN (p#>>'{after,register_id}')::uuid END,
      CASE WHEN coalesce(p#>>'{after,fiscal_device_snapshot,device_id}','') ~* '^[0-9a-f-]{36}$' THEN (p#>>'{after,fiscal_device_snapshot,device_id}')::uuid END,
      nullif(p#>>'{after,fiscal_device_snapshot,fiscal_device_number}',''),NEW.payload->>'unp',
      CASE WHEN coalesce(p#>>'{after,client_sale_surrogate_id}','') ~* '^[0-9a-f-]{36}$' THEN (p#>>'{after,client_sale_surrogate_id}')::uuid END,
      CASE WHEN coalesce(p#>>'{after,fiscal_operation_id}','') ~* '^[0-9a-f-]{36}$' THEN (p#>>'{after,fiscal_operation_id}')::uuid END,
      null,'fiscal-backend','2026-08-14','BG',coalesce(nullif(p#>>'{after,country_profile_version}',''),'bg-supto-full-2026-08-10'),
      p,NEW.occurred_at,nullif(p->>'prev_hash',''),NEW.event_hash,nullif(p->>'signature',''))
    ON CONFLICT (tenant_id,event_id) DO NOTHING;
  END IF;
  IF NEW.action IN ('LOGIN_SUCCEEDED','LOGIN_FAILED','LOGOUT','OPERATOR_CREATED','OPERATOR_CHANGED','OPERATOR_ROLE_CHANGED','OPERATOR_DEACTIVATED') THEN
    INSERT INTO operator_security_events(tenant_id,event_id,actor_subject,operator_id,event_type,source_app_instance_id,details,occurred_at,previous_hash,event_hash,signature)
    VALUES(tenant_uuid,event_uuid,NEW.actor_id,nullif(NEW.object_id,''),NEW.action,
           nullif(p#>>'{after,source_app_instance_id}',''),p,NEW.occurred_at,
           nullif(p->>'prev_hash',''),NEW.event_hash,nullif(p->>'signature',''))
    ON CONFLICT (tenant_id,event_id) DO NOTHING;
  END IF;
  RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS fiscal_runtime_audit_supto_projection ON fiscal_runtime_audit;
CREATE TRIGGER fiscal_runtime_audit_supto_projection
AFTER INSERT OR UPDATE ON fiscal_runtime_audit
FOR EACH ROW EXECUTE FUNCTION project_runtime_audit_to_supto_events();
