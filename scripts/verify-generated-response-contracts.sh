#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/beeloy-response-contracts.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
RESPONSE_CONTRACT_OUTPUT_ROOT="$work" ruby "$root/scripts/generate_response_contracts.rb" >/dev/null
gofmt -w "$work/fiscal-backend/internal/api"/*.go "$work/minipos/beeminipos-backend/internal/api"/*.go
for file in fiscal-backend/internal/api/response_contracts_gen.go fiscal-backend/internal/api/response_schema_validator_gen.go minipos/beeminipos-backend/internal/api/response_contracts_gen.go minipos/beeminipos-backend/internal/api/response_schema_validator_gen.go; do
  cmp "$work/$file" "$root/$file" || {
    echo "generated response contract drift: $file" >&2
    exit 1
  }
done
echo "generated OpenAPI contracts drift gate OK: canonical + runtime request/response validators match across 2 Go services (S3 uses its own contract tests)"
