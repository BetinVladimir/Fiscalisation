"""TST-GO-04.1 — TestFiscalOperationPrepareFailureMarksOperationFailed (BUG-004)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTSTGO04(unittest.TestCase):
    def test_fiscal_operation_prepare_failure_marks_failed(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestFiscalOperationPrepareFailureMarksOperationFailed'
                     '|TestFiscalOperationUsesBoundDriverBeforeDefault)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestFiscalOperationPrepareFailureMarksOperationFailed',
                'TestFiscalOperationUsesBoundDriverBeforeDefault',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
