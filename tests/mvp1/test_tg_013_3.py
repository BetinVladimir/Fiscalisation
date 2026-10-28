"""TG-013.3 — docs/MVP1/CI_REQUIRED_CHECKS.md has contracts job listed as required."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG013CIChecks(unittest.TestCase):
    def test_ci_required_checks_lists_contracts_job(self):
        ci_md = ROOT / "docs" / "MVP1" / "CI_REQUIRED_CHECKS.md"
        self.assertTrue(ci_md.exists(), f"CI_REQUIRED_CHECKS.md not found at {ci_md}")
        content = ci_md.read_text()
        self.assertIn("contracts", content.lower(),
                      "CI_REQUIRED_CHECKS.md must list 'contracts' job")
        self.assertIn("contract-test", content,
                      "CI_REQUIRED_CHECKS.md must reference 'contract-test' command")


if __name__ == "__main__":
    unittest.main()
