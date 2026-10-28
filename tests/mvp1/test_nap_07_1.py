"""NAP-07.1 — docs/DATA_LOCATION.md must exist (чл.52г, 52з(14), Прил.32 п.10-13)."""
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DOC = ROOT / "docs" / "DATA_LOCATION.md"


class TestDataLocationDocExists(unittest.TestCase):
    def test_data_location_file_exists(self):
        self.assertTrue(
            DOC.exists(),
            f"docs/DATA_LOCATION.md not found: {DOC}",
        )

    def test_data_location_has_title(self):
        self.assertTrue(DOC.exists(), "docs/DATA_LOCATION.md not found")
        text = DOC.read_text(encoding="utf-8")
        text_upper = text.upper()
        self.assertTrue(
            "DATA LOCATION" in text_upper or "DATA_LOCATION" in text_upper,
            "Document must mention 'Data Location' in its content",
        )

    def test_data_location_has_table(self):
        self.assertTrue(DOC.exists(), "docs/DATA_LOCATION.md not found")
        text = DOC.read_text(encoding="utf-8")
        # Markdown table requires a | separator line
        table_rows = [l for l in text.splitlines() if l.strip().startswith("|") and "|" in l[1:]]
        self.assertGreater(len(table_rows), 0, "docs/DATA_LOCATION.md must contain a Markdown table")

    def test_script_exit_zero(self):
        script = ROOT / "scripts" / "check-data-location.sh"
        self.assertTrue(script.exists(), f"Script not found: {script}")
        result = subprocess.run(
            ["bash", str(script)],
            capture_output=True, text=True, timeout=30,
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-data-location.sh failed:\nstdout: {result.stdout}\nstderr: {result.stderr}",
        )
        self.assertIn("OK", result.stdout)


if __name__ == "__main__":
    unittest.main()
