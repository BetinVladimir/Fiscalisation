create extension if not exists pgcrypto;
create table if not exists tenants(id uuid primary key default gen_random_uuid(),status text not null check(status in('ACTIVE','SUSPENDED')),created_at timestamptz not null default now());
create table if not exists locations(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,code text not null,name text not null,address text not null,timezone text not null default 'Europe/Sofia',status text not null default 'ACTIVE',version bigint not null default 1,unique(tenant_id,code));
create table if not exists registers(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,location_id uuid not null references locations,code text not null,status text not null default 'ACTIVE',authority_generation bigint not null default 0,current_unp_sequence bigint not null default 0,version bigint not null default 1,unique(tenant_id,code));
create table if not exists operators(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,code char(4) not null,first_name text not null,last_name text not null,active_from timestamptz not null default now(),active_to timestamptz,version bigint not null default 1,unique(tenant_id,code));
create table if not exists shifts(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,register_id uuid not null references registers,operator_id uuid not null references operators,state text not null,business_date date not null,timezone text not null,opened_at timestamptz not null,closed_at timestamptz,version bigint not null default 1);
create unique index if not exists one_open_shift_per_register on shifts(register_id) where state in('OPEN','CLOSING','RECONCILIATION_REQUIRED');
create table if not exists devices(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,kind text not null,vendor text not null,model text not null,serial text not null,firmware text,fiscal_device_number text,fiscal_memory_number text,status text not null,environment text not null,simulated boolean not null default false,evidence_state text not null,version bigint not null default 1,unique(tenant_id,serial));
create table if not exists sales(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,register_id uuid not null references registers,shift_id uuid references shifts,operator_id uuid references operators,external_id text not null,unp text,state text not null,version bigint not null,policy_version text not null,business_date date,timezone text,official_currency char(3) not null check(official_currency='EUR'),rule_snapshot jsonb not null,opened_at timestamptz,completed_at timestamptz,unique(tenant_id,external_id),unique(tenant_id,unp));
create table if not exists sale_lines(id uuid primary key,sale_id uuid not null references sales,line_no int not null,name text not null,quantity numeric(18,3) not null check(quantity>0),unit_price numeric(18,2) not null,tax_group char(1) not null,gross numeric(18,2) not null,snapshot jsonb not null,unique(sale_id,line_no));
create table if not exists fiscal_operations(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,sale_id uuid references sales,register_id uuid not null references registers,external_id text not null,command_type text not null,state text not null,request_hash char(64) not null,fencing_token bigint not null,execution_id uuid,fiscal_reference text,simulated boolean not null default false,created_at timestamptz not null,updated_at timestamptz not null,unique(tenant_id,external_id,command_type));
create table if not exists operation_events(id uuid primary key default gen_random_uuid(),operation_id uuid not null references fiscal_operations,seq bigint not null,type text not null,payload jsonb not null,occurred_at timestamptz not null,prev_hash char(64),event_hash char(64) not null,signature text,unique(operation_id,seq));
create table if not exists idempotency_records(tenant_id uuid not null,endpoint text not null,key text not null,request_hash char(64) not null,response_status int,response_body jsonb,operation_id uuid,created_at timestamptz not null,expires_at timestamptz not null,primary key(tenant_id,endpoint,key));
create table if not exists audit_events(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,actor_id text not null,action text not null,object_type text not null,object_id text not null,before_data jsonb,after_data jsonb,occurred_at timestamptz not null,prev_hash char(64),event_hash char(64) not null unique,signature text);
create table if not exists artifacts(id uuid primary key default gen_random_uuid(),tenant_id uuid not null references tenants,kind text not null,media_type text not null,object_key text not null,size_bytes bigint not null,sha256 char(64) not null,signature text,created_at timestamptz not null,retain_until timestamptz not null,legal_hold boolean not null default false);
create table if not exists edge_sync_acks(edge_id uuid not null,journal_seq bigint not null,event_id uuid not null,committed_at timestamptz not null,ack_id uuid not null,ack_signature text not null,primary key(edge_id,journal_seq));
create table if not exists runtime_snapshots(aggregate text primary key,payload jsonb not null,version bigint not null,updated_at timestamptz not null);
-- Row-granular durable state used by the runtime repository. The typed legal
-- tables above remain the reporting/constraint model; this table preserves the
-- exact evolving API aggregates without a single all-or-nothing JSON snapshot.
create table if not exists fiscal_state_rows(
  collection text not null,
  entity_key text not null,
  payload jsonb not null,
  updated_at timestamptz not null default now(),
  primary key(collection,entity_key)
);
create index if not exists fiscal_state_rows_updated on fiscal_state_rows(updated_at);
create table if not exists fiscal_state_meta(
  singleton boolean primary key default true check(singleton),
  generation bigint not null,
  updated_at timestamptz not null default now()
);
