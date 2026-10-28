-- Derived projection only. Source audit payload, hashes and append-only rows are never rewritten.
BEGIN;
SET LOCAL search_path=fiscal,pg_temp;
LOCK TABLE fiscal.fiscal_runtime_audit, fiscal.sale_events, fiscal.operator_security_events IN SHARE ROW EXCLUSIVE MODE;
DROP POLICY tenant_isolation ON fiscal.sale_events;
DROP POLICY tenant_isolation ON fiscal.operator_security_events;
ALTER TABLE fiscal.sale_events
 ALTER tenant_id TYPE text USING tenant_id::text, ALTER event_id TYPE text USING event_id::text,
 ALTER sale_id TYPE text USING sale_id::text, ALTER workstation_id TYPE text USING workstation_id::text,
 ALTER fiscal_device_id TYPE text USING fiscal_device_id::text, ALTER client_surrogate_id TYPE text USING client_surrogate_id::text,
 ALTER operation_id TYPE text USING operation_id::text;
ALTER TABLE fiscal.operator_security_events
 ALTER tenant_id TYPE text USING tenant_id::text, ALTER event_id TYPE text USING event_id::text,
 ALTER workstation_id TYPE text USING workstation_id::text;
CREATE POLICY tenant_isolation ON fiscal.sale_events USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));
CREATE POLICY tenant_isolation ON fiscal.operator_security_events USING(tenant_id=current_setting('app.tenant_id',true)) WITH CHECK(tenant_id=current_setting('app.tenant_id',true));

CREATE TABLE fiscal.typed_projection_diagnostics (
 tenant_id text NOT NULL, event_id text NOT NULL, projection_version integer NOT NULL CHECK(projection_version=2),
 source_event_hash text NOT NULL, reason text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,event_id,projection_version)
);
CREATE TABLE fiscal.typed_projection_corrections (
 tenant_id text NOT NULL, event_id text NOT NULL, projection_version integer NOT NULL CHECK(projection_version=2),
 event_kind text NOT NULL CHECK(event_kind IN ('sale','operator')), source_event_hash text NOT NULL,
 projection jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,event_id,projection_version),
 CHECK(projection->>'tenant_id'=tenant_id AND projection->>'event_id'=event_id AND projection->>'event_hash'=source_event_hash)
);
CREATE TABLE fiscal.typed_projection_reconciliation_runs (
 tenant_id text NOT NULL, range_from text NOT NULL, range_until text NOT NULL, source_sha256 text NOT NULL CHECK(source_sha256 ~ '^[0-9a-f]{64}$'),
 projection_version integer NOT NULL CHECK(projection_version=2), source_count bigint NOT NULL, inserted_count bigint NOT NULL,
 correction_count bigint NOT NULL, diagnostic_count bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,range_from,range_until,source_sha256,projection_version)
);
DO $$ DECLARE t text; BEGIN
 FOREACH t IN ARRAY ARRAY['typed_projection_diagnostics','typed_projection_corrections','typed_projection_reconciliation_runs'] LOOP
  EXECUTE format('ALTER TABLE fiscal.%I ENABLE ROW LEVEL SECURITY',t);
  EXECUTE format('ALTER TABLE fiscal.%I FORCE ROW LEVEL SECURITY',t);
  EXECUTE format('CREATE POLICY tenant_isolation ON fiscal.%I USING(tenant_id=current_setting(''app.tenant_id'',true)) WITH CHECK(tenant_id=current_setting(''app.tenant_id'',true))',t);
  EXECUTE format('CREATE TRIGGER append_only BEFORE UPDATE OR DELETE ON fiscal.%I FOR EACH ROW EXECUTE FUNCTION fiscal.reject_append_only_mutation()',t);
 END LOOP;
END $$;

