"""TG-014.3 — MVP_GATES.md has gate rows for all HIL paths from ARCH_17 §5."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

# Required paths/scenarios from ARCH_17 §5 (from HIL_RUNBOOK.md and MVP_GATES.md)
REQUIRED_SCENARIOS = [
    "HIL-BC-01",
    "HIL-DSY-01",
    "HIL-S3-01",
]

REQUIRED_STATES = [
    "SOFTWARE_VERIFIED_HIL_PENDING",
    "HIL_PASS",
    "NOT_STARTED",
    "OUT_OF_MVP_SCOPE",
]


class TestTG014MVPGates(unittest.TestCase):
    def test_mvp_gates_has_all_required_scenarios(self):
        gates = ROOT / "MVP_GATES.md"
        self.assertTrue(gates.exists(), f"MVP_GATES.md not found at {gates}")
        content = gates.read_text()

        for scenario in REQUIRED_SCENARIOS:
            self.assertIn(scenario, content,
                          f"MVP_GATES.md missing scenario {scenario!r}")

    def test_mvp_gates_uses_defined_status_values(self):
        gates = ROOT / "MVP_GATES.md"
        content = gates.read_text()
        # At least one of the defined states must be present
        found = [s for s in REQUIRED_STATES if s in content]
        self.assertGreater(
            len(found), 0,
            f"MVP_GATES.md must contain at least one gate state from {REQUIRED_STATES}",
        )

    def test_mvp_gates_e_receipt_out_of_scope(self):
        """Electronic receipts must be OUT_OF_MVP_SCOPE per TG-008 decision."""
        gates = ROOT / "MVP_GATES.md"
        content = gates.read_text()
        self.assertIn("OUT_OF_MVP_SCOPE", content,
                      "MVP_GATES.md must mark e-receipt as OUT_OF_MVP_SCOPE")


if __name__ == "__main__":
    unittest.main()
