"""BUG-061.1 — decimal.golden.test.ts passes with golden receipt values."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG061_1(unittest.TestCase):
    def test_decimal_golden_jest(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'decimal.golden', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}',
        )
        self.assertIn('PASS', r.stdout + r.stderr,
                      'Expected PASS in jest output')
        # Verify non-zero test count
        self.assertRegex(r.stdout + r.stderr, r'Tests:\s+\d+',
                         'Expected test count in jest output')


if __name__ == '__main__':
    unittest.main()
