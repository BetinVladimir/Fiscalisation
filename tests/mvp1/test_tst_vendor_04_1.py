"""TST-VENDOR-04.1 — Daisy golden frame encoding; runs native C++ protocol test suite."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "IoT/common-modules/daisy/run-tests.sh"

REQUIRED_MARKER = "Daisy protocol tests passed"


class TestTSTVendor04_1(unittest.TestCase):
    def test_daisy_protocol_golden_frames(self):
        self.assertTrue(SCRIPT.exists(), f"run-tests.sh not found: {SCRIPT}")
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=300,
            cwd=str(SCRIPT.parent),
        )
        self.assertEqual(
            result.returncode, 0,
            f"run-tests.sh exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-04-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
