-- Built-in Beeloy ERP integration identity. Credentials and signing secrets are
-- provisioned through the platform external-system API and are never seeded.
INSERT INTO external_systems(code, display_name, status, webhook_url, webhook_events, created_by, updated_by)
SELECT 'beeloy', 'Beeloy ERP', 'ACTIVE',
       'https://public-api-core.invalid/company-admin/v1/device-management/fiscal-webhook',
       ARRAY['integration.command.updated']::text[], 'migration', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM external_systems WHERE code='beeloy');

