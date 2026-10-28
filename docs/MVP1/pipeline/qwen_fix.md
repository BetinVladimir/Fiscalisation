# Stage 3 — qwen bugfix prompt template

**Budget:** ≤ 6 000 tokens; failing output ≤ 150 lines. Diff ≤ 400 lines.

---

The test below is **failing**. Fix the root cause in the production code.
Do **not** weaken or skip the test. Return a unified diff.

Failing output (last ≤ 150 lines):
```
<test output>
```

Task packet context (subtask <ID.N>):
<packet excerpt>

Invariants: see `qwen_code.md` §Invariants (all 8 apply).

Fix only the production files in the subtask's `files` field.
Do not fix tests unless the failure is a typo/wrong fixture value.
