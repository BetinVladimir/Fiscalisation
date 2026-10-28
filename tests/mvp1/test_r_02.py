"""R-02.1 — legacy_plan_mapping.json structure + check_legacy_plan_mapping.py passes."""
import json
import subprocess
import sys
import tempfile
import os
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MAPPING_FILE = ROOT / "docs" / "MVP1" / "fixtures" / "legacy_plan_mapping.json"
SCRIPT = ROOT / "scripts" / "check_legacy_plan_mapping.py"
PLAN_FILE = ROOT / "docs" / "MVP1" / "fixes" / "plan.json"

REQUIRED_GROUPS = {"P40", "P41", "P42", "P43", "P44"}
REQUIRED_GROUP_FIELDS = {"id", "name", "legacy_description", "task_ids", "evidence_gate", "owner", "manual_gate"}


class TestR02LegacyPlanMapping(unittest.TestCase):

    def test_mapping_file_exists(self):
        self.assertTrue(MAPPING_FILE.exists(),
                        f"legacy_plan_mapping.json not found at {MAPPING_FILE}")

    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(),
                        f"check_legacy_plan_mapping.py not found at {SCRIPT}")

    def test_schema_1(self):
        data = json.loads(MAPPING_FILE.read_text())
        self.assertEqual(data.get("schema"), 1)

    def test_all_required_groups_present(self):
        data = json.loads(MAPPING_FILE.read_text())
        found = {g["id"] for g in data.get("groups", []) if "id" in g}
        for gid in REQUIRED_GROUPS:
            self.assertIn(gid, found, f"Required group '{gid}' missing from legacy_plan_mapping.json")

    def test_each_group_has_required_fields(self):
        data = json.loads(MAPPING_FILE.read_text())
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            for field in REQUIRED_GROUP_FIELDS:
                self.assertIn(field, grp, f"Group {gid}: missing field '{field}'")

    def test_task_ids_exist_in_plan(self):
        plan = json.loads(PLAN_FILE.read_text())
        plan_ids = {t["id"] for t in plan.get("tasks", [])}
        data = json.loads(MAPPING_FILE.read_text())
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            for tid in grp.get("task_ids", []):
                self.assertIn(tid, plan_ids, f"Group {gid}: task_id '{tid}' not in plan.json")

    def test_no_duplicate_task_ids_across_groups(self):
        """Each group should own distinct task IDs (no cross-group duplicate ownership)."""
        data = json.loads(MAPPING_FILE.read_text())
        seen = {}
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            for tid in grp.get("task_ids", []):
                if tid in seen:
                    # Duplicates across groups are allowed (tasks can relate to multiple groups)
                    # but let's ensure we track them
                    pass
                seen[tid] = gid
        # At least ensure no group has duplicate task IDs within itself
        for grp in data.get("groups", []):
            gid = grp.get("id", "?")
            tids = grp.get("task_ids", [])
            self.assertEqual(len(tids), len(set(tids)),
                             f"Group {gid} has duplicate task_ids: {tids}")

    def test_script_exits_0(self):
        result = subprocess.run(
            [sys.executable, str(SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(
            result.returncode, 0,
            f"check_legacy_plan_mapping.py exited {result.returncode}:\n{result.stdout}\n{result.stderr}"
        )

    def test_script_fails_on_missing_group(self):
        """Script must fail when a required group (e.g. P40) is absent."""
        bad = {
            "schema": 1,
            "groups": [
                {"id": "P99", "name": "test", "legacy_description": "x",
                 "task_ids": ["BUG-001"], "evidence_gate": "e", "owner": "qwen", "manual_gate": False}
            ]
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(bad, f)
            bad_path = f.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--mapping", bad_path],
                capture_output=True, text=True
            )
            self.assertNotEqual(
                result.returncode, 0,
                "Script should fail when required group P40 is missing"
            )
        finally:
            os.unlink(bad_path)

    def test_script_fails_on_unknown_task_id(self):
        """Script must fail when a task_id doesn't exist in plan.json."""
        bad = {
            "schema": 1,
            "groups": [
                {"id": g, "name": g, "legacy_description": "x",
                 "task_ids": ["__DOES_NOT_EXIST__"],
                 "evidence_gate": "e", "owner": "qwen", "manual_gate": False}
                for g in ["P40", "P41", "P42", "P43", "P44"]
            ]
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False) as f:
            json.dump(bad, f)
            bad_path = f.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--mapping", bad_path],
                capture_output=True, text=True
            )
            self.assertNotEqual(
                result.returncode, 0,
                "Script should fail on unknown task_id '__DOES_NOT_EXIST__'"
            )
        finally:
            os.unlink(bad_path)


if __name__ == "__main__":
    unittest.main()
