"""AA-04.2 — AdminApp api/transition/createSystem Jest tests pass (12 green)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
ADMINAPP = ROOT / 'AdminApp'


class TestAA04_2(unittest.TestCase):
    def test_api_tests_green(self):
        r = subprocess.run(
            ['npx', '--no-install', 'jest', '--runInBand', '__tests__/api.test.tsx'],
            cwd=str(ADMINAPP),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            r.returncode, 0,
            f'jest exited {r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )
        combined = r.stdout + r.stderr
        self.assertIn(
            'Tests:       12 passed', combined,
            f'Expected 12 passing tests:\n{combined[-2000:]}',
        )


if __name__ == '__main__':
    unittest.main()
