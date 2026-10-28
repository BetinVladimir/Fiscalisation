"""NAP-07.3 — check-data-location.sh: EU regions → exit 0; us-east-1 → exit 1."""
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts" / "check-data-location.sh"


def _run_script(data_location_content: str) -> subprocess.CompletedProcess:
    with tempfile.TemporaryDirectory(prefix="nap07-3-") as td:
        tdp = Path(td)
        doc = tdp / "DATA_LOCATION.md"
        doc.write_text(data_location_content, encoding="utf-8")
        env = os.environ.copy()
        env["DATA_LOCATION_FILE"] = str(doc)
        env["SKIP_HOSTNAME_CHECK"] = "1"
        return subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True, text=True, timeout=30, env=env,
        )


_EU_DOC = textwrap.dedent("""\
    # Data Location Register
    | Data Store | Technology | Region | Purpose |
    |---|---|---|---|
    | Primary DB | PostgreSQL | EU (Frankfurt, DE) | Fiscal records |
    | Replica DB | PostgreSQL | EU (Frankfurt, DE) | Read replica |
    | Message Broker | RabbitMQ | EU (Frankfurt, DE) | Event queue |
    | Object Storage | S3-compatible | EU (Frankfurt, DE) | Receipts |
    | Backups | pg_dump | EU (Frankfurt, DE) | DR backups |
    | App Logs | Loki | EU (Frankfurt, DE) | Observability |
    | IoT Store | SQLite | BG (on-device) | Edge durable |
    | EMQX Broker | EMQX | EU (Frankfurt, DE) | MQTT IoT |
    | Audit Logs | PostgreSQL | BG (on-premise) | Compliance |
""")

_US_DOC = textwrap.dedent("""\
    # Data Location Register
    | Data Store | Technology | Region | Purpose |
    |---|---|---|---|
    | Primary DB | PostgreSQL | us-east-1 | Fiscal records |
""")


class TestCheckDataLocationScript(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"Script not found: {SCRIPT}")

    def test_eu_regions_exit_zero(self):
        result = _run_script(_EU_DOC)
        self.assertEqual(
            result.returncode, 0,
            f"Expected exit 0 for EU regions:\nstdout: {result.stdout}\nstderr: {result.stderr}",
        )
        self.assertIn("OK", result.stdout)

    def test_us_east_1_exits_one(self):
        result = _run_script(_US_DOC)
        self.assertNotEqual(
            result.returncode, 0,
            "Expected exit 1 for us-east-1 region, got exit 0",
        )
        combined = result.stdout + result.stderr
        self.assertTrue(
            "us-east-1" in combined or "FAIL" in combined or "region" in combined.lower(),
            f"Expected error mentioning us-east-1 or FAIL:\nstdout: {result.stdout}\nstderr: {result.stderr}",
        )


if __name__ == "__main__":
    unittest.main()
