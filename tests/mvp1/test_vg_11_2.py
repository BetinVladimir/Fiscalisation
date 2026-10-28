"""VG-11.2 — DRIFT_WARN/DRIFT_BLOCK thresholds; cmd61 absent (no SetDeviceTime)."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestVG11_2(unittest.TestCase):
    def test_drift_threshold_tests(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsClockTest.driftWarnAtExactly5Min',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsClockTest.driftBlockAt2Hours',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsClockTest.driftBlockWhenFuBehindBy3Hours'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('BUILD SUCCESSFUL', r.stdout + r.stderr)

    def test_no_set_device_time_in_datecs_clock(self):
        clock_kt = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsClock.kt'
        self.assertTrue(clock_kt.exists(), f'DatecsClock.kt missing: {clock_kt}')
        src = clock_kt.read_text(encoding='utf-8')
        self.assertNotIn('CMD_SET_TIME', src, 'DatecsClock must not reference CMD_SET_TIME (cmd61 forbidden on BC-50)')
        self.assertNotIn('SetDeviceTime', src, 'DatecsClock must not contain SetDeviceTime')
        self.assertIn('DRIFT_WARN_SECONDS', src, 'DRIFT_WARN_SECONDS constant must be defined')
        self.assertIn('DRIFT_BLOCK_SECONDS', src, 'DRIFT_BLOCK_SECONDS constant must be defined')

if __name__ == '__main__':
    unittest.main()
