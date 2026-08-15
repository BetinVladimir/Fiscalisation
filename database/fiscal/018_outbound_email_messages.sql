create table if not exists outbound_email_messages(
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  sent_at timestamptz,
  recipient text not null,
  subject text not null,
  message_text text not null,
  status text not null check(status in ('SENDING','SENT','FAILED')),
  last_error text
);

create index if not exists outbound_email_messages_created_at_idx
  on outbound_email_messages(created_at desc);
create index if not exists outbound_email_messages_status_created_at_idx
  on outbound_email_messages(status,created_at);
