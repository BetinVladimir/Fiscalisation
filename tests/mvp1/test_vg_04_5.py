"""VG-04.5 — FU state emulator (FakeDatecsDevice) for tests."""
import subprocess, time, unittest, xml.etree.ElementTree as ET
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

def _find_junit(cwd, started):
    found = {}
    for p in cwd.glob('bluecash-app/build/test-results/**/TEST-*.xml'):
        if p.stat().st_mtime_ns < started:
            continue
        try:
            root = ET.parse(p).getroot()
        except Exception:
            continue
        for tc in root.iter('testcase'):
            found[tc.get('name', '')] = tc
    return found

class TestVG04_5(unittest.TestCase):
    def test_fake_datecs_device_state_tests(self):
        started = time.time_ns()
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon', '--rerun-tasks'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0, f'rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1500:]}')
        cases = _find_junit(SMARTDEVICES, started)
        self.assertGreater(len(cases), 0, 'No JUnit XML found')
        fake_tests = [k for k in cases if 'FakeDatecs' in k]
        self.assertGreater(len(fake_tests), 0, f'No FakeDatecs tests found; cases: {sorted(cases)[:15]}')
        for name, tc in {k: v for k, v in cases.items() if 'FakeDatecs' in k}.items():
            failures = tc.findall('failure') + tc.findall('error')
            self.assertEqual(len(failures), 0, f'{name}: ' + ''.join(f.text or '' for f in failures))

if __name__ == '__main__':
    unittest.main()
