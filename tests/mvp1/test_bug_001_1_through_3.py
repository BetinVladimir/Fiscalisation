"""BUG-001.1-3 — receiptMoneyCents regex + negative-payment rejection."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG001_1_3(unittest.TestCase):
    def test_receipt_money_cents_rejects_malformed(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestReceiptMoneyCentsRejectsSignedAndMalformed|TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReceiptMoneyCentsRejectsSignedAndMalformed',
                'TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
