"""TST-VENDOR-07.1 — DatecsRecovery.resolve() COMMITTED/NOT_COMMITTED/UNKNOWN; Kotlin host test."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "SmartDevices/bluecash-app/tests/host/run-tstvendor07.py"

REQUIRED_MARKER = "actual Kotlin DatecsRecovery vendor tests: OK"


class TestTSTVendor07_1(unittest.TestCase):
    def test_datecs_recovery_resolve_vendor_host(self):
        self.assertTrue(SCRIPT.exists(), f"run-tstvendor07.py not found: {SCRIPT}")
        result = subprocess.run(
            ["python3", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"run-tstvendor07.py exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-07-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
