"""TST-DEEP-01.2 — concurrent sales persist + restart (green)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTDEEP01_2(unittest.TestCase):
    def test_concurrent_sales_persist_after_restart_green(self):
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
