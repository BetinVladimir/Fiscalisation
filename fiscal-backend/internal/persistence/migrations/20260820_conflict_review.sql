DROP INDEX IF EXISTS tenant_unique_tax_identity;
CREATE UNIQUE INDEX tenant_unique_tax_identity
  ON tenant_source_bindings(tax_country,tax_type,tax_normalized_value)
  WHERE status='ACTIVE';

CREATE TABLE IF NOT EXISTS enrollment_conflict_decisions(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  challenge_id uuid NOT NULL UNIQUE REFERENCES external_enrollment_challenges(id),
  existing_binding_id uuid NOT NULL REFERENCES tenant_source_bindings(id),
  decision text NOT NULL CHECK(decision IN ('KEEP_EXISTING','REPLACE_EXISTING')),
  reason text NOT NULL,
  actor_subject text NOT NULL,
  request_id text NOT NULL,
  before_redacted jsonb NOT NULL,
  after_redacted jsonb NOT NULL,
  decided_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE external_enrollment_challenges ADD COLUMN IF NOT EXISTS tax_policy_version integer NOT NULL DEFAULT 1;
CREATE TABLE IF NOT EXISTS integration_security_events(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), external_system_id uuid, tenant_id uuid,
  event_type text NOT NULL, actor_type text, actor_id text, request_id text,
  detail_redacted jsonb, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS integration_security_events_time ON integration_security_events(occurred_at,event_type);
