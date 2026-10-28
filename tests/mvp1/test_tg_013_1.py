"""TG-013.1 — docs/MVP1/CONTRACT_LOCK_MATRIX.md lists components without contract-check."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG013ContractLockMatrix(unittest.TestCase):
    def test_contract_lock_matrix_exists_and_lists_gaps(self):
        matrix = ROOT / "docs" / "MVP1" / "CONTRACT_LOCK_MATRIX.md"
        self.assertTrue(matrix.exists(), f"CONTRACT_LOCK_MATRIX.md not found at {matrix}")
        content = matrix.read_text()

        # Must list the three components known to have no contract-check
        for component in ("S3", "BlueCash", "pos-gateway"):
            self.assertIn(
                component, content,
                f"CONTRACT_LOCK_MATRIX.md must list {component!r} as a gap",
            )

        # Must reference the fix tracking card
        self.assertIn("TG-013", content, "CONTRACT_LOCK_MATRIX.md must reference TG-013")

        # Must have a table structure
        self.assertIn("|", content, "CONTRACT_LOCK_MATRIX.md must contain a table")


if __name__ == "__main__":
    unittest.main()
