#!/usr/bin/env bash
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
COMMON="$HERE/../common-modules"
OUT="${TMPDIR:-/tmp}/beefiscal-protocol-facade-test"

c++ -std=c++17 -Wall -Wextra -Werror \
  -I"$HERE/tests" \
  -I"$HERE/include" -I"$HERE/src" \
  -I"$COMMON/daisy" -I"$COMMON/datecs" -I"$COMMON/termol" -I"$COMMON/datecspay" \
  "$HERE/src/ProtocolFactory.cpp" \
  "$HERE/src/DaisyFiscalAdapter.cpp" "$HERE/src/DatecsFiscalAdapter.cpp" \
  "$HERE/src/TremolFiscalAdapter.cpp" "$HERE/src/DatecsPayTerminalAdapter.cpp" \
  "$COMMON/daisy/DaisyProtocol.cpp" "$COMMON/daisy/DaisyPrinter.cpp" \
  "$COMMON/datecs/DatecsProtocol.cpp" "$COMMON/datecs/DatecsPrinter.cpp" \
  "$COMMON/termol/TremolProtocol.cpp" "$COMMON/termol/TremolPrinter.cpp" \
  "$COMMON/datecspay/DatecsPay.cpp" \
  "$HERE/tests/ProtocolFacadeTest.cpp" -o "$OUT"
"$OUT"
echo "protocol-abstraction tests: OK"
