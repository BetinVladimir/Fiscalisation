# Stage 4 — codex e2e prompt template

**Budget:** full context; large logs permitted.

---

Run the e2e scenarios for wave **<N>** three times each, following ARCH_15 / ARCH_22.

For each scenario report:
```
scenario: <ID>  runs: 3/3 PASS | <k>/3 PASS (FAIL)
```

For any failure identify the root cause:
- **Product bug** → open a new `BUG-<NNN>` card in `docs/MVP1/fixes/` referencing the scenario.
- **Test / environment issue** → open `BUG-<NNN>` with tag `[test-infra]` and propose a fix.

Save the full run report to `docs/MVP1/execution/runs/e2e-<wave>.json` with schema:
```json
{
  "wave": <N>,
  "run_at": "<ISO timestamp>",
  "scenarios": [
    { "id": "...", "runs": 3, "passed": 3, "status": "PASS"|"FAIL", "bug": null | "BUG-NNN" }
  ],
  "overall": "PASS" | "FAIL"
}
```

Acceptance gate: every scenario in the wave achieves 3 of 3.
Red scenarios must become `BUG-*` entries before the wave closes.
