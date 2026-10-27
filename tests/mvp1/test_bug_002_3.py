"""BUG-002.3 — TestValidMoney with golden_money_validation.json fixture."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0023(unittest.TestCase):
    def test_native(self):
        check_native_tests(
            self,
            ['go', 'test', '-json', './internal/domain', '-run', '^TestValidMoney$', '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestValidMoney',
                'TestValidMoney/valid_zero',
                'TestValidMoney/valid_price',
                'TestValidMoney/reject_currency',
                'TestValidMoney/reject_minus',
                'TestValidMoney/reject_plus',
            ),
            minimum=6,
        )


if __name__ == '__main__':
    unittest.main()
