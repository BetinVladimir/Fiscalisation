"""NAP-07.2 — docs/DATA_LOCATION.md must have ≥8 table rows listing storage + region."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DOC = ROOT / "docs" / "DATA_LOCATION.md"

REQUIRED_STORES = {
    "postgresql",
    "rabbitm",   # RabbitMQ or EMQX
    "sqlite",
    "backup",
    "log",
    "object",    # object storage / S3
}


class TestDataLocationTableRows(unittest.TestCase):
    def _table_rows(self):
        self.assertTrue(DOC.exists(), "docs/DATA_LOCATION.md not found")
        text = DOC.read_text(encoding="utf-8")
        rows = [
            l for l in text.splitlines()
            if l.strip().startswith("|") and "|" in l[1:] and "---" not in l
        ]
        return rows

    def test_at_least_eight_table_rows(self):
        rows = self._table_rows()
        # Skip the header row (first table row) — data rows are all others
        data_rows = rows[1:] if rows else []
        self.assertGreaterEqual(
            len(data_rows), 8,
            f"Expected ≥8 data rows, got {len(data_rows)}. All rows found:\n" + "\n".join(rows),
        )

    def test_region_column_present(self):
        self.assertTrue(DOC.exists(), "docs/DATA_LOCATION.md not found")
        text = DOC.read_text(encoding="utf-8").lower()
        self.assertIn("region", text, "DATA_LOCATION.md must have a 'Region' column")

    def test_known_stores_mentioned(self):
        self.assertTrue(DOC.exists(), "docs/DATA_LOCATION.md not found")
        text = DOC.read_text(encoding="utf-8").lower()
        missing = [s for s in REQUIRED_STORES if s not in text]
        self.assertFalse(
            missing,
            f"DATA_LOCATION.md must mention these storage technologies (case-insensitive): {missing}",
        )

    def test_all_regions_in_eu_or_bg(self):
        rows = self._table_rows()
        # Skip header row
        for row in rows[1:]:
            cells = [c.strip() for c in row.strip("|").split("|")]
            # region column is 3rd (index 2)
            if len(cells) < 3:
                continue
            region = cells[2].lower()
            if not region or region in ("region", "---"):
                continue
            is_ok = (
                "eu" in region
                or "bg" in region
                or region.startswith("eu-")
                or any(r in region for r in ("frankfurt", "amsterdam", "ireland", "paris", "stockholm", "milan", "sofia"))
            )
            self.assertTrue(
                is_ok,
                f"Region {cells[2]!r} in row {row!r} is not in EU/BG — only EU and BG regions allowed",
            )


if __name__ == "__main__":
    unittest.main()
