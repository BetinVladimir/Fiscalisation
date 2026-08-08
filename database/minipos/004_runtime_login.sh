#!/bin/sh
set -eu

: "${MINIPOS_RLS_DB_PASSWORD:?MINIPOS_RLS_DB_PASSWORD is required}"

psql -q -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=runtime_password="$MINIPOS_RLS_DB_PASSWORD" <<'SQL'
select format('alter role beeminipos_tenant login password %L', :'runtime_password') \gexec
SQL
