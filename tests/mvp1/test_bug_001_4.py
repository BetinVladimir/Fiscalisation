"""BUG-001.4 — TestValidateReceiptPlanRejectsItemsTotalMismatch."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG0014(unittest.TestCase):
    def test_items_total_mismatch(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestValidateReceiptPlanRejectsItemsTotalMismatch$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestValidateReceiptPlanRejectsItemsTotalMismatch',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
