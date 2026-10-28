"""NAP-P-01.2 — SUPTO readiness matrix: device-profile requirements (п.5, п.8, п.9, п.15).

Covers Annex-29 trace items related to device readiness across BC50, Datecs/S3,
Daisy Compact/USB, Smart S:
  п.5  время/синхронизация → go test ./internal/domain -run ClockDrift
  п.8  готовность ФУ (readyz) → go test ./internal/api -run Readyz
  п.9  УНП                   → go test ./internal/domain -run UNP
  п.15 журнал (ACK hash)     → Kotlin gradle :bluecash-app:testProdDebugUnitTest
                                 --tests *.TSTKT11Test
"""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / 'fiscal-backend'
SMARTDEVICES = ROOT / 'SmartDevices'


def _run_go(pattern: str, pkg: str, timeout: int = 120) -> subprocess.CompletedProcess:
    return subprocess.run(
        ['go', 'test', '-v', '-count=1', '-run', pattern, pkg],
        cwd=str(BACKEND),
        capture_output=True,
        text=True,
        timeout=timeout,
    )


class TestNAPP01DeviceReadiness(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def _save(self, name: str, result: subprocess.CompletedProcess):
        with open(os.path.join(self._artifact, name), 'w') as f:
            f.write(result.stdout + result.stderr)

    def test_p5_clock_drift_policy(self):
        """п.5: clock drift policy: >5min WARN, >2h BLOCK (Europe/Sofia)."""
        r = _run_go('TestClockDrift', './internal/domain/...')
        self._save('nap_p01_2_clock_drift.txt', r)
        self.assertEqual(r.returncode, 0,
                         'ClockDrift tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        passed = [line for line in r.stdout.splitlines() if '--- PASS: TestClockDrift' in line]
        self.assertGreater(len(passed), 0,
                           'Expected at least one TestClockDrift* to pass')

    def test_p8_readyz_health_check(self):
        """п.8: /readyz fails when repository unavailable; OK when healthy."""
        r = _run_go('TestReadyz', './internal/api/...')
        self._save('nap_p01_2_readyz.txt', r)
        self.assertEqual(r.returncode, 0,
                         'Readyz tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        self.assertIn('PASS: TestReadyzOKWhenRepositoryHealthy', r.stdout,
                      'TestReadyzOKWhenRepositoryHealthy not found')
        self.assertIn('PASS: TestReadyzFailsWhenRepositoryUnavailable', r.stdout,
                      'TestReadyzFailsWhenRepositoryUnavailable not found')

    def test_p9_unp_fiscal_number(self):
        """п.9/9.1: УНП is assigned at first item; reused-receipt and reused-UNP rejected."""
        r = _run_go('UNP|SpecialUNP', './internal/domain/...')
        self._save('nap_p01_2_unp.txt', r)
        self.assertEqual(r.returncode, 0,
                         'UNP tests failed:\n' + r.stdout[-2000:] + r.stderr[-500:])
        passed = [line for line in r.stdout.splitlines() if '--- PASS: Test' in line and 'UNP' in line]
        self.assertGreater(len(passed), 0,
                           'Expected at least one UNP test to pass')

    def test_p15_journal_ack_hash_rejected(self):
        """п.15: journal ACK with wrong hash is rejected (Kotlin TSTKT11Test)."""
        gradlew = str(SMARTDEVICES / 'gradlew')
        self.assertTrue(Path(gradlew).exists(), f'gradlew not found: {gradlew}')
        r = subprocess.run(
            [
                gradlew, ':bluecash-app:testProdDebugUnitTest',
                '--tests', 'com.beeloy.fiscal.bluecash.TSTKT11Test',
                '-q', '--no-daemon',
            ],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self._save('nap_p01_2_tstkt11.txt', r)
        self.assertEqual(r.returncode, 0,
                         'TSTKT11Test failed:\n' + r.stdout[-3000:] + r.stderr[-1000:])

        import xml.etree.ElementTree as ET
        xml_path = (
            SMARTDEVICES / 'bluecash-app' / 'build' / 'test-results'
            / 'testProdDebugUnitTest'
            / 'TEST-com.beeloy.fiscal.bluecash.TSTKT11Test.xml'
        )
        self.assertTrue(xml_path.exists(), f'JUnit XML not found: {xml_path}')
        tree = ET.parse(xml_path)
        tests = int(tree.getroot().get('tests', 0))
        failures = int(tree.getroot().get('failures', 0))
        self.assertGreater(tests, 0, 'No tests found in TSTKT11Test')
        self.assertEqual(failures, 0, f'{failures} failures in TSTKT11Test')


if __name__ == '__main__':
    unittest.main()
