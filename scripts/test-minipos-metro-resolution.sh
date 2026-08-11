#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output=${1:-.regression-native}
log=$(mktemp /tmp/beeminipos-metro.XXXXXX)
cleanup() {
  rm -f "$log"
}
trap cleanup EXIT INT TERM

if ! (cd "$root/minipos/BeeMiniPOS" && EXPO_PUBLIC_APP_ENV=dev npx expo export --clear --platform all --output-dir "$output") >"$log" 2>&1; then
  cat "$log"
  exit 1
fi
cat "$log"

if grep -Fq '@noble/hashes/crypto.js" which is not listed in the "exports"' "$log"; then
  echo "Metro used the forbidden @noble/hashes file-resolution fallback" >&2
  exit 1
fi

echo "MiniPOS Metro resolution OK: package exports enforced without noble fallback"
