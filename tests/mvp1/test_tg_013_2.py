"""TG-013.2 — contracts/*.lock files exist; make contract-test target exists."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG013ContractLocks(unittest.TestCase):
    def test_at_least_one_lock_file_exists(self):
        lock_files = list((ROOT / "contracts").glob("*.lock"))
        self.assertGreater(
            len(lock_files), 0,
            "No *.lock files found in contracts/",
        )
        # Each lock file must contain 'sha256' field
        for lock_file in lock_files:
            content = lock_file.read_text()
            self.assertIn(
                "sha256", content,
                f"{lock_file.name} must contain a sha256 field",
            )

    def test_makefile_has_contract_test_target(self):
        makefile = ROOT / "Makefile"
        self.assertTrue(makefile.exists(), "Makefile not found")
        content = makefile.read_text()
        self.assertIn(
            "contract-test:", content,
            "Makefile must have contract-test target",
        )


if __name__ == "__main__":
    unittest.main()
