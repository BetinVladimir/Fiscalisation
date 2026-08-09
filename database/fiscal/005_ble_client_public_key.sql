-- The client X25519 public key is signed into the REST-issued ticket and
-- compared with the HELLO ephemeral key by Edge. NULL legacy rows cannot be
-- refreshed or used as proof-of-possession sessions and must be reissued.
alter table fiscal_runtime_ble_sessions
  add column if not exists client_public_key text;

alter table fiscal_runtime_ble_sessions
  add constraint fiscal_runtime_ble_client_public_key_shape
  check (client_public_key is null or client_public_key ~ '^[A-Za-z0-9_-]{43}$')
  not valid;

alter table fiscal_runtime_ble_sessions
  validate constraint fiscal_runtime_ble_client_public_key_shape;
