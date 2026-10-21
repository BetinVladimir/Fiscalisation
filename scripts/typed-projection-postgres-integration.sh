#!/bin/sh
# Disposable synthetic fixtures only. No production database or credentials.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
fixture=$(mktemp -d "${TMPDIR:-/tmp}/fiscal-typed-projection.XXXXXX")
name="fiscal-typed-projection-$(date -u +%Y%m%d%H%M%S)-$$"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  rm -rf "$fixture"
}
trap cleanup EXIT INT TERM
mkdir "$fixture/init"
printf 'CREATE SCHEMA fiscal;\n' > "$fixture/init/000_fiscal_schema.sql"
for migration in "$root"/database/fiscal/*.sql; do
  number=$(basename "$migration")
  number=${number%%_*}
  if [ "$number" -lt 22 ]; then
    { printf 'SET search_path=fiscal,public;\n'; sed 's/SET search_path = public, pg_temp/SET search_path = fiscal, pg_temp/g' "$migration"; } > "$fixture/init/$(basename "$migration")"
  fi
done
(
  cd "$root/fiscal-backend"
  GOCACHE="${TMPDIR:-/tmp}/fiscal-typed-projection-go-cache" TYPED_PROJECTION_FIXTURE="$fixture/domain.json" \
    go test ./internal/domain -run '^TestTypedProjectionDomainFixture$' -count=1
)
docker run -d --name "$name" -e POSTGRES_PASSWORD=synthetic-fixture-only -e POSTGRES_DB=app \
  --tmpfs /var/lib/postgresql/data:rw,size=256m \
  -v "$fixture/init:/docker-entrypoint-initdb.d:ro" -v "$fixture:/fixture:ro" \
  -v "$root:/source:ro" postgres:16.10 >/dev/null
attempts=0
stable=0
while [ "$stable" -lt 4 ]; do
  if docker exec "$name" psql -U postgres -d app -Atc \
    "select exists(select 1 from pg_constraint where conname='fiscal_audit_predecessor_consistent' and connamespace='fiscal'::regnamespace)" 2>/dev/null | grep -qx t; then
    stable=$((stable+1))
  else stable=0; fi
  attempts=$((attempts+1))
  if [ "$attempts" -ge 90 ]; then docker logs "$name" >&2; exit 1; fi
  sleep 1
done
docker exec "$name" psql -U postgres -d app -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA fiscal TO beefiscal_tenant"
docker exec "$name" psql -U postgres -d app -v ON_ERROR_STOP=1 -f /source/fiscal-backend/integration/typed_projection/pre.sql
docker exec "$name" psql -U postgres -d app -v ON_ERROR_STOP=1 -f /source/database/fiscal/022_runtime_typed_projection.sql
docker exec "$name" psql -U postgres -d app -v ON_ERROR_STOP=1 -f /source/fiscal-backend/integration/typed_projection/post.sql
