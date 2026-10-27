"""BUG-005.1-2 — host-clock rejected + Sofia business_date timezone conversion."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG005_1_2(unittest.TestCase):
    def test_host_clock_rejected_and_sofia_business_date(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestClockSyncRejectsHostClockAsDeviceClock|TestHasDailyClockSyncUsesSofiaBusinessDate)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestClockSyncRejectsHostClockAsDeviceClock',
                'TestHasDailyClockSyncUsesSofiaBusinessDate',
            ),
            minimum=2,
        )


if __name__ == '__main__':
    unittest.main()
