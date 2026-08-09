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
check 29d0d8833132c2616c0b1a47fd68b6ad3b705c5d02230a27e982f3f65b413fc4 "$root/contracts/openapi-runtime-v1.yaml"
check eb1c6cde0ca2b0c22714c116b0f6e9c40296bd269f3cf114093666745ad90d10 "$root/contracts/openapi-corrections-v1.yaml"
