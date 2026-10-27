"""BUG-008.3 — acceptance: FiscalDocumentAt set in reversal."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG0083(unittest.TestCase):
    def test_fiscal_document_at(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^TestReverseUsesFiscalDocumentTimestamp$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestReverseUsesFiscalDocumentTimestamp',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
