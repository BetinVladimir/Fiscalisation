#!/bin/sh
# run-golden-tests.sh — TST-DEEP-14.1: compile and run DatecsPayGoldenFramesTest.cpp
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
c++ -std=c++17 -Wall -Wextra -Werror -fsanitize=undefined,bounds \
  -I"$root/tests" -I"$root" \
  "$root/DatecsPay.cpp" "$root/tests/DatecsPayGoldenFramesTest.cpp" \
  -o /tmp/beefiscal-datecspay-golden-test
/tmp/beefiscal-datecspay-golden-test
printf '%s\n' 'Datecs Pay golden frames test passed'
