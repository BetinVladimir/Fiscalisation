"""VG-19.2 — docs/vendor/ has >= 4 PDF files; all OBTAINED entries exist on disk."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
VENDOR_DIR = ROOT / "docs" / "vendor"


class TestVG192VendorFiles(unittest.TestCase):
    def test_vendor_dir_exists(self):
        self.assertTrue(VENDOR_DIR.is_dir(), f"docs/vendor/ directory not found at {VENDOR_DIR}")

    def test_at_least_4_pdf_files(self):
        pdfs = list(VENDOR_DIR.glob("*.pdf"))
        self.assertGreaterEqual(
            len(pdfs), 4,
            f"docs/vendor/ must contain >= 4 PDF files, found {len(pdfs)}: {[p.name for p in pdfs]}"
        )

    def test_obtained_files_exist(self):
        import json
        manifest = VENDOR_DIR / "manifest.json"
        self.assertTrue(manifest.exists(), "manifest.json not found")
        with open(manifest) as f:
            docs = json.load(f)
        for entry in docs:
            if entry.get("status") == "OBTAINED" and entry.get("file"):
                full = ROOT / entry["file"]
                self.assertTrue(
                    full.exists(),
                    f"Entry {entry.get('id')} has status=OBTAINED but file not found: {entry['file']}"
                )

    def test_readme_exists(self):
        readme = VENDOR_DIR / "README.md"
        self.assertTrue(readme.exists(), "docs/vendor/README.md not found")


if __name__ == "__main__":
    unittest.main()
