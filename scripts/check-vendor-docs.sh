#!/usr/bin/env bash
# check-vendor-docs.sh — SHA256 integrity check for vendor documents (VND-001..003)
# Uses docs/vendor/manifest.json (array of doc entries with file/sha256 fields).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VENDOR_DIR="$ROOT/docs/vendor"
MANIFEST="$ROOT/docs/vendor/manifest.json"

errors=0

# VND-001: vendor docs directory and manifest exist
if [ ! -d "$VENDOR_DIR" ]; then
  echo "VND-001: docs/vendor/ directory not found" >&2
  errors=$((errors+1))
fi

if [ ! -f "$MANIFEST" ]; then
  echo "VND-001: docs/vendor/manifest.json not found" >&2
  errors=$((errors+1))
fi

# VND-002: manifest has at least 7 entries
if [ -f "$MANIFEST" ]; then
  count=$(python3 -c "import json,sys; d=json.load(open('$MANIFEST')); print(len(d))" 2>/dev/null || echo 0)
  if [ "$count" -lt 7 ] 2>/dev/null; then
    echo "VND-002: manifest.json has only $count entries, need >= 7" >&2
    errors=$((errors+1))
  fi
fi

# VND-003: SHA256 verification for all entries where file and sha256 are non-null
if [ -f "$MANIFEST" ]; then
  python3 - "$ROOT" "$MANIFEST" <<'PYEOF'
import json
import sys
import hashlib
import os

root = sys.argv[1]
manifest_path = sys.argv[2]

with open(manifest_path) as f:
    entries = json.load(f)

errors = 0
for entry in entries:
    file_rel = entry.get("file")
    sha256_expected = entry.get("sha256")
    partial = entry.get("sha256_partial", False)
    entry_id = entry.get("id", "?")

    # Skip entries with no file or no sha256
    if not file_rel or not sha256_expected:
        continue

    full_path = os.path.join(root, file_rel)
    if not os.path.exists(full_path):
        print(f"VND-003: file not found for entry {entry_id}: {file_rel}", file=sys.stderr)
        errors += 1
        continue

    with open(full_path, "rb") as f:
        actual = hashlib.sha256(f.read()).hexdigest()

    if partial:
        # Partial SHA256 — check only the prefix
        if not actual.startswith(sha256_expected):
            print(f"VND-003: SHA256 prefix mismatch for {entry_id} ({file_rel}): "
                  f"expected prefix {sha256_expected!r}, got {actual[:len(sha256_expected)]!r}", file=sys.stderr)
            errors += 1
    else:
        if actual != sha256_expected:
            print(f"VND-003: SHA256 mismatch for {entry_id} ({file_rel}): "
                  f"expected {sha256_expected!r}, got {actual!r}", file=sys.stderr)
            errors += 1

sys.exit(errors)
PYEOF
  py_rc=$?
  if [ $py_rc -ne 0 ]; then
    errors=$((errors + py_rc))
  fi
fi

if [ $errors -eq 0 ]; then
  echo "VENDOR-DOCS OK"
  exit 0
else
  echo "VENDOR-DOCS FAIL: $errors checks failed" >&2
  exit 1
fi
