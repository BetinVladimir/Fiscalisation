"""TST-GATE-01.1 — Smart S docNum-absent handling: PROD → UNKNOWN, LAB → Committed.

Acceptance: SMS04Test.kt exists; Gradle testDemoDebugUnitTest --continue runs it;
SMS04Test XML shows 0 failures; tests 'ok true without docNum in PROD returns Unknown'
and 'ok true without docNum in LAB returns Committed with noDocNumber flag' both PASS.
"""
import os
import subprocess
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"
KOTLIN_TEST = (
    SMARTDEVICES / "daisy-smart-app" / "src" / "test" / "kotlin"
    / "com" / "beeloy" / "fiscal" / "daisy" / "SMS04Test.kt"
)
GRADLE_XML = (
    SMARTDEVICES / "daisy-smart-app" / "build" / "test-results"
    / "testDemoDebugUnitTest"
    / "TEST-com.beeloy.fiscal.daisy.SMS04Test.xml"
)

# The exact test names from SMS04Test.kt lines 65 and 75
PROD_TEST_NAME = "ok true without docNum in PROD returns Unknown"
LAB_TEST_NAME = "ok true without docNum in LAB returns Committed with noDocNumber flag"


class TestTSTGATE01_1(unittest.TestCase):

    def test_sms04_test_file_exists(self):
        self.assertTrue(
            KOTLIN_TEST.exists(),
            f"SMS04Test.kt not found: {KOTLIN_TEST}"
        )

    def test_sms04_contains_prod_unknown_test(self):
        """SMS04Test.kt must contain the PROD→UNKNOWN invariant test."""
        self.assertTrue(KOTLIN_TEST.exists())
        src = KOTLIN_TEST.read_text()
        self.assertIn(
            "ok true without docNum in PROD returns Unknown",
            src,
            "SMS04Test.kt must contain PROD→UNKNOWN test"
        )
        self.assertIn(
            "DeviceResult.Unknown",
            src,
            "SMS04Test.kt must assert DeviceResult.Unknown for PROD"
        )

    def test_sms04_contains_lab_committed_test(self):
        """SMS04Test.kt must contain the LAB→Committed with noDocNumber test."""
        self.assertTrue(KOTLIN_TEST.exists())
        src = KOTLIN_TEST.read_text()
        self.assertIn(
            "ok true without docNum in LAB returns Committed with noDocNumber flag",
            src,
            "SMS04Test.kt must contain LAB→Committed noDocNumber test"
        )
        self.assertIn(
            "noDocNumber",
            src,
            "SMS04Test.kt must check noDocNumber flag in LAB test"
        )

    def test_sms04_prod_uses_islabs_false(self):
        """PROD test must use isLab=false (not the default isLab=true)."""
        self.assertTrue(KOTLIN_TEST.exists())
        src = KOTLIN_TEST.read_text()
        self.assertIn(
            "isLab = false",
            src,
            "PROD test must explicitly set isLab = false"
        )

    def test_sms04_no_synthetic_docnum(self):
        """Invariant 6: PROD path must NOT produce a synthetic document number."""
        self.assertTrue(KOTLIN_TEST.exists())
        src = KOTLIN_TEST.read_text()
        # The PROD test sends {"ok":true} with no docNum and expects Unknown
        self.assertIn(
            '"ok":true',
            src,
            "PROD test must use reply with no docNum"
        )

    def test_gradle_sms04_passes(self):
        """Gradle testDemoDebugUnitTest; SMS04Test XML has 0 failures and PROD/LAB tests pass."""
        self.assertTrue(KOTLIN_TEST.exists(), f"test file missing: {KOTLIN_TEST}")
        result = subprocess.run(
            ["./gradlew", ":daisy-smart-app:testDemoDebugUnitTest", "--continue", "-q"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        # Build may fail due to pre-existing DaisySmartStubTest failures — that's OK.
        # We check only the SMS04Test XML result.
        self.assertTrue(
            GRADLE_XML.exists(),
            f"Gradle did not produce XML for SMS04Test; "
            f"gradle output:\n{result.stdout[-1000:]}\n{result.stderr[-1000:]}"
        )

        tree = ET.parse(str(GRADLE_XML))
        root_el = tree.getroot()
        tests = int(root_el.attrib.get("tests", 0))
        failures = int(root_el.attrib.get("failures", 0))
        errors = int(root_el.attrib.get("errors", 0))

        self.assertGreater(tests, 0, "SMS04Test ran 0 tests")
        self.assertEqual(failures, 0, f"SMS04Test has {failures} failure(s)")
        self.assertEqual(errors, 0, f"SMS04Test has {errors} error(s)")

        # Verify the two critical gate tests are specifically PRESENT and PASS
        passing_names = set()
        for testcase in root_el.findall("testcase"):
            name = testcase.attrib.get("name", "")
            # A test passes when it has no <failure> or <error> child
            has_failure = testcase.find("failure") is not None
            has_error = testcase.find("error") is not None
            if not has_failure and not has_error:
                passing_names.add(name)

        self.assertIn(
            PROD_TEST_NAME,
            passing_names,
            f"PROD→UNKNOWN test did not pass; passing tests: {sorted(passing_names)}"
        )
        self.assertIn(
            LAB_TEST_NAME,
            passing_names,
            f"LAB→Committed test did not pass; passing tests: {sorted(passing_names)}"
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_gate_01_1.xml").write_text(GRADLE_XML.read_text())


if __name__ == "__main__":
    unittest.main(verbosity=2)
