#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
c++ -std=c++17 -Wall -Wextra -Werror \
  -I"$root/tests" -I"$root" \
  "$root/DatecsProtocol.cpp" "$root/DatecsPrinter.cpp" \
  "$root/tests/DatecsProtocolTest.cpp" \
  -o /tmp/beefiscal-datecs-protocol-test
/tmp/beefiscal-datecs-protocol-test
printf '%s\n' 'Datecs protocol tests passed'
