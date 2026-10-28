"""VG-11.1 — DatecsClock.parse() DST + Europe/Sofia timezone conversion."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestVG11_1(unittest.TestCase):
    def test_datecs_clock_parse_tests(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsClockTest'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('BUILD SUCCESSFUL', r.stdout + r.stderr)

    def test_xml_results_no_failures(self):
        import xml.etree.ElementTree as ET
        xml_path = SMARTDEVICES / 'bluecash-app/build/test-results/testProdDebugUnitTest/TEST-com.beeloy.fiscal.bluecash.DatecsClockTest.xml'
        self.assertTrue(xml_path.exists(), f'Missing XML: {xml_path}')
        suite = ET.parse(str(xml_path)).getroot()
        tests = int(suite.get('tests', 0))
        failures = int(suite.get('failures', 0))
        errors = int(suite.get('errors', 0))
        self.assertGreater(tests, 0, 'No tests found in XML')
        self.assertEqual(failures, 0, f'{failures} test(s) failed')
        self.assertEqual(errors, 0, f'{errors} error(s) in test run')

if __name__ == '__main__':
    unittest.main()
