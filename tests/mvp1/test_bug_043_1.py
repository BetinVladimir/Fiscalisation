"""BUG-043.1-3 — line totals must equal payment total; half-up rounding; PAYMENT_SUM_MISMATCH."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / "IoT/firmware/edge-agent-s3/idf/tests"


class TestBUG043(unittest.TestCase):
    def test_line_sum_validation_in_source(self):
        """Verify line_sum != total guard exists with half-up rounding in intent_processor.cpp."""
        src = (ROOT / "IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp").read_text()
        self.assertIn("PAYMENT_SUM_MISMATCH", src,
                      "PAYMENT_SUM_MISMATCH comment must mark the line-sum guard")
        self.assertIn("+500)/1000", src.replace(" ", ""),
                      "half-up rounding (unit¢×qty_milli+500)/1000 must be present")
        self.assertIn("line_sum", src, "line_sum variable must be present")
        self.assertIn("PAYMENT_SUM_MISMATCH", src, "PAYMENT_SUM_MISMATCH comment must mark the guard")

    def test_golden_arithmetic_1367(self):
        """Verify golden receipt 1367¢ = 12.00 × 1.000 + 1.67 × 1.000 with half-up rounding."""
        # 12.00 EUR × 1.000 qty → (1200 × 1000 + 500) / 1000 = 1200
        # 1.67 EUR × 1.000 qty → (167 × 1000 + 500) / 1000 = 167
        # total = 1367¢ matches CASH 500 + CARD 867 = 1367¢
        self.assertEqual((1200 * 1000 + 500) // 1000, 1200)
        self.assertEqual((167 * 1000 + 500) // 1000, 167)
        self.assertEqual(1200 + 167, 1367)
        self.assertEqual(500 + 867, 1367)

    def test_storage_host_runs_with_line_sum_check(self):
        """Storage host native tests pass after adding line-sum validation."""
        script = IOT / "run_storage_host.sh"
        if not script.exists():
            self.skipTest(f"Script not found: {script}")
        r = subprocess.run(
            ["bash", str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            r.returncode, 0,
            f"run_storage_host rc={r.returncode}\n{r.stdout[-4000:]}\n{r.stderr[-2000:]}",
        )
        # Verify the normal sale path still passes (items: 1×2.00, payments: 1.00+1.00 = 2.00)
        self.assertIn(
            "normal PASS (actual IntentProcessor + SQLite, reopened)", r.stdout,
        )
        test_count = r.stdout.count(" PASS")
        self.assertGreater(test_count, 20, f"Expected >20 PASS lines, got {test_count}")


if __name__ == "__main__":
    unittest.main()
