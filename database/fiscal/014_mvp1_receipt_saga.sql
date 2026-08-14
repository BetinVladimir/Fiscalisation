BEGIN;

CREATE TABLE IF NOT EXISTS client_operation (
 tenant_id text NOT NULL, client_operation_id uuid NOT NULL, operation_type text NOT NULL,
 sale_id text, receipt_session_id uuid, payload_sha256 char(64) NOT NULL CHECK(payload_sha256 ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('RESERVED','EXECUTING','COMMITTED','COMPENSATED','RECOVERY_REQUIRED','FAILED')),
 result jsonb, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,client_operation_id)
);
CREATE TABLE IF NOT EXISTS device_route (
 tenant_id text NOT NULL, route_id uuid NOT NULL, location_id text NOT NULL, register_id text NOT NULL,
 adapter_device_id text NOT NULL, adapter_kind text NOT NULL CHECK(adapter_kind IN ('BLUECASH_ANDROID','EDGE_AGENT_S3')),
 binding_generation bigint NOT NULL CHECK(binding_generation>0), status text NOT NULL CHECK(status IN ('ACTIVE','BLOCKED','RETIRED')),
 active_from timestamptz NOT NULL, active_to timestamptz, configuration jsonb NOT NULL,
 PRIMARY KEY(tenant_id,route_id), CHECK(active_to IS NULL OR active_to>active_from)
);
CREATE UNIQUE INDEX IF NOT EXISTS device_route_one_active_register ON device_route(tenant_id,register_id) WHERE status='ACTIVE' AND active_to IS NULL;
CREATE TABLE IF NOT EXISTS device_route_endpoint (
 tenant_id text NOT NULL, route_id uuid NOT NULL, endpoint_role text NOT NULL CHECK(endpoint_role IN ('FISCAL_DEVICE','PAYMENT_TERMINAL')),
 vendor text NOT NULL, model text NOT NULL, transport text NOT NULL CHECK(transport IN ('EMBEDDED_ANDROID','COM_RS232','BLE','USB_HOST')),
 device_identity text, configuration jsonb NOT NULL, PRIMARY KEY(tenant_id,route_id,endpoint_role),
 FOREIGN KEY(tenant_id,route_id) REFERENCES device_route(tenant_id,route_id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS receipt_session (
 tenant_id text NOT NULL, receipt_session_id uuid NOT NULL, client_operation_id uuid NOT NULL,
 sale_id text NOT NULL, route_id uuid NOT NULL, binding_generation bigint NOT NULL, unp text NOT NULL,
 plan_sha256 char(64) NOT NULL CHECK(plan_sha256 ~ '^[0-9a-f]{64}$'), ordered_plan jsonb NOT NULL,
 state text NOT NULL CHECK(state IN ('RESERVED','CARD_AUTHORIZING','CARD_APPROVED','FISCAL_OPENING','FISCAL_OPEN','LINES_REGISTERING','PAYMENTS_REGISTERING','FISCAL_CLOSING','COMMITTED','COMPENSATION_REQUIRED','CARD_REVERSING','COMPENSATED','RECOVERY_REQUIRED')),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,receipt_session_id), UNIQUE(tenant_id,client_operation_id),
 FOREIGN KEY(tenant_id,client_operation_id) REFERENCES client_operation(tenant_id,client_operation_id)
);
CREATE TABLE IF NOT EXISTS receipt_physical_step (
 tenant_id text NOT NULL, receipt_session_id uuid NOT NULL, step_id uuid NOT NULL, ordinal integer NOT NULL CHECK(ordinal>=0),
 step_type text NOT NULL, request_sha256 char(64) NOT NULL CHECK(request_sha256 ~ '^[0-9a-f]{64}$'),
 state text NOT NULL CHECK(state IN ('PREPARED','SENT','SUCCEEDED','DECLINED','UNKNOWN','COMPENSATED')),
 certainty text NOT NULL CHECK(certainty IN ('NOT_SENT','SENT_RESULT_KNOWN','SENT_RESULT_UNKNOWN')),
 request jsonb NOT NULL, result jsonb, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,receipt_session_id,step_id), UNIQUE(tenant_id,receipt_session_id,ordinal),
 FOREIGN KEY(tenant_id,receipt_session_id) REFERENCES receipt_session(tenant_id,receipt_session_id)
);
CREATE TABLE IF NOT EXISTS payment_transaction (
 tenant_id text NOT NULL, receipt_session_id uuid NOT NULL, payment_id uuid NOT NULL, step_id uuid NOT NULL,
 terminal_id text, amount_minor bigint NOT NULL CHECK(amount_minor>0), currency char(3) NOT NULL DEFAULT 'EUR' CHECK(currency='EUR'),
 stan text, rrn text, authorization_code text, response_code text,
 state text NOT NULL CHECK(state IN ('PREPARED','SENT','APPROVED','DECLINED','UNKNOWN','REVERSING','REVERSED')),
 evidence_sha256 char(64), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,payment_id), UNIQUE(tenant_id,receipt_session_id,step_id),
 FOREIGN KEY(tenant_id,receipt_session_id,step_id) REFERENCES receipt_physical_step(tenant_id,receipt_session_id,step_id)
);
CREATE TABLE IF NOT EXISTS transport_epoch (
 tenant_id text NOT NULL, sale_id text NOT NULL, epoch bigint NOT NULL CHECK(epoch>0),
 transport text NOT NULL CHECK(transport IN ('CLOUD_REST','DIRECT_BLE')), reason text NOT NULL,
 switched_at timestamptz NOT NULL, PRIMARY KEY(tenant_id,sale_id,epoch)
);
CREATE TABLE IF NOT EXISTS sync_quarantine (
 tenant_id text NOT NULL, device_id text NOT NULL, batch_id text NOT NULL, reason text NOT NULL,
 payload_sha256 char(64) NOT NULL, received_at timestamptz NOT NULL DEFAULT now(), payload jsonb NOT NULL,
 PRIMARY KEY(tenant_id,device_id,batch_id)
);

DO $$ DECLARE t text; BEGIN FOREACH t IN ARRAY ARRAY['client_operation','device_route','device_route_endpoint','receipt_session','receipt_physical_step','payment_transaction','transport_epoch','sync_quarantine'] LOOP EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t); EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t); EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I',t); EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=current_setting(''app.tenant_id'',true)) WITH CHECK (tenant_id=current_setting(''app.tenant_id'',true))',t); END LOOP; END $$;
COMMIT;
