# Stage 1 — qwen code prompt template

**Budget:** ≤ 6 000 tokens input, ≤ 8 000 tokens output, diff ≤ 400 lines.

---

You are executing subtask **<ID.N>**.

Edit **only** the files listed in the subtask's `files` field.
First write the test from the `test` field and show it failing (red phase).
Then implement the minimal fix. Return a unified diff.
Do **not** change any file outside the list. Do **not** refactor.

Invariants (enforce without exception):
1. Money = integer cents; no `float`/`double`/`parseFloat`/`Number(price)`; rounding `(unit¢×qty_milli+500)/1000`.
2. UNKNOWN ≠ FAILED: after a write timeout do not retry or auto-refund.
3. Storno-first: card refund only after confirmed fiscal void.
4. Idempotency: same key must not produce a second document.
5. Storage error stops monetary side-effect.
6. Document number comes only from the fiscal unit response — no synthetic numbers.
7. Datecs: LEN=data+0x2A, SEQ 0x20…0x7F. Daisy: LEN=data+0x24, SEQ 0x20…0xFF. Text in CP1251.
8. Secrets must not appear in logs, commits, or tests.

Return only the diff, no narrative.
