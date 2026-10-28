"""
BF-02.2 acceptance test: GREEN phase — all 3 email-otp-refresh tests must pass.
Verifies:
  - network error preserves session
  - 401 clears session
  - expires_in=30 schedules timer >= 30s
"""
import os
import re
import subprocess
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
APP_DIR = os.path.join(REPO_ROOT, "BeeFiscalApp")
TEST_FILE = os.path.join(APP_DIR, "__tests__", "email-otp-refresh.test.tsx")
SRC_FILE = os.path.join(APP_DIR, "src", "emailOtpAuth.ts")
JEST_CONFIG = os.path.join(APP_DIR, "jest.ui.config.cjs")


class TestBF022EmailOtpGreen(unittest.TestCase):

    def test_src_uses_ApiError(self):
        """emailOtpAuth.ts must import and use ApiError."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertIn("ApiError", src, "emailOtpAuth.ts must use ApiError")

    def test_src_timer_minimum_30s(self):
        """Timer must use 30_000 ms minimum, not 1000 ms."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertIn("30_000", src, "Timer minimum must be 30_000 ms")
        self.assertNotIn("Math.max(1000,", src.replace(" ", ""),
                         "Old 1000 ms timer minimum must be removed")

    def test_src_retryable_branch(self):
        """refresh() must have retryable branch that does not clear session."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertIn("retryable", src, "refresh must check retryable")

    def test_jest_all_pass(self):
        """All 3 Jest tests in email-otp-refresh.test.tsx must pass."""
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

        match_failed = re.search(r"Tests:.*?(\d+)\s+failed", combined)
        match_passed = re.search(r"Tests:.*?(\d+)\s+passed", combined)

        self.assertIsNotNone(
            match_passed,
            f"Jest produced no passed tests.\n{combined[:3000]}",
        )
        failed = int(match_failed.group(1)) if match_failed else 0
        passed = int(match_passed.group(1)) if match_passed else 0

        self.assertEqual(
            failed, 0,
            f"Jest reported {failed} failed test(s).\n{combined[:3000]}",
        )
        self.assertGreaterEqual(
            passed, 3,
            f"Expected >= 3 passing tests, got {passed}.\n{combined[:3000]}",
        )


if __name__ == "__main__":
    unittest.main()
