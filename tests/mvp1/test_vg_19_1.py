"""VG-19.1 — docs/vendor/manifest.json exists with >= 7 entries and required fields."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MANIFEST = ROOT / "docs" / "vendor" / "manifest.json"
REQUIRED_FIELDS = {"id", "doc", "name", "status"}


class TestVG191ManifestStructure(unittest.TestCase):
    def setUp(self):
        self.assertTrue(MANIFEST.exists(), f"manifest.json not found at {MANIFEST}")
        with open(MANIFEST) as f:
            self.docs = json.load(f)

    def test_manifest_is_array(self):
        self.assertIsInstance(self.docs, list, "manifest.json root must be a JSON array")

    def test_manifest_has_at_least_7_entries(self):
        self.assertGreaterEqual(len(self.docs), 7,
                                f"manifest.json must have >= 7 entries, got {len(self.docs)}")

    def test_all_entries_have_required_fields(self):
        for i, entry in enumerate(self.docs):
            missing = REQUIRED_FIELDS - set(entry.keys())
            self.assertSetEqual(missing, set(),
                                f"Entry {i} ({entry.get('id','?')}) missing fields: {missing}")

    def test_ids_are_unique(self):
        ids = [e.get("id") for e in self.docs]
        self.assertEqual(len(ids), len(set(ids)),
                         f"Duplicate IDs in manifest: {[x for x in ids if ids.count(x) > 1]}")

    def test_obtained_entries_have_sha256(self):
        for entry in self.docs:
            if entry.get("status") == "OBTAINED" and entry.get("file"):
                self.assertIsNotNone(
                    entry.get("sha256"),
                    f"Entry {entry.get('id')} with status=OBTAINED and file must have sha256"
                )

    def test_doc_codes_cover_doc_v3_and_v4(self):
        doc_codes = {e.get("doc") for e in self.docs}
        self.assertIn("DOC-V3", doc_codes, "manifest must include a DOC-V3 entry")
        self.assertIn("DOC-V4", doc_codes, "manifest must include a DOC-V4 entry")


if __name__ == "__main__":
    unittest.main()
