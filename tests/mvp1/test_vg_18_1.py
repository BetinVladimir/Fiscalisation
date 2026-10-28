"""VG-18.1 — Tremol outside MVP: capability gate rejects before I/O."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCOPE_FILE = ROOT / "contracts" / "supto-feature-scope.json"


class TestVG181TremolOutOfScope(unittest.TestCase):
    def setUp(self):
        self.assertTrue(SCOPE_FILE.exists(), f"supto-feature-scope.json not found at {SCOPE_FILE}")
        with open(SCOPE_FILE) as f:
            self.scope = json.load(f)

    def test_tremol_feature_exists(self):
        features = self.scope.get("features", {})
        self.assertIn(
            "fiscal_unit_tremol",
            features,
            "supto-feature-scope.json must contain 'fiscal_unit_tremol' feature entry (VG-18)"
        )

    def test_tremol_not_in_mvp_scope(self):
        feature = self.scope["features"]["fiscal_unit_tremol"]
        self.assertFalse(
            feature.get("in_product_scope", True),
            "fiscal_unit_tremol.in_product_scope must be false"
        )
        self.assertFalse(
            feature.get("in_mvp_scope", True),
            "fiscal_unit_tremol.in_mvp_scope must be false"
        )

    def test_tremol_status_not_in_mvp(self):
        feature = self.scope["features"]["fiscal_unit_tremol"]
        status = feature.get("status", "")
        self.assertEqual(
            status,
            "NOT_IN_MVP_SCOPE",
            f"fiscal_unit_tremol.status must be NOT_IN_MVP_SCOPE, got {status!r}"
        )

    def test_tremol_capability_gate_rejected_before_io(self):
        feature = self.scope["features"]["fiscal_unit_tremol"]
        gate = feature.get("capability_gate", "")
        self.assertEqual(
            gate,
            "REJECTED_BEFORE_IO",
            f"fiscal_unit_tremol.capability_gate must be REJECTED_BEFORE_IO, got {gate!r}"
        )

    def test_tremol_note_references_vg18(self):
        feature = self.scope["features"]["fiscal_unit_tremol"]
        note = feature.get("note", "")
        self.assertIn(
            "VG-18",
            note,
            "fiscal_unit_tremol.note must reference VG-18"
        )

    def test_all_required_fields_present(self):
        feature = self.scope["features"]["fiscal_unit_tremol"]
        required = {"in_product_scope", "in_mvp_scope", "status", "capability_gate", "note"}
        missing = required - set(feature.keys())
        self.assertSetEqual(missing, set(), f"Missing fields in fiscal_unit_tremol: {missing}")


if __name__ == "__main__":
    unittest.main()
