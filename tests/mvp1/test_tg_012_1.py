"""TG-012.1 — secrets/config: empty HMAC key in prod config fails startup."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"


class TestTG012ConfigSecrets(unittest.TestCase):
    def test_config_tests_pass(self):
        result = subprocess.run(
            ["go", "test", "./internal/config/...", "-count=1", "-v"],
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
        output = result.stdout + result.stderr
        self.assertGreater(output.count("--- PASS"), 0, "No passing tests found")


if __name__ == "__main__":
    unittest.main()
