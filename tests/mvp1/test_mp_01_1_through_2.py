"""MP-01.1-2 — minipos dispatch: idempotency key + ErrDispatchOutcomeUnknown."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MINIPOS = ROOT / "minipos" / "beeminipos-backend"

REQUIRED_TESTS = (
    "TestDispatchSendsDeterministicIdempotencyKey",
    "TestDispatchNetworkErrorReturnsErrDispatchOutcomeUnknown",
)


class TestMP011DispatchIdempotencyAndUnknown(unittest.TestCase):
    def test_dispatch_idempotency_and_unknown(self):
        result = subprocess.run(
            [
                "go", "test",
                "./internal/api/...", "./internal/fiscalintegration/...",
                "-run", "TestDispatchSendsDeterministicIdempotencyKey|TestDispatchNetworkErrorReturnsErrDispatchOutcomeUnknown",
                "-count=1", "-v",
            ],
            cwd=str(MINIPOS),
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
        for name in REQUIRED_TESTS:
            self.assertIn(name, output, f"Required test not found: {name!r}")
        self.assertGreater(output.count("--- PASS"), 0, "No passing tests found")


if __name__ == "__main__":
    unittest.main()
