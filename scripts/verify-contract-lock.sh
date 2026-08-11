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
check 9718dc4c9444c514ed49197134724ef7733972febb6742b28b11f26a4f92a5cb "$root/contracts/openapi-runtime-v1.yaml"
check 03b3da61dee0b66513b1dea23f47749bbc283deb3f22377b1d1e55bbc612a0a0 "$root/contracts/openapi-corrections-v1.yaml"
