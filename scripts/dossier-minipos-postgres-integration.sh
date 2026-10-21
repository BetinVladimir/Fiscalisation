#!/bin/sh
# Disposable fixtures only. Does not accept production credentials or reuse existing databases.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
name="beeloy-minipos-dossier-pg-$$"
image=postgres:16.10
fixture=$(mktemp -d "${TMPDIR:-/tmp}/beeloy-minipos-fixture.XXXXXX")
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; rm -rf "$fixture"; }
trap cleanup EXIT INT TERM
python3 "$root/scripts/prepare-fiscal-pg-fixture.py" --source "$root/database/minipos" --output "$fixture/init" --namespace minipos
# Random host port prevents collision with existing developer databases.
docker run -d --name "$name" -e POSTGRES_PASSWORD=fixture -e POSTGRES_DB=dossier \
 -e MINIPOS_RLS_DB_PASSWORD=fixture-reader -p 127.0.0.1::5432 \
 --tmpfs /var/lib/postgresql/data:rw,size=256m \
 -v "$fixture/init:/docker-entrypoint-initdb.d:ro" "$image" >/dev/null
port=$(docker port "$name" 5432/tcp | sed -n 's/^127\.0\.0\.1://p')
attempt=0
# Init server can briefly accept SQL before restarting: require final server plus marker.
until docker logs "$name" 2>&1 | tail -12 | grep -q 'database system is ready to accept connections' && \
 docker exec "$name" psql -U postgres -d dossier -Atc "select to_regclass('minipos.minipos_runtime_tax_groups') is not null" 2>/dev/null | grep -qx t; do
 attempt=$((attempt+1)); if [ "$attempt" -ge 90 ]; then docker logs "$name"; exit 1; fi
 sleep 1
done
# pg_isready plus an actual SQL query after initialization stabilizes the server.
sleep 2
cd "$root/fiscal-backend"
if [ -n "${DOSSIER_DICTIONARY_OUTPUT:-}" ]; then
 DOSSIER_DATABASE_URL="postgres://postgres:fixture@127.0.0.1:$port/dossier?sslmode=disable" \
 go run ./cmd/dossier-export --dictionary --profile minipos --schema minipos --output "$DOSSIER_DICTIONARY_OUTPUT"
 exit 0
fi
DOSSIER_MINIPOS_TEST_ADMIN_URL="postgres://postgres:fixture@127.0.0.1:$port/dossier?sslmode=disable" \
DOSSIER_MINIPOS_TEST_READER_URL="postgres://beeminipos_tenant:fixture-reader@127.0.0.1:$port/dossier?sslmode=disable" \
go test ./internal/dossier ./cmd/dossier-export -v -count=1
