#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
CORE="$HERE/../../../../protocol-abstraction"
OUT="$(mktemp "${TMPDIR:-/tmp}/beefiscal-serial-native.XXXXXX")"
trap 'rm -f "$OUT"' EXIT
c++ -std=c++17 -Wall -Wextra -Werror ${CXXFLAGS:-} -I"$HERE/../main" -I"$CORE/include" "$HERE/../main/serial_exchange.cpp" "$CORE/src/FrameCodec.cpp" "$HERE/serial_exchange_native.cpp" -o "$OUT"
"$OUT"
