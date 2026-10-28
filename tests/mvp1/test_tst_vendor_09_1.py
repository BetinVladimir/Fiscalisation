"""TST-VENDOR-09.1 — Datecs error catalog completeness check script."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts/check-datecs-error-catalog.sh"

REQUIRED_MARKER = "DATECS-ERROR-CATALOG OK"


class TestTSTVendor09_1(unittest.TestCase):
    def test_datecs_error_catalog_check(self):
        self.assertTrue(SCRIPT.exists(), f"check-datecs-error-catalog.sh not found: {SCRIPT}")
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=120,
            cwd=str(ROOT),
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-datecs-error-catalog.sh exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-09-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
