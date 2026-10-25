# Recovery and unresolved failures

The runner must continue independent work after an ordinary task exhausts its repair attempts. Failed tasks are never PASS; dependent tasks remain BLOCKED. Final acceptance is forbidden while selected implementation tasks remain unresolved.

## Timeout diagnosis and repair

- `--timeout` limits each model invocation (default 1800 seconds).
- `--model-idle-timeout` limits inactivity in captured model output (default 900 seconds; 0 disables). Silence is a suspected stall, not proof that the GPU is idle.
- `--recoveries-per-stage` permits 0–2 fresh repair sessions per stage (default 1), within the outer `--attempts` budget.
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
