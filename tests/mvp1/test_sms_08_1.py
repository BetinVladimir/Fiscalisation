"""SMS-08.1 — SmartFiscalHttpClient: CONFIG_MISSING, ok, Rejected(1008), NotSent, Unknown."""
import os
import subprocess
import time
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

REQUIRED_TESTS = (
    "baseUrl missing returns CONFIG_MISSING",
    "ok true response returns Ok",
    "ok false code 1008 returns Rejected 1008",
    "connection refused before write returns NotSent",
    "disconnect after body write returns Unknown",
)


def _find_junit_results(cwd, started):
    found = {}
    for pattern in (
        "daisy-smart-app/build/test-results/**/TEST-*.xml",
        "build/test-results/**/TEST-*.xml",
    ):
        for xml_file in cwd.glob(pattern):
            if xml_file.stat().st_mtime_ns < started:
                continue
            try:
                root = ET.parse(xml_file).getroot()
            except (ET.ParseError, OSError):
                continue
            for tc in root.iter("testcase"):
                name = tc.get("name", "")
                found[name] = tc
    return found


class TestSMS08_1(unittest.TestCase):
    def test_smart_fiscal_http_client(self):
        started = time.time_ns()
        result = subprocess.run(
            ["./gradlew", ":daisy-smart-app:testProdDebugUnitTest",
             "--no-daemon", "--rerun-tasks",
             "--tests", "com.beeloy.fiscal.daisy.SmartFiscalHttpClientTest"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"Gradle rc={result.returncode}\n{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )
        cases = _find_junit_results(SMARTDEVICES, started)
        self.assertGreater(len(cases), 0, "No JUnit XML test cases found")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-08-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)

        for test_name in REQUIRED_TESTS:
            self.assertIn(
                test_name, cases,
                f"Required test not found: {test_name!r}; found: {sorted(cases)!r}",
            )
            tc = cases[test_name]
            failures = tc.findall("failure") + tc.findall("error")
            self.assertEqual(
                len(failures), 0,
                f"Test {test_name!r} failed: " + "".join(f.text or "" for f in failures),
            )


if __name__ == "__main__":
    unittest.main()
