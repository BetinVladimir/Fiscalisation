#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
docs=$(CDPATH= cd -- "$root/../BeeloyBackend/docs/Fiscal" && pwd)
generator="$root/minipos/BeeMiniPOS/node_modules/.bin/openapi-typescript"
test -x "$generator" || { echo "openapi-typescript is not installed; run make deps" >&2; exit 1; }
work=$(mktemp -d "${TMPDIR:-/tmp}/beeloy-openapi.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
ruby "$root/scripts/build_effective_openapi.rb" "$work/openapi-public-v1.yaml"
"$generator" "$work/openapi-public-v1.yaml" --output "$work/openapi-public-v1.d.ts.generated" >/dev/null
"$generator" "$root/contracts/openapi-runtime-v1.yaml" --output "$work/openapi-runtime-v1.d.ts" >/dev/null
cmp "$work/openapi-public-v1.d.ts.generated" "$root/contracts/generated/openapi-public-v1.d.ts" || {
  echo "generated canonical OpenAPI types drifted; regenerate them with make generate-openapi" >&2
  exit 1
}
cmp "$work/openapi-runtime-v1.d.ts" "$root/contracts/generated/openapi-runtime-v1.d.ts" || {
  echo "generated runtime OpenAPI types drifted; regenerate them with make generate-openapi" >&2
  exit 1
}
echo "generated OpenAPI TypeScript drift gate OK"
