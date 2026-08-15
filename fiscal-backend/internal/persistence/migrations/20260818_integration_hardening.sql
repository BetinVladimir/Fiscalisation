ALTER TABLE external_enrollment_challenges
  ADD COLUMN IF NOT EXISTS purpose text NOT NULL DEFAULT 'ENROLLMENT',
  ADD COLUMN IF NOT EXISTS request_ip_hash bytea;

ALTER TABLE integration_commands
  ADD COLUMN IF NOT EXISTS processing_started_at timestamptz;

CREATE INDEX IF NOT EXISTS enrollment_rate_email
  ON external_enrollment_challenges(normalized_email, created_at DESC);
CREATE INDEX IF NOT EXISTS enrollment_rate_system
  ON external_enrollment_challenges(external_system_id, created_at DESC);
CREATE INDEX IF NOT EXISTS integration_commands_processing_lease
  ON integration_commands(processing_started_at)
  WHERE status='PROCESSING';
