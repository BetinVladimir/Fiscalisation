"""BUG-003.6 — регрессия: нет Simulated:true в service.go/exports.go + оба теста зелёные."""
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests

_SCOPED_FILES = [
    'fiscal-backend/internal/domain/service.go',
    'fiscal-backend/internal/domain/exports.go',
]


class TestBUG0036(unittest.TestCase):
    def test_no_simulated_true_literals(self):
        """No hardcoded Simulated: true in production files."""
        for rel in _SCOPED_FILES:
            path = ROOT / rel
            text = path.read_text(encoding='utf-8')
            self.assertNotIn(
                'Simulated: true',
                text,
                f'{rel} still contains hardcoded Simulated: true',
            )

    def test_regression_both_simulated_tests(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestOperationSimulatedFlagFollowsDriver|TestExportOperationIsNeverSimulated)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestOperationSimulatedFlagFollowsDriver',
                'TestExportOperationIsNeverSimulated',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
