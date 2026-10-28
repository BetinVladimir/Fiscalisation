"""TST-DEEP-01.1 — concurrent sales persist + restart (test executes)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTSTDEEP01_1(unittest.TestCase):
    def test_concurrent_sales_persist_after_restart(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/persistence/',
             '-run', '^TestConcurrentSalesPersistAfterRestart$',
             '-count=1', '-timeout=60s'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestConcurrentSalesPersistAfterRestart',),
            minimum=1,
            timeout=120,
        )


if __name__ == '__main__':
    unittest.main()
