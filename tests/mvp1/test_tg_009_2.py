"""TG-009.2 — TestReversalPolicy_SR01..04 all pass."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTG009_2(unittest.TestCase):
    def test_reversal_policy_named_cases(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestReversalPolicy_',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReversalPolicy_SR01',
                'TestReversalPolicy_SR02',
                'TestReversalPolicy_SR03',
                'TestReversalPolicy_SR04',
            ),
            minimum=4,
        )


if __name__ == '__main__':
    unittest.main()
