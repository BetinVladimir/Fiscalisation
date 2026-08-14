#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
descriptor="$root/minipos/miniposweb/dist/.well-known/beeloy-pos-deployment.json"
work=$(mktemp -d "${TMPDIR:-/tmp}/beeloy-spa-repro.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM

(cd "$root/minipos/miniposweb" && npm run deployment >/dev/null)
cp "$descriptor" "$work/first.json"
(cd "$root/minipos/miniposweb" && npm run deployment >/dev/null)
cmp -s "$work/first.json" "$descriptor" || {
  echo "SPA deployment descriptor is not reproducible" >&2
  exit 1
}

if (cd "$root/minipos/miniposweb" && APP_ENV=prod npm run deployment >"$work/prod.log" 2>&1); then
  echo "PROD deployment descriptor accepted a missing SOURCE_DATE_EPOCH" >&2
  exit 1
fi
grep -q SOURCE_DATE_EPOCH_REQUIRED "$work/prod.log"
echo "SPA deployment reproducibility OK: deterministic DEV signature; PROD requires SOURCE_DATE_EPOCH and managed key"
