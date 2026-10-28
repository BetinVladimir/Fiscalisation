"""
BF-01.2 acceptance test: Run smart-device-ble-native.test.ts via Jest.
Verifies RED phase: tests exist and run (must fail due to stub implementation).
"""
import os
import subprocess
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
APP_DIR = os.path.join(REPO_ROOT, "BeeFiscalApp")
TEST_FILE = os.path.join(APP_DIR, "__tests__", "smart-device-ble-native.test.ts")
JEST_CONFIG = os.path.join(APP_DIR, "jest.ui.config.cjs")


class TestBF012BleNativeRed(unittest.TestCase):

    def test_test_file_exists(self):
        """smart-device-ble-native.test.ts must exist."""
        self.assertTrue(
            os.path.isfile(TEST_FILE),
            f"Test file missing: {TEST_FILE}",
        )

    def test_jest_runs_and_reports_results(self):
        """Jest must run the test file and report test counts (not error out on config issues)."""
        result = subprocess.run(
            [
                "npx",
                "jest",
                "--config",
                JEST_CONFIG,
                "__tests__/smart-device-ble-native.test.ts",
                "--no-coverage",
            ],
            cwd=APP_DIR,
            capture_output=True,
            text=True,
            timeout=120,
        )
        combined = result.stdout + result.stderr
        # Jest must report test counts (passes or failures)
        self.assertTrue(
            "Tests:" in combined,
            f"Jest did not produce test count output. Output:\n{combined[:2000]}",
        )
        # Must report at least 1 test
        import re
        match = re.search(r"Tests:.*?(\d+)\s+(?:failed|passed)", combined)
        self.assertIsNotNone(
            match,
            f"Jest did not report any test results. Output:\n{combined[:2000]}",
        )
        count = int(match.group(1))
        self.assertGreater(count, 0, "Jest must run at least 1 test")

    def test_src_file_exists(self):
        """src/smartDeviceBle.native.ts must exist."""
        src = os.path.join(APP_DIR, "src", "smartDeviceBle.native.ts")
        self.assertTrue(os.path.isfile(src), f"Source file missing: {src}")


if __name__ == "__main__":
    unittest.main()
