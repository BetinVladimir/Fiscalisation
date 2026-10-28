"""BUG-040.1-3 — executor.ready() checked before reserve_command(); REJECTED_BEFORE_IO on not-ready."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / "IoT/firmware/edge-agent-s3/idf/tests"


class TestBUG040(unittest.TestCase):
    def test_ready_check_before_reserve_command_in_source(self):
        """Verify executor_.ready() check appears before storage_.reserve_command() in execute()."""
        src = (ROOT / "IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp").read_text()
        self.assertIn("REJECTED_BEFORE_IO", src,
                      "REJECTED_BEFORE_IO state must be present in intent_processor.cpp")
        # Find the execute() function body (starts after its signature)
        exec_start = src.index("IntentProcessor::execute(")
        ready_pos = src.index("executor_.ready()", exec_start)
        reserve_pos = src.index("storage_.reserve_command(", exec_start)
        self.assertLess(ready_pos, reserve_pos,
                        "executor_.ready() check must precede storage_.reserve_command() in execute()")

    def test_rejected_before_io_no_storage_write(self):
        """Run native saga tests — device_unavailable_does_not_consume_unp behavior covered."""
        script = IOT / "run_native_saga.sh"
        if not script.exists():
            self.skipTest(f"Script not found: {script}")
        r = subprocess.run(
            ["bash", str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f"run_native_saga rc={r.returncode}\n{r.stdout}\n{r.stderr}",
        )
        self.assertIn("PASS", r.stdout + r.stderr)

    def test_storage_host_all_pass(self):
        """Storage host tests verify IntentProcessor behaviour end-to-end."""
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
        test_count = r.stdout.count(" PASS")
        self.assertGreater(test_count, 20, f"Expected >20 PASS lines, got {test_count}")


if __name__ == "__main__":
    unittest.main()
