create table if not exists fiscal_device_activation_challenges(
  activation_challenge_id uuid primary key, device_instance_id uuid not null,
  nonce_hash char(64) not null, expires_at timestamptz not null, consumed_at timestamptz,
  created_at timestamptz not null, payload jsonb not null
);
create index if not exists fiscal_device_activation_challenges_expiry on fiscal_device_activation_challenges(expires_at) where consumed_at is null;

create table if not exists fiscal_device_activation_requests(
  activation_request_id uuid primary key, device_instance_id uuid not null,
  request_secret_hash char(64) not null, user_code_hash char(64) not null,
  device_key_thumbprint text not null, state text not null,
  claimed_tenant_id text, claimed_location_id text, claimed_register_id text,
  binding_version bigint, expires_at timestamptz not null, created_at timestamptz not null,
  updated_at timestamptz not null, payload jsonb not null,
  check(state in('PENDING','CONFIRMED','CREDENTIAL_ISSUED','ACTIVE','EXPIRED','CANCELLED','REJECTED','QUARANTINED','REVOKED')),
  check((state='PENDING' and claimed_tenant_id is null and claimed_location_id is null and claimed_register_id is null) or state<>'PENDING')
);
create unique index if not exists fiscal_device_activation_live_instance on fiscal_device_activation_requests(device_instance_id) where state in('PENDING','CONFIRMED','CREDENTIAL_ISSUED','ACTIVE');
create unique index if not exists fiscal_device_activation_live_key on fiscal_device_activation_requests(device_key_thumbprint) where state in('PENDING','CONFIRMED','CREDENTIAL_ISSUED','ACTIVE');
create index if not exists fiscal_device_activation_user_code on fiscal_device_activation_requests(user_code_hash) where state='PENDING';

revoke all on fiscal_device_activation_challenges,fiscal_device_activation_requests from public;
comment on table fiscal_device_activation_requests is 'Tenantless bootstrap authority. Never expose via tenant RLS listing APIs.';
