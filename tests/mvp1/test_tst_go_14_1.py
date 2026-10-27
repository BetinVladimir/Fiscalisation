"""TST-GO-14.1 — TestMoneyRoundingGolden (BUG-023)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO14(unittest.TestCase):
    def test_money_rounding_golden(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestMoneyRoundingGolden$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestMoneyRoundingGolden',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
