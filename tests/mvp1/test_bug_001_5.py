"""BUG-001.5 — регрессия domain + api."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent

from native_harness import check_native_tests


class TestBUG0015(unittest.TestCase):
    def test_domain_regression(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestReceiptMoneyCentsRejectsSignedAndMalformed'
                     '|TestValidateReceiptPlanRejectsNegativePaymentThatSumsCorrectly'
                     '|TestValidateReceiptPlanRejectsItemsTotalMismatch'
                     '|TestValidMoney'
                     '|TestOperationSimulatedFlagFollowsDriver'
                     '|TestExportOperationIsNeverSimulated)$',
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

    def test_api_regression(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/api/',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReadyzFailsWhenRepositoryUnavailable',
                'TestReadyzOKWhenRepositoryHealthy',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
