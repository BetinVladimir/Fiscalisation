# Recovery and unresolved failures

The runner must continue independent work after an ordinary task exhausts its repair attempts. Failed tasks are never PASS; dependent tasks remain BLOCKED. Final acceptance is forbidden while selected implementation tasks remain unresolved.

## Timeout diagnosis and repair

- `--timeout` is the total model wall-time budget for one unit attempt (default 1800 seconds), shared across red, green and any explicitly enabled recovery sessions. Controller checks/tests run outside this model budget.
- `--model-idle-timeout` limits inactivity in captured model output (default 900 seconds; 0 disables). Silence is a suspected stall, not proof that the GPU is idle.
- `--attempts` defaults to 1. `--recoveries-per-stage` defaults to 0: no hidden second attempt after timeout. Explicit legacy overrides permit up to 2 repair sessions per stage, still within the shared model-time budget.
- On a timeout, terminate the owned process group, preserve the transcript and changed-file snapshots, and write `artifacts/<unit>/recovery/<attempt>/<stage><retry>/diagnosis.json`.
- Check scope, original repositories, immutable harness and sealed green tests before repair. Restore the original scoped production bytes before a red retry. Keep normal test expectations in every phase.
- The repair session receives the diagnosis, transcript tail and concrete repair instructions for the same unit/stage. It must diagnose before editing and run the exact acceptance command.
- Tests still require verified red evidence followed by green. A repaired model session alone is not acceptance.

## Skip and continue

Ordinary exhausted generation/acceptance failures are recorded as `FAILED_UNRESOLVED`. Remaining units in that task are not executed; independent tasks continue. Dependent tasks are recorded as `BLOCKED_DEPENDENCY`.

The execution report is **`<state-dir>/UNRESOLVED_FAILURES.md`**. Machine-readable entries are `<unit>.failure.json`; transcripts and mutation snapshots remain alongside them. Successful receipts use their existing schema and never stand in for failure records.

Unchanged failures are skipped on resume. Use `--retry-failed` after a concrete repair, or update the reviewed task packet. A failure marker is archived as resolved only after a real red/green receipt is written; the Markdown history remains.

`--fail-fast` restores stopping at the first exhausted ordinary task. Scope violations, modified original repositories, tampered tests/harnesses and invalid receipts remain integrity stops: record `INTEGRITY_STOP` and halt rather than continue against a potentially corrupted baseline.

The complete queue exits nonzero while unresolved tasks remain and must not emit ACCEPTED. External/HIL gates require their own evidence. The monitoring agent diagnoses remaining failures and resumes repaired work until the authorized development scope is complete; final code review stays a separate gate.

After each red phase, the runner also saves exact declared test-file bytes and their hashes under `artifacts/<unit>/red/sealed/<attempt>/`. Integrity stops must be diagnosed before any automatic update/restart; the safe-boundary installer refuses exit code 2.


## Lightweight inventory guards

`--guard-mode light` (default) reuses complete repository inventories only during
controller-owned, quiescent portions of one unit. Every model subprocess, acceptance
run, scaffold preparation and restoration of red mutations invalidates the cache.
Failure and timeout paths invalidate it too. The cache is discarded on unit exit,
including exceptions. `--guard-mode strict` disables this reuse for comparison.

All workspace repositories and protected source repositories are still checked after
every external model/test operation. Scope, Git state, immutable harnesses and sealed
red tests retain their existing gates. Task closure and final regression use fresh
inventories; no cached or skipped test result can create a PASS. This deliberately
keeps stronger guards than checking only the affected repository. Concurrent manual
changes to the isolated workspace are unsupported and are detected at the next fresh
scan. Inventory metrics are saved to `.mvp-fixes-state/guard-metrics.json` on runner
exit; timings cover inventories rather than model generation or test execution.

Measurement on 2026-10-25: six quiescent inventory requests over 186,553 Backend
entries took 41.614 s in strict mode and 6.874 s in light mode, with identical
snapshots (initial warm-up excluded). A complete deterministic red/green runner
fixture used 32 full scans in strict mode versus 16 in light mode; both returned
TESTED_REVIEW_REQUIRED with genuine red evidence. These measurements do not predict
Qwen generation speed or the duration of the complete MVP1 queue.


## One-attempt policy

The supervised full run uses `--attempts 1 --recoveries-per-stage 0 --timeout 1800`.
A successful attempt still has separate red and green model sessions, sharing one
30-minute model budget. A failed acceptance, missing harness or timeout ends that
unit attempt. No extra model call is made to decompose the failure.

Alongside `UNRESOLVED_FAILURES.md`, `DECOMPOSITION_QUEUE.md` and private
`artifacts/<unit>/decomposition/*.json` retain the original criterion, exact scope,
acceptance, failure and a proposal for smaller work. Proposals are explicitly
`PROPOSED_NOT_EXECUTABLE`: inspect real code, preserve all original requirements
and verify replacement packets before retrying after the independent queue.
These records never count as completion or PASS. Integrity failures still stop
for diagnosis and safe restoration.
