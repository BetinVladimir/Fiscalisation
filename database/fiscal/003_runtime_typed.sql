-- Authoritative runtime projection for hot fiscal aggregates. Text identifiers
-- preserve the public API identifiers during migration from the compatibility store.
create table fiscal_runtime_sales(
  id text primary key, tenant_id text not null, external_id text not null,
  register_id text not null, operator_id text not null, unp text,
  state text not null, version bigint not null check(version>0),
  lines jsonb not null check(jsonb_typeof(lines)='array'),
  payments jsonb not null check(jsonb_typeof(payments)='array'),
  fiscal_operation_id text, receipt_artifact_id text,
  created_at timestamptz not null, updated_at timestamptz not null,
  unique(tenant_id,external_id), unique(tenant_id,unp)
);
create index fiscal_runtime_sales_tenant_state on fiscal_runtime_sales(tenant_id,state,updated_at);

create table fiscal_runtime_operations(
  id text primary key, tenant_id text not null, sale_id text,
  type text not null, state text not null, version bigint not null check(version>0),
  fiscal_reference text, simulated boolean not null, error_code text,
  created_at timestamptz not null, updated_at timestamptz not null
);
create index fiscal_runtime_operations_tenant_state on fiscal_runtime_operations(tenant_id,state,updated_at);

create table fiscal_runtime_shifts(
  id text primary key, tenant_id text not null, register_id text not null,
  operator_id text not null, state text not null,
  version bigint not null check(version>0), opened_at timestamptz not null,
  closed_at timestamptz
);
create unique index fiscal_runtime_one_open_shift on fiscal_runtime_shifts(tenant_id,register_id) where state='OPEN';
create index fiscal_runtime_shifts_tenant_state on fiscal_runtime_shifts(tenant_id,state,opened_at);

create table fiscal_runtime_resources(
  kind text not null, id text not null, tenant_id text not null,
  version bigint not null check(version>0), data jsonb not null check(jsonb_typeof(data)='object'),
  created_at timestamptz not null, updated_at timestamptz not null,
  primary key(kind,id)
);
create index fiscal_runtime_resources_tenant_kind on fiscal_runtime_resources(tenant_id,kind,updated_at);

create table fiscal_runtime_outbox(
  id text primary key, tenant_id text not null, event_type text not null,
  resource_id text not null, attempts integer not null check(attempts>=0),
  next_attempt timestamptz not null, delivered_at timestamptz,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_outbox_pending on fiscal_runtime_outbox(tenant_id,next_attempt) where delivered_at is null;

create table fiscal_runtime_ble_sessions(
  id text primary key, tenant_id text not null, register_id text not null,
  operator_id text not null, app_instance_id text not null, actor_subject text,
  client_public_key text, device_id text not null,
  fencing_token bigint not null check(fencing_token>0), expires_at timestamptz not null,
  revoked boolean not null, payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_ble_sessions_tenant_expiry on fiscal_runtime_ble_sessions(tenant_id,expires_at);

create table fiscal_runtime_connectivity_probes(
  id text primary key, tenant_id text not null, register_id text not null,
  state text not null, observed_at timestamptz not null,
  recommended_transport text not null, payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_connectivity_tenant_observed on fiscal_runtime_connectivity_probes(tenant_id,observed_at desc);

create table fiscal_runtime_edge_pending(
  operation_id text primary key, tenant_id text not null, register_id text not null,
  device_id text not null, command_type text not null,
  operation_sequence bigint not null check(operation_sequence>=0),
  unp_sequence bigint not null check(unp_sequence>=0), accepted_at timestamptz not null,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_edge_pending_tenant_accepted on fiscal_runtime_edge_pending(tenant_id,accepted_at);

create table fiscal_runtime_audit(
  event_id text primary key, tenant_id text not null, actor_id text not null,
  action text not null, object_type text not null, object_id text not null,
  occurred_at timestamptz not null, event_hash text not null,
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_audit_tenant_time on fiscal_runtime_audit(tenant_id,occurred_at,event_id);

create table fiscal_runtime_replays(
  replay_key text primary key, tenant_id text not null,
  method text not null, path text not null, request_hash text not null,
  status integer not null check(status between 100 and 599),
  payload jsonb not null check(jsonb_typeof(payload)='object')
);
create index fiscal_runtime_replays_tenant_path on fiscal_runtime_replays(tenant_id,path);

create table fiscal_runtime_unp_sequences(
  tenant_id text not null, register_id text not null,
  last_sequence bigint not null check(last_sequence>=0),
  primary key(tenant_id,register_id)
);

create table fiscal_runtime_artifacts(
  tenant_id text not null, id text not null,
  body bytea not null, sha256 text not null check(sha256 ~ '^[0-9a-f]{64}$'),
  primary key(tenant_id,id)
);
create index fiscal_runtime_artifacts_tenant_id on fiscal_runtime_artifacts(tenant_id,id);

create table fiscal_runtime_sync_acks(
  tenant_id text not null, edge_id text not null, ack_id text not null,
  committed_through_seq bigint not null check(committed_through_seq>0),
  committed_event_hash text not null check(committed_event_hash ~ '^[0-9a-f]{64}$'),
  committed_at timestamptz not null, payload jsonb not null check(jsonb_typeof(payload)='object'),
  primary key(tenant_id,edge_id)
);

create or replace function app_tenant_key() returns text language sql stable parallel safe
as $$ select nullif(current_setting('app.tenant_id',true),'') $$;
alter table fiscal_runtime_sales enable row level security;
alter table fiscal_runtime_sales force row level security;
create policy tenant_boundary on fiscal_runtime_sales using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
alter table fiscal_runtime_operations enable row level security;
alter table fiscal_runtime_operations force row level security;
create policy tenant_boundary on fiscal_runtime_operations using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
alter table fiscal_runtime_shifts enable row level security;
alter table fiscal_runtime_shifts force row level security;
create policy tenant_boundary on fiscal_runtime_shifts using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
alter table fiscal_runtime_resources enable row level security;
alter table fiscal_runtime_resources force row level security;
create policy tenant_boundary on fiscal_runtime_resources using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key());
do $$ declare t text; begin foreach t in array array['fiscal_runtime_outbox','fiscal_runtime_ble_sessions','fiscal_runtime_connectivity_probes','fiscal_runtime_edge_pending','fiscal_runtime_audit','fiscal_runtime_replays','fiscal_runtime_unp_sequences','fiscal_runtime_artifacts','fiscal_runtime_sync_acks'] loop execute format('alter table %I enable row level security',t);execute format('alter table %I force row level security',t);execute format('create policy tenant_boundary on %I using(tenant_id=app_tenant_key()) with check(tenant_id=app_tenant_key())',t);end loop;end $$;
grant select,insert,update,delete on fiscal_runtime_sales,fiscal_runtime_operations,fiscal_runtime_shifts,fiscal_runtime_resources,fiscal_runtime_outbox,fiscal_runtime_ble_sessions,fiscal_runtime_connectivity_probes,fiscal_runtime_edge_pending,fiscal_runtime_audit,fiscal_runtime_replays,fiscal_runtime_unp_sequences,fiscal_runtime_artifacts,fiscal_runtime_sync_acks to beefiscal_tenant;
grant execute on function app_tenant_key() to beefiscal_tenant;
