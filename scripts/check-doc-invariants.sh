#!/bin/sh
# check-doc-invariants.sh — verify Invariants: doc-comment blocks before listed functions.
# Reads docs/MVP1/doc-required-functions.txt; for each entry searches the source file
# for an "Invariants:" marker in the 40 lines preceding the function definition.
# Exit 0 if all found; exit 1 with list of missing on stderr.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
list="$root/docs/MVP1/doc-required-functions.txt"

missing=0
while IFS=' ' read -r relfile funcpat; do
  case "$relfile" in '#'*|'') continue ;; esac
  [ -z "$funcpat" ] && continue
  srcfile="$root/$relfile"
  if [ ! -f "$srcfile" ]; then
    printf 'MISSING_FILE %s (%s)\n' "$relfile" "$funcpat" >&2
    missing=$((missing+1))
    continue
  fi
  # Find the line number of the function definition.
  lineno=$(grep -n "$funcpat" "$srcfile" | grep -v '^\s*//' | grep -v '^\s*\*' | head -1 | cut -d: -f1)
  if [ -z "$lineno" ]; then
    printf 'NOT_FOUND %s %s\n' "$relfile" "$funcpat" >&2
    missing=$((missing+1))
    continue
  fi
  # Check the 40 lines preceding the function for an Invariants: block.
  start=$((lineno - 40))
  [ "$start" -lt 1 ] && start=1
  lines=$((lineno - start))
  found=$(sed -n "${start},${lineno}p" "$srcfile" | grep -c 'Invariants:' || true)
  if [ "$found" -eq 0 ]; then
    printf 'NO_INVARIANTS %s:%s (%s)\n' "$relfile" "$lineno" "$funcpat" >&2
    missing=$((missing+1))
  fi
done < "$list"

if [ "$missing" -gt 0 ]; then
  printf '%d function(s) missing Invariants: block\n' "$missing" >&2
  exit 1
fi
printf 'all %d functions have Invariants: doc-comment\n' \
  "$(grep -v '^#' "$list" | grep -v '^[[:space:]]*$' | wc -l | tr -d ' ')"
exit 0
