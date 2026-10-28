"""
BF-01.3 acceptance test: Run smart-device-ble-native.test.ts via Jest.
GREEN phase: all 7 tests must pass (authenticate/setWifi/bindStore implemented).
"""
import os
import re
import subprocess
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
APP_DIR = os.path.join(REPO_ROOT, "BeeFiscalApp")
TEST_FILE = os.path.join(APP_DIR, "__tests__", "smart-device-ble-native.test.ts")
JEST_CONFIG = os.path.join(APP_DIR, "jest.ui.config.cjs")
SRC_FILE = os.path.join(APP_DIR, "src", "smartDeviceBle.native.ts")


class TestBF013BleNativeGreen(unittest.TestCase):

    def test_test_file_exists(self):
        self.assertTrue(os.path.isfile(TEST_FILE), f"Test file missing: {TEST_FILE}")

    def test_src_implements_authenticate(self):
        """smartDeviceBle.native.ts must implement authenticate without always throwing."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertNotIn(
            "EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE",
            src,
            "Source must not throw EDGE_BLE_CRYPTO_PROVIDER_NOT_AVAILABLE — stub must be replaced",
        )

    def test_src_implements_setwifi(self):
        """setWifi must have real implementation (not just empty body)."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertIn(
            "EDGE_V2_SESSION_TX",
            src,
            "setWifi must write to EDGE_V2_SESSION_TX",
        )

    def test_src_has_connect_timeout(self):
        """connectEdgeDeviceV2 must implement a connect timeout."""
        with open(SRC_FILE, encoding="utf-8") as f:
            src = f.read()
        self.assertIn(
            "EDGE_CONNECT_TIMEOUT",
            src,
            "Source must handle EDGE_CONNECT_TIMEOUT",
        )

    def test_jest_all_pass(self):
        """All Jest tests in smart-device-ble-native.test.ts must pass."""
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

        # Extract test counts from the "Tests:" summary line
        match_failed = re.search(r"Tests:.*?(\d+)\s+failed", combined)
        match_passed = re.search(r"Tests:.*?(\d+)\s+passed", combined)

        self.assertIsNotNone(
            match_passed,
            f"Jest produced no passed tests. Output:\n{combined[:3000]}",
        )

        failed = int(match_failed.group(1)) if match_failed else 0
        passed = int(match_passed.group(1)) if match_passed else 0

        self.assertEqual(
            failed,
            0,
            f"Jest reported {failed} failed test(s). Output:\n{combined[:3000]}",
        )
        self.assertGreaterEqual(
            passed,
            7,
            f"Expected ≥ 7 passing tests, got {passed}. Output:\n{combined[:3000]}",
        )


if __name__ == "__main__":
    unittest.main()
