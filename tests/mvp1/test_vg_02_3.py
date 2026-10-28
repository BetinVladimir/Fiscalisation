"""VG-02.3 — acceptance: requireOk(expectedFields) rejects empty data, ErrorCode int."""
import os
import subprocess
import time
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'

REQUIRED_TESTS = (
    'OkRejectsEmptyData',
    'OkPassesOnZeroErrorCode',
    'OkRejectsNonZeroErrorCode',
    'OkRejectsTooFewFields',
)


def _find_junit_results(cwd, started):
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
                name = tc.get('name', '')
                found[name] = tc
    return found


class TestVG023(unittest.TestCase):
    def test_require_ok_rejects_empty_data(self):
        started = time.time_ns()
        result = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest',
             '--tests', 'com.beeloy.fiscal.bluecash.DatecsRequireOkTest',
             '--no-daemon', '--rerun-tasks'],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'Gradle rc=' + str(result.returncode) + '\n' +
            result.stdout[-4000:] + '\n' + result.stderr[-2000:],
        )
        cases = _find_junit_results(SMARTDEVICES, started)
        self.assertGreater(len(cases), 0, 'No JUnit XML test cases found after Gradle run')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'native-output'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'vg02_3.stdout.log').write_text(result.stdout)

        for test_name in REQUIRED_TESTS:
            self.assertIn(
                test_name, cases,
                f'Required test not found: {test_name!r}; found: {sorted(cases)!r}',
            )
            tc = cases[test_name]
            failures = tc.findall('failure') + tc.findall('error')
            self.assertEqual(
                len(failures), 0,
                f'Test {test_name!r} has failures: '
                + ''.join(f.text or '' for f in failures),
            )


if __name__ == '__main__':
    unittest.main()
