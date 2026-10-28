"""NAP-11.1 — EUR/BGN transition boundary: periodized export splits correctly."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"


class TestNap111EuroBGNTransition(unittest.TestCase):
    def test_periodized_export_euro_bgn_boundary(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain",
             "-run", r"Export.*Euro|Periodized",
             "-count=1", "-v", "-json"],
            capture_output=True, text=True, timeout=120, cwd=str(BACKEND),
        )
        self.assertEqual(
            result.returncode, 0,
            f"go test failed (rc={result.returncode}):\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-1000:]}",
        )
        events = []
        for line in result.stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        passed = [
            e.get("Test", "") for e in events
            if e.get("Action") == "pass" and e.get("Test")
        ]
        self.assertGreater(len(passed), 0, f"No tests passed. stdout:\n{result.stdout[-2000:]}")
        # Verify at least the BGN/EUR split test ran
        split_tests = [t for t in passed if "Periodized" in t or "Euro" in t]
        self.assertGreater(
            len(split_tests), 0,
            f"No Periodized/Euro tests found in passed list: {passed}",
        )

    def test_split_test_covers_bgn_eur_boundary(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain",
             "-run", "TestPeriodizedExportSplitsBGNAndEURWithoutBoundaryOverlap",
             "-count=1", "-v"],
            capture_output=True, text=True, timeout=60, cwd=str(BACKEND),
        )
        self.assertEqual(
            result.returncode, 0,
            f"BGN/EUR split test failed:\n{result.stdout}\n{result.stderr}",
        )
        self.assertIn("PASS", result.stdout)

    def test_currency_relabeling_rejected(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain",
             "-run", "TestPeriodizedExportRejectsCurrencyRelabeling",
             "-count=1", "-v"],
            capture_output=True, text=True, timeout=60, cwd=str(BACKEND),
        )
        self.assertEqual(
            result.returncode, 0,
            f"Currency relabeling test failed:\n{result.stdout}\n{result.stderr}",
        )
        self.assertIn("PASS", result.stdout)


if __name__ == "__main__":
    unittest.main()
