#!/usr/bin/env bash
# check-data-location.sh — fiscal/receipt data stays in EU/BG (DLC-001..003)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ALLOWLIST="$ROOT/scripts/data-location-allowlist.txt"
DATA_LOCATION_FILE="${DATA_LOCATION_FILE:-$ROOT/docs/DATA_LOCATION.md}"
SKIP_HOSTNAME_CHECK="${SKIP_HOSTNAME_CHECK:-0}"

errors=0

# DLC-001/002: hostname allowlist checks (skipped when SKIP_HOSTNAME_CHECK=1)
if [ "$SKIP_HOSTNAME_CHECK" != "1" ]; then
  CONFIGS=()
  while IFS= read -r -d '' f; do
    CONFIGS+=("$f")
  done < <(find "$ROOT" \( -name 'compose.*.yaml' -o -name 'compose.*.yml' -o -name 'deploy.*.yaml' \) \
           -not -path '*/.git/*' -print0 2>/dev/null)

  if [ ${#CONFIGS[@]} -gt 0 ]; then
    # Collect external hostnames from compose files
    external_hosts=()
    while IFS= read -r line; do
      host=$(echo "$line" | grep -oE '://[a-zA-Z0-9._-]+|@[a-zA-Z0-9._-]+' | tr -d '://@')
      [ -z "$host" ] && continue
      # Must contain a dot to be a real external hostname (not a Docker service name)
      echo "$host" | grep -qF '.' || continue
      echo "$host" | grep -qE 'localhost|127\.|0\.0\.0\.|\.local$|\.internal$|\.svc$' && continue
      external_hosts+=("$host")
    done < <(cat "${CONFIGS[@]}" 2>/dev/null | grep -E '(url|host|endpoint|broker)\s*[:=]' | head -100)

    if [ ${#external_hosts[@]} -gt 0 ]; then
      # DLC-001: allowlist required when external hosts exist
      if [ ! -f "$ALLOWLIST" ]; then
        echo "DLC-001: scripts/data-location-allowlist.txt not found but ${#external_hosts[@]} external host(s) detected" >&2
        errors=$((errors+1))
      else
        # DLC-002: each external host must be in allowlist
        for host in "${external_hosts[@]}"; do
          if ! grep -qF "$host" "$ALLOWLIST" 2>/dev/null; then
            echo "DLC-002: host '$host' not in data-location-allowlist.txt" >&2
            errors=$((errors+1))
          fi
        done
      fi
    fi
  fi
fi

# DLC-003: DATA_LOCATION.md must exist and all listed regions must be in BG or EU
if [ ! -f "$DATA_LOCATION_FILE" ]; then
  echo "DLC-003: $DATA_LOCATION_FILE not found" >&2
  errors=$((errors+1))
else
  # Parse table rows (lines starting with |), skip separator rows (---)
  # Extract region column (3rd pipe-delimited field, index 3 after leading |)
  while IFS='|' read -r _ _svc _tech region _rest; do
    # Trim whitespace
    region=$(echo "$region" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
    [ -z "$region" ] && continue
    # Skip header/separator
    echo "$region" | grep -qiE '^(region|-+)$' && continue
    region_lc=$(echo "$region" | tr '[:upper:]' '[:lower:]')
    # Approved: contains "eu" or "bg" or known EU city names
    if ! echo "$region_lc" | grep -qiE 'eu|bg|frankfurt|amsterdam|ireland|paris|stockholm|milan|sofia|brussels|vienna|warsaw|prague|budapest|bucharest|lisbon|madrid|rome|athens'; then
      echo "DLC-003: region '$region' is not in BG or EU — forbidden region" >&2
      errors=$((errors+1))
    fi
  done < <(grep '|' "$DATA_LOCATION_FILE" | grep -v -- '---')
fi

if [ $errors -eq 0 ]; then
  echo "DATA-LOCATION OK"
  exit 0
else
  echo "DATA-LOCATION FAIL: $errors checks failed" >&2
  exit 1
fi
