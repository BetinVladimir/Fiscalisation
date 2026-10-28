# Contributing to Fiscalisation

## Language policy

- **Code comments and identifiers** must be in **English**.
- **Operator-facing documentation** (runbooks, deployment guides, UI strings) must be in **Russian or Bulgarian** as appropriate for the target audience.
- **ADRs and architecture documents** in `docs/MVP1/` are written in Russian/Bulgarian for the development team.
- **Commit messages** should be in English.
- **Test names and code identifiers** must be in English.

This policy applies to all modules: fiscal-backend (Go), SmartDevices (Kotlin), IoT firmware (C/C++), POS (TypeScript), MiniPOS (TypeScript).

## Invariants

Money arithmetic must use integer cents only. Float/double are forbidden in monetary calculations. Rounding: half-up `(unit¢ × qty_milli + 500) / 1000`.

## Test standards

Tests must be sensitive to defects. A test that always passes without the feature is rejected. Never use `skip`, `expectedFailure`, or weaken assertions to make tests pass.

## Git workflow

1. One branch per task: `task/<ID>`.
2. Each subtask gets its own commit after acceptance.
3. Do not commit `docs/MVP1/execution/status.json` manually; only `apply_reverify.py` may change it.
4. Secrets (passwords, keys, keystores) must never appear in commits, logs, or tests.
