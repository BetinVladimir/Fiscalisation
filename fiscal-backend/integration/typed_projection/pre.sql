\set ON_ERROR_STOP on
SET search_path=fiscal,pg_temp;
CREATE FUNCTION fiscal.test_assert(ok boolean,msg text) RETURNS void LANGUAGE plpgsql AS $$BEGIN IF ok IS DISTINCT FROM true THEN RAISE EXCEPTION 'ASSERT: %',msg; END IF; END$$;
CREATE FUNCTION fiscal.test_reject(q text,expected text) RETURNS void LANGUAGE plpgsql AS $$BEGIN BEGIN EXECUTE q; EXCEPTION WHEN OTHERS THEN IF position(expected IN SQLERRM)>0 THEN RETURN; END IF; RAISE; END; RAISE EXCEPTION 'ASSERT: expected rejection %',expected; END$$;
SELECT fiscal.test_assert(to_regclass('public.fiscal_runtime_audit') IS NULL AND to_regclass('fiscal.fiscal_runtime_audit') IS NOT NULL,'fixture uses actual fiscal namespace');
CREATE TABLE source_before AS SELECT * FROM fiscal.fiscal_runtime_audit WITH NO DATA;
CREATE TABLE fixture AS SELECT pg_read_file('/fixture/domain.json')::jsonb p;
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT p->>'event_id',p->>'tenant_id',p->>'actor_id',p->>'action',p->>'object_type',p->>'object_id',(p->>'occurred_at')::timestamptz,p->>'event_hash',p FROM fixture;
SELECT test_assert((SELECT count(*)=0 FROM fiscal.sale_events),'RED actual sale- ID silently omitted by021');
-- A historical UUID snapshot is projected by021 but actual operator/device paths are lost.
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
SELECT '20000000-0000-4000-8000-000000000002',p->>'tenant_id',p->>'actor_id',p->>'action','sale','30000000-0000-4000-8000-000000000003',(p->>'occurred_at')::timestamptz,repeat('a',64),
jsonb_set(jsonb_set(jsonb_set(p,'{after,sale_id}','"30000000-0000-4000-8000-000000000003"'),'{event_id}','"20000000-0000-4000-8000-000000000002"'),'{event_hash}',to_jsonb(repeat('a',64))) FROM fixture;
SELECT test_assert((SELECT count(*)=1 AND bool_and(operator_code IS NULL AND fiscal_device_number IS NULL) FROM fiscal.sale_events),'RED UUID projection loses current snapshot fields');
-- Legacy partial source event was omitted by021;022 reconciles one diagnostic.
INSERT INTO fiscal.fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
VALUES('legacy-partial','10000000-0000-4000-8000-000000000001','actor','EDGE_INTENT_ACCEPTED','sale','sale-partial','2026-01-01'::timestamptz,repeat('6',64),'{"after":{"intent_id":"legacy-intent"}}');
INSERT INTO source_before SELECT * FROM fiscal.fiscal_runtime_audit;
