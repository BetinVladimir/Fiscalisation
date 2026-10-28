# scripts/

Utility and governance scripts for the Fiscalisation monorepo.

## Plan integrity

| Script | Description |
|--------|-------------|
| `check_plan_coverage.rb` | SCRIPT-01: status.json card IDs match fix files |
| `check_plan_acceptance.rb` | SCRIPT-02: acceptance commands reference real files |
| `check-test-matrix.sh` | SCRIPT-03: every TST-* card has an acceptance test |

## Security & artifacts

| Script | Description |
|--------|-------------|
| `check-no-secrets.sh` | SCRIPT-04: no hardcoded secrets in repo |
| `check-prod-artifact.sh` | SCRIPT-05: no stubs/simulators in prod build |

## Database & schema

| Script | Description |
|--------|-------------|
| `check-migrations.sh` | SCRIPT-06: migration numbering and idempotency |
| `check-db-schema.sh` | SCRIPT-22: DB indexes and RLS after migrations |

## Documentation

| Script | Description |
|--------|-------------|
| `check-ops-docs.sh` | SCRIPT-07: ops runbooks complete |
| `check-vendor-docs.sh` | SCRIPT-08: SHA256 manifest of vendor documents |
| `check-adr-links.sh` | SCRIPT-15: ADR references resolve |
| `check-doc-drift.sh` | SCRIPT-16: docs match code file paths |
| `check-doc-invariants.sh` | SCRIPT-17: ARCH_*.md H1/date, status.json schema |
| `check-decisions.sh` | SCRIPT-14: D-1..D-11 filled in DECISIONS.md |
| `check-norm-decisions.sh` | SCRIPT-13: NORM-* cards have decisions |
| `check-nap-baseline.sh` | SCRIPT-12: NAP regulation freeze baseline |

## Testing & vectors

| Script | Description |
|--------|-------------|
| `check-golden-vectors.sh` | SCRIPT-09: golden vector files exist |
| `check-datecs-error-catalog.sh` | SCRIPT-10: Datecs §7 error catalog ≥13 entries |
| `gen-datecs-error-catalog.py` | SCRIPT-11: generate error catalog from vendor spec |
| `check-rounding-golden.sh` | SCRIPT-20: rounding vectors match Go+Kotlin |
| `check-storno-coverage.sh` | SCRIPT-21: storno paths are covered |
| `mutate-vector.sh` | SCRIPT-32: mutation test for golden vectors |
| `verify_trace_tests_exist.rb` | SCRIPT-29: NAP req → test traceability |

## Lint & static analysis

| Script | Description |
|--------|-------------|
| `check-lint-rules.sh` | SCRIPT-18: custom lint rules enforced |
| `check-swallowed-errors.sh` | SCRIPT-19: no swallowed errors in Go/Kotlin |

## Infrastructure

| Script | Description |
|--------|-------------|
| `check-mqtt-tls.sh` | SCRIPT-23: MQTT uses TLS only in prod |
| `check-rabbit-topology.sh` | SCRIPT-24: RabbitMQ topology matches code |
| `check-data-location.sh` | SCRIPT-25: data stays in EU |
| `check-version-bump.sh` | SCRIPT-26: SUPTO version bumped on sensitive changes |
| `gen-annex29-columns.py` | SCRIPT-28: generate Annex 29 Go constants from CSV |
| `e2e-trace-check.sh` | SCRIPT-27: end-to-end trace identifier coverage |

## HIL & load testing

| Script | Description |
|--------|-------------|
| `hil/run.sh` | SCRIPT-30: HIL scenario runner |
| `loadtest/run.sh` | SCRIPT-31: load test runner (vegeta / go bench) |

## Makefile targets

| Target | Description |
|--------|-------------|
| `make lint` | SCRIPT-33: golangci-lint + ktlint + tsc + custom lint scripts |
| `make plan-check` | SCRIPT-34: aggregate all document integrity checks |
| `make fuzz-short` | SCRIPT-35: 30s fuzz run on codec decoders |
| `make contract-lock` | SCRIPT-36: alias to verify-contract-lock.sh |
