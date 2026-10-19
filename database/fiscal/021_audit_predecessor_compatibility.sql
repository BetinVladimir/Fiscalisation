-- Correct derived predecessor metadata; never rewrite the signed audit payload.
-- Apply through the normal schema migration path, with an owner account and a
-- maintenance window: replacing a stored generated column requires table lock.
BEGIN;
LOCK TABLE fiscal_runtime_audit IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM fiscal_runtime_audit
    WHERE nullif(payload->>'previous_hash','') IS NOT NULL
      AND nullif(payload->>'prev_hash','') IS NOT NULL
      AND payload->>'previous_hash' <> payload->>'prev_hash') THEN
    RAISE EXCEPTION 'FISCAL_AUDIT_PREDECESSOR_CONFLICT';
  END IF;
END $$;
ALTER TABLE fiscal_runtime_audit ADD CONSTRAINT fiscal_audit_predecessor_consistent
  CHECK (nullif(payload->>'previous_hash','') IS NULL
    OR nullif(payload->>'prev_hash','') IS NULL
    OR payload->>'previous_hash' = payload->>'prev_hash');
DROP INDEX IF EXISTS fiscal_runtime_audit_tenant_chain;
ALTER TABLE fiscal_runtime_audit DROP COLUMN previous_hash;
ALTER TABLE fiscal_runtime_audit ADD COLUMN previous_hash text GENERATED ALWAYS AS
  (coalesce(nullif(payload->>'previous_hash',''),nullif(payload->>'prev_hash',''),'')) STORED;
CREATE INDEX fiscal_runtime_audit_tenant_chain
  ON fiscal_runtime_audit(tenant_id,occurred_at,event_id,previous_hash,event_hash);

CREATE OR REPLACE FUNCTION project_runtime_audit_to_supto_events() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_temp AS $$
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
      p,NEW.occurred_at,coalesce(nullif(p->>'previous_hash',''),nullif(p->>'prev_hash','')),NEW.event_hash,nullif(p->>'signature',''))
    ON CONFLICT (tenant_id,event_id) DO NOTHING;
  END IF;
  IF NEW.action IN ('LOGIN_SUCCEEDED','LOGIN_FAILED','LOGOUT','OPERATOR_CREATED','OPERATOR_CHANGED','OPERATOR_ROLE_CHANGED','OPERATOR_DEACTIVATED') THEN
    INSERT INTO operator_security_events(tenant_id,event_id,actor_subject,operator_id,event_type,source_app_instance_id,details,occurred_at,previous_hash,event_hash,signature)
    VALUES(tenant_uuid,event_uuid,NEW.actor_id,nullif(NEW.object_id,''),NEW.action,
           nullif(p#>>'{after,source_app_instance_id}',''),p,NEW.occurred_at,
           coalesce(nullif(p->>'previous_hash',''),nullif(p->>'prev_hash','')),NEW.event_hash,nullif(p->>'signature',''))
    ON CONFLICT (tenant_id,event_id) DO NOTHING;
  END IF;
  RETURN NEW;
END $$;

-- Typed historical projections are append-only and are not rewritten here.
-- Existing source payloads retain their original lexical field and event hash.
COMMIT;
