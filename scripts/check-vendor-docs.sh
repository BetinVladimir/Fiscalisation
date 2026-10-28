#!/usr/bin/env bash
# check-vendor-docs.sh — SHA256 manifest of vendor documents (VND-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENDOR_DIR="$ROOT/docs/vendor"
MANIFEST="$ROOT/docs/vendor/manifest.sha256"

errors=0

# VND-001: vendor docs directory and manifest exist
if [ ! -d "$VENDOR_DIR" ]; then
  echo "VND-001: docs/vendor/ directory not found" >&2
  errors=$((errors+1))
fi

if [ ! -f "$MANIFEST" ]; then
  echo "VND-001: docs/vendor/manifest.sha256 not found" >&2
  errors=$((errors+1))
fi

# VND-002: all files in manifest exist and checksums match
if [ -f "$MANIFEST" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    [[ "$line" =~ ^# ]] && continue
    checksum="${line%% *}"
    path="${line##* }"
    path="${path#\*}"  # strip leading * if present
    full="$ROOT/$path"
    if [ ! -f "$full" ]; then
      echo "VND-002: manifest file not found: $path" >&2
      errors=$((errors+1))
    else
      actual=$(shasum -a 256 "$full" | cut -d' ' -f1)
      if [ "$actual" != "$checksum" ]; then
        echo "VND-002: checksum mismatch: $path (expected $checksum, got $actual)" >&2
        errors=$((errors+1))
      fi
    fi
  done < "$MANIFEST"
fi

if [ $errors -eq 0 ]; then
  echo "VENDOR-DOCS OK"
  exit 0
else
  echo "VENDOR-DOCS FAIL: $errors checks failed" >&2
  exit 1
fi
