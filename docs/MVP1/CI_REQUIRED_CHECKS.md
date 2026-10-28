# CI Required Checks — MVP1

All jobs listed here must pass before merging to `main`.
A failing required check blocks the PR.

## Required jobs

| Job | Command | Blocks merge |
|-----|---------|:------------:|
| `unit-tests` | `make test` | ✓ |
| `vet` | `make vet` | ✓ |
| `contracts` | `make contract-test` | ✓ |
| `governance` | `make governance-test` | ✓ |
| `supto-trace` | `make supto-trace-test` | ✓ |
| `security-scan` | `make security-test` | ✓ |
| `iot-build` | `make iot-build-check` | ✓ |
| `android-build` | `make android-build` | ✓ |
| `check-no-secrets` | `bash scripts/check-no-secrets.sh` | ✓ |
| `check-migrations` | `bash scripts/check-migrations.sh` | ✓ |

## Optional / advisory jobs

| Job | Command | Notes |
|-----|---------|-------|
| `loadtest` | `bash scripts/loadtest/run.sh --dry-run` | Run on release branches |
| `hil-dry-run` | `bash scripts/hil/run.sh --dry-run HIL-BC-01` | Confirms HIL harness ready |
| `e2e-smoke` | `make compose-e2e` | Requires MQTT stack |

## `contracts` job definition

The `contracts` job runs `make contract-test`, which:
1. Regenerates OpenAPI types into an isolated temp dir.
2. Requires byte-for-byte equality with committed generated files.
3. Compiles TypeScript client factories.
4. Verifies every `operationId` exists in the generated surface.

This job is **required** — it prevents schema drift from landing in `main`.

## Policy

- New CI checks must be added to this file before they become required.
- Optional checks may be promoted to required by updating this table and the CI pipeline file.
- HIL PASS evidence must come from real hardware; emulator runs do not satisfy the gate.
