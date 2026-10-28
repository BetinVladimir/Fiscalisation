"""BUG-041.1-3 — execute() rejects when device clock returns 0 (untrusted, pre-2024)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / "IoT/firmware/edge-agent-s3/idf/tests"


class TestBUG041(unittest.TestCase):
    def test_clock_zero_check_in_source(self):
        """Verify at==0 guard is present in execute() after now()."""
        src = (ROOT / "IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp").read_text()
        self.assertIn("if (at == 0) return ESP_ERR_INVALID_STATE;", src,
                      "at==0 guard must be present in intent_processor.cpp")
        # Guard must follow the now() call in execute()
        at_pos = src.index("const int64_t at = now();")
        guard_pos = src.index("if (at == 0) return ESP_ERR_INVALID_STATE;")
        self.assertLess(at_pos, guard_pos, "at==0 guard must follow now() call in execute()")

    def test_storage_host_clock_zero_mode(self):
        """Run storage host native binary with validity-clock-zero mode."""
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
        self.assertIn(
            "validity-clock-zero actual HMAC+IntentProcessor+SQLite PASS",
            r.stdout,
            "validity-clock-zero test must be present and PASS",
        )
        self.assertIn(
            "validity-clock-failed actual HMAC+IntentProcessor+SQLite PASS",
            r.stdout,
        )


if __name__ == "__main__":
    unittest.main()
