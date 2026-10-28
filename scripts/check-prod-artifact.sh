#!/usr/bin/env bash
# check-prod-artifact.sh — no stubs/simulators/test keys in prod build (PROD-001..003)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

errors=0

# PROD-001: no stub/simulator/fake markers in non-test production Go/Kotlin/TS source
STUB_PATTERNS=('//\s*STUB' 'FakeDevice\b' 'SimulatorAdapter\b' 'TODO.*prod' 'FIXME.*prod' 'AUTHORIZED_TEST')
for pat in "${STUB_PATTERNS[@]}"; do
  if rg --no-messages \
      --glob='*.go' --glob='*.kt' --glob='*.ts' --glob='*.tsx' \
      --glob='!*_test.*' --glob='!*Test.*' --glob='!*.spec.*' --glob='!*fake*' --glob='!*mock*' \
      -l "$pat" "$ROOT/fiscal-backend" "$ROOT/SmartDevices" "$ROOT/minipos" 2>/dev/null | grep -q .; then
    echo "PROD-001: stub pattern '$pat' in production source" >&2
    errors=$((errors+1))
  fi
done

# PROD-002: no test key literals in non-test files
if rg --no-messages \
    --glob='*.go' --glob='*.kt' --glob='*.ts' \
    --glob='!*_test.*' --glob='!*Test.*' \
    -l 'test[_-]?(key|secret|token|password)' "$ROOT/fiscal-backend/internal" 2>/dev/null | grep -q .; then
  echo "PROD-002: test key literal in production source" >&2
  errors=$((errors+1))
fi

# PROD-003: compose.prod must not reference :demo or :test image tags
COMPOSE_PROD=("$ROOT"/compose.*.prod.yaml "$ROOT"/compose.*.prod.yml)
for f in "${COMPOSE_PROD[@]}"; do
  [ -f "$f" ] || continue
  if grep -E 'image:.*:(demo|test|staging)\b' "$f" >/dev/null 2>&1; then
    echo "PROD-003: demo/test image tag in $f" >&2
    errors=$((errors+1))
  fi
done

if [ $errors -eq 0 ]; then
  echo "PROD-ARTIFACT OK"
  exit 0
else
  echo "PROD-ARTIFACT FAIL: $errors checks failed" >&2
  exit 1
fi
