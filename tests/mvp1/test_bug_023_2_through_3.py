"""BUG-023.2-3 — Go TestMoneyRoundingGolden reads rounding_golden.json and passes."""
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'


class TestBUG023_2_3(unittest.TestCase):
    def test_go_money_rounding_golden(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', '^TestMoneyRoundingGolden$',
             '-count=1', './internal/domain/'],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            'Go TestMoneyRoundingGolden failed:\n' + output[-4000:],
        )
        self.assertIn(
            'PASS',
            output,
            'Go test output must contain PASS',
        )
        self.assertIn(
            'TestMoneyRoundingGolden',
            output,
            'Output must reference TestMoneyRoundingGolden',
        )
        self.assertIn(
            'ROUND OK',
            output,
            'Test must print ROUND OK on success',
        )


if __name__ == '__main__':
    unittest.main()
