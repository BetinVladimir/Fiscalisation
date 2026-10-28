"""AA-03.2 — fetchWithTimeout AbortController tests."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
ADMIN_APP = ROOT / 'AdminApp'

class TestAA03_2(unittest.TestCase):
    def test_fetchwith_timeout_jest(self):
        r = subprocess.run(
            ['npx', 'jest', 'src/fetchWithTimeout.test.ts', '--no-coverage'],
            cwd=str(ADMIN_APP), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0, f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('PASS', r.stdout + r.stderr)

if __name__ == '__main__':
    unittest.main()
