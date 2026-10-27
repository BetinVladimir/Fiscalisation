"""BUG-026.3 — acceptance: saleResultCarriesRealDocumentNumber + emptyCloseResponseIsNotSuccess."""
import os
import subprocess
import time
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

REQUIRED_TESTS = (
    'saleResultCarriesRealDocumentNumber',
    'emptyCloseResponseIsNotSuccess',
)


def _junit_cases(cwd, started):
    found = {}
    for pattern in (
        'bluecash-app/build/test-results/**/TEST-*.xml',
        'build/test-results/**/TEST-*.xml',
    ):
        for xml_file in cwd.glob(pattern):
            if xml_file.stat().st_mtime_ns < started:
                continue
            try:
                root = ET.parse(xml_file).getroot()
            except (ET.ParseError, OSError):
                continue
            for tc in root.iter('testcase'):
                found[tc.get('name', '')] = tc
    return found


class TestBUG0263(unittest.TestCase):
    def test_acceptance(self):
        started = time.time_ns()
        result = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--rerun-tasks', '--no-daemon'],
            cwd=SMARTDEVICES,
            capture_output=True,
            text=True,
            timeout=300,
        )
        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'native-output'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'bug026_3.stdout.log').write_text(result.stdout)
            (out_dir / 'bug026_3.stderr.log').write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            'Gradle rc=' + str(result.returncode) + '\n' +
            result.stdout[-4000:] + '\n' + result.stderr[-2000:],
        )
        cases = _junit_cases(SMARTDEVICES, started)
        self.assertGreater(len(cases), 0, 'No JUnit XML after Gradle run')

        for test_name in REQUIRED_TESTS:
            self.assertIn(
                test_name, cases,
                f'Required test not in JUnit XML: {test_name!r}; found: {sorted(cases)!r}',
            )
            failures = cases[test_name].findall('failure') + cases[test_name].findall('error')
            self.assertEqual(
                len(failures), 0,
                f'{test_name!r} has failures: ' + ''.join(f.text or '' for f in failures),
            )

    def test_no_synthetic_reference_grep(self):
        """Invariant 6: no synthetic DATECS- prefix in production code."""
        engine = ROOT / 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/BlueCashEngine.kt'
        self.assertNotIn(
            '"DATECS-"',
            engine.read_text(encoding='utf-8'),
            'Synthetic document reference "DATECS-" found in BlueCashEngine.kt',
        )


if __name__ == '__main__':
    unittest.main()
