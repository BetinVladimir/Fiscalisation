"""MQ-02.1 — BLE fragmentation/assembly and UNKNOWN on lost fragment (run_native_ble.sh)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BLE_SCRIPT = ROOT / "IoT" / "firmware" / "edge-agent-s3" / "idf" / "tests" / "run_native_ble.sh"


class TestMQ021BLEFragmentation(unittest.TestCase):
    def test_run_native_ble_passes(self):
        self.assertTrue(BLE_SCRIPT.exists(), f"run_native_ble.sh not found at {BLE_SCRIPT}")
        result = subprocess.run(
            ["bash", str(BLE_SCRIPT)],
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            "run_native_ble.sh rc=" + str(result.returncode) + "\n" +
            result.stdout[-4000:] + "\n" + result.stderr[-2000:],
        )
        output = result.stdout + result.stderr
        self.assertIn("actual BLE runtime host tests: OK", output,
                      "Expected 'actual BLE runtime host tests: OK' in output")


if __name__ == "__main__":
    unittest.main()
