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
check 948669af9a4b1625dab08cf9aab679f73f427c461fb3fc7c0a3eea05c60bf494 "$root/contracts/openapi-runtime-v1.yaml"
check 03b3da61dee0b66513b1dea23f47749bbc283deb3f22377b1d1e55bbc612a0a0 "$root/contracts/openapi-corrections-v1.yaml"
