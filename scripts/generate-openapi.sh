#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/beeloy-effective-openapi.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
ruby "$root/scripts/build_effective_openapi.rb" "$work/openapi-public-v1.yaml"
"$root/minipos/BeeMiniPOS/node_modules/.bin/openapi-typescript" "$work/openapi-public-v1.yaml" --output "$root/contracts/generated/openapi-public-v1.d.ts"
"$root/minipos/BeeMiniPOS/node_modules/.bin/openapi-typescript" "$root/contracts/openapi-runtime-v1.yaml" --output "$root/contracts/generated/openapi-runtime-v1.d.ts"
ruby "$root/scripts/generate_response_contracts.rb"
gofmt -w "$root/fiscal-backend/internal/api/response_contracts_gen.go" "$root/fiscal-backend/internal/api/response_schema_validator_gen.go" \
  "$root/minipos/beeminipos-backend/internal/api/response_contracts_gen.go" "$root/minipos/beeminipos-backend/internal/api/response_schema_validator_gen.go" \
  "$root/edge-agent/localapi/response_contracts_gen.go" "$root/edge-agent/localapi/response_schema_validator_gen.go"
