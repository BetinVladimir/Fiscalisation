"""VG-19.3 — scripts/check-vendor-docs.sh: SHA256 check fails when a file is tampered."""
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "scripts" / "check-vendor-docs.sh"
MANIFEST = ROOT / "docs" / "vendor" / "manifest.json"


def _first_obtainable_entry():
    """Return first manifest entry that has both file and sha256 and file exists."""
    with open(MANIFEST) as f:
        docs = json.load(f)
    for entry in docs:
        if entry.get("status") == "OBTAINED" and entry.get("file") and entry.get("sha256"):
            full = ROOT / entry["file"]
            if full.exists():
                return entry, full
    return None, None


class TestVG193CheckVendorDocs(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"check-vendor-docs.sh not found at {SCRIPT}")

    def test_script_exits_0_on_clean_tree(self):
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-vendor-docs.sh should exit 0 on clean tree.\nstdout: {result.stdout}\nstderr: {result.stderr}"
        )

    def test_script_exits_nonzero_on_tampered_file(self):
        entry, original_path = _first_obtainable_entry()
        if entry is None:
            self.skipTest("No OBTAINED entry with file+sha256 found for tamper test")

        # Copy original to temp, write corrupted bytes
        with tempfile.NamedTemporaryFile(delete=False, suffix=".pdf") as tmp:
            tmp_path = Path(tmp.name)
        try:
            shutil.copy2(original_path, tmp_path)
            # Overwrite original with corrupted content
            original_path.write_bytes(b"TAMPERED_CONTENT_FOR_TEST")
            result = subprocess.run(
                ["bash", str(SCRIPT)],
                capture_output=True,
                text=True,
                timeout=60,
            )
            self.assertNotEqual(
                result.returncode, 0,
                f"check-vendor-docs.sh must exit non-zero when file is tampered.\n"
                f"stdout: {result.stdout}\nstderr: {result.stderr}"
            )
            self.assertIn("VND-003", result.stderr,
                          "Error output must reference VND-003 on SHA256 mismatch")
        finally:
            # Restore original
            shutil.copy2(tmp_path, original_path)
            tmp_path.unlink(missing_ok=True)


if __name__ == "__main__":
    unittest.main()
