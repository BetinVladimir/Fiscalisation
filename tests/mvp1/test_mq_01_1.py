"""MQ-01.1 — MQTT topic namespace reverify GAP-001; TRACE_EXISTING_REGISTERS.md has entry."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"
TRACE_FILE = ROOT / "docs" / "MVP1" / "TRACE_EXISTING_REGISTERS.md"


class TestMQ011TopicGap001(unittest.TestCase):
    def test_trace_file_has_gap001_reverify(self):
        self.assertTrue(TRACE_FILE.exists(), f"TRACE_EXISTING_REGISTERS.md not found at {TRACE_FILE}")
        content = TRACE_FILE.read_text()
        self.assertIn("GAP-001", content, "TRACE_EXISTING_REGISTERS.md must contain GAP-001 entry")
        # The reverify column must mention the outcome (PARTIAL / FIXED / OPEN)
        for keyword in ("PARTIAL", "FIXED", "OPEN"):
            if keyword in content:
                break
        else:
            self.fail("TRACE_EXISTING_REGISTERS.md GAP-001 row must contain a reverify status")

    def test_mqttclient_topic_patterns_present(self):
        result = subprocess.run(
            ["go", "test", "./internal/mqttclient/...", "-count=1", "-v"],
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
        self.assertGreater((result.stdout + result.stderr).count("--- PASS"), 0, "No passing tests found")


if __name__ == "__main__":
    unittest.main()
