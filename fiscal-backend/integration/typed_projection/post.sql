\set ON_ERROR_STOP on
SET search_path=fiscal,pg_temp;
-- Historical dry-run proposes one missing row and one correction, without writing.
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SET LOCAL app.tenant_id='10000000-0000-4000-8000-000000000001';
SELECT fiscal.reconcile_runtime_projection_v2('','~') AS dry \gset
COMMIT;
SELECT test_assert((:'dry'::jsonb->>'inserted')::int=1 AND (:'dry'::jsonb->>'corrections')::int=1,'dry-run predicts exact work');
SELECT test_assert((SELECT count(*)=1 FROM fiscal.sale_events) AND (SELECT count(*)=0 FROM fiscal.typed_projection_corrections),'dry-run writes nothing');
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SET LOCAL app.tenant_id='10000000-0000-4000-8000-000000000001';
SELECT fiscal.reconcile_runtime_projection_v2('','~',:'dry'::jsonb->>'source_sha256',true) AS applied \gset
COMMIT;
SELECT test_assert((:'applied'::jsonb->>'inserted')::int=1 AND (:'applied'::jsonb->>'corrections')::int=1,'apply historical work');
SELECT fiscal.test_assert((:'dry'::jsonb->>'diagnostics')::int=1 AND (:'applied'::jsonb->>'diagnostics')::int=1,'new historical diagnostic counted once by dry-run and apply');
SELECT test_assert((SELECT count(*)=1 FROM fiscal.typed_projection_corrections),'append correction preserves original row');
SELECT test_assert((SELECT operator_code IS NULL FROM fiscal.sale_events WHERE event_id='20000000-0000-4000-8000-000000000002'),'original UUID row unchanged');
SELECT test_assert((SELECT count(*)=2 AND bool_and(operator_code='A001' AND fiscal_device_number='BL000001' AND workstation_id=(SELECT p#>>'{after,register_id}' FROM fixture) AND fiscal_device_id=(SELECT p#>>'{after,fiscal_device,device_id}' FROM fixture)) FROM fiscal.sale_events_effective),'effective exact snapshot fields');
SELECT test_assert((SELECT count(*)=1 FROM fiscal.sale_events WHERE sale_id LIKE 'sale-%'),'opaque sale preserved');
SELECT test_assert(NOT EXISTS((SELECT * FROM source_before EXCEPT SELECT * FROM fiscal.fiscal_runtime_audit) UNION ALL (SELECT * FROM fiscal.fiscal_runtime_audit EXCEPT SELECT * FROM source_before)),'source bytes/hash unchanged');
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SET LOCAL app.tenant_id='10000000-0000-4000-8000-000000000001';
SELECT fiscal.reconcile_runtime_projection_v2('','~',:'dry'::jsonb->>'source_sha256',true) AS retry \gset
COMMIT;
SELECT test_assert((:'retry'::jsonb->>'inserted')::int=0 AND (:'retry'::jsonb->>'corrections')::int=0,'retry is exact no-op');
SELECT fiscal.test_assert((:'retry'::jsonb->>'diagnostics')::int=0,'existing historical diagnostic no-op on retry');
SELECT test_assert((SELECT count(*)=1 FROM fiscal.typed_projection_corrections) AND (SELECT count(*)=1 FROM fiscal.typed_projection_reconciliation_runs),'retry adds no history');
-- New opaque tenant/event/reference IDs and malformed partial events.
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-opaque-not-uuid','tenant-opaque','actor','SALE_OPENED','sale','sale-not-uuid',now(),repeat('b',64),jsonb_set(jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-not-uuid"'),'{after,fiscal_operation_id}','"op-not-uuid"') FROM fixture;
SELECT test_assert((SELECT operation_id='op-not-uuid' FROM fiscal.sale_events WHERE event_id='event-opaque-not-uuid'),'future exact opaque references');
INSERT INTO fiscal.fiscal_runtime_audit VALUES('event-partial','tenant-opaque','actor','EDGE_INTENT_ACCEPTED','sale','sale-partial',now(),repeat('c',64),'{"after":{"intent_id":"intent"}}',DEFAULT);
SELECT test_assert((SELECT reason='NOT_A_VERSIONED_SALE_SNAPSHOT' FROM fiscal.typed_projection_diagnostics WHERE event_id='event-partial'),'partial event diagnostic');
SELECT test_assert((SELECT count(*)=0 FROM fiscal.sale_events WHERE event_id='event-partial'),'partial event never fake version1');
BEGIN;
SET LOCAL ROLE beefiscal_tenant;
SET LOCAL app.tenant_id='tenant-other';
SELECT test_assert((SELECT count(*)=0 FROM fiscal.sale_events_effective) AND (SELECT count(*)=0 FROM fiscal.typed_projection_corrections),'effective view RLS');
COMMIT;
SELECT test_reject('UPDATE fiscal.sale_events SET operator_code=''B002''','append-only');
SELECT test_reject('DELETE FROM fiscal.typed_projection_corrections','append-only');
SELECT test_reject('UPDATE fiscal.fiscal_runtime_audit SET action=''X''','FISCAL_AUDIT_IS_IMMUTABLE');
SELECT test_assert(NOT has_function_privilege('beefiscal_tenant','fiscal.reconcile_runtime_projection_v2(text,text,text,boolean)','EXECUTE'),'tenant no historical reconcile permission');
SELECT test_assert(NOT has_function_privilege('public','fiscal.apply_runtime_projection_v2(fiscal.fiscal_runtime_audit,boolean)','EXECUTE'),'PUBLIC no direct writer');
SELECT test_assert((SELECT count(*)=3 FROM pg_class WHERE relnamespace='fiscal'::regnamespace AND relname IN('typed_projection_diagnostics','typed_projection_corrections','typed_projection_reconciliation_runs') AND relrowsecurity AND relforcerowsecurity),'derived FORCE RLS');
-- No locale/time-zone changes may change equality or digest.
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL ROLE beefiscal_projection_reconciler;
SET LOCAL app.tenant_id='10000000-0000-4000-8000-000000000001';
SET LOCAL timezone='Pacific/Auckland';
SELECT fiscal.reconcile_runtime_projection_v2('','~', :'dry'::jsonb->>'source_sha256',true) AS zone_retry \gset
COMMIT;
SELECT test_assert((:'zone_retry'::jsonb->>'inserted')::int=0 AND (:'zone_retry'::jsonb->>'corrections')::int=0,'timezone stable digest and equality');
BEGIN;
SET LOCAL app.tenant_id='tenant-opaque';
SELECT test_assert((SELECT fiscal.apply_runtime_projection_v2(a)='exact' FROM fiscal.fiscal_runtime_audit a WHERE event_id='event-opaque-not-uuid'),'direct exact replay');
SELECT test_reject($q$SELECT fiscal.apply_runtime_projection_v2(jsonb_populate_record(NULL::fiscal.fiscal_runtime_audit,to_jsonb(a)||jsonb_build_object('event_hash',repeat('f',64)))) FROM fiscal.fiscal_runtime_audit a WHERE event_id='event-opaque-not-uuid'$q$,'TYPED_PROJECTION_CONFLICT');
COMMIT;
-- Every version rejection is diagnosed, never projected as version1.
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-invalid-version','tenant-opaque','actor','SALE_OPENED','sale','sale-not-uuid',now(),repeat('d',64),jsonb_set(jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-not-uuid"'),'{after,version}','0') FROM fixture;
SELECT test_assert((SELECT reason='NOT_A_VERSIONED_SALE_SNAPSHOT' FROM fiscal.typed_projection_diagnostics WHERE event_id='event-invalid-version'),'invalid version diagnostic');
-- Legacy keys are supported when actual keys are absent; mixed identities fail closed.
WITH src AS (SELECT jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-legacy"') p FROM fixture), legacy AS (SELECT jsonb_set(p,'{after}',((p->'after')-'operator_code'-'fiscal_device')||jsonb_build_object('operator_snapshot',jsonb_build_object('code','A001'),'fiscal_device_snapshot',p#>'{after,fiscal_device}')) p FROM src)
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-legacy','tenant-opaque','actor','SALE_OPENED','sale','sale-legacy',now(),repeat('e',64),p FROM legacy;
SELECT test_assert((SELECT operator_code='A001' AND fiscal_device_number='BL000001' FROM fiscal.sale_events WHERE event_id='event-legacy'),'legacy fallback');
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-conflicting-legacy','tenant-opaque','actor','SALE_OPENED','sale','sale-not-uuid',now(),repeat('9',64),jsonb_set(jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-not-uuid"'),'{after,operator_snapshot}','{"code":"B002"}') FROM fixture;
SELECT test_assert((SELECT reason='CONFLICTING_OPERATOR_SNAPSHOTS' FROM fiscal.typed_projection_diagnostics WHERE event_id='event-conflicting-legacy'),'mixed identities diagnosed');
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL app.tenant_id='tenant-opaque';
SELECT test_reject($q$SELECT fiscal.reconcile_runtime_projection_v2('','~',repeat('0',64),true)$q$,'SOURCE_DIGEST_MISMATCH');
COMMIT;
SELECT test_assert((SELECT count(*)=0 FROM fiscal.sale_events WHERE event_id IN('event-invalid-version','event-conflicting-legacy')),'invalid sources never become typed snapshots');
BEGIN;
SET LOCAL app.tenant_id='10000000-0000-4000-8000-000000000001';
SELECT test_reject($q$SELECT fiscal.apply_runtime_projection_v2(jsonb_populate_record(NULL::fiscal.fiscal_runtime_audit,to_jsonb(a)||jsonb_build_object('actor_id','changed-actor')),true) FROM fiscal.fiscal_runtime_audit a WHERE event_id='20000000-0000-4000-8000-000000000002'$q$,'TYPED_CORRECTION_CONFLICT');
COMMIT;
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-opaque-references','tenant-opaque','actor','SALE_OPENED','sale','sale-opaque-refs',now(),repeat('8',64),jsonb_set(jsonb_set(jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-opaque-refs"'),'{after,register_id}','"register-opaque-tail"'),'{after,fiscal_device,device_id}','"device-opaque-tail"') FROM fixture;
SELECT test_assert((SELECT workstation_id='register-opaque-tail' AND fiscal_device_id='device-opaque-tail' FROM fiscal.sale_events WHERE event_id='event-opaque-references'),'opaque register/device IDs preserved without alias');
-- Caller search_path and a shadow public table must not redirect runtime writes.
CREATE TABLE public.sale_events (LIKE fiscal.sale_events INCLUDING ALL);
BEGIN;
SET LOCAL search_path=public,pg_temp;
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'event-fiscal-qualified','tenant-opaque','actor','SALE_OPENED','sale','sale-qualified',now(),repeat('7',64),jsonb_set(jsonb_set(p,'{after,tenant_id}','"tenant-opaque"'),'{after,sale_id}','"sale-qualified"') FROM fiscal.fixture;
SELECT fiscal.test_assert((SELECT count(*)=1 FROM fiscal.sale_events WHERE event_id='event-fiscal-qualified') AND (SELECT count(*)=0 FROM public.sale_events),'public caller and shadow table cannot redirect projection');
COMMIT;
SELECT fiscal.test_assert((SELECT n.nspname='fiscal' FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace n ON n.oid=p.pronamespace WHERE t.tgrelid='fiscal.fiscal_runtime_audit'::regclass AND p.proname='project_runtime_audit_to_supto_events'),'runtime trigger invokes fiscal function');
-- Existing diagnostics must be no-op in dry-run and apply counters.
BEGIN ISOLATION LEVEL REPEATABLE READ;
SET LOCAL app.tenant_id='tenant-opaque';
SELECT fiscal.reconcile_runtime_projection_v2('event-partial','event-partiam') AS diag_dry \gset
SELECT fiscal.reconcile_runtime_projection_v2('event-partial','event-partiam',:'diag_dry'::jsonb->>'source_sha256',true) AS diag_apply \gset
SELECT fiscal.reconcile_runtime_projection_v2('event-partial','event-partiam',:'diag_dry'::jsonb->>'source_sha256',true) AS diag_replay \gset
SELECT fiscal.test_assert((:'diag_dry'::jsonb->>'source_count')::int=1 AND (:'diag_dry'::jsonb->>'diagnostics')::int=0 AND (:'diag_apply'::jsonb->>'diagnostics')::int=0 AND (:'diag_replay'::jsonb->>'diagnostics')::int=0,'existing diagnostic dry/apply/replay counts agree at zero');
SELECT fiscal.test_assert((SELECT count(*)=1 FROM fiscal.typed_projection_diagnostics WHERE tenant_id='tenant-opaque' AND event_id='event-partial'),'diagnostic replay adds no row');
SELECT fiscal.test_assert((SELECT fiscal.apply_runtime_projection_v2(a)='exact' FROM fiscal.fiscal_runtime_audit a WHERE event_id='event-partial'),'matching diagnostic direct replay is exact');
SELECT fiscal.test_reject($q$SELECT fiscal.apply_runtime_projection_v2(jsonb_populate_record(NULL::fiscal.fiscal_runtime_audit,to_jsonb(a)||jsonb_build_object('event_hash','changed-hash'))) FROM fiscal.fiscal_runtime_audit a WHERE event_id='event-partial'$q$,'PROJECTION_DIAGNOSTIC_CONFLICT');
-- A genuine insert counts once, then returns exact without changing its row.
SELECT fiscal.test_assert(fiscal.apply_runtime_projection_v2(ROW('direct-new-diag','tenant-opaque','actor','EDGE_INTENT_ACCEPTED','sale','partial',now(),'fixture-hash','{}'::jsonb,NULL)::fiscal.fiscal_runtime_audit)='diagnostic','genuine diagnostic insert counted');
SELECT fiscal.test_assert(fiscal.apply_runtime_projection_v2(ROW('direct-new-diag','tenant-opaque','actor','EDGE_INTENT_ACCEPTED','sale','partial',now(),'fixture-hash','{}'::jsonb,NULL)::fiscal.fiscal_runtime_audit)='exact','genuine diagnostic subsequent replay exact');
ROLLBACK;
-- Minimal metadata rows, active source triggers, rollback only owned fixture.
BEGIN ISOLATION LEVEL REPEATABLE READ;
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT 'limit-'||n,'limit-tenant','a','X','x','x','2026-01-01'::timestamptz,'','{}'::jsonb FROM generate_series(1,100001) n;
SELECT fiscal.test_assert((SELECT count(*)=100001 FROM fiscal.typed_projection_diagnostics WHERE tenant_id='limit-tenant'),'limit fixture retains active source projection trigger');
-- Exact-type overload poisons any digest row serialization; no large payloads.
CREATE FUNCTION fiscal.to_jsonb(fiscal.fiscal_runtime_audit) RETURNS jsonb LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'AGGREGATION_MUST_NOT_RUN'; END$$;
DISCARD PLANS;
SET LOCAL app.tenant_id='tenant-opaque';
SELECT fiscal.test_reject($q$SELECT fiscal.reconcile_runtime_projection_v2('event-partial','event-partiam')$q$,'AGGREGATION_MUST_NOT_RUN');
SET LOCAL app.tenant_id='limit-tenant';
SELECT fiscal.test_reject($q$SELECT fiscal.reconcile_runtime_projection_v2('limit-','limit.~')$q$,'SOURCE_RANGE_TOO_LARGE');
ROLLBACK;
SELECT 'PASS actual fiscal schema typed projection migration/reconcile/source immutability';
