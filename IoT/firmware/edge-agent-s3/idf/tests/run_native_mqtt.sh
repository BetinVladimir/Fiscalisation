#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$(mktemp "${TMPDIR:-/tmp}/beefiscal-mqtt-native.XXXXXX")"
trap 'rm -f "$OUT"' EXIT
c++ -std=c++17 -Wall -Wextra -Werror ${CXXFLAGS:-} -I"$HERE/mqtt_native_include" -I"$HERE/../main" "$HERE/../main/mqtt_runtime.cpp" "$HERE/mqtt_runtime_native.cpp" -o "$OUT"
"$OUT"
