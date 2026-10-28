"""NAP-14.1 — Scope exclusions documented with reason+source; verify_supto_trace.rb passes."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCOPE_JSON = ROOT / "contracts" / "supto-feature-scope.json"
EXCLUSIONS_DOC = ROOT / "docs" / "SUPTO" / "SCOPE_EXCLUSIONS.md"
TRACE_SCRIPT = ROOT / "scripts" / "verify_supto_trace.rb"

REQUIRED_EXCLUSIONS = {
    "п.9.2",       # continuous supplies
    "18.6",        # inventory
    "18.7",        # supplies
    "18.8",        # inventory annex
    "31",          # fuel/vending
    "43",          # KLEN - manufacturer requirement
}


class TestScopeExclusions(unittest.TestCase):
    def test_scope_exclusions_doc_exists(self):
        self.assertTrue(
            EXCLUSIONS_DOC.exists(),
            f"docs/SUPTO/SCOPE_EXCLUSIONS.md not found: {EXCLUSIONS_DOC}",
        )

    def test_exclusions_doc_has_table(self):
        self.assertTrue(EXCLUSIONS_DOC.exists(), "SCOPE_EXCLUSIONS.md not found")
        text = EXCLUSIONS_DOC.read_text(encoding="utf-8")
        rows = [l for l in text.splitlines() if l.strip().startswith("|") and "---" not in l]
        # Need at least header + 6 exclusion rows
        self.assertGreater(len(rows), 6, "SCOPE_EXCLUSIONS.md table has too few rows")

    def test_exclusions_doc_contains_required_references(self):
        self.assertTrue(EXCLUSIONS_DOC.exists(), "SCOPE_EXCLUSIONS.md not found")
        text = EXCLUSIONS_DOC.read_text(encoding="utf-8")
        missing = [ref for ref in REQUIRED_EXCLUSIONS if ref not in text]
        self.assertFalse(
            missing,
            f"SCOPE_EXCLUSIONS.md is missing references to: {missing}",
        )

    def test_exclusions_doc_has_reason_and_source_columns(self):
        self.assertTrue(EXCLUSIONS_DOC.exists(), "SCOPE_EXCLUSIONS.md not found")
        text = EXCLUSIONS_DOC.read_text(encoding="utf-8").lower()
        self.assertIn("причина", text, "Table must have a 'Причина' (reason) column")
        self.assertIn("основание", text, "Table must have an 'Основание' (source/basis) column")

    def test_inventory_supplies_not_in_product_scope(self):
        self.assertTrue(SCOPE_JSON.exists(), "contracts/supto-feature-scope.json not found")
        data = json.loads(SCOPE_JSON.read_text(encoding="utf-8"))
        features = data.get("features", {})
        for name in ("inventory", "supplies"):
            self.assertIn(name, features, f"'{name}' must be in supto-feature-scope.json features")
            entry = features[name]
            self.assertFalse(entry.get("in_product_scope", True), f"{name}: in_product_scope must be false")
            self.assertEqual(entry.get("status"), "NOT_IN_PRODUCT_SCOPE", f"{name}: status must be NOT_IN_PRODUCT_SCOPE")

    def test_verify_supto_trace_passes(self):
        self.assertTrue(TRACE_SCRIPT.exists(), f"verify_supto_trace.rb not found: {TRACE_SCRIPT}")
        result = subprocess.run(
            ["ruby", str(TRACE_SCRIPT)],
            capture_output=True, text=True, timeout=30,
            cwd=str(ROOT),
        )
        self.assertEqual(
            result.returncode, 0,
            f"verify_supto_trace.rb failed:\nstdout: {result.stdout}\nstderr: {result.stderr}",
        )
        self.assertIn("OK", result.stdout)


if __name__ == "__main__":
    unittest.main()
