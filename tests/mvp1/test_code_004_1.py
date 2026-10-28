"""CODE-004.1 — No swallowed errors in domain code without best-effort annotation."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestCODE004_1(unittest.TestCase):
    def test_script_exists(self):
        f = ROOT / 'scripts/check-swallowed-errors.sh'
        self.assertTrue(f.exists(), f'check-swallowed-errors.sh not found: {f}')

    def test_no_swallowed_errors(self):
        script = ROOT / 'scripts/check-swallowed-errors.sh'
        if not script.exists():
            self.skipTest('script not found')
        r = subprocess.run(['bash', str(script)], cwd=str(ROOT),
                           capture_output=True, text=True, timeout=60)
        self.assertEqual(r.returncode, 0,
                         f'Swallowed errors found:\n{r.stdout}\n{r.stderr}')


if __name__ == '__main__':
    unittest.main()
