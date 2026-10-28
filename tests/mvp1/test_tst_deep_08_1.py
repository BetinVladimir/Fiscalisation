"""
TST-DEEP-08.1 — crash after inbox commit, re-execute on restart (red/characteristic).

Acceptance criterion: test EXECUTES (non-zero native test count) and the
crash-recovery tests are present in crash_test.go.
"""
import json
import os
import subprocess
import unittest

ERP_DIR = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..", "..", "POS", "erp-connector")
)
CRASH_TEST_FILE = os.path.join(ERP_DIR, "internal", "orchestration", "crash_test.go")
ARTIFACT_DIR = os.environ.get("MVP1_ARTIFACT_DIR", "/tmp/mvp1_artifacts_tst_deep_08_1")


class TestCrashAfterInboxCommit(unittest.TestCase):
    """TST-DEEP-08.1: verify crash_test.go exists and native tests execute."""

    def setUp(self):
        os.makedirs(ARTIFACT_DIR, exist_ok=True)

    def test_crash_test_go_exists(self):
        """crash_test.go must exist in orchestration package."""
        self.assertTrue(
            os.path.isfile(CRASH_TEST_FILE),
            f"crash_test.go not found at {CRASH_TEST_FILE}",
        )

    def test_crash_test_go_contains_required_tests(self):
        """crash_test.go must contain the crash-recovery test functions."""
        with open(CRASH_TEST_FILE, encoding="utf-8") as fh:
            content = fh.read()
        self.assertIn(
            "TestIdempotentHandleRetryAfterCrash",
            content,
            "crash_test.go must define TestIdempotentHandleRetryAfterCrash",
        )
        self.assertIn(
            "TestIdempotentHandleNoDoubleExecuteAfterDone",
            content,
            "crash_test.go must define TestIdempotentHandleNoDoubleExecuteAfterDone",
        )

    def test_native_go_tests_execute_and_report_nonzero_count(self):
        """Run crash_test.go tests via go test -json and verify ≥1 test result."""
        result = subprocess.run(
            [
                "go", "test", "-json",
                "-run", "TestIdempotentHandle",
                "./internal/orchestration/...",
            ],
            capture_output=True,
            text=True,
            cwd=ERP_DIR,
        )

        stdout = result.stdout
        stderr = result.stderr

        # Write evidence.
        evidence_path = os.path.join(ARTIFACT_DIR, "go_test_08_1.jsonl")
        with open(evidence_path, "w", encoding="utf-8") as fh:
            fh.write(stdout)
        if stderr:
            with open(os.path.join(ARTIFACT_DIR, "go_test_08_1_stderr.txt"), "w") as fh:
                fh.write(stderr)

        # Parse JSON lines to count test events.
        test_results = []
        for line in stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            if ev.get("Action") in ("pass", "fail", "skip") and ev.get("Test"):
                test_results.append(ev)

        self.assertGreater(
            len(test_results),
            0,
            f"Expected ≥1 native test event; got 0. stdout={stdout[:500]} stderr={stderr[:300]}",
        )

        # All executed tests must pass.
        failed = [ev["Test"] for ev in test_results if ev["Action"] == "fail"]
        self.assertEqual(
            failed,
            [],
            f"Native crash tests failed: {failed}",
        )


if __name__ == "__main__":
    unittest.main()
