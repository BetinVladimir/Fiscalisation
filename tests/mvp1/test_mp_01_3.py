"""MP-01.3 — minipos dispatch: writeExecutionUpstreamError + 202 UNKNOWN."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MINIPOS = ROOT / "minipos" / "beeminipos-backend"

REQUIRED_TESTS = (
    "TestDispatchTimeoutReturnsUnknownNot409",
    "TestDispatch5xxMapsTo503",
    "TestDispatch403MapsTo403",
)


class TestMP013DispatchUnknownHandler(unittest.TestCase):
    def test_dispatch_unknown_handler(self):
        result = subprocess.run(
            [
                "go", "test",
                "./internal/api/...",
                "-run", "TestDispatchTimeoutReturnsUnknownNot409|TestDispatch5xxMapsTo503|TestDispatch403MapsTo403",
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
