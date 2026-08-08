#!/bin/sh
set -eu

: "${FISCAL_RLS_DB_PASSWORD:?FISCAL_RLS_DB_PASSWORD is required}"

psql -q -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=runtime_password="$FISCAL_RLS_DB_PASSWORD" <<'SQL'
select format('alter role beefiscal_tenant login password %L', :'runtime_password') \gexec
SQL
