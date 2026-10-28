"""BUG-061.3 — Full jest suite for decimal module passes."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG061_3(unittest.TestCase):
    def test_decimal_full_jest(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'decimal', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}',
        )


if __name__ == '__main__':
    unittest.main()
