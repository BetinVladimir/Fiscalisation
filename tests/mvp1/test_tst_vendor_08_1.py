"""TST-VENDOR-08.1 — DatecsClock drift WARN/BLOCK + currency mode 2/3 readiness; Kotlin host test."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "SmartDevices/bluecash-app/tests/host/run-tstvendor08.py"

REQUIRED_MARKER = "actual Kotlin DatecsClock vendor tests: OK"


class TestTSTVendor08_1(unittest.TestCase):
    def test_datecs_clock_currency_vendor_host(self):
        self.assertTrue(SCRIPT.exists(), f"run-tstvendor08.py not found: {SCRIPT}")
        result = subprocess.run(
            ["python3", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"run-tstvendor08.py exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-08-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
