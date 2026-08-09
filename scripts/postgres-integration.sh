#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
network="beeloy_pg_it_$$"
fiscal_db="beeloy_fiscal_pg_it_$$"
minipos_db="beeloy_minipos_pg_it_$$"
postgres_image=postgres:16.10
go_image=golang:1.26.3

cleanup() {
  docker rm -f "$fiscal_db" "$minipos_db" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_postgres() {
  container=$1
  attempts=0
  until docker exec "$container" pg_isready -U postgres -d app >/dev/null 2>&1; do
    attempts=$((attempts+1))
    if [ "$attempts" -ge 60 ]; then
      docker logs "$container" >&2 || true
      return 1
    fi
    sleep 1
  done
}
wait_schema() {
  container=$1
  marker=$2
  attempts=0
  until [ "$(docker exec "$container" psql -At -U postgres -d app -c "select to_regclass('public.$marker') is not null" 2>/dev/null || true)" = "t" ]; do
    attempts=$((attempts+1))
    if [ "$attempts" -ge 60 ]; then
      docker logs "$container" >&2 || true
      echo "timeout waiting for schema marker $marker" >&2
      return 1
    fi
    sleep 1
  done
}

docker network create "$network" >/dev/null
docker run -d --name "$fiscal_db" --network "$network" \
  -e POSTGRES_PASSWORD=test -e POSTGRES_DB=app -e FISCAL_RLS_DB_PASSWORD=test-reader \
  --tmpfs /var/lib/postgresql/data:rw,size=256m \
  -v "$root/database/fiscal:/docker-entrypoint-initdb.d:ro" \
  "$postgres_image" >/dev/null
docker run -d --name "$minipos_db" --network "$network" \
  -e POSTGRES_PASSWORD=test -e POSTGRES_DB=app -e MINIPOS_RLS_DB_PASSWORD=test-reader \
  --tmpfs /var/lib/postgresql/data:rw,size=256m \
  -v "$root/database/minipos:/docker-entrypoint-initdb.d:ro" \
  "$postgres_image" >/dev/null

wait_postgres "$fiscal_db"
wait_postgres "$minipos_db"
wait_schema "$fiscal_db" fiscal_runtime_operations
wait_schema "$minipos_db" minipos_runtime_orders

# Prove database-enforced isolation using the non-owner/no-BYPASSRLS roles.
fiscal_forced=$(docker exec "$fiscal_db" psql -At -v ON_ERROR_STOP=1 -U postgres -d app -c \
  "select count(*) from pg_class where relname=any(array['tenants','locations','registers','operators','shifts','devices','sales','sale_lines','fiscal_operations','operation_events','idempotency_records','audit_events','artifacts','edge_sync_acks','fiscal_runtime_sales','fiscal_runtime_operations','fiscal_runtime_shifts','fiscal_runtime_resources','fiscal_runtime_artifacts','fiscal_runtime_sync_acks']) and relrowsecurity and relforcerowsecurity")
if [ "$fiscal_forced" != "20" ]; then
  echo "Fiscal typed model does not FORCE RLS on every tenant table ($fiscal_forced/20)" >&2
  exit 1
fi
docker exec "$fiscal_db" psql -v ON_ERROR_STOP=1 -U postgres -d app -q -c \
  "insert into tenants(id,status) values('10000000-0000-0000-0000-000000000001','ACTIVE'),('10000000-0000-0000-0000-000000000002','ACTIVE')"
fiscal_visible=$(docker exec "$fiscal_db" psql -At -v ON_ERROR_STOP=1 -U postgres -d app -c \
  "set role beefiscal_tenant; set app.tenant_id='10000000-0000-0000-0000-000000000001'; select count(*) from tenants")
if [ "$(printf '%s\n' "$fiscal_visible" | tail -n 1)" != "1" ]; then
  echo "Fiscal RLS exposed another tenant" >&2
  exit 1
fi
if docker exec "$fiscal_db" psql -v ON_ERROR_STOP=1 -U postgres -d app -q -c \
  "set role beefiscal_tenant; set app.tenant_id='10000000-0000-0000-0000-000000000001'; insert into tenants(id,status) values('10000000-0000-0000-0000-000000000003','ACTIVE')" >/dev/null 2>&1; then
  echo "Fiscal RLS accepted a cross-tenant insert" >&2
  exit 1
fi

minipos_forced=$(docker exec "$minipos_db" psql -At -v ON_ERROR_STOP=1 -U postgres -d app -c \
  "select count(*) from pg_class where relname=any(array['organizations','locations','registers','employees','products','shifts','orders','order_lines','webhook_inbox','outbox','minipos_runtime_products','minipos_runtime_employees','minipos_runtime_shifts','minipos_runtime_orders','minipos_runtime_configurations']) and relrowsecurity and relforcerowsecurity")
if [ "$minipos_forced" != "15" ]; then
  echo "MiniPOS typed model does not FORCE RLS on every organization table ($minipos_forced/15)" >&2
  exit 1
fi
docker exec "$minipos_db" psql -v ON_ERROR_STOP=1 -U postgres -d app -q -c \
  "insert into organizations(id,name,fiscal_external_id,status) values('20000000-0000-0000-0000-000000000001','A','30000000-0000-0000-0000-000000000001','ACTIVE'),('20000000-0000-0000-0000-000000000002','B','30000000-0000-0000-0000-000000000002','ACTIVE')"
minipos_visible=$(docker exec "$minipos_db" psql -At -v ON_ERROR_STOP=1 -U postgres -d app -c \
  "set role beeminipos_tenant; set app.organization_id='20000000-0000-0000-0000-000000000001'; select count(*) from organizations")
if [ "$(printf '%s\n' "$minipos_visible" | tail -n 1)" != "1" ]; then
  echo "MiniPOS RLS exposed another organization" >&2
  exit 1
fi
if docker exec "$minipos_db" psql -v ON_ERROR_STOP=1 -U postgres -d app -q -c \
  "set role beeminipos_tenant; set app.organization_id='20000000-0000-0000-0000-000000000001'; insert into organizations(id,name,fiscal_external_id,status) values('20000000-0000-0000-0000-000000000003','X','30000000-0000-0000-0000-000000000003','ACTIVE')" >/dev/null 2>&1; then
  echo "MiniPOS RLS accepted a cross-organization insert" >&2
  exit 1
fi

fiscal_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$fiscal_db")
minipos_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$minipos_db")

docker run --rm --network "$network" -v "$root:/src:ro" -w /src/fiscal-backend \
  -e "PG_INTEGRATION_URL=postgres://postgres:test@$fiscal_ip:5432/app?sslmode=disable" \
  -e "PG_RLS_INTEGRATION_URL=postgres://beefiscal_tenant:test-reader@$fiscal_ip:5432/app?sslmode=disable" \
  "$go_image" go test ./internal/persistence
docker run --rm --network "$network" -v "$root:/src:ro" -w /src/beeminipos-backend \
  -e "PG_INTEGRATION_URL=postgres://postgres:test@$minipos_ip:5432/app?sslmode=disable" \
  -e "PG_RLS_INTEGRATION_URL=postgres://beeminipos_tenant:test-reader@$minipos_ip:5432/app?sslmode=disable" \
  "$go_image" go test ./internal/persistence

echo "PostgreSQL persistence integration tests passed"
