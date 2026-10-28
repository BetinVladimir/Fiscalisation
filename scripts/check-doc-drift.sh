#!/usr/bin/env bash
# check-doc-drift.sh — verify all rows in docs/MVP1/DOC_DRIFT.md have status RESOLVED.
# Exit 0 if no OPEN rows; exit 1 with list of open items on stderr.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
drift_file="$root/docs/MVP1/DOC_DRIFT.md"

if [ ! -f "$drift_file" ]; then
  printf 'ERROR: %s not found\n' "$drift_file" >&2
  exit 1
fi

# Count OPEN rows (table rows ending with | OPEN |)
open_count=$(grep -c '| OPEN |' "$drift_file" || true)
total_count=$(grep -c '^| DRIFT-' "$drift_file" || true)

if [ "$open_count" -gt 0 ]; then
  printf '%d OPEN drift item(s) remain:\n' "$open_count" >&2
  grep '| OPEN |' "$drift_file" >&2
  exit 1
fi

printf 'OK: all %d drift items RESOLVED\n' "$total_count"
exit 0
