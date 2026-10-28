"""TST-CT-03.1 — MQTT envelope vectors: contracts/mqtt-vectors/*.json parsed by three impls.

Acceptance: >=10 vectors exist; all required fields present; topics match
tenants/{t}/devices/{d}/commands; COMMITTED/NOT_COMMITTED/UNKNOWN all covered;
invalid signature and expired → REJECTED with error_code; UNKNOWN → no-retry note.
Python harness runs tests/e2e/contracts/test_tst_ct_03.py to validate vectors.
"""
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
VECTORS_DIR = ROOT / "contracts" / "mqtt-vectors"
E2E_TEST = ROOT / "tests" / "e2e" / "contracts" / "test_tst_ct_03.py"

MINIMUM_VECTORS = 10


class TestTSTCT03_1(unittest.TestCase):
    def test_vectors_dir_exists(self):
        self.assertTrue(VECTORS_DIR.exists(), f"contracts/mqtt-vectors/ not found: {VECTORS_DIR}")

    def test_minimum_vector_count(self):
        self.assertTrue(VECTORS_DIR.exists())
        count = len(list(VECTORS_DIR.glob("*.json")))
        self.assertGreaterEqual(
            count, MINIMUM_VECTORS,
            f"expected >= {MINIMUM_VECTORS} MQTT vectors, got {count}"
        )

    def test_e2e_contract_test_exists(self):
        self.assertTrue(
            E2E_TEST.exists(),
            f"tests/e2e/contracts/test_tst_ct_03.py not found: {E2E_TEST}"
        )

    def test_mqtt_vector_validation_suite_passes(self):
        """Run tests/e2e/contracts/test_tst_ct_03.py and verify all checks pass."""
        self.assertTrue(E2E_TEST.exists(), f"test file missing: {E2E_TEST}")
        result = subprocess.run(
            [sys.executable, "-m", "unittest", str(E2E_TEST), "-v"],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=30,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"MQTT vector tests failed (rc={result.returncode}):\n{output}"
        )
        passed = output.count("... ok")
        self.assertGreater(passed, 0, f"no tests passed:\n{output}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_ct_03_1.txt").write_text(output)

    def test_topic_format_consistent(self):
        """Canonical topic pattern used consistently across publisher and subscriber."""
        import json, re
        self.assertTrue(VECTORS_DIR.exists())
        pattern = re.compile(r'^tenants/[^/]+/devices/[^/]+/(commands|results|commands/acks)$')
        valid_count = 0
        for f in sorted(VECTORS_DIR.glob("*.json")):
            v = json.loads(f.read_text())
            if v.get("error_code") == "TOPIC_FORMAT_INVALID":
                continue
            self.assertRegex(
                v["topic"], pattern,
                f"{f.name}: topic '{v['topic']}' does not match canonical format"
            )
            valid_count += 1
        self.assertGreater(valid_count, 0, "no valid topic vectors found")


if __name__ == "__main__":
    unittest.main(verbosity=2)
