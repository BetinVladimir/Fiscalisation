"""SCR-39.1 — check-mvp-cut.py: duplicate ID→1, missing ID→1, clean→0."""
import json
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts' / 'check-mvp-cut.py'


def _run(data: dict) -> tuple[int, str]:
    with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        json.dump(data, f)
        path = f.name
    r = subprocess.run(['python3', str(SCRIPT), path], capture_output=True, text=True)
    return r.returncode, r.stdout + r.stderr


class TestSCR39_1(unittest.TestCase):
    def test_duplicate_id_exits_nonzero(self):
        data = {
            'must_before_hil': ['TASK-A', 'TASK-B'],
            'after_hil': ['TASK-B', 'TASK-C'],
            'all_tasks': ['TASK-A', 'TASK-B', 'TASK-C'],
        }
        rc, out = _run(data)
        self.assertNotEqual(rc, 0, f'Expected nonzero exit for duplicate ID\n{out}')
        self.assertIn('DUPLICATE', out)

    def test_missing_id_exits_nonzero(self):
        data = {
            'must_before_hil': ['TASK-A'],
            'after_hil': ['TASK-C'],
            'all_tasks': ['TASK-A', 'TASK-B', 'TASK-C'],
        }
        rc, out = _run(data)
        self.assertNotEqual(rc, 0, f'Expected nonzero exit for missing ID\n{out}')
        self.assertIn('MISSING', out)

    def test_clean_file_exits_zero(self):
        data = {
            'must_before_hil': ['TASK-A', 'TASK-B'],
            'after_hil': ['TASK-C', 'TASK-D'],
            'all_tasks': ['TASK-A', 'TASK-B', 'TASK-C', 'TASK-D'],
        }
        rc, out = _run(data)
        self.assertEqual(rc, 0, f'Expected exit 0 for clean file\n{out}')
        self.assertIn('OK', out)

    def test_real_mvp_cut_passes(self):
        real_path = ROOT / 'docs' / 'MVP1' / 'execution' / 'mvp-cut.v1.json'
        self.assertTrue(real_path.exists(), f'mvp-cut.v1.json not found at {real_path}')
        r = subprocess.run(
            ['python3', str(SCRIPT), str(real_path)],
            capture_output=True, text=True,
        )
        self.assertEqual(r.returncode, 0, f'Real mvp-cut.v1.json failed:\n{r.stdout}{r.stderr}')


if __name__ == '__main__':
    unittest.main()
