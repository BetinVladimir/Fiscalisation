#!/usr/bin/env bash
# check-data-location.sh — fiscal/receipt data stays in EU (DLC-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ALLOWLIST="$ROOT/scripts/data-location-allowlist.txt"

errors=0

# DLC-001: allowlist file must exist
if [ ! -f "$ALLOWLIST" ]; then
  echo "DLC-001: scripts/data-location-allowlist.txt not found" >&2
  errors=$((errors+1))
fi

# DLC-002: all external hostnames in configs must be on allowlist
CONFIGS=()
while IFS= read -r -d '' f; do
  CONFIGS+=("$f")
done < <(find "$ROOT" \( -name 'compose.*.yaml' -o -name 'compose.*.yml' -o -name 'deploy.*.yaml' \) \
         -not -path '*/.git/*' -print0 2>/dev/null)

if [ -f "$ALLOWLIST" ] && [ ${#CONFIGS[@]} -gt 0 ]; then
  while IFS= read -r line; do
    # Extract hostnames: anything after :// or @ up to / or :
    host=$(echo "$line" | grep -oE '://[a-zA-Z0-9._-]+|@[a-zA-Z0-9._-]+' | tr -d '://@')
    [ -z "$host" ] && continue
    # Skip localhost and internal DNS
    echo "$host" | grep -qE 'localhost|127\.|0\.0\.0\.|\.local$|\.internal$|\.svc$' && continue
    if ! grep -qF "$host" "$ALLOWLIST" 2>/dev/null; then
      echo "DLC-002: host '$host' not in data-location-allowlist.txt" >&2
      errors=$((errors+1))
    fi
  done < <(cat "${CONFIGS[@]}" 2>/dev/null | grep -E '(url|host|endpoint|broker)\s*[:=]' | head -100)
fi

if [ $errors -eq 0 ]; then
  echo "DATA-LOCATION OK"
  exit 0
else
  echo "DATA-LOCATION FAIL: $errors checks failed" >&2
  exit 1
fi
