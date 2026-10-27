"""BUG-005.5 — Bridge.DeviceTime returns DEVICE_CLOCK_NOT_MEASURED (fail-closed)."""
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / 'docs/MVP1/fixes'))
from native_harness import check_native_tests


class TestBUG005_5(unittest.TestCase):
    def test_device_time_not_measured(self):
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
