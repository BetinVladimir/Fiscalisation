"""SCR-40.1 — check-hil-matrix.py validates mvp_matrix.json structure."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts' / 'check-hil-matrix.py'
MATRIX = ROOT / 'docs' / 'MVP1' / 'fixtures' / 'mvp_matrix.json'


class TestSCR40_1(unittest.TestCase):
    def test_matrix_file_exists(self):
        self.assertTrue(MATRIX.exists(), f'mvp_matrix.json not found: {MATRIX}')

    def test_checker_exits_zero_on_real_matrix(self):
        r = subprocess.run(
            ['python3', str(SCRIPT), str(MATRIX)],
            capture_output=True, text=True,
        )
        self.assertEqual(
            r.returncode, 0,
            f'check-hil-matrix.py failed (rc={r.returncode}):\n{r.stdout}{r.stderr}',
        )

    def test_checker_reports_20_routes(self):
        r = subprocess.run(
            ['python3', str(SCRIPT), str(MATRIX)],
            capture_output=True, text=True,
        )
        self.assertEqual(r.returncode, 0, f'Checker failed:\n{r.stdout}{r.stderr}')
        self.assertIn('20 routes', r.stdout, 'Expected "20 routes" in checker output')

    def test_checker_reports_all_hardware_paths(self):
        r = subprocess.run(
            ['python3', str(SCRIPT), str(MATRIX)],
            capture_output=True, text=True,
        )
        self.assertEqual(r.returncode, 0, f'Checker failed:\n{r.stdout}{r.stderr}')
        for hw in ('DatecsBC50', 'BluePay', 'DaisyCompact', 'SmartS', 'BlueCash'):
            self.assertIn(hw, r.stdout, f'Hardware path {hw!r} missing from checker output')

    def test_checker_nonzero_on_missing_file(self):
        r = subprocess.run(
            ['python3', str(SCRIPT), '/nonexistent/path/mvp_matrix.json'],
            capture_output=True, text=True,
        )
        self.assertNotEqual(r.returncode, 0, 'Expected nonzero exit for missing file')


if __name__ == '__main__':
    unittest.main()
