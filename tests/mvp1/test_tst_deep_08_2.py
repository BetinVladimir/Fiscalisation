"""
TST-DEEP-08.2 — crash-recovery regression: IdempotentHandle green.

Acceptance criterion: all crash-recovery Go tests in crash_test.go PASS
(zero failures), confirming the idempotent-handle fix is in place.
"""
import json
import os
import subprocess
import unittest

ERP_DIR = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..", "..", "POS", "erp-connector")
)
ARTIFACT_DIR = os.environ.get("MVP1_ARTIFACT_DIR", "/tmp/mvp1_artifacts_tst_deep_08_2")


class TestCrashRecoveryGreen(unittest.TestCase):
    """TST-DEEP-08.2: all IdempotentHandle tests pass (green regression)."""

    def setUp(self):
        os.makedirs(ARTIFACT_DIR, exist_ok=True)

    def test_all_idempotent_handle_tests_pass(self):
        """go test must report PASS for every TestIdempotentHandle* test."""
        result = subprocess.run(
            [
                "go", "test", "-json", "-v",
                "-run", "TestIdempotentHandle",
                "./internal/orchestration/...",
            ],
            capture_output=True,
            text=True,
            cwd=ERP_DIR,
        )

        stdout = result.stdout
        stderr = result.stderr

        evidence_path = os.path.join(ARTIFACT_DIR, "go_test_08_2.jsonl")
        with open(evidence_path, "w", encoding="utf-8") as fh:
            fh.write(stdout)
        if stderr:
            with open(os.path.join(ARTIFACT_DIR, "go_test_08_2_stderr.txt"), "w") as fh:
                fh.write(stderr)

        # go test exit code 0 means all tests passed.
        self.assertEqual(
            result.returncode,
            0,
            f"go test returned non-zero (tests failed).\nstdout={stdout[:600]}\nstderr={stderr[:300]}",
        )

        # Parse JSON lines to count and inspect results.
        passed = []
        failed = []
        for line in stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            if ev.get("Action") == "pass" and ev.get("Test"):
                passed.append(ev["Test"])
            elif ev.get("Action") == "fail" and ev.get("Test"):
                failed.append(ev["Test"])

        self.assertGreater(
            len(passed),
            0,
            f"Expected ≥1 passing test; got 0. stdout={stdout[:500]}",
        )
        self.assertEqual(
            failed,
            [],
            f"Unexpected failing tests: {failed}",
        )

        # Ensure the three mandatory tests are among the passed ones.
        required = {
            "TestIdempotentHandleRetryAfterCrash",
            "TestIdempotentHandleNoDoubleExecuteAfterDone",
            "TestIdempotentHandleExecutesOnFirstCall",
        }
        missing = required - set(passed)
        self.assertEqual(
            missing,
            set(),
            f"Required crash-recovery tests did not pass: {missing}",
        )

    def test_full_orchestration_suite_passes(self):
        """Full orchestration test suite must not regress."""
        result = subprocess.run(
            [
                "go", "test", "-json",
                "./internal/orchestration/...",
            ],
            capture_output=True,
            text=True,
            cwd=ERP_DIR,
        )

        stdout = result.stdout
        evidence_path = os.path.join(ARTIFACT_DIR, "go_test_08_2_full.jsonl")
        with open(evidence_path, "w", encoding="utf-8") as fh:
            fh.write(stdout)

        self.assertEqual(
            result.returncode,
            0,
            f"Full orchestration suite failed.\nstdout={stdout[:800]}\nstderr={result.stderr[:300]}",
        )

        test_results = []
        for line in stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            if ev.get("Action") in ("pass", "fail") and ev.get("Test"):
                test_results.append(ev)

        self.assertGreater(
            len(test_results),
            0,
            "Expected ≥1 test result from full orchestration suite",
        )

        failed = [ev["Test"] for ev in test_results if ev["Action"] == "fail"]
        self.assertEqual(failed, [], f"Regressions in orchestration suite: {failed}")


if __name__ == "__main__":
    unittest.main()
