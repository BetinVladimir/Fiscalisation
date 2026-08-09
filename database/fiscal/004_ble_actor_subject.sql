-- Existing installations gain the OIDC subject projection without granting
-- legacy sessions authority. NULL legacy values are deliberately fail-closed
-- by the service and require a newly issued BLE session.
alter table fiscal_runtime_ble_sessions
  add column if not exists actor_subject text;

create index if not exists fiscal_runtime_ble_sessions_actor
  on fiscal_runtime_ble_sessions(tenant_id, actor_subject, expires_at)
  where revoked = false and actor_subject is not null;
