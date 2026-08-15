create table if not exists minipos_email_outbox(
  id uuid primary key default gen_random_uuid(), purpose text not null, recipient text not null,
  subject text not null, body_text text not null, status text not null check(status in ('PENDING','SENDING','SENT','FAILED')),
  attempts integer not null default 0, available_at timestamptz not null default now(),
  lease_until timestamptz, sent_at timestamptz, last_error text,
  created_at timestamptz not null default now(), updated_at timestamptz not null default now()
);
create index if not exists minipos_email_outbox_ready on minipos_email_outbox(available_at,id) where status in ('PENDING','FAILED');
