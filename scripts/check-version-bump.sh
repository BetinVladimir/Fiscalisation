#!/usr/bin/env bash
# check-version-bump.sh — SUPTO version bumped when sensitive files change (VER-001..002)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$ROOT/fiscal-backend/internal/version/version.go"
SENSITIVE_PATHS="$ROOT/scripts/supto-sensitive-paths.txt"
BASE="${BASE_BRANCH:-main}"

errors=0

# VER-001: version file exists
if [ ! -f "$VERSION_FILE" ]; then
  echo "VER-001: fiscal-backend/internal/version/version.go not found" >&2
  errors=$((errors+1))
fi

# VER-002: if sensitive paths changed since base, version must change or have trailer
VERSION_CHANGED=false
if git -C "$ROOT" diff --name-only "${BASE}...HEAD" 2>/dev/null | grep -qF "fiscal-backend/internal/version/version.go"; then
  VERSION_CHANGED=true
fi

if [ -f "$SENSITIVE_PATHS" ]; then
  sensitive_changed=false
  while IFS= read -r path; do
    [ -z "$path" ] && continue
    [[ "$path" =~ ^# ]] && continue
    if git -C "$ROOT" diff --name-only "${BASE}...HEAD" 2>/dev/null | grep -qF "$path"; then
      sensitive_changed=true
      break
    fi
  done < "$SENSITIVE_PATHS"

  if $sensitive_changed && ! $VERSION_CHANGED; then
    # Check for Supto-Version-Unchanged trailer in recent commits
    if ! git -C "$ROOT" log "${BASE}..HEAD" --format="%B" 2>/dev/null | grep -q 'Supto-Version-Unchanged:'; then
      echo "VER-002: sensitive SUPTO files changed but version not bumped and no Supto-Version-Unchanged trailer" >&2
      errors=$((errors+1))
    fi
  fi
fi

if [ $errors -eq 0 ]; then
  echo "VERSION-BUMP OK"
  exit 0
else
  echo "VERSION-BUMP FAIL: $errors checks failed" >&2
  exit 1
fi
