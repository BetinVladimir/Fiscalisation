"""TST-VENDOR-10.1 — Vendor manifest docs/vendor/manifest.json completeness check script."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts/check-vendor-docs.sh"

REQUIRED_MARKER = "VENDOR-DOCS OK"


class TestTSTVendor10_1(unittest.TestCase):
    def test_vendor_manifest_check(self):
        self.assertTrue(SCRIPT.exists(), f"check-vendor-docs.sh not found: {SCRIPT}")
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=120,
            cwd=str(ROOT),
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-vendor-docs.sh exited {result.returncode}\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-2000:]}",
        )
        self.assertIn(
            REQUIRED_MARKER, result.stdout,
            f"Marker not found: {result.stdout[-3000:]}",
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tst-vendor-10-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)


if __name__ == "__main__":
    unittest.main()
