#!/bin/sh
# check-adr-links.sh — verify test names in ADR files can be found via grep -r in repo.
# For each ADR-*-001.md in docs/decisions/, extract test names from
# "## Тесты" section and verify each is findable in the codebase.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
adr_dir="$root/docs/decisions"
missing=0

for adr in "$adr_dir"/ADR-*-001.md; do
  [ -f "$adr" ] || continue
  name=$(basename "$adr")
  # Extract test names: lines containing backtick-quoted identifiers under "Тесты" section
  in_tests=0
  while IFS= read -r line; do
    case "$line" in
      '## Тесты'*) in_tests=1; continue ;;
      '## '*) in_tests=0 ;;
    esac
    [ "$in_tests" -eq 0 ] && continue
    # Extract Test* identifiers from backtick-quoted words
    test_name=$(printf '%s\n' "$line" | grep -o '`Test[A-Za-z0-9_]*`' | tr -d '`' | head -1)
    [ -z "$test_name" ] && continue
    found=$(grep -r --include="*.go" --include="*.kt" --include="*.ts" --include="*.py" \
      -l "$test_name" "$root" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$found" -eq 0 ]; then
      printf 'MISSING_TEST %s in %s\n' "$test_name" "$name" >&2
      missing=$((missing+1))
    fi
  done < "$adr"
done

if [ "$missing" -gt 0 ]; then
  printf '%d test name(s) not found in codebase\n' "$missing" >&2
  exit 1
fi
adr_count=$(ls "$adr_dir"/ADR-*-001.md 2>/dev/null | wc -l | tr -d ' ')
printf 'OK: %d ADR files checked\n' "$adr_count"
exit 0
