alter table order_lines
  add column if not exists discount numeric(18,2) not null default 0
  check (discount >= 0 and discount <= gross);
