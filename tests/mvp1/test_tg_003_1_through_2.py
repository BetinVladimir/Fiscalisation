"""TG-003.1-2 — Bridge.DeviceTime() returns ErrDeviceClockNotMeasured; SetDeviceTime returns ErrDeviceClockSetUnsupported."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestTG003_12(unittest.TestCase):
    def test_bridge_device_time_fails_without_measurement(self):
        r = subprocess.run(
            ['go', 'test', './internal/mqttclient/...', '-run',
             'TestBridgeDeviceTimeFailsClosedWithoutMeasurement|TestBridgeSetDeviceTimeUnsupportedForBC50',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('PASS', r.stdout + r.stderr)
        self.assertIn('TestBridgeDeviceTimeFailsClosedWithoutMeasurement', r.stdout)
        self.assertIn('TestBridgeSetDeviceTimeUnsupportedForBC50', r.stdout)


if __name__ == '__main__':
    unittest.main()
