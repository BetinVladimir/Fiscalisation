#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
c++ -std=c++17 -Wall -Wextra -Werror -fsanitize=undefined,bounds \
  -I"$root/tests" -I"$root" \
  "$root/DaisyProtocol.cpp" "$root/DaisyPrinter.cpp" \
  "$root/tests/DaisyProtocolTest.cpp" \
  -o /tmp/beefiscal-daisy-protocol-test
/tmp/beefiscal-daisy-protocol-test
printf '%s\n' 'Daisy protocol tests passed'
