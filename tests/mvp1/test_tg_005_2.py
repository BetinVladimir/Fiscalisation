"""TG-005.2 — Full StartupRecoveryTest suite."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

class TestTG005_2(unittest.TestCase):
    def test_startup_recovery_all_tests(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.StartupRecoveryTest'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('BUILD SUCCESSFUL', r.stdout + r.stderr)

    def test_xml_results_no_failures(self):
        import xml.etree.ElementTree as ET
        xml_path = SMARTDEVICES / 'bluecash-app/build/test-results/testProdDebugUnitTest/TEST-com.beeloy.fiscal.bluecash.StartupRecoveryTest.xml'
        self.assertTrue(xml_path.exists(), f'Missing XML: {xml_path}')
        suite = ET.parse(str(xml_path)).getroot()
        self.assertGreater(int(suite.get('tests', 0)), 0)
        self.assertEqual(int(suite.get('failures', 0)), 0)
        self.assertEqual(int(suite.get('errors', 0)), 0)

if __name__ == '__main__':
    unittest.main()
