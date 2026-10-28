"""R-04.1 — split_execution_cards.py: one file per unit, index, <=150 lines/20KB, deterministic."""
import json
import subprocess
import sys
import tempfile
import os
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / "docs" / "MVP1" / "tools" / "split_execution_cards.py"
PLAN_FILE = ROOT / "docs" / "MVP1" / "fixes" / "plan.json"
GEN_DIR = ROOT / "docs" / "MVP1" / "generated_cards"
MAX_LINES = 150
MAX_BYTES = 20 * 1024


def _run_generator(output_dir, plan_path=None):
    cmd = [sys.executable, str(SCRIPT), "--output-dir", str(output_dir)]
    if plan_path:
        cmd += ["--plan", str(plan_path)]
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result


class TestR04SplitExecutionCards(unittest.TestCase):

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"split_execution_cards.py not found at {SCRIPT}")

    def test_generated_dir_exists(self):
        self.assertTrue(GEN_DIR.is_dir(), f"generated_cards/ not found at {GEN_DIR}")

    def test_index_json_exists(self):
        idx = GEN_DIR / "index.json"
        self.assertTrue(idx.exists(), f"index.json not found in generated_cards/")

    def test_index_schema_1(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        self.assertEqual(data.get("schema"), 1)

    def test_index_cards_nonempty(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        cards = data.get("cards", [])
        self.assertGreater(len(cards), 0, "index.cards must be non-empty")

    def test_each_card_file_exists(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        for entry in data["cards"]:
            fpath = GEN_DIR / entry["file"]
            self.assertTrue(fpath.exists(), f"Card file missing: {entry['file']}")

    def test_each_card_within_line_limit(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        for entry in data["cards"]:
            self.assertLessEqual(
                entry["lines"], MAX_LINES,
                f"Card {entry['unit_id']}: {entry['lines']} lines exceeds {MAX_LINES}"
            )

    def test_each_card_within_size_limit(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        for entry in data["cards"]:
            self.assertLessEqual(
                entry["bytes"], MAX_BYTES,
                f"Card {entry['unit_id']}: {entry['bytes']} bytes exceeds {MAX_BYTES}"
            )

    def test_index_has_required_fields(self):
        idx = GEN_DIR / "index.json"
        if not idx.exists():
            self.skipTest("index.json not found")
        data = json.loads(idx.read_text())
        required = {"unit_id", "task_id", "priority", "source", "file", "lines", "bytes"}
        for entry in data.get("cards", [])[:5]:
            for field in required:
                self.assertIn(field, entry, f"index entry missing field '{field}'")

    def test_deterministic_regeneration(self):
        """Running the generator twice produces the same index."""
        with tempfile.TemporaryDirectory() as d1, tempfile.TemporaryDirectory() as d2:
            r1 = _run_generator(d1)
            r2 = _run_generator(d2)
            self.assertEqual(r1.returncode, 0, f"First run failed: {r1.stderr}")
            self.assertEqual(r2.returncode, 0, f"Second run failed: {r2.stderr}")
            idx1 = json.loads((Path(d1) / "index.json").read_text())
            idx2 = json.loads((Path(d2) / "index.json").read_text())
            self.assertEqual(
                [c["unit_id"] for c in idx1["cards"]],
                [c["unit_id"] for c in idx2["cards"]],
                "Generator is not deterministic: unit order differs between runs"
            )

    def test_bad_fixture_exceeds_limit_exits_nonzero(self):
        """Generator must fail (not silently truncate) if a unit exceeds limits."""
        # 200 lines, each 200 chars => 40000 bytes > 20KB and > 150 lines
        huge_spec = "\n".join(["x" * 200] * 200)
        bad_plan = {
            "schema": 1,
            "policy": "",
            "source_documents": {},
            "tasks": [
                {
                    "id": "HUGE-001",
                    "priority": "P1",
                    "source": "test",
                    "source_status": "NEW",
                    "mode": "code",
                    "depends": [],
                    "alias_of": None,
                    "units": [
                        {"id": "HUGE-001.1", "title": huge_spec}
                    ],
                    "spec": huge_spec,
                    "notes": "",
                    "external_requires": [],
                }
            ]
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(bad_plan, f)
            plan_path = f.name
        try:
            with tempfile.TemporaryDirectory() as outd:
                result = _run_generator(outd, plan_path=plan_path)
                self.assertNotEqual(
                    result.returncode, 0,
                    "Generator must exit non-zero when a unit exceeds line/size limits"
                )
        finally:
            os.unlink(plan_path)


if __name__ == "__main__":
    unittest.main()
