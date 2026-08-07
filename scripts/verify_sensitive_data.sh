#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
pattern='card.?number|primary.?account|pin.?block|track.?[12]|cvv|cvc|(^|[^[:alpha:]])pan([^[:alpha:]]|$)'
if rg --line-number --ignore-case "$pattern" \
  "$root/fiscal-backend" "$root/beeminipos-backend" "$root/edge-agent" \
  "$root/BeeMiniPOS" "$root/BeeFiscalApp" "$root/SmartDevices" "$root/IoT" "$root/database" \
  -g '*.go' -g '*.ts' -g '*.tsx' -g '*.kt' -g '*.java' -g '*.cpp' -g '*.hpp' -g '*.h' -g '*.sql'; then
  echo "forbidden payment-sensitive field/token found in executable or storage source" >&2
  exit 1
fi
echo "sensitive payment data scan OK: no PAN/PIN/track/CVV fields in source or DDL"
