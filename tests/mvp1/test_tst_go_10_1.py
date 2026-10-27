"""TST-GO-10.1 — TestReversalReasonsFailClosed and reversal deadline tests (TG-009)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO10(unittest.TestCase):
    def test_reversal_policy(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestReversalReasonsFailClosed'
                     '|TestOperatorErrorDeadlineHandlesYearAndDSTBoundaries'
                     '|TestOperatorErrorReversalDeadlineUsesSofiaCalendar)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestReversalReasonsFailClosed',
                'TestOperatorErrorDeadlineHandlesYearAndDSTBoundaries',
                'TestOperatorErrorReversalDeadlineUsesSofiaCalendar',
            ),
            minimum=3,
        )


if __name__ == '__main__':
    unittest.main()
