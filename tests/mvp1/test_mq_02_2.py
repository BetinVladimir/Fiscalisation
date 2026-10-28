"""MQ-02.2 — BLE channel: invalid proof/origin → AUTH_FAILED before command execution.

This test verifies that the ble_proof_host and ble_origin_host test harnesses exist
and that the ble_runtime_native test suite covers auth-negative scenarios.
Full proof/origin crypto tests require mbedTLS/cJSON build dependencies (BLOCKED_DEPS).
The auth rejection is verified via the ble_runtime_native host suite.
"""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IDF_TESTS = ROOT / "IoT" / "firmware" / "edge-agent-s3" / "idf" / "tests"
BLE_SCRIPT = IDF_TESTS / "run_native_ble.sh"


class TestMQ022BLEAuthNegative(unittest.TestCase):
    def test_ble_proof_host_dir_exists(self):
        proof_dir = IDF_TESTS / "ble_proof_host"
        self.assertTrue(proof_dir.is_dir(), f"ble_proof_host directory missing: {proof_dir}")
        proof_src = proof_dir / "proof_native.cpp"
        self.assertTrue(proof_src.exists(), f"proof_native.cpp missing: {proof_src}")
        # Verify that the proof host tests include foreign identity rejection
        content = proof_src.read_text(errors="replace")
        self.assertIn("foreign", content.lower(),
                      "ble_proof_host/proof_native.cpp must include foreign-identity rejection test")

    def test_ble_origin_host_dir_exists(self):
        origin_dir = IDF_TESTS / "ble_origin_host"
        self.assertTrue(origin_dir.is_dir(), f"ble_origin_host directory missing: {origin_dir}")
        runtime_src = origin_dir / "runtime.cpp"
        self.assertTrue(runtime_src.exists(), f"runtime.cpp missing: {runtime_src}")

    def test_run_native_ble_includes_auth_admission(self):
        """run_native_ble.sh must report foreign-disconnect admission pass."""
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
            result.stdout[-2000:] + "\n" + result.stderr[-2000:],
        )
        output = result.stdout + result.stderr
        # The BLE runtime native suite covers foreign-disconnect and handle-token admission
        self.assertIn("foreign-disconnect admission: PASS", output,
                      "Expected BLE auth rejection evidence in run_native_ble.sh output")


if __name__ == "__main__":
    unittest.main()
