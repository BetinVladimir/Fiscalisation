CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS external_systems (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), code text UNIQUE NOT NULL,
  display_name text NOT NULL, status text NOT NULL CHECK(status IN ('ACTIVE','SUSPENDED','REVOKED')),
  webhook_url text NOT NULL, webhook_events text[] NOT NULL DEFAULT '{}',
  webhook_signing_secret_ciphertext bytea NOT NULL, version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  created_by text NOT NULL, updated_by text NOT NULL
);
CREATE TABLE IF NOT EXISTS external_system_credentials (
  credential_id uuid PRIMARY KEY DEFAULT gen_random_uuid(), external_system_id uuid NOT NULL REFERENCES external_systems(id),
  secret_hash bytea NOT NULL, key_fingerprint text NOT NULL, version bigint NOT NULL DEFAULT 1,
  status text NOT NULL CHECK(status IN ('ACTIVE','REVOKED')), created_at timestamptz NOT NULL DEFAULT now(),
  created_by text NOT NULL, revoked_at timestamptz, revoked_by text, revoke_reason text, last_used_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS external_system_one_active_key ON external_system_credentials(external_system_id) WHERE status='ACTIVE';
CREATE TABLE IF NOT EXISTS external_system_audit_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), external_system_id uuid NOT NULL,
  action text NOT NULL, actor_subject text NOT NULL, request_id text NOT NULL,
  before_redacted jsonb, after_redacted jsonb, occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS external_enrollment_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), external_system_id uuid NOT NULL REFERENCES external_systems(id),
  source_company_id text NOT NULL, normalized_email text NOT NULL,
  tax_country char(2) NOT NULL, tax_type text NOT NULL, tax_normalized_value text NOT NULL,
  temporary_token_hash bytea UNIQUE NOT NULL, otp_hash bytea NOT NULL, payload_hash bytea NOT NULL,
  legal_profile jsonb NOT NULL, idempotency_key text NOT NULL, response_ciphertext bytea,
  attempts integer NOT NULL DEFAULT 0, status text NOT NULL CHECK(status IN ('PENDING','VERIFIED','EXPIRED','LOCKED','CANCELLED','CONFLICT')),
  expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), verified_at timestamptz, tenant_id uuid,
  UNIQUE(external_system_id,idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS enrollment_one_active_source ON external_enrollment_challenges(external_system_id,source_company_id) WHERE status='PENDING';
CREATE TABLE IF NOT EXISTS tenant_source_bindings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, external_system_id uuid NOT NULL REFERENCES external_systems(id),
  source_company_id text NOT NULL, tax_country char(2) NOT NULL, tax_type text NOT NULL, tax_normalized_value text NOT NULL,
  source_metadata jsonb NOT NULL DEFAULT '{}', status text NOT NULL CHECK(status IN ('ACTIVE','SUSPENDED','REVOKED')),
  version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(external_system_id,source_company_id), UNIQUE(tenant_id,external_system_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_unique_tax_identity ON tenant_source_bindings(tax_country,tax_type,tax_normalized_value) WHERE status<>'REVOKED';
CREATE TABLE IF NOT EXISTS tenant_integration_credentials (
  credential_id uuid PRIMARY KEY DEFAULT gen_random_uuid(), binding_id uuid NOT NULL REFERENCES tenant_source_bindings(id),
  secret_hash bytea NOT NULL, scopes text[] NOT NULL, status text NOT NULL CHECK(status IN ('ACTIVE','REVOKED')),
  created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
  last_used_at timestamptz, revoked_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS tenant_one_active_credential ON tenant_integration_credentials(binding_id) WHERE status='ACTIVE';
CREATE TABLE IF NOT EXISTS integration_commands (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, external_system_id uuid NOT NULL,
  idempotency_key text NOT NULL, http_method text NOT NULL, resource_type text NOT NULL,
  aggregate_source_id text NOT NULL, source_version bigint NOT NULL, payload jsonb NOT NULL, payload_hash bytea NOT NULL,
  authenticated_system_id uuid NOT NULL, asserted_actor_type text NOT NULL CHECK(asserted_actor_type IN ('USER','SERVICE')),
  asserted_actor_id text NOT NULL, asserted_actor_session_id text,
  status text NOT NULL CHECK(status IN ('ACCEPTED','QUEUED','PROCESSING','SUCCEEDED','FAILED','DEAD','SUPERSEDED')),
  attempts integer NOT NULL DEFAULT 0, result jsonb, last_error_code text, last_error_detail text,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(external_system_id,tenant_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS integration_commands_resource_version ON integration_commands(tenant_id,resource_type,aggregate_source_id,source_version DESC);
CREATE TABLE IF NOT EXISTS integration_command_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), command_id uuid UNIQUE NOT NULL REFERENCES integration_commands(id),
  topic text NOT NULL, payload jsonb NOT NULL, status text NOT NULL CHECK(status IN ('PENDING','LEASED','PUBLISHED','FAILED')),
  attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), lease_id uuid, lease_until timestamptz,
  published_at timestamptz, last_error text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS integration_outbox_ready ON integration_command_outbox(available_at,id) WHERE status IN ('PENDING','FAILED');
CREATE TABLE IF NOT EXISTS webhook_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), event_id uuid NOT NULL, external_system_id uuid NOT NULL, tenant_id uuid NOT NULL,
  event_type text NOT NULL, payload jsonb NOT NULL, payload_hash bytea NOT NULL,
  status text NOT NULL CHECK(status IN ('PENDING','LEASED','QUEUED','DELIVERING','DELIVERED','RETRY','DEAD')),
  attempts integer NOT NULL DEFAULT 0, next_attempt_at timestamptz NOT NULL DEFAULT now(), lease_id uuid, lease_until timestamptz,
  last_http_status integer, last_error_code text, last_error_detail text, delivered_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), UNIQUE(event_id,external_system_id)
);
CREATE INDEX IF NOT EXISTS webhook_deliveries_ready ON webhook_deliveries(next_attempt_at,id) WHERE status IN ('PENDING','RETRY');
CREATE TABLE IF NOT EXISTS integration_change_journal (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, external_system_id uuid,
  authenticated_system_id uuid, asserted_actor_type text, asserted_actor_id text, asserted_actor_session_id text,
  fiscal_platform_actor_subject text, operation_id uuid, idempotency_key text, resource_type text NOT NULL,
  source_entity_id text, action text NOT NULL, outcome text NOT NULL,
  before_redacted jsonb, after_redacted jsonb, change_diff_redacted jsonb, reason_code text,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE OR REPLACE FUNCTION integration_journal_immutable() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'integration journal is append-only'; END $$;
DROP TRIGGER IF EXISTS integration_change_journal_no_update ON integration_change_journal;
CREATE TRIGGER integration_change_journal_no_update BEFORE UPDATE OR DELETE ON integration_change_journal FOR EACH ROW EXECUTE FUNCTION integration_journal_immutable();
CREATE TABLE IF NOT EXISTS integration_idempotency_replays (
  external_system_id uuid NOT NULL, scope text NOT NULL, idempotency_key text NOT NULL,
  request_hash bytea NOT NULL, response_status integer NOT NULL, response_ciphertext bytea NOT NULL,
  expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(external_system_id,scope,idempotency_key)
);
CREATE TABLE IF NOT EXISTS fiscal_email_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), purpose text NOT NULL, recipient text NOT NULL,
  subject text NOT NULL, body_text text NOT NULL, status text NOT NULL DEFAULT 'PENDING', attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL DEFAULT now(), sent_at timestamptz, last_error text,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS tenant_user_memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, user_id uuid NOT NULL DEFAULT gen_random_uuid(),
  normalized_email text NOT NULL, display_name text, roles text[] NOT NULL,
  status text NOT NULL CHECK(status IN ('ACTIVE','SUSPENDED','REVOKED')), version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz,
  UNIQUE(tenant_id,normalized_email), UNIQUE(tenant_id,user_id)
);
CREATE INDEX IF NOT EXISTS tenant_memberships_email ON tenant_user_memberships(normalized_email) WHERE status='ACTIVE';
CREATE TABLE IF NOT EXISTS app_auth_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), normalized_email text NOT NULL, temporary_token_hash bytea UNIQUE NOT NULL,
  otp_hash bytea, attempts integer NOT NULL DEFAULT 0,
  status text NOT NULL CHECK(status IN ('PENDING','VERIFIED','EXPIRED','LOCKED','CANCELLED','UNKNOWN_EMAIL')),
  expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), verified_at timestamptz,
  request_ip_hash bytea, app_instance_id uuid NOT NULL
);
CREATE TABLE IF NOT EXISTS app_auth_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL, normalized_email text NOT NULL,
  selected_tenant_id uuid, refresh_token_hash bytea UNIQUE NOT NULL, app_instance_id uuid NOT NULL,
  status text NOT NULL CHECK(status IN ('ACTIVE','REVOKED','EXPIRED')), created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL, last_used_at timestamptz, revoked_at timestamptz
);
CREATE TABLE IF NOT EXISTS app_issued_tokens (
  jti uuid PRIMARY KEY, session_id uuid NOT NULL REFERENCES app_auth_sessions(id) ON DELETE CASCADE,
  tenant_id uuid NOT NULL, issued_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
  status text NOT NULL CHECK(status IN ('ACTIVE','REVOKED','EXPIRED')), revoked_at timestamptz, revoke_reason text
);
