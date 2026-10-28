"""ARCH-004.1-2 — ParseMoney/LineTotal/FormatMoney in money.go + golden rounding."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestARCH004_1_2(unittest.TestCase):
    def test_money_module_and_golden(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestParseMoney|TestFormatMoney|TestLineTotalFromGoldenFile|TestMoneyRoundingGolden|TestLineTotalOverflow)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestParseMoney',
                'TestLineTotalFromGoldenFile',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
