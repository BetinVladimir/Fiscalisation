"""TST-CRIT-01.1 — review_criteria.json: 112 criteria {id, summary, area} for PAY..IDEM.

Acceptance: file exists with exactly 112 entries; each entry has id, summary, area;
all 11 expected areas present; no duplicate IDs; jq length == 112.
"""
import json
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CRITERIA_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "review_criteria.json"

EXPECTED_COUNT = 112
EXPECTED_AREAS = {"PAY", "REC", "FW", "TIME", "WIRE", "UI", "BE", "AUTH", "CON", "MONEY", "IDEM"}
REQUIRED_FIELDS = {"id", "summary", "area"}


class TestTSTCRIT01_1(unittest.TestCase):
    def _load(self):
        self.assertTrue(CRITERIA_FILE.exists(), f"review_criteria.json not found: {CRITERIA_FILE}")
        data = json.loads(CRITERIA_FILE.read_text())
        self.assertIsInstance(data, list, "review_criteria.json must be a JSON array")
        return data

    def test_file_exists(self):
        self.assertTrue(CRITERIA_FILE.exists(), f"review_criteria.json not found: {CRITERIA_FILE}")

    def test_exactly_112_criteria(self):
        data = self._load()
        self.assertEqual(
            len(data), EXPECTED_COUNT,
            f"expected {EXPECTED_COUNT} criteria, got {len(data)}"
        )

    def test_required_fields_present(self):
        data = self._load()
        for i, c in enumerate(data):
            missing = REQUIRED_FIELDS - set(c.keys())
            self.assertEqual(
                missing, set(),
                f"criteria[{i}] (id={c.get('id', '?')}) missing fields: {missing}"
            )

    def test_all_areas_present(self):
        data = self._load()
        found_areas = {c["area"] for c in data}
        missing_areas = EXPECTED_AREAS - found_areas
        self.assertEqual(
            missing_areas, set(),
            f"missing areas: {missing_areas}"
        )

    def test_no_unknown_areas(self):
        data = self._load()
        unknown = {c["area"] for c in data} - EXPECTED_AREAS
        self.assertEqual(
            unknown, set(),
            f"unexpected areas: {unknown}"
        )

    def test_no_duplicate_ids(self):
        data = self._load()
        ids = [c["id"] for c in data]
        seen = set()
        duplicates = []
        for cid in ids:
            if cid in seen:
                duplicates.append(cid)
            seen.add(cid)
        self.assertEqual(duplicates, [], f"duplicate IDs: {duplicates}")

    def test_ids_not_empty(self):
        data = self._load()
        for i, c in enumerate(data):
            self.assertTrue(c.get("id"), f"criteria[{i}] has empty id")
            self.assertTrue(c.get("summary"), f"criteria[{i}] has empty summary")
            self.assertTrue(c.get("area"), f"criteria[{i}] has empty area")

    def test_jq_count_112(self):
        """jq length == 112 (uses python3 subprocess, same spirit)."""
        result = subprocess.run(
            ["python3", "-c",
             f"import json,sys; d=json.load(open('{CRITERIA_FILE}')); "
             f"assert len(d)=={EXPECTED_COUNT}, f'got {{len(d)}}'; "
             f"print('jq_length_ok:', len(d))"],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=15,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"jq length check failed (rc={result.returncode}):\n{output}"
        )
        self.assertIn(f"jq_length_ok: {EXPECTED_COUNT}", result.stdout)

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_crit_01_1.txt").write_text(output)


if __name__ == "__main__":
    unittest.main(verbosity=2)
