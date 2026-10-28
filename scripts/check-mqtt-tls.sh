#!/usr/bin/env bash
# check-mqtt-tls.sh — MQTT connections use TLS only in prod configs (TLS-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

errors=0

# TLS-001: no tcp:// or plain mqtt:// in prod configs
PROD_FILES=()
while IFS= read -r -d '' f; do
  PROD_FILES+=("$f")
done < <(find "$ROOT" \( -name 'compose.*.prod.yaml' -o -name 'compose.*.prod.yml' \
         -o -name '*.prod.env' \) -not -path '*/.git/*' -print0 2>/dev/null)

for f in "${PROD_FILES[@]+"${PROD_FILES[@]}"}"; do
  if grep -nE 'tcp://|mqtt://[^s]' "$f" 2>/dev/null | grep -q .; then
    echo "TLS-001: unencrypted MQTT URL in $f" >&2
    grep -nE 'tcp://|mqtt://[^s]' "$f" >&2 || true
    errors=$((errors+1))
  fi
done

# TLS-002: EMQX config requires client cert (tls_required = true or verify = verify_peer)
EMQX_CONF=()
while IFS= read -r -d '' f; do
  EMQX_CONF+=("$f")
done < <(find "$ROOT" -name 'emqx*.conf' -o -name 'emqx*.yaml' -not -path '*/.git/*' -print0 2>/dev/null)

for f in "${EMQX_CONF[@]+"${EMQX_CONF[@]}"}"; do
  if ! grep -qE 'verify\s*=\s*verify_peer|tls_required\s*=\s*true|require_certificate\s*=\s*true' "$f" 2>/dev/null; then
    echo "TLS-002: EMQX config $f does not require client cert" >&2
    errors=$((errors+1))
  fi
done

if [ ${#PROD_FILES[@]} -eq 0 ] && [ ${#EMQX_CONF[@]} -eq 0 ]; then
  echo "MQTT-TLS: no prod configs or EMQX configs found (OK — nothing to check)"
  exit 0
fi

if [ $errors -eq 0 ]; then
  echo "MQTT-TLS OK"
  exit 0
else
  echo "MQTT-TLS FAIL: $errors checks failed" >&2
  exit 1
fi
