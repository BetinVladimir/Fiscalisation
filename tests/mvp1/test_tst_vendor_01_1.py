"""TST-VENDOR-01.1 — DatecsCodecVendorHost: full vendor TX/RX/corruption/limits test."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"
SCRIPT = SMARTDEVICES / "bluecash-app/tests/host/run-datecs-codec.py"

REQUIRED_MARKER = "actual Kotlin Datecs codec host tests: OK"


class TestTSTVendor01_1(unittest.TestCase):
    def test_datecs_codec_vendor_host_all_cases(self):
        self.assertTrue(SCRIPT.exists(), f"run-datecs-codec.py not found: {SCRIPT}")
        result = subprocess.run(
            ["python3", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"run-datecs-codec.py exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-01-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
