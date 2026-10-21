#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
command -v node >/dev/null || { echo 'Node is required for actual HTTP/native confirmation gate' >&2; exit 1; }
command -v go >/dev/null || { echo 'Go is required for actual HTTP/native confirmation gate' >&2; exit 1; }
cd "$root/minipos/beeminipos-backend"
MINIPOS_CLOUD_CONFIRM_INTEROP=1 go test ./internal/api -run 'TestNativeCloudConfirmationActualHTTP|TestPublicOrderConfirmationRedactsPrivateFields' -count=1 -v
