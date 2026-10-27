"""TST-GO-02.1 — TestValidMoney (BUG-002)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO02(unittest.TestCase):
    def test_valid_money_table(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestValidMoney$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestValidMoney',
                'TestValidMoney/valid_zero',
                'TestValidMoney/valid_price',
                'TestValidMoney/reject_minus',
                'TestValidMoney/reject_plus',
            ),
            minimum=5,
        )


if __name__ == '__main__':
    unittest.main()
