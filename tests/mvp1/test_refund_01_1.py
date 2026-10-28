"""REFUND-01.1 — durable refund recovery: storno-first, UNKNOWN≠FAILED, idempotency."""
import subprocess
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMART_DEVICES = ROOT / 'SmartDevices'

REQUIRED_TESTS = (
    'storno_unknown_no_card_refund_attempted',
    'in_flight_journal_returns_recovery_required',
    'refund_pending_is_idempotent',
    'reversed_is_idempotent',
    'multi_card_payments_each_get_refund_prepared_entry',
)


class TestREFUND01(unittest.TestCase):
    def test_refund_recovery_tests_pass(self):
        result = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.RefundRecoveryTest'],
            cwd=str(SMART_DEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'Gradle test run failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )

        xml_files = list(
            (SMART_DEVICES / 'bluecash-app' / 'build' / 'test-results').rglob(
                'TEST-com.beeloy.fiscal.bluecash.RefundRecoveryTest.xml'
            )
        )
        self.assertTrue(xml_files, 'JUnit XML not found for RefundRecoveryTest')

        tree = ET.parse(str(xml_files[0]))
        root = tree.getroot()
        suite = root if root.tag == 'testsuite' else root.find('testsuite')
        self.assertIsNotNone(suite, 'testsuite element not found')

        failures = int(suite.get('failures', 0)) + int(suite.get('errors', 0))
        self.assertEqual(failures, 0, f'{failures} test(s) failed in RefundRecoveryTest')

        found = {tc.get('name') for tc in suite.findall('testcase')}
        for name in REQUIRED_TESTS:
            self.assertIn(name, found, f'Required test not found: {name!r}')


if __name__ == '__main__':
    unittest.main()
