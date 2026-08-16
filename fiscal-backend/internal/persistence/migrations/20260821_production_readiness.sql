ALTER TABLE external_enrollment_challenges
  ADD COLUMN IF NOT EXISTS verification_idempotency_key text,
  ADD COLUMN IF NOT EXISTS verification_request_hash bytea;

CREATE UNIQUE INDEX IF NOT EXISTS enrollment_verification_idempotency
  ON external_enrollment_challenges(external_system_id,verification_idempotency_key)
  WHERE verification_idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS enrollment_rate_source_company
  ON external_enrollment_challenges(external_system_id,source_company_id,created_at DESC);
