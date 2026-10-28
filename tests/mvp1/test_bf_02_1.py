"""
BF-02.1 acceptance test: RED phase — email-otp-refresh.test.tsx exists and runs.
"""
import os
import re
import subprocess
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
APP_DIR = os.path.join(REPO_ROOT, "BeeFiscalApp")
TEST_FILE = os.path.join(APP_DIR, "__tests__", "email-otp-refresh.test.tsx")
JEST_CONFIG = os.path.join(APP_DIR, "jest.ui.config.cjs")


class TestBF021EmailOtpRed(unittest.TestCase):

    def test_test_file_exists(self):
        """email-otp-refresh.test.tsx must exist."""
        self.assertTrue(os.path.isfile(TEST_FILE), f"Test file missing: {TEST_FILE}")

    def test_apiError_file_exists(self):
        """src/apiError.ts must exist."""
        src = os.path.join(APP_DIR, "src", "apiError.ts")
        self.assertTrue(os.path.isfile(src), f"apiError.ts missing: {src}")

    def test_jest_runs_and_reports_results(self):
        """Jest must run the test file and report test counts."""
        result = subprocess.run(
            [
                "npx", "jest",
                "--config", JEST_CONFIG,
                "__tests__/email-otp-refresh.test.tsx",
                "--no-coverage",
            ],
            cwd=APP_DIR,
            capture_output=True,
            text=True,
            timeout=120,
        )
        combined = result.stdout + result.stderr
        self.assertIn(
            "Tests:",
            combined,
            f"Jest did not produce test count output.\n{combined[:2000]}",
        )
        match = re.search(r"Tests:.*?(\d+)\s+(?:failed|passed)", combined)
        self.assertIsNotNone(match, f"No test results.\n{combined[:2000]}")
        count = int(match.group(1))
        self.assertGreater(count, 0, "Jest must run at least 1 test")


if __name__ == "__main__":
    unittest.main()
