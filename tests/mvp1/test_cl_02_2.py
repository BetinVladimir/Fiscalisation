"""CL-02.2 — status.json has no dangling dependency references."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
STATUS_JSON = ROOT / 'docs/MVP1/execution/status.json'


class TestCL02_2(unittest.TestCase):
    def test_no_dangling_depends(self):
        self.assertTrue(STATUS_JSON.exists(), f'status.json not found: {STATUS_JSON}')
        data = json.loads(STATUS_JSON.read_text())
        tasks = data['tasks']

        # Collect all defined task and subtask IDs
        all_ids: set[str] = set()
        for task in tasks:
            all_ids.add(task['id'])
            for st in task.get('subtasks', []):
                all_ids.add(st['id'])

        # Check every depends entry refers to an existing ID
        dangling: list[tuple[str, str]] = []
        for task in tasks:
            for dep in task.get('depends', []):
                if dep not in all_ids:
                    dangling.append((task['id'], dep))
            for st in task.get('subtasks', []):
                for dep in st.get('depends', []):
                    if dep not in all_ids:
                        dangling.append((st['id'], dep))

        self.assertEqual(
            dangling, [],
            f'Dangling dependency references found: {dangling}',
        )

    def test_has_task_ids(self):
        data = json.loads(STATUS_JSON.read_text())
        tasks = data['tasks']
        self.assertGreater(len(tasks), 0, 'status.json must have tasks')
        for task in tasks:
            self.assertIn('id', task, 'Each task must have an id field')


if __name__ == '__main__':
    unittest.main()
