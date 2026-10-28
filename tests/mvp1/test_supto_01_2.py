"""SUPTO-01.2 — LoadSutpoProfiles Go tests pass."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestSupto01_2(unittest.TestCase):
    def test_go_supto_profiles(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/',
             '-run', 'TestLoadSutpoProfiles|TestSutpoProfile',
             '-v', '-count=1'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('PASS', r.stdout)
        self.assertEqual(r.stdout.count('--- PASS'), 4,
            f'expected 4 passing tests:\n{r.stdout}')


if __name__ == '__main__':
    unittest.main()
