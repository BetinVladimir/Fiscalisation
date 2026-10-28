#!/usr/bin/env bash
# check-db-schema.sh — DB schema checks after migrations (DBS-001..002)
# Requires: PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE or DATABASE_URL
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [ -z "${DATABASE_URL:-}" ] && [ -z "${PGHOST:-}" ]; then
  echo "DBS-ENV: no DATABASE_URL or PGHOST set" >&2
  exit 3
fi

errors=0

psql_q() { psql "${DATABASE_URL:-}" -At -c "$1" 2>/dev/null; }

# DBS-001: fiscal.unp_allocations has required unique indexes
idx_out=$(psql_q "\di fiscal.unp_allocations_*" 2>/dev/null || echo "")
if ! echo "$idx_out" | grep -q 'tenant_id.*fiscal_device_number.*unp_sequence\|uniq.*tenant.*device.*seq'; then
  echo "DBS-001: missing UNIQUE index (tenant_id, fiscal_device_number, unp_sequence) on fiscal.unp_allocations" >&2
  errors=$((errors+1))
fi
if ! echo "$idx_out" | grep -q 'tenant_id.*allocation_id\|uniq.*tenant.*alloc'; then
  echo "DBS-001: missing UNIQUE index (tenant_id, allocation_id) on fiscal.unp_allocations" >&2
  errors=$((errors+1))
fi

# DBS-002: RLS enabled on listed tables
RLS_LIST="$ROOT/scripts/rls-tables.txt"
if [ -f "$RLS_LIST" ]; then
  while IFS= read -r tbl; do
    [ -z "$tbl" ] && continue
    [[ "$tbl" =~ ^# ]] && continue
    rls=$(psql_q "SELECT relrowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname||'.'||c.relname='${tbl}'" 2>/dev/null || echo f)
    if [ "$rls" != "t" ]; then
      echo "DBS-002: RLS not enabled on $tbl" >&2
      errors=$((errors+1))
    fi
  done < "$RLS_LIST"
fi

if [ $errors -eq 0 ]; then
  echo "DB-SCHEMA OK"
  exit 0
else
  echo "DB-SCHEMA FAIL: $errors checks failed" >&2
  exit 1
fi
