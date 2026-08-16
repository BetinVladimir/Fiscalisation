CREATE TABLE IF NOT EXISTS integration_command_outbox_archive
  (LIKE integration_command_outbox INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING IDENTITY);
CREATE UNIQUE INDEX IF NOT EXISTS integration_command_outbox_archive_id ON integration_command_outbox_archive(id);
CREATE TABLE IF NOT EXISTS integration_commands_archive
  (LIKE integration_commands INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING IDENTITY);
CREATE UNIQUE INDEX IF NOT EXISTS integration_commands_archive_id ON integration_commands_archive(id);
CREATE TABLE IF NOT EXISTS webhook_delivery_attempts_archive
  (LIKE webhook_delivery_attempts INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING IDENTITY);
CREATE UNIQUE INDEX IF NOT EXISTS webhook_delivery_attempts_archive_id ON webhook_delivery_attempts_archive(id);
CREATE TABLE IF NOT EXISTS webhook_deliveries_archive
  (LIKE webhook_deliveries INCLUDING DEFAULTS INCLUDING GENERATED INCLUDING IDENTITY);
CREATE UNIQUE INDEX IF NOT EXISTS webhook_deliveries_archive_id ON webhook_deliveries_archive(id);
CREATE TABLE IF NOT EXISTS integration_retention_runs(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), started_at timestamptz NOT NULL,
  completed_at timestamptz NOT NULL DEFAULT now(), moved jsonb NOT NULL,
  policy_version text NOT NULL DEFAULT '2026-08-16-v1'
);

CREATE OR REPLACE FUNCTION archive_integration_operational_rows(batch_size integer DEFAULT 1000)
RETURNS TABLE(kind text,moved bigint) LANGUAGE plpgsql AS $$
DECLARE n bigint;
BEGIN
  IF batch_size<1 OR batch_size>10000 THEN RAISE EXCEPTION 'invalid retention batch size'; END IF;

  WITH selected AS (
    SELECT o.id FROM integration_command_outbox o JOIN integration_commands c ON c.id=o.command_id
    WHERE o.status='PUBLISHED' AND c.status IN ('SUCCEEDED','FAILED','DEAD','SUPERSEDED') AND o.updated_at<now()-interval '30 days'
    ORDER BY o.updated_at,o.id LIMIT batch_size FOR UPDATE OF o SKIP LOCKED
  ), archived AS (
    INSERT INTO integration_command_outbox_archive SELECT o.* FROM integration_command_outbox o JOIN selected s USING(id)
    ON CONFLICT(id) DO NOTHING RETURNING id
  ) DELETE FROM integration_command_outbox o USING selected s WHERE o.id=s.id AND (EXISTS(SELECT 1 FROM archived a WHERE a.id=o.id) OR EXISTS(SELECT 1 FROM integration_command_outbox_archive a WHERE a.id=o.id));
  GET DIAGNOSTICS n=ROW_COUNT; kind:='command_outbox'; moved:=n; RETURN NEXT;

  WITH selected AS (
    SELECT id FROM integration_commands c WHERE c.status IN ('SUCCEEDED','FAILED','DEAD','SUPERSEDED')
      AND c.updated_at<now()-interval '90 days' AND NOT EXISTS(SELECT 1 FROM integration_command_outbox o WHERE o.command_id=c.id)
    ORDER BY c.updated_at,c.id LIMIT batch_size FOR UPDATE SKIP LOCKED
  ), archived AS (
    INSERT INTO integration_commands_archive SELECT c.* FROM integration_commands c JOIN selected s USING(id)
    ON CONFLICT(id) DO NOTHING RETURNING id
  ) DELETE FROM integration_commands c USING selected s WHERE c.id=s.id AND (EXISTS(SELECT 1 FROM archived a WHERE a.id=c.id) OR EXISTS(SELECT 1 FROM integration_commands_archive a WHERE a.id=c.id));
  GET DIAGNOSTICS n=ROW_COUNT; kind:='commands'; moved:=n; RETURN NEXT;

  WITH selected AS (
    SELECT id FROM webhook_deliveries d WHERE d.status IN ('DELIVERED','DEAD') AND d.updated_at<now()-interval '90 days'
    ORDER BY d.updated_at,d.id LIMIT batch_size FOR UPDATE SKIP LOCKED
  ), archived AS (
    INSERT INTO webhook_delivery_attempts_archive SELECT a.* FROM webhook_delivery_attempts a JOIN selected s ON s.id=a.delivery_id
    ON CONFLICT(id) DO NOTHING RETURNING id
  ) DELETE FROM webhook_delivery_attempts a USING selected s WHERE a.delivery_id=s.id AND (EXISTS(SELECT 1 FROM archived x WHERE x.id=a.id) OR EXISTS(SELECT 1 FROM webhook_delivery_attempts_archive x WHERE x.id=a.id));
  GET DIAGNOSTICS n=ROW_COUNT; kind:='webhook_attempts'; moved:=n; RETURN NEXT;

  WITH selected AS (
    SELECT id FROM webhook_deliveries d WHERE d.status IN ('DELIVERED','DEAD') AND d.updated_at<now()-interval '90 days'
      AND NOT EXISTS(SELECT 1 FROM webhook_delivery_attempts a WHERE a.delivery_id=d.id)
    ORDER BY d.updated_at,d.id LIMIT batch_size FOR UPDATE SKIP LOCKED
  ), archived AS (
    INSERT INTO webhook_deliveries_archive SELECT d.* FROM webhook_deliveries d JOIN selected s USING(id)
    ON CONFLICT(id) DO NOTHING RETURNING id
  ) DELETE FROM webhook_deliveries d USING selected s WHERE d.id=s.id AND (EXISTS(SELECT 1 FROM archived a WHERE a.id=d.id) OR EXISTS(SELECT 1 FROM webhook_deliveries_archive a WHERE a.id=d.id));
  GET DIAGNOSTICS n=ROW_COUNT; kind:='webhook_deliveries'; moved:=n; RETURN NEXT;

  WITH selected AS (
    SELECT external_system_id,scope,idempotency_key FROM integration_idempotency_replays
    WHERE expires_at<now() AND created_at<now()-interval '7 days'
    ORDER BY created_at LIMIT batch_size FOR UPDATE SKIP LOCKED
  ) DELETE FROM integration_idempotency_replays r USING selected s
    WHERE r.external_system_id=s.external_system_id AND r.scope=s.scope AND r.idempotency_key=s.idempotency_key;
  GET DIAGNOSTICS n=ROW_COUNT; kind:='expired_replays'; moved:=n; RETURN NEXT;
END $$;
