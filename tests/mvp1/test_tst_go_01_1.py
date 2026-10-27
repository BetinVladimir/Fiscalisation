"""TST-GO-01.1 — TestReceiptMoneyCentsRejectsSignedAndMalformed (BUG-001)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO01(unittest.TestCase):
    def test_receipt_money_cents_rejects_signed_and_malformed(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestReceiptMoneyCentsRejectsSignedAndMalformed'
                     '|TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly'
                     '|TestValidateReceiptPlanRejectsItemsTotalMismatch)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReceiptMoneyCentsRejectsSignedAndMalformed',
                'TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly',
                'TestValidateReceiptPlanRejectsItemsTotalMismatch',
            ),
            minimum=3,
        )


if __name__ == '__main__':
    unittest.main()
