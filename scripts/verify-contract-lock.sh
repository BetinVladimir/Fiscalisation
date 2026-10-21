#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
docs=$(CDPATH= cd -- "$root/../BeeloyBackend/docs/Fiscal" && pwd)
check() {
  expected=$1
  file=$2
  actual=$(shasum -a 256 "$file" | awk '{print $1}')
  test "$actual" = "$expected" || { echo "contract drift: $file" >&2; exit 1; }
}
check e705067f12b695581922327ab9f85269ad622fcf9a5355050ff1a1235fe38b8c "$docs/api/openapi-public-v1.yaml"
check 16c9ebb272c272189d08843739b1e6ca439aff16844e7d307cab835212cfe797 "$docs/events/asyncapi-device-v1.yaml"
check d21c47241323915eb67d17d924eb2261915edf416c43991c338e9d18f7640397 "$root/contracts/openapi-runtime-v1.yaml"
check 54517a6826003ce5b7cf42d067ca8a6f76014943c5c2959ddfa327dd22e7884b "$root/contracts/openapi-corrections-v1.yaml"
