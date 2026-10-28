"""TG-008.1-2 — electronic receipts permanently disabled, returns ERECEIPT_UNSUPPORTED."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"

REQUIRED_TESTS = ("TestEReceiptUnsupported",)


class TestTG008EReceiptDisabled(unittest.TestCase):
    def test_ereceipt_rejected_with_unsupported(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "EReceipt", "-count=1", "-v"],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            "go test rc=" + str(result.returncode) + "\n" +
            result.stdout[-4000:] + "\n" + result.stderr[-2000:],
        )
        self.assertGreater(result.stdout.count("--- PASS"), 0, "No passing tests found")
        for name in REQUIRED_TESTS:
            self.assertIn(name, result.stdout, f"Required test not found: {name!r}")


if __name__ == "__main__":
    unittest.main()
