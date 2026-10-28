"""TG-014.1 — docs/MVP1/HIL_RUNBOOK.md exists with connection checklist and test data."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG014HILRunbook(unittest.TestCase):
    def test_hil_runbook_exists_with_required_sections(self):
        runbook = ROOT / "docs" / "MVP1" / "HIL_RUNBOOK.md"
        self.assertTrue(runbook.exists(), f"HIL_RUNBOOK.md not found at {runbook}")
        content = runbook.read_text()

        for required in (
            "HIL-BC-01",       # scenario name
            "1367",            # golden fixture amount
            "dry-run",         # dry-run usage
            "scripts/hil/run.sh",  # script reference
            "artifacts/hil",   # artifact path
        ):
            self.assertIn(required, content,
                          f"HIL_RUNBOOK.md missing required content: {required!r}")

    def test_hil_result_schema_exists(self):
        schema = ROOT / "contracts" / "hil-result.schema.json"
        self.assertTrue(schema.exists(), f"hil-result.schema.json not found at {schema}")
        import json
        data = json.loads(schema.read_text())
        self.assertIn("status", data.get("properties", {}),
                      "hil-result.schema.json must have 'status' property")
        self.assertIn("PASS", data["properties"]["status"].get("enum", []),
                      "status must allow 'PASS'")
        self.assertIn("FAIL", data["properties"]["status"].get("enum", []),
                      "status must allow 'FAIL'")

    def test_mvp_gates_exists(self):
        gates = ROOT / "MVP_GATES.md"
        self.assertTrue(gates.exists(), f"MVP_GATES.md not found at {gates}")
        content = gates.read_text()
        self.assertIn("HIL-BC-01", content, "MVP_GATES.md must list HIL-BC-01")


if __name__ == "__main__":
    unittest.main()
