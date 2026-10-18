#!/bin/sh
# Disposable fixtures only. Does not accept production credentials or reuse existing databases.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
name="beeloy-dossier-pg-$$"
image=postgres:16.10
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
# Random host port prevents collision with existing developer databases.
docker run -d --name "$name" -e POSTGRES_PASSWORD=fixture -e POSTGRES_DB=dossier \
 -e FISCAL_RLS_DB_PASSWORD=fixture-reader -p 127.0.0.1::5432 \
 --tmpfs /var/lib/postgresql/data:rw,size=256m \
 -v "$root/database/fiscal:/docker-entrypoint-initdb.d:ro" "$image" >/dev/null
port=$(docker port "$name" 5432/tcp | sed -n 's/^127\.0\.0\.1://p')
attempt=0
# Init server can briefly accept SQL before restarting: require final server plus marker.
until docker logs "$name" 2>&1 | tail -12 | grep -q 'database system is ready to accept connections' && \
 docker exec "$name" psql -U postgres -d dossier -Atc "select to_regclass('public.fiscal_supto_export_schemas') is not null" 2>/dev/null | grep -qx t; do
 attempt=$((attempt+1)); if [ "$attempt" -ge 90 ]; then docker logs "$name"; exit 1; fi
 sleep 1
done
# pg_isready plus an actual SQL query after initialization stabilizes the server.
sleep 2
cd "$root/fiscal-backend"
DOSSIER_TEST_ADMIN_URL="postgres://postgres:fixture@127.0.0.1:$port/dossier?sslmode=disable" \
DOSSIER_TEST_READER_URL="postgres://beefiscal_tenant:fixture-reader@127.0.0.1:$port/dossier?sslmode=disable" \
go test ./internal/dossier ./cmd/dossier-export -v -count=1
