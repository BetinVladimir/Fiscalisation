ALTER TABLE fiscal_email_outbox ADD COLUMN IF NOT EXISTS lease_until timestamptz;

ALTER TABLE webhook_deliveries
  ADD COLUMN IF NOT EXISTS destination_url text,
  ADD COLUMN IF NOT EXISTS signing_secret_ciphertext bytea,
  ADD COLUMN IF NOT EXISTS signing_key_id text;

CREATE TABLE IF NOT EXISTS webhook_delivery_attempts(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  delivery_id uuid NOT NULL REFERENCES webhook_deliveries(id),
  attempt_number integer NOT NULL,
  outcome text NOT NULL CHECK(outcome IN ('DELIVERED','FAILED')),
  destination_url text NOT NULL,
  http_status integer,
  error_code text,
  error_detail text,
  attempted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(delivery_id,attempt_number)
);
CREATE INDEX IF NOT EXISTS webhook_delivery_attempts_delivery ON webhook_delivery_attempts(delivery_id,attempt_number);

CREATE INDEX IF NOT EXISTS fiscal_email_outbox_recoverable
  ON fiscal_email_outbox(available_at,id)
  WHERE status IN ('PENDING','FAILED','SENDING');
