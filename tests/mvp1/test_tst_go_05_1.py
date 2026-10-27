"""TST-GO-05.1 — Clock sync: host-clock fallback rejected, sync durability (BUG-005)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestTSTGO05(unittest.TestCase):
    def test_clock_sync_fail_closed(self):
        check_native_tests(
            self,
            ['go', 'test', '-json',
             './internal/domain/',
             '-run', '^(TestClockSyncFailsWhenDriverLacksClockCapability'
                     '|TestClockSyncFallbackToHostClockRejected'
                     '|TestClockSyncDurabilityAndBlocksWithoutSync)$',
             '-count=1'],
            root=ROOT,
            cwd='fiscal-backend',
            required_tests=(
                'TestClockSyncFailsWhenDriverLacksClockCapability',
                'TestClockSyncFallbackToHostClockRejected',
                'TestClockSyncDurabilityAndBlocksWithoutSync',
            ),
            minimum=3,
        )

    def test_device_time_error_when_not_measured(self):
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
