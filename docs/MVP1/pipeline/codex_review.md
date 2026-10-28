# Stage 2 — codex review prompt template

**Budget:** diff + task packet; large files permitted. Return JSON only.

---

Review the diff below against the 12-point checklist. Reply **only** with:

```json
{
  "verdict": "approve" | "changes",
  "findings": [
    { "file": "...", "line": 0, "sev": "P0"|"P1"|"P2"|"P3", "fix": "..." }
  ]
}
```

Do **not** rewrite code. `verdict=approve` only when no P0/P1 findings remain.

**Checklist:**
1. Money = integer cents; no `parseFloat`/`Number(price)`/`float`/`double` in monetary paths; rounding half-up.
2. UNKNOWN ≠ FAILED: no retry or card refund after write timeout.
3. Storno-first (D-12): card refund only after confirmed fiscal void.
4. Idempotency: same key must not produce a second document.
5. Storage error must stop monetary side-effect (GAP-017).
6. No synthetic document numbers (GAP-021); number comes from fiscal-unit response.
7. Frame encoding: Datecs LEN=data+0x2A, SEQ 0x20…0x7F; Daisy LEN=data+0x24, SEQ 0x20…0xFF; CP1251.
8. Secrets not in logs/commits/tests.
9. Test actually fails without the fix (not an empty test).
10. No changes outside the subtask file list; no opportunistic refactoring.
11. New numbers in tests come from fixtures or golden receipt, not from the code under test.
12. Error handling is typed; parsers do not throw exceptions outward.

**Diff:**
<diff>
