"""CODE-002.3 — All fiscal calls use typed exception classification."""
import subprocess, unittest
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestCODE002_3(unittest.TestCase):
    def test_no_message_in_setof_anywhere(self):
        r = subprocess.run(
            ['grep', '-rn', 'e.message in', str(SMARTDEVICES / 'bluecash-app/src/main'), '--include=*.kt'],
            capture_output=True, text=True)
        self.assertEqual(r.stdout.strip(), '', f'Found e.message in:\n{r.stdout}')

    def test_full_test_suite_passes(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0, f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')

if __name__ == '__main__':
    unittest.main()
