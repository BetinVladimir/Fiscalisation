"""TST-GO-13.1 — TestPaymentTypeCapabilityMatrix (BUG-042)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestTSTGO13(unittest.TestCase):
    def test_payment_type_capability_matrix(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestPaymentTypeCapabilityMatrix'
                     '|TestPaymentTypeRejectedBeforeSideEffect)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestPaymentTypeCapabilityMatrix',
                'TestPaymentTypeRejectedBeforeSideEffect',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
