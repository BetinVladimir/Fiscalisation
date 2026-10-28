"""BUG-003.2-3 — exports.go Simulated=false + TestExportOperationIsNeverSimulated."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG003_2_3(unittest.TestCase):
    def test_export_never_simulated(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestExportOperationIsNeverSimulated$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestExportOperationIsNeverSimulated',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
