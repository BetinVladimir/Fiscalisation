#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
c++ -std=c++17 -Wall -Wextra -Werror -fsanitize=undefined,bounds \
  -I"$root/tests" -I"$root" \
  "$root/DatecsPay.cpp" "$root/tests/DatecsPayTest.cpp" \
  -o /tmp/beefiscal-datecspay-test
/tmp/beefiscal-datecspay-test
printf '%s\n' 'Datecs Pay protocol tests passed'
