"""MATRIX-01.1 — Canonical mvp_matrix.json: 20 routes validated by check-hil-matrix.py.

Acceptance: mvp_matrix.json exists with 20 routes (5 hw x POS/MiniPOS x MQTT/BLE);
S1..S8 scenarios per capabilities; Daisy Compact CARD/split negative explicit;
Smart S LAB/PROD negative explicit; check-hil-matrix.py exits 0; selftest POS BLE.
"""
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MATRIX_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "mvp_matrix.json"
SCRIPT = ROOT / "scripts" / "check-hil-matrix.py"
SCRIPT_TESTS = ROOT / "scripts" / "tests" / "test_hil_matrix.py"

EXPECTED_ROUTE_COUNT = 20


class TestMATRIX01_1(unittest.TestCase):
    def test_matrix_file_exists(self):
        self.assertTrue(MATRIX_FILE.exists(), f"mvp_matrix.json not found: {MATRIX_FILE}")

    def test_matrix_has_20_routes(self):
        self.assertTrue(MATRIX_FILE.exists())
        data = json.loads(MATRIX_FILE.read_text())
        routes = data.get("routes", [])
        self.assertEqual(
            len(routes), EXPECTED_ROUTE_COUNT,
            f"expected {EXPECTED_ROUTE_COUNT} routes, got {len(routes)}"
        )

    def test_check_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"check-hil-matrix.py not found: {SCRIPT}")

    def test_check_script_passes(self):
        """scripts/check-hil-matrix.py must exit 0 on canonical matrix."""
        self.assertTrue(SCRIPT.exists())
        result = subprocess.run(
            [sys.executable, str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=15,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"check-hil-matrix.py failed (rc={result.returncode}):\n{output}"
        )
        self.assertIn("HIL-MATRIX OK", result.stdout)

    def test_script_unit_tests_pass(self):
        """scripts/tests/test_hil_matrix.py passes all matrix unit tests."""
        self.assertTrue(SCRIPT_TESTS.exists(), f"test_hil_matrix.py not found: {SCRIPT_TESTS}")
        result = subprocess.run(
            [sys.executable, "-m", "unittest", str(SCRIPT_TESTS), "-v"],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=30,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"test_hil_matrix.py failed (rc={result.returncode}):\n{output}"
        )
        # Verify non-zero test count
        self.assertIn("OK", output, f"no tests passed:\n{output}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "matrix_01_1.txt").write_text(output)

    def test_selftest_pos_ble_deletion_fails(self):
        """Deleting POS BLE row → validator fails (selftest requirement)."""
        self.assertTrue(MATRIX_FILE.exists())
        data = json.loads(MATRIX_FILE.read_text())
        modified = dict(data)
        modified["routes"] = [r for r in data["routes"]
                               if not (r["client"] == "POS" and r["transport"] == "BLE")]
        self.assertLess(len(modified["routes"]), EXPECTED_ROUTE_COUNT)

        with tempfile.NamedTemporaryFile(suffix=".json", mode="w", delete=False) as f:
            json.dump(modified, f)
            tmp_path = f.name
        try:
            # Patch script to use temp file
            content = SCRIPT.read_text()
            patched = content.replace(
                'ROOT / "docs" / "MVP1" / "fixtures" / "mvp_matrix.json"',
                f'Path("{tmp_path}")',
            )
            with tempfile.NamedTemporaryFile(suffix=".py", mode="w", delete=False) as sf:
                sf.write(patched)
                tmp_script = sf.name
            try:
                result = subprocess.run(
                    [sys.executable, tmp_script],
                    capture_output=True, text=True, timeout=15,
                )
                self.assertNotEqual(result.returncode, 0,
                                    "removing POS/BLE routes should cause validation FAIL")
            finally:
                os.unlink(tmp_script)
        finally:
            os.unlink(tmp_path)

    def test_no_hil_pass_markers(self):
        """No EMU/HIL physical PASS markers in routes (invariant: no fake PASS)."""
        self.assertTrue(MATRIX_FILE.exists())
        data = json.loads(MATRIX_FILE.read_text())
        for r in data.get("routes", []):
            self.assertFalse(r.get("hil_pass"), f"route {r['route_id']} has forbidden hil_pass")
            self.assertFalse(r.get("emu_pass"), f"route {r['route_id']} has forbidden emu_pass")

    def test_smart_s_lab_only(self):
        """Smart S routes are lab_only=true."""
        data = json.loads(MATRIX_FILE.read_text())
        for r in data.get("routes", []):
            if r["hw"] == "SmartS":
                self.assertTrue(r.get("lab_only"), f"SmartS {r['route_id']} must be lab_only")


if __name__ == "__main__":
    unittest.main(verbosity=2)
