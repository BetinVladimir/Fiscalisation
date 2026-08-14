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
check 5aeacae5be26b5f8c6cb19b48e6725bb751dd7ad2ce6bee2773044964cdce203 "$docs/api/openapi-public-v1.yaml"
check 16c9ebb272c272189d08843739b1e6ca439aff16844e7d307cab835212cfe797 "$docs/events/asyncapi-device-v1.yaml"
check 2debc855b6006d38d3f14dc28b4da948e6fb2511e8f4982e486de71a60d2d4e4 "$root/contracts/openapi-runtime-v1.yaml"
check 5daf7640e20a610bc6c5bedfed84ca3e428d867cebb555be8bb9c043716342a4 "$root/contracts/openapi-corrections-v1.yaml"
