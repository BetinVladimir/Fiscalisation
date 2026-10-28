"""TST-CRIT-01.2 — TRACE_CRITERIA.md: every criteria ID from review_criteria.json is present.

Acceptance: TRACE_CRITERIA.md exists; each of the 112 criteria IDs appears in it;
every row has a TST-*/BUG-*/out_of_mvp1 reference (no blank references).
"""
import json
import os
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CRITERIA_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "review_criteria.json"
TRACE_FILE = ROOT / "docs" / "MVP1" / "TRACE_CRITERIA.md"

KNOWN_PREFIX_PATTERN = re.compile(
    r'^(TST-|BUG-|VG-|S3D-|REFUND-|PERSIST-|MATRIX-|EVIDENCE-|EMU-|HILMX-|TG-|BF-|out_of_mvp1)'
)


class TestTSTCRIT01_2(unittest.TestCase):
    def _load_criteria_ids(self):
        self.assertTrue(CRITERIA_FILE.exists(), f"review_criteria.json missing: {CRITERIA_FILE}")
        data = json.loads(CRITERIA_FILE.read_text())
        return [c["id"] for c in data]

    def _load_trace_text(self):
        self.assertTrue(TRACE_FILE.exists(), f"TRACE_CRITERIA.md missing: {TRACE_FILE}")
        return TRACE_FILE.read_text()

    def test_trace_file_exists(self):
        self.assertTrue(TRACE_FILE.exists(), f"TRACE_CRITERIA.md not found: {TRACE_FILE}")

    def test_all_criteria_ids_present_in_trace(self):
        ids = self._load_criteria_ids()
        trace = self._load_trace_text()
        missing = [cid for cid in ids if cid not in trace]
        self.assertEqual(
            missing, [],
            f"{len(missing)} criteria IDs not found in TRACE_CRITERIA.md: {missing[:10]}"
        )

    def test_112_ids_covered(self):
        """Exactly 112 criteria IDs are mentioned in the trace file."""
        ids = self._load_criteria_ids()
        trace = self._load_trace_text()
        found = [cid for cid in ids if cid in trace]
        self.assertEqual(
            len(found), 112,
            f"expected 112 IDs found, got {len(found)}"
        )

    def test_each_table_row_has_reference(self):
        """Every row in TRACE_CRITERIA.md table has a non-empty reference column."""
        trace = self._load_trace_text()
        # Match table rows: | ID | Summary | Area | Ref |
        rows = re.findall(r'\|\s*([\w-]+)\s*\|[^|]+\|[^|]+\|\s*([^\n|]+)\s*\|', trace)
        empty_refs = []
        for row_id, ref in rows:
            ref = ref.strip()
            if not ref:
                empty_refs.append(row_id)
        self.assertEqual(
            empty_refs, [],
            f"rows with empty reference: {empty_refs}"
        )

    def test_references_use_known_prefixes(self):
        """Reference column uses TST-*/BUG-*/VG-*/S3D-*/out_of_mvp1 etc."""
        trace = self._load_trace_text()
        rows = re.findall(r'\|\s*([\w-]+)\s*\|[^|]+\|[^|]+\|\s*([^\n|]+)\s*\|', trace)
        bad_refs = []
        for row_id, ref in rows:
            ref = ref.strip()
            if ref and not KNOWN_PREFIX_PATTERN.match(ref):
                # Allow header row
                if row_id not in ("ID", "---"):
                    bad_refs.append((row_id, ref))
        self.assertEqual(
            bad_refs, [],
            f"rows with unexpected reference format: {bad_refs[:5]}"
        )

    def test_criteria_ids_match_file(self):
        """IDs in TRACE_CRITERIA.md match IDs in review_criteria.json."""
        ids_in_json = set(self._load_criteria_ids())
        trace = self._load_trace_text()
        ids_in_trace = set(re.findall(r'\|\s*([A-Z]+-\d+)\s*\|', trace))
        # All JSON IDs should be in trace
        missing = ids_in_json - ids_in_trace
        self.assertEqual(
            missing, set(),
            f"IDs in JSON but not in TRACE table: {missing}"
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_crit_01_2.txt").write_text(
                f"ids_in_json={len(ids_in_json)} ids_in_trace={len(ids_in_trace)}\n"
            )


if __name__ == "__main__":
    unittest.main(verbosity=2)
