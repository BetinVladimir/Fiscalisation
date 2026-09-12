#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS=(
  build-fiscal-backend-dev.sh
  build-beeminipos-backend-dev.sh
)

for script in "${SCRIPTS[@]}"; do
  echo "Building and publishing ${script}"
  "${SCRIPT_DIR}/${script}"
done

echo "All dev images have been built and published"

