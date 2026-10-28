"""scripts/tests/test_hil_matrix.py — unit tests for check-hil-matrix.py script.

Tests the matrix validator logic directly including the selftest for POS BLE.
"""
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts" / "check-hil-matrix.py"
MATRIX_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "mvp_matrix.json"

EXPECTED_ROUTE_COUNT = 20
EXPECTED_HW = {"DatecsBC50", "BluePay", "DaisyCompact", "SmartS", "BlueCash"}


class TestHilMatrix(unittest.TestCase):
    def _run_script(self, matrix_path=None):
        env = os.environ.copy()
        if matrix_path is not None:
            # Patch by replacing the path inline - use a wrapper
            content = SCRIPT.read_text()
            patched = content.replace(
                'ROOT / "docs" / "MVP1" / "fixtures" / "mvp_matrix.json"',
                f'Path("{matrix_path}")',
            )
            with tempfile.NamedTemporaryFile(suffix=".py", mode="w", delete=False) as f:
                f.write(patched)
                tmp = f.name
            try:
                result = subprocess.run(
                    [sys.executable, tmp],
                    capture_output=True, text=True, timeout=15,
                )
            finally:
                os.unlink(tmp)
        else:
            result = subprocess.run(
                [sys.executable, str(SCRIPT)],
                capture_output=True, text=True, timeout=15,
            )
        return result

    def test_canonical_matrix_passes(self):
        """The canonical mvp_matrix.json must pass the validator."""
        self.assertTrue(MATRIX_FILE.exists(), f"mvp_matrix.json missing: {MATRIX_FILE}")
        result = self._run_script()
        self.assertEqual(
            result.returncode, 0,
            f"check-hil-matrix.py failed:\n{result.stdout}\n{result.stderr}"
        )
        self.assertIn("HIL-MATRIX OK", result.stdout)

    def test_20_routes(self):
        """Matrix must have exactly 20 routes."""
        data = json.loads(MATRIX_FILE.read_text())
        self.assertEqual(len(data["routes"]), EXPECTED_ROUTE_COUNT)

    def test_all_5_hw_paths(self):
        """All 5 hardware paths present."""
        data = json.loads(MATRIX_FILE.read_text())
        hw_found = {r["hw"] for r in data["routes"]}
        self.assertEqual(hw_found, EXPECTED_HW)

    def test_daisy_compact_no_card(self):
        """DaisyCompact routes must not have CARD/SPLIT capability."""
        data = json.loads(MATRIX_FILE.read_text())
        for r in data["routes"]:
            if r["hw"] == "DaisyCompact":
                caps = set(r.get("capabilities", []))
                self.assertNotIn("CARD", caps, f"DaisyCompact {r['route_id']} has CARD capability")
                self.assertNotIn("SPLIT", caps, f"DaisyCompact {r['route_id']} has SPLIT capability")

    def test_daisy_compact_negative_scenarios(self):
        """DaisyCompact routes must have CARD/SPLIT negative scenarios."""
        data = json.loads(MATRIX_FILE.read_text())
        for r in data["routes"]:
            if r["hw"] == "DaisyCompact":
                neg = [n["scenario"] for n in r.get("negative", [])]
                self.assertIn("CARD_before_io", neg, f"{r['route_id']} missing CARD_before_io negative")
                self.assertIn("SPLIT_before_io", neg, f"{r['route_id']} missing SPLIT_before_io negative")

    def test_smart_s_prod_unknown(self):
        """Smart S PROD: no doc number → UNKNOWN (not synthetic)."""
        data = json.loads(MATRIX_FILE.read_text())
        for r in data["routes"]:
            if r["hw"] == "SmartS":
                neg = {n["scenario"]: n["expect"] for n in r.get("negative", [])}
                self.assertEqual(
                    neg.get("prod_no_doc_number"), "UNKNOWN",
                    f"{r['route_id']}: prod_no_doc_number must expect UNKNOWN"
                )

    def test_smart_s_lab_committed_no_reference(self):
        """Smart S LAB: no doc number → COMMITTED_NO_REFERENCE."""
        data = json.loads(MATRIX_FILE.read_text())
        for r in data["routes"]:
            if r["hw"] == "SmartS":
                neg = {n["scenario"]: n["expect"] for n in r.get("negative", [])}
                self.assertEqual(
                    neg.get("lab_no_doc_number"), "COMMITTED_NO_REFERENCE",
                    f"{r['route_id']}: lab_no_doc_number must expect COMMITTED_NO_REFERENCE"
                )

    def test_selftest_pos_ble_row_required(self):
        """Selftest: removing POS BLE row causes validator failure."""
        data = json.loads(MATRIX_FILE.read_text())
        # Remove POS/BLE routes
        modified = dict(data)
        modified["routes"] = [r for r in data["routes"]
                               if not (r["client"] == "POS" and r["transport"] == "BLE")]
        self.assertLess(len(modified["routes"]), len(data["routes"]))

        with tempfile.NamedTemporaryFile(suffix=".json", mode="w", delete=False) as f:
            json.dump(modified, f)
            tmp_path = f.name
        try:
            result = self._run_script(matrix_path=tmp_path)
            self.assertNotEqual(result.returncode, 0,
                                f"selftest: removing POS BLE rows should FAIL\n{result.stdout}")
        finally:
            os.unlink(tmp_path)

    def test_supto_in_all_routes(self):
        """SUPTO capability or flag in every route."""
        data = json.loads(MATRIX_FILE.read_text())
        no_supto = [r["route_id"] for r in data["routes"] if not r.get("supto")]
        self.assertEqual(no_supto, [], f"routes missing supto: {no_supto}")


if __name__ == "__main__":
    unittest.main(verbosity=2)
