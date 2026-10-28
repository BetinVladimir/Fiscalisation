"""CODE-007.1 — acceptance: Clock interface exists in domain package."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestCODE0071(unittest.TestCase):
    def test_real_clock_returns_non_zero(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestRealClockReturnsNonZero$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestRealClockReturnsNonZero',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
