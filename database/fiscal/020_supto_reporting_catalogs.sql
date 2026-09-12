-- Immutable, effective-dated legal catalogs used by Appendix 29 exports.
-- Latin protocol codes A-D correspond to the Bulgarian groups А-Б-В-Г.
create table if not exists fiscal_supto_tax_groups (
  code text not null check (code in ('A','B','C','D')),
  legal_code text not null check (legal_code in ('А','Б','В','Г')),
  rate numeric(5,2) not null check (rate >= 0 and rate <= 100),
  valid_from timestamptz not null,
  valid_to timestamptz,
  legal_basis text not null,
  primary key (code, valid_from),
  check (valid_to is null or valid_to > valid_from)
);

insert into fiscal_supto_tax_groups(code,legal_code,rate,valid_from,valid_to,legal_basis) values
  ('A','А',0.00,'2026-01-01T00:00:00Z',null,'Наредба Н-18, чл. 27, ал. 1, т. 1'),
  ('B','Б',20.00,'2026-01-01T00:00:00Z',null,'Наредба Н-18, чл. 27, ал. 1, т. 2'),
  ('C','В',20.00,'2026-01-01T00:00:00Z',null,'Наредба Н-18, чл. 27, ал. 1, т. 3'),
  ('D','Г',9.00,'2026-01-01T00:00:00Z',null,'Наредба Н-18, чл. 27, ал. 1, т. 4')
on conflict do nothing;

create table if not exists fiscal_supto_payment_types (
  code text primary key,
  fiscal_device_slot smallint not null check (fiscal_device_slot between 0 and 11),
  description_bg text not null,
  conditional_use boolean not null default false,
  legal_basis text not null
);

insert into fiscal_supto_payment_types(code,fiscal_device_slot,description_bg,conditional_use,legal_basis) values
  ('CASH',0,'в брой',false,'Наредба Н-18, чл. 26, ал. 1, т. 8'),
  ('CARD',1,'с платежна карта',false,'Наредба Н-18, чл. 26, ал. 1, т. 8'),
  ('CHEQUE',2,'с чек',true,'Наредба Н-18, чл. 26, ал. 1, т. 8'),
  ('VOUCHER',3,'с ваучер',true,'Наредба Н-18, чл. 26, ал. 1, т. 8'),
  ('COUPON',3,'с купон',true,'Наредба Н-18, чл. 26, ал. 1, т. 8'),
  ('NHIF',4,'по линия на НЗОК',true,'Приложение №29, таблица 18.9'),
  ('DEFERRED',5,'отложено плащане',true,'Приложение №29, таблица 18.9'),
  ('INTERNAL_CONSUMPTION',5,'вътрешно потребление',true,'Приложение №29, таблица 18.9')
on conflict do nothing;

create table if not exists fiscal_supto_export_schemas (
  export_type text not null check (export_type ~ '^SUPTO_18_[1-9]$'),
  ordinal smallint not null check (ordinal > 0),
  column_code text not null,
  required boolean not null default true,
  legal_basis text not null default 'Приложение №29, т. 18',
  primary key (export_type, ordinal),
  unique (export_type, column_code)
);

comment on table fiscal_supto_export_schemas is
  'Machine-readable completeness registry for the normative SUPTO 18.1-18.9 layouts; application tests pin the ordered columns.';
