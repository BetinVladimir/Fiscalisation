#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
IMAGE="${IMAGE:-ghcr.io/betinvladimir/fiscal-backend}"
GIT_SHA="$(git -C "${REPO_ROOT}" rev-parse --short=12 HEAD)"

docker buildx build \
  --platform "${PLATFORMS:-linux/amd64}" \
  --tag "${IMAGE}:dev" \
  --tag "${IMAGE}:dev-${GIT_SHA}" \
  --label "org.opencontainers.image.revision=$(git -C "${REPO_ROOT}" rev-parse HEAD)" \
  --push \
  "${REPO_ROOT}/fiscal-backend"
