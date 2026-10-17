#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
CORE="$HERE/../../../../protocol-abstraction"
OUT="$(mktemp "${TMPDIR:-/tmp}/beefiscal-ble-native.XXXXXX")"
trap 'rm -f "$OUT"' EXIT
c++ -std=c++17 -pthread -Wall -Wextra -Werror ${CXXFLAGS:-} -I"$HERE/ble_native_include" -I"$HERE/../main" -I"$CORE/include" "$HERE/ble_runtime_native.cpp" -o "$OUT"
"$OUT"
