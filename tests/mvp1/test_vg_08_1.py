"""VG-08.1 — StornoReasonMapping: POS reason → FU storno code 0/1/2."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestVG08_1(unittest.TestCase):
    def test_storno_reason_mapping(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsStornoTest.StornoReasonMapping'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('BUILD SUCCESSFUL', r.stdout + r.stderr)

if __name__ == '__main__':
    unittest.main()
