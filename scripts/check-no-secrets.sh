#!/usr/bin/env bash
# Scan the repository for leaked secrets/passwords/keys/tokens.
# Exits non-zero if any pattern is found outside approved locations.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAIL=0

# Patterns to flag (Extended regex, case-insensitive where noted)
check() {
  local label="$1"
  local pattern="$2"
  local hits
  hits=$(grep -rn --include="*.go" --include="*.ts" --include="*.kt" --include="*.py" --include="*.sh" --include="*.env" \
    --exclude-dir=".git" --exclude-dir="node_modules" --exclude-dir="build" \
    --exclude-dir=".store-build" --exclude-dir="dist" --exclude-dir="vendor" \
    -E "$pattern" "$ROOT" 2>/dev/null \
    | grep -v 'check-no-secrets' \
    | grep -v '_test\.' \
    | grep -v 'Test\.kt' \
    | grep -v '/test/kotlin/' \
    | grep -v 'tests/mvp1/' \
    | grep -v 'example\.' \
    | grep -v 'docs/MVP1/fixes' \
    | grep -v 'docs/MVP1/fixed' \
    | grep -v 'test-only\|test_only\|for-test\|for_test' \
    || true)
  if [ -n "$hits" ]; then
    echo "FAIL ($label):"
    echo "$hits" | head -5
    FAIL=1
  fi
}

# Hard-coded passwords/secrets in assignments (exclude env-var references like "$VAR")
check "password-assignment" '(?i)(password|passwd|secret|api_key)\s*[:=]\s*"[^\$\{"][^"]{7,}"'
# Private key headers
check "private-key" 'BEGIN (RSA )?PRIVATE KEY'
# AWS access key format
check "aws-key" 'AKIA[0-9A-Z]{16}'
# JWT token (3-part base64url)
check "jwt-token" 'eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}'

if [ "$FAIL" -eq 0 ]; then
  echo "OK: no secrets detected"
fi
exit "$FAIL"
