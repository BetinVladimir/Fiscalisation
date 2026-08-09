-- Durable first-request claim for cross-replica public API idempotency.
alter table fiscal_runtime_replays
  add column if not exists pending boolean not null default false;
