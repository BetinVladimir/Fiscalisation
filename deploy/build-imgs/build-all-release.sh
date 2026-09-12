#!/usr/bin/env bash
set -Eeuo pipefail

if [[ $# -ne 1 || -z "${1}" ]]; then
  echo "Usage: $0 <version> (example: $0 1.2.3)" >&2
  exit 64
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${1}"
SCRIPTS=(
  build-fiscal-backend-release.sh
  build-beeminipos-backend-release.sh
)

for script in "${SCRIPTS[@]}"; do
  echo "Building and publishing ${script} ${VERSION}"
  "${SCRIPT_DIR}/${script}" "${VERSION}"
done

echo "All release images for ${VERSION} have been built and published"

