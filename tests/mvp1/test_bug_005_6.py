"""BUG-005.6 — Full clock sync acceptance: all subtasks green."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG005_6(unittest.TestCase):
    def test_full_clock_sync_suite(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestClockSyncRejectsHostClockAsDeviceClock|TestHasDailyClockSyncUsesSofiaBusinessDate|TestClockSyncWarnDriftOver5Min|TestClockSyncBlockDriftOver2Hours|TestClockSyncFailsWhenDriverLacksClockCapability|TestClockSyncFallbackToHostClockRejected|TestClockSyncDurabilityAndBlocksWithoutSync)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestClockSyncRejectsHostClockAsDeviceClock',
                'TestHasDailyClockSyncUsesSofiaBusinessDate',
                'TestClockSyncWarnDriftOver5Min',
                'TestClockSyncBlockDriftOver2Hours',
                'TestClockSyncFailsWhenDriverLacksClockCapability',
                'TestClockSyncFallbackToHostClockRejected',
                'TestClockSyncDurabilityAndBlocksWithoutSync',
            ),
            minimum=7,
        )

    def test_device_time_fail_closed(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/mqttclient/',
             '-run', '^TestDeviceTimeReturnsErrorWhenNotMeasured$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=('TestDeviceTimeReturnsErrorWhenNotMeasured',),
            minimum=1,
        )


if __name__ == '__main__':
    unittest.main()
