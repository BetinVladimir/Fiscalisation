"""BUG-044.1 — PayloadConflict delivers result via result_sink_ before returning."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
IOT = ROOT / "IoT/firmware/edge-agent-s3/idf/tests"


class TestBUG044(unittest.TestCase):
    def test_payload_conflict_calls_result_sink_in_source(self):
        """Verify result_sink_ is called before return on PayloadConflict."""
        src = (ROOT / "IoT/firmware/edge-agent-s3/idf/main/intent_processor.cpp").read_text()
        conflict_pos = src.index("ReservationStatus::PayloadConflict")
        sink_pos = src.index("PAYLOAD_CONFLICT")
        # Find ESP_ERR_INVALID_CRC that comes AFTER the PayloadConflict check
        crc_pos = src.index("ESP_ERR_INVALID_CRC", conflict_pos)
        self.assertLess(conflict_pos, sink_pos,
                        "PAYLOAD_CONFLICT result_sink_ call must follow PayloadConflict check")
        self.assertLess(sink_pos, crc_pos,
                        "result_sink_ PAYLOAD_CONFLICT must precede ESP_ERR_INVALID_CRC return")

    def test_storage_host_runs_with_payload_conflict_path(self):
        """Verify storage host tests still pass (includes PayloadConflict path via collision)."""
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
