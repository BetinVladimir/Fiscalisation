"""BUG-005.4 — O(1) daily sync key workstation|business_date."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG005_4(unittest.TestCase):
    def test_o1_sync_key(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestHasDailyClockSyncUsesSofiaBusinessDate|TestClockSyncBlockDriftOver2Hours)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestHasDailyClockSyncUsesSofiaBusinessDate',
                'TestClockSyncBlockDriftOver2Hours',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
