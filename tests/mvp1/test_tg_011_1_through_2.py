"""TG-011.1-2 — /metrics endpoint + PII log masking in fiscal-backend/internal/api."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"

REQUIRED_TESTS = (
    "TestMetricsEndpointExposesOperationCounters",
    "TestLogMaskingHidesEmailAndEGN",
)


class TestTG011MetricsAndLogMasking(unittest.TestCase):
    def test_metrics_and_pii_masking(self):
        result = subprocess.run(
            ["go", "test", "./internal/api/...",
             "-run", "TestMetricsEndpointExposesOperationCounters|TestLogMaskingHidesEmailAndEGN",
             "-count=1", "-v"],
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
        for name in REQUIRED_TESTS:
            self.assertIn(name, output, f"Required test not found: {name!r}")
        self.assertGreater(output.count("--- PASS"), 0, "No passing tests found")

    def test_alerts_runbook_exists(self):
        alerts_md = ROOT / "docs" / "runbooks" / "ALERTS.md"
        self.assertTrue(alerts_md.exists(), f"ALERTS.md not found at {alerts_md}")
        content = alerts_md.read_text()
        for required in ("fiscal_unknown_total", "fiscal_operations_total", "mqtt_command_latency_seconds"):
            self.assertIn(required, content, f"ALERTS.md missing metric {required!r}")


if __name__ == "__main__":
    unittest.main()
