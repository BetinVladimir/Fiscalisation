-- Durable first-request claim for cross-replica public API idempotency.
alter table minipos_runtime_api_replays
  add column if not exists pending boolean not null default false;
