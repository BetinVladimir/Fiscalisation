CREATE TABLE IF NOT EXISTS integration_resources(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(), tenant_id uuid NOT NULL, external_system_id uuid NOT NULL,
  resource_type text NOT NULL CHECK(resource_type IN ('organization','location','register','operator')),
  source_entity_id text NOT NULL, source_version bigint NOT NULL, payload jsonb NOT NULL,
  status text NOT NULL CHECK(status IN ('ACTIVE','INACTIVE')), created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id,external_system_id,resource_type,source_entity_id)
);
CREATE INDEX IF NOT EXISTS integration_resources_tenant_type ON integration_resources(tenant_id,resource_type,updated_at,id);
