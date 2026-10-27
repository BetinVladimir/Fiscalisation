"""BUG-003.5 — exports.go: Simulated: false в двух местах."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0035(unittest.TestCase):
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
