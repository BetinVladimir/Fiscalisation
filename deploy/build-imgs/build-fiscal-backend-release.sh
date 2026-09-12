#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -ne 1 || -z "${1}" ]]; then
  echo "Usage: $0 <version> (example: $0 1.2.3)" >&2
  exit 64
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
IMAGE="${IMAGE:-ghcr.io/betinvladimir/fiscal-backend}"
VERSION="${1#v}"

docker buildx build \
  --platform "${PLATFORMS:-linux/amd64}" \
  --tag "${IMAGE}:${VERSION}" \
  --tag "${IMAGE}:latest" \
  --label "org.opencontainers.image.version=${VERSION}" \
  --label "org.opencontainers.image.revision=$(git -C "${REPO_ROOT}" rev-parse HEAD)" \
  --push \
  "${REPO_ROOT}/fiscal-backend"
