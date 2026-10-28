"""ARCH-004.3 — full money module regression (no parseFixed outside money.go)."""
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestARCH004_3(unittest.TestCase):
    def test_money_regression(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/', './internal/api/',
             '-run', '^(TestParseMoney|TestFormatMoney|TestLineTotalFromGoldenFile|TestMoneyRoundingGolden)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestParseMoney',
                'TestLineTotalFromGoldenFile',
            ),
            minimum=2,
        )

    def test_no_strconv_in_money_paths(self):
        """Ensure no raw strconv.ParseInt on amount fields outside money.go."""
        result = subprocess.run(
            ['grep', '-rn', r'strconv.ParseInt', 'internal/domain'],
            cwd=str(ROOT / 'fiscal-backend'),
            capture_output=True, text=True,
        )
        # AmountMinor is already in minor integer units — not a decimal money parse.
        hits = [l for l in result.stdout.splitlines()
                if 'amount' in l.lower() and 'money.go' not in l and '_test' not in l
                and 'AmountMinor' not in l]
        self.assertEqual(hits, [], 'raw strconv.ParseInt on amount fields: ' + '\n'.join(hits))


if __name__ == '__main__':
    unittest.main()
