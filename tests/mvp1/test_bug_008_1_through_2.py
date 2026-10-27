"""BUG-008.1-2 — acceptance: reversal uses FiscalDocumentAt, not UpdatedAt."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0081_2(unittest.TestCase):
    def test_reverse_uses_fiscal_document_timestamp(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestReverseUsesFiscalDocumentTimestamp$|^TestReverseFailsWhenFiscalDocumentAtMissing$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReverseUsesFiscalDocumentTimestamp',
                'TestReverseFailsWhenFiscalDocumentAtMissing',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
