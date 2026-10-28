# Contract Lock Matrix — MVP1

## Components with active contract-check

| Component | Lock file | Check command |
|-----------|-----------|---------------|
| `fiscal-backend` | `contracts/CONTRACT_LOCK.md` | `make contract-test` |
| `minipos/BeeMiniPOS` | `contracts/generated/` (TypeScript) | `make contract-test` (tsc step) |
| `POS/pos-gateway` | `contracts/openapi-runtime-v1.yaml` | `make contract-test` |

## Components WITHOUT contract-check (gap list)

The following components have no automated contract-check target.
Each row records the gap and the responsible card.

| Component | Gap | Tracking card |
|-----------|-----|---------------|
| `IoT/firmware/edge-agent-s3` (S3) | No schema lock for MQTT command envelope or CBOR frame format | TG-013 |
| `SmartDevices/bluecash-app` (BlueCash BC-50) | No lock on Datecs/Daisy serial-frame contract; version drift undetected | TG-013 |
| `POS/pos-gateway` | `openapi-corrections-v1.yaml` overlay not re-validated on schema change | TG-013 |

## Contract hash inventory

Schema SHAs are stored in `contracts/` lock files and must be regenerated via
`make contract-test` after any schema change.

| Schema file | Lock location |
|-------------|---------------|
| `contracts/openapi-runtime-v1.yaml` | `contracts/CONTRACT_LOCK.md` §sha256 |
| `contracts/openapi-corrections-v1.yaml` | `contracts/CONTRACT_LOCK.md` §sha256 |
| `contracts/beeloy-pos-deployment-v1.schema.json` | `contracts/pos-deployment-v1.lock` |
| HIL result schema | `contracts/hil-result.schema.json` + `contracts/hil-result.lock` |

## Closing the gap

Priority order to close each gap:
1. Add `bluecash_protocol.lock` with Datecs/Daisy frame version and feature flags.
2. Add IoT edge-agent CBOR envelope schema and `s3-envelope.lock`.
3. Expand `make contract-test` to validate overlay SHA after every `generate-openapi` run.
