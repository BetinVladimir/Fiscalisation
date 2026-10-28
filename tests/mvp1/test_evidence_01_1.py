"""EVIDENCE-01.1 — factory manifest image-type and hash; evidence key integrity."""
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SELF_TEST = ROOT / "scripts" / "tests" / "test_evidence_trust.py"
DEVICE_MFG = ROOT / "IoT" / "device-manufacturing"


class TestEvidence011(unittest.TestCase):
    def test_evidence_trust_suite(self):
        self.assertTrue(SELF_TEST.exists(), f"test not found: {SELF_TEST}")
        result = subprocess.run(
            [sys.executable, str(SELF_TEST), "-v"],
            cwd=str(DEVICE_MFG),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"test_evidence_trust.py failed:\n{output[-3000:]}",
        )
        required = (
            "test_valid_fixture_passes",
            "test_replaced_key_fails",
            "test_broken_hash_fails",
            "test_wrong_image_type_fails",
        )
        for marker in required:
            self.assertIn(marker, output, f"Required test not found in output: {marker!r}")
        self.assertIn("OK", output, "Test suite did not report OK")


if __name__ == "__main__":
    unittest.main()
