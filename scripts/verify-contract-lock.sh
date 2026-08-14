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
check f7f1a83122371336c7ac732ba8c2f7e7929c9a9d925bb1752213beb24d254c41 "$root/contracts/openapi-runtime-v1.yaml"
check b46b930eb09fcc1a284238b546e92b8bf663ba2cd27956a5504046f2ff6fca1a "$root/contracts/openapi-corrections-v1.yaml"
