"""W0-2.1 — plan_coverage.json structure + check_plan_coverage.rb passes."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
COVERAGE_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "plan_coverage.json"
SCRIPT = ROOT / "scripts" / "check_plan_coverage.rb"
PLAN_FILE = ROOT / "docs" / "MVP1" / "fixes" / "plan.json"

REQUIRED_GROUPS = {"P40", "P41", "P42", "P43", "P44"}
REQUIRED_GROUP_FIELDS = {"id", "name", "description", "task_ids", "evidence_gate"}


class TestW02PlanCoverage(unittest.TestCase):

    def test_coverage_file_exists(self):
        self.assertTrue(COVERAGE_FILE.exists(), f"plan_coverage.json not found at {COVERAGE_FILE}")

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"check_plan_coverage.rb not found at {SCRIPT}")

    def test_coverage_schema_1(self):
        data = json.loads(COVERAGE_FILE.read_text())
        self.assertEqual(data.get("schema"), 1, "coverage.schema must be 1")

    def test_coverage_has_groups(self):
        data = json.loads(COVERAGE_FILE.read_text())
        groups = data.get("groups", [])
        self.assertIsInstance(groups, list)
        self.assertGreater(len(groups), 0, "groups must be non-empty")

    def test_all_required_groups_present(self):
        data = json.loads(COVERAGE_FILE.read_text())
        found = {g["id"] for g in data.get("groups", []) if "id" in g}
        for gid in REQUIRED_GROUPS:
            self.assertIn(gid, found, f"Required group '{gid}' not found in plan_coverage.json")

    def test_each_group_has_required_fields(self):
        data = json.loads(COVERAGE_FILE.read_text())
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            for field in REQUIRED_GROUP_FIELDS:
                self.assertIn(field, grp, f"Group {gid}: missing required field '{field}'")

    def test_each_group_task_ids_nonempty(self):
        data = json.loads(COVERAGE_FILE.read_text())
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            task_ids = grp.get("task_ids", [])
            self.assertIsInstance(task_ids, list)
            self.assertGreater(len(task_ids), 0, f"Group {gid}: task_ids must not be empty")

    def test_task_ids_exist_in_plan(self):
        """All task_ids in coverage must exist in plan.json."""
        plan = json.loads(PLAN_FILE.read_text())
        plan_ids = {t["id"] for t in plan.get("tasks", [])}
        data = json.loads(COVERAGE_FILE.read_text())
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            for tid in grp.get("task_ids", []):
                self.assertIn(tid, plan_ids,
                              f"Group {gid}: task_id '{tid}' not found in plan.json")

    def test_script_exits_0(self):
        """check_plan_coverage.rb must exit 0 against the current fixture."""
        result = subprocess.run(
            ["ruby", str(SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(
            result.returncode, 0,
            f"check_plan_coverage.rb exited {result.returncode}:\n{result.stdout}\n{result.stderr}"
        )

    def test_script_bad_coverage_exits_nonzero(self):
        """Script must fail on an invalid coverage file (missing required fields)."""
        import tempfile, os
        bad = {"schema": 1, "groups": [{"id": "P99", "name": "bad"}]}
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(bad, f)
            bad_path = f.name
        try:
            result = subprocess.run(
                ["ruby", str(SCRIPT), "--coverage", bad_path],
                capture_output=True, text=True
            )
            self.assertNotEqual(
                result.returncode, 0,
                "Script should fail on invalid coverage (missing groups P40-P44)"
            )
        finally:
            os.unlink(bad_path)


if __name__ == "__main__":
    unittest.main()