-- Pure normalizer shared by future writes and explicit historical reconciliation.
CREATE FUNCTION fiscal.normalize_runtime_audit_v2(a fiscal.fiscal_runtime_audit) RETURNS jsonb
LANGUAGE plpgsql IMMUTABLE SET search_path=fiscal,pg_temp SET timezone='UTC' SET datestyle='ISO, YMD' AS $$
DECLARE p jsonb:=a.payload; s jsonb; fd jsonb; version_value bigint; r jsonb; kind text;
BEGIN
 IF a.tenant_id='' OR a.event_id='' OR a.object_id='' THEN RETURN jsonb_build_object('status','EMPTY_SOURCE_ID'); END IF;
 IF a.object_type='sale' THEN
  kind:='sale'; s:=coalesce(nullif(p->'after','null'::jsonb),nullif(p->'before','null'::jsonb));
  IF jsonb_typeof(s)<>'object' OR coalesce(s->>'version','') !~ '^[1-9][0-9]*$' THEN RETURN jsonb_build_object('status','NOT_A_VERSIONED_SALE_SNAPSHOT'); END IF;
  IF (s->>'version')::numeric>9223372036854775807 THEN RETURN jsonb_build_object('status','INVALID_SALE_VERSION'); END IF;
  version_value:=(s->>'version')::bigint;
  IF nullif(s->>'sale_id','') IS DISTINCT FROM a.object_id THEN RETURN jsonb_build_object('status','SALE_ID_MISMATCH'); END IF;
  IF nullif(s->>'tenant_id','') IS DISTINCT FROM a.tenant_id THEN RETURN jsonb_build_object('status','SALE_TENANT_MISMATCH'); END IF;
  IF s ? 'fiscal_device' AND s ? 'fiscal_device_snapshot' AND s->'fiscal_device' IS DISTINCT FROM s->'fiscal_device_snapshot' THEN RETURN jsonb_build_object('status','CONFLICTING_DEVICE_SNAPSHOTS'); END IF;
  IF s ? 'operator_code' AND s#>>'{operator_snapshot,code}' IS NOT NULL AND s->>'operator_code' IS DISTINCT FROM s#>>'{operator_snapshot,code}' THEN RETURN jsonb_build_object('status','CONFLICTING_OPERATOR_SNAPSHOTS'); END IF;
  fd:=coalesce(s->'fiscal_device',s->'fiscal_device_snapshot','{}'::jsonb);
  IF coalesce(nullif(s->>'operator_code',''),nullif(s#>>'{operator_snapshot,code}','')) IS NOT NULL AND coalesce(nullif(s->>'operator_code',''),nullif(s#>>'{operator_snapshot,code}','')) !~ '^[A-Za-z0-9]{4}$' THEN RETURN jsonb_build_object('status','INVALID_OPERATOR_CODE'); END IF;
  IF nullif(fd->>'fiscal_device_number','') IS NOT NULL AND fd->>'fiscal_device_number' !~ '^[A-Za-z0-9]{8}$' THEN RETURN jsonb_build_object('status','INVALID_FISCAL_SERIAL'); END IF;
  r:=jsonb_build_object('tenant_id',a.tenant_id,'event_id',a.event_id,'sale_id',a.object_id,'event_type',a.action,'sale_version',version_value,'actor_subject',a.actor_id,
   'operator_id',nullif(s->>'operator_id',''),'operator_code',coalesce(nullif(s->>'operator_code',''),nullif(s#>>'{operator_snapshot,code}','')),
   'workstation_id',nullif(s->>'register_id',''),'fiscal_device_id',nullif(fd->>'device_id',''),'fiscal_device_number',nullif(fd->>'fiscal_device_number',''),
   'unp',nullif(p->>'unp',''),'client_surrogate_id',nullif(s->>'client_sale_surrogate_id',''),'operation_id',nullif(s->>'fiscal_operation_id',''),
   'idempotency_key',null,'source_component','fiscal-backend','source_version','typed-projection-v2','country_code','BG',
   'country_profile_version',coalesce(nullif(s->>'country_profile_version',''),'bg-supto-full-2026-08-10'),
   'payload',p,'occurred_at',a.occurred_at,'previous_hash',coalesce(nullif(p->>'previous_hash',''),nullif(p->>'prev_hash','')),'event_hash',a.event_hash,'signature',nullif(p->>'signature',''));
 ELSIF a.action IN ('LOGIN_SUCCEEDED','LOGIN_FAILED','LOGOUT','OPERATOR_CREATED','OPERATOR_CHANGED','OPERATOR_ROLE_CHANGED','OPERATOR_DEACTIVATED') THEN
  kind:='operator'; r:=jsonb_build_object('tenant_id',a.tenant_id,'event_id',a.event_id,'actor_subject',a.actor_id,'operator_id',nullif(a.object_id,''),'event_type',a.action,
   'source_app_instance_id',nullif(p#>>'{after,source_app_instance_id}',''),'workstation_id',nullif(p#>>'{after,register_id}',''),
   'details',p,'occurred_at',a.occurred_at,'previous_hash',coalesce(nullif(p->>'previous_hash',''),nullif(p->>'prev_hash','')),'event_hash',a.event_hash,'signature',nullif(p->>'signature',''));
 ELSE RETURN jsonb_build_object('status','OUTSIDE_SALE_OPERATOR_MODEL'); END IF;
 RETURN jsonb_build_object('status','PROJECTED','kind',kind,'row',r);
END $$;

CREATE FUNCTION fiscal.apply_runtime_projection_v2(a fiscal.fiscal_runtime_audit, historical boolean DEFAULT false) RETURNS text
LANGUAGE plpgsql SET search_path=fiscal,pg_temp SET timezone='UTC' SET datestyle='ISO, YMD' AS $$
DECLARE n jsonb:=fiscal.normalize_runtime_audit_v2(a); r jsonb:=n->'row'; prior jsonb; c fiscal.typed_projection_corrections; d fiscal.typed_projection_diagnostics; diagnostic_inserted boolean;
BEGIN
 IF a.tenant_id IS DISTINCT FROM current_setting('app.tenant_id',true) THEN RAISE EXCEPTION 'PROJECTION_TENANT_MISMATCH'; END IF;
 IF n->>'status'<>'PROJECTED' THEN
  INSERT INTO fiscal.typed_projection_diagnostics(tenant_id,event_id,projection_version,source_event_hash,reason) VALUES(a.tenant_id,a.event_id,2,a.event_hash,n->>'status') ON CONFLICT DO NOTHING;
  diagnostic_inserted:=FOUND;
  SELECT * INTO STRICT d FROM fiscal.typed_projection_diagnostics WHERE tenant_id=a.tenant_id AND event_id=a.event_id AND projection_version=2;
  IF d.source_event_hash<>a.event_hash OR d.reason<>n->>'status' THEN RAISE EXCEPTION 'PROJECTION_DIAGNOSTIC_CONFLICT'; END IF;
  IF diagnostic_inserted THEN RETURN 'diagnostic'; END IF;
  RETURN 'exact';
 END IF;
 IF n->>'kind'='sale' THEN
  SELECT to_jsonb(e) INTO prior FROM fiscal.sale_events e WHERE e.tenant_id=a.tenant_id AND e.event_id=a.event_id FOR UPDATE;
  IF prior IS NULL THEN INSERT INTO fiscal.sale_events SELECT (jsonb_populate_record(NULL::fiscal.sale_events,r)).*; RETURN 'inserted'; END IF;
 ELSE
  SELECT to_jsonb(e) INTO prior FROM fiscal.operator_security_events e WHERE e.tenant_id=a.tenant_id AND e.event_id=a.event_id FOR UPDATE;
  IF prior IS NULL THEN INSERT INTO fiscal.operator_security_events SELECT (jsonb_populate_record(NULL::fiscal.operator_security_events,r)).*; RETURN 'inserted'; END IF;
 END IF;
 IF prior=r THEN RETURN 'exact'; END IF;
 IF NOT historical OR prior->>'event_hash' IS DISTINCT FROM a.event_hash OR coalesce(prior->'payload',prior->'details') IS DISTINCT FROM a.payload THEN RAISE EXCEPTION 'TYPED_PROJECTION_CONFLICT'; END IF;
 SELECT * INTO c FROM fiscal.typed_projection_corrections WHERE tenant_id=a.tenant_id AND event_id=a.event_id AND projection_version=2;
 IF FOUND THEN
  IF c.projection IS DISTINCT FROM r OR c.source_event_hash<>a.event_hash OR c.event_kind<>n->>'kind' THEN RAISE EXCEPTION 'TYPED_CORRECTION_CONFLICT'; END IF;
  RETURN 'exact';
 END IF;
 INSERT INTO fiscal.typed_projection_corrections(tenant_id,event_id,projection_version,event_kind,source_event_hash,projection) VALUES(a.tenant_id,a.event_id,2,n->>'kind',a.event_hash,r) ON CONFLICT DO NOTHING;
 SELECT * INTO STRICT c FROM fiscal.typed_projection_corrections WHERE tenant_id=a.tenant_id AND event_id=a.event_id AND projection_version=2;
 IF c.projection IS DISTINCT FROM r OR c.source_event_hash<>a.event_hash OR c.event_kind<>n->>'kind' THEN RAISE EXCEPTION 'TYPED_CORRECTION_CONFLICT'; END IF;
 RETURN 'correction';
END $$;

CREATE OR REPLACE FUNCTION fiscal.project_runtime_audit_to_supto_events() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=fiscal,pg_temp SET timezone='UTC' SET datestyle='ISO, YMD' AS $$ DECLARE old_tenant text:=current_setting('app.tenant_id',true); BEGIN
 PERFORM set_config('app.tenant_id',NEW.tenant_id,true);
 PERFORM fiscal.apply_runtime_projection_v2(NEW,false);
 PERFORM set_config('app.tenant_id',coalesce(old_tenant,''),true); RETURN NEW;
END $$;

CREATE VIEW fiscal.sale_events_effective WITH(security_invoker=true) AS
 SELECT e.* FROM fiscal.sale_events e WHERE NOT EXISTS(SELECT 1 FROM fiscal.typed_projection_corrections c WHERE c.tenant_id=e.tenant_id AND c.event_id=e.event_id AND c.event_kind='sale')
 UNION ALL SELECT x.* FROM fiscal.typed_projection_corrections c CROSS JOIN LATERAL jsonb_populate_record(NULL::fiscal.sale_events,c.projection) x WHERE c.event_kind='sale';
CREATE VIEW fiscal.operator_security_events_effective WITH(security_invoker=true) AS
 SELECT e.* FROM fiscal.operator_security_events e WHERE NOT EXISTS(SELECT 1 FROM fiscal.typed_projection_corrections c WHERE c.tenant_id=e.tenant_id AND c.event_id=e.event_id AND c.event_kind='operator')
 UNION ALL SELECT x.* FROM fiscal.typed_projection_corrections c CROSS JOIN LATERAL jsonb_populate_record(NULL::fiscal.operator_security_events,c.projection) x WHERE c.event_kind='operator';

CREATE FUNCTION fiscal.reconcile_runtime_projection_v2(range_from text,range_until text,expected_sha256 text DEFAULT NULL,apply_changes boolean DEFAULT false) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=fiscal,pg_temp SET timezone='UTC' SET datestyle='ISO, YMD' AS $$
DECLARE tenant text:=current_setting('app.tenant_id',true); total bigint; sha text; a fiscal.fiscal_runtime_audit; status text; n jsonb; prior jsonb; ins bigint:=0; corr bigint:=0; diag bigint:=0;
BEGIN
 IF tenant IS NULL OR tenant='' OR range_from IS NULL OR range_until IS NULL OR range_from COLLATE "C">=range_until COLLATE "C" THEN RAISE EXCEPTION 'EXPLICIT_TENANT_RANGE_REQUIRED'; END IF;
 IF current_setting('transaction_isolation') NOT IN ('repeatable read','serializable') THEN RAISE EXCEPTION 'REPEATABLE_READ_REQUIRED'; END IF;
 SELECT count(*) INTO total FROM (SELECT 1 FROM fiscal.fiscal_runtime_audit WHERE tenant_id=tenant AND event_id COLLATE "C">=range_from COLLATE "C" AND event_id COLLATE "C"<range_until COLLATE "C" LIMIT 100001) bounded_source;
 IF total>100000 THEN RAISE EXCEPTION 'SOURCE_RANGE_TOO_LARGE'; END IF;
 SELECT encode(pg_catalog.sha256(pg_catalog.convert_to(coalesce(string_agg(encode(pg_catalog.sha256(pg_catalog.convert_to(to_jsonb(e)::text,'UTF8')),'hex'),'' ORDER BY event_id COLLATE "C"),''),'UTF8')),'hex') INTO sha FROM fiscal.fiscal_runtime_audit e WHERE tenant_id=tenant AND event_id COLLATE "C">=range_from COLLATE "C" AND event_id COLLATE "C"<range_until COLLATE "C";
 IF NOT apply_changes THEN
  FOR a IN SELECT * FROM fiscal.fiscal_runtime_audit WHERE tenant_id=tenant AND event_id COLLATE "C">=range_from COLLATE "C" AND event_id COLLATE "C"<range_until COLLATE "C" ORDER BY event_id COLLATE "C" LOOP
   n:=fiscal.normalize_runtime_audit_v2(a); prior:=NULL;
   IF n->>'status'<>'PROJECTED' THEN
    IF NOT EXISTS(SELECT 1 FROM fiscal.typed_projection_diagnostics WHERE tenant_id=tenant AND event_id=a.event_id AND projection_version=2) THEN diag:=diag+1;
    ELSIF NOT EXISTS(SELECT 1 FROM fiscal.typed_projection_diagnostics WHERE tenant_id=tenant AND event_id=a.event_id AND projection_version=2 AND source_event_hash=a.event_hash AND reason=n->>'status') THEN RAISE EXCEPTION 'PROJECTION_DIAGNOSTIC_CONFLICT'; END IF;
   ELSE
    SELECT projection INTO prior FROM fiscal.typed_projection_corrections WHERE tenant_id=tenant AND event_id=a.event_id AND projection_version=2;
    IF prior IS NULL THEN
     IF n->>'kind'='sale' THEN SELECT to_jsonb(e) INTO prior FROM fiscal.sale_events e WHERE e.tenant_id=tenant AND e.event_id=a.event_id;
     ELSE SELECT to_jsonb(e) INTO prior FROM fiscal.operator_security_events e WHERE e.tenant_id=tenant AND e.event_id=a.event_id; END IF;
     IF prior IS NULL THEN ins:=ins+1; ELSIF prior IS DISTINCT FROM n->'row' THEN corr:=corr+1; END IF;
    ELSIF prior IS DISTINCT FROM n->'row' THEN RAISE EXCEPTION 'TYPED_CORRECTION_CONFLICT'; END IF;
   END IF;
  END LOOP;
 ELSE
  IF expected_sha256 IS NULL OR expected_sha256 !~ '^[0-9a-f]{64}$' OR expected_sha256<>sha THEN RAISE EXCEPTION 'SOURCE_DIGEST_MISMATCH'; END IF;
  FOR a IN SELECT * FROM fiscal.fiscal_runtime_audit WHERE tenant_id=tenant AND event_id COLLATE "C">=range_from COLLATE "C" AND event_id COLLATE "C"<range_until COLLATE "C" ORDER BY event_id COLLATE "C" LOOP
   status:=fiscal.apply_runtime_projection_v2(a,true);
   ins:=ins+(status='inserted')::int; corr:=corr+(status='correction')::int; diag:=diag+(status='diagnostic')::int;
  END LOOP;
  INSERT INTO fiscal.typed_projection_reconciliation_runs(tenant_id,range_from,range_until,source_sha256,projection_version,source_count,inserted_count,correction_count,diagnostic_count) VALUES(tenant,range_from,range_until,sha,2,total,ins,corr,diag) ON CONFLICT DO NOTHING;
 END IF;
 RETURN jsonb_build_object('tenant_id',tenant,'range_from',range_from,'range_until',range_until,'source_sha256',sha,'source_count',total,'applied',apply_changes,'inserted',ins,'corrections',corr,'diagnostics',diag,'projection_version',2);
END $$;
REVOKE ALL ON FUNCTION fiscal.normalize_runtime_audit_v2(fiscal.fiscal_runtime_audit),fiscal.apply_runtime_projection_v2(fiscal.fiscal_runtime_audit,boolean),fiscal.reconcile_runtime_projection_v2(text,text,text,boolean) FROM PUBLIC;
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='beefiscal_projection_reconciler') THEN CREATE ROLE beefiscal_projection_reconciler NOLOGIN NOINHERIT NOSUPERUSER NOBYPASSRLS; END IF; END $$;
GRANT USAGE ON SCHEMA fiscal TO beefiscal_projection_reconciler;
GRANT SELECT ON fiscal.fiscal_runtime_audit,fiscal.sale_events,fiscal.operator_security_events,fiscal.typed_projection_diagnostics,fiscal.typed_projection_corrections,fiscal.typed_projection_reconciliation_runs,fiscal.sale_events_effective,fiscal.operator_security_events_effective TO beefiscal_projection_reconciler;
-- The reconciler may only request source-derived changes via the guarded entry point.
REVOKE INSERT,UPDATE,DELETE,TRUNCATE ON fiscal.sale_events,fiscal.operator_security_events,fiscal.typed_projection_diagnostics,fiscal.typed_projection_corrections,fiscal.typed_projection_reconciliation_runs FROM beefiscal_projection_reconciler;
GRANT EXECUTE ON FUNCTION fiscal.reconcile_runtime_projection_v2(text,text,text,boolean) TO beefiscal_projection_reconciler;
GRANT SELECT ON fiscal.sale_events,fiscal.operator_security_events,fiscal.typed_projection_diagnostics,fiscal.typed_projection_corrections,fiscal.sale_events_effective,fiscal.operator_security_events_effective TO beefiscal_tenant;
REVOKE UPDATE,DELETE,TRUNCATE ON fiscal.typed_projection_diagnostics,fiscal.typed_projection_corrections,fiscal.typed_projection_reconciliation_runs FROM PUBLIC,beefiscal_tenant,beefiscal_projection_reconciler;
COMMIT;
