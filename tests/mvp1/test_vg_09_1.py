"""VG-09.1 — ReadsCurrencyParameterAtActivation."""
import subprocess, time, unittest, xml.etree.ElementTree as ET
from pathlib import Path
ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'
REQUIRED = ('ReadsCurrencyParameterAtActivation',)

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

class TestVG09_1(unittest.TestCase):
    def test_required_tests_pass(self):
        started = time.time_ns()
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon', '--rerun-tasks'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0, f'rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1500:]}')
        cases = _find_junit(SMARTDEVICES, started)
        self.assertGreater(len(cases), 0, 'No JUnit XML found')
        for name in REQUIRED:
            matching = [k for k in cases if name in k]
            self.assertGreater(len(matching), 0, f'Test containing {name!r} not found; got {sorted(cases)[:10]}')
            for m in matching:
                failures = cases[m].findall('failure') + cases[m].findall('error')
                self.assertEqual(len(failures), 0, f'{m}: ' + ''.join(f.text or '' for f in failures))

if __name__ == '__main__':
    unittest.main()
