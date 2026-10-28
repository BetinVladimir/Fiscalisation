"""TST-DEEP-06.1 — Android journal crash recovery: ACCEPTED→EXECUTING, kill, restart → RECOVERY_REQUIRED.

Native deliverable: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/JournalCrashRecoveryTest.kt
"""
import os
import subprocess
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SMARTDEVICES = ROOT / "SmartDevices"
GRADLEW = SMARTDEVICES / "gradlew"
TEST_XML = (SMARTDEVICES / "bluecash-app" / "build" / "test-results" /
            "testProdDebugUnitTest" / "TEST-com.beeloy.fiscal.bluecash.JournalCrashRecoveryTest.xml")
KOTLIN_TEST = (ROOT / "SmartDevices" / "bluecash-app" / "src" / "test" / "kotlin" /
               "com" / "beeloy" / "fiscal" / "bluecash" / "JournalCrashRecoveryTest.kt")


class TestTstDeep06_1(unittest.TestCase):

    def test_journal_crash_recovery_native(self):
        """6 Kotlin tests: crash simulation returns RECOVERY_REQUIRED; no re-execution; no auto refund."""
        result = subprocess.run(
            [str(GRADLEW), ":bluecash-app:testProdDebugUnitTest", "--no-daemon",
             "--tests", "com.beeloy.fiscal.bluecash.JournalCrashRecoveryTest"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"gradlew exited {result.returncode}\n{result.stdout[-3000:]}\n{result.stderr[-2000:]}",
        )
        self.assertTrue(TEST_XML.exists(), f"JUnit XML not found: {TEST_XML}")
        tree = ET.parse(TEST_XML)
        suite = tree.getroot()
        total = int(suite.get("tests", 0))
        failures = int(suite.get("failures", 0))
        errors = int(suite.get("errors", 0))
        skipped = int(suite.get("skipped", 0))
        self.assertGreaterEqual(total, 5, f"Need >= 5 tests; got {total}")
        self.assertEqual(failures + errors + skipped, 0,
            f"Expected 0 failures/errors/skipped; got failures={failures} errors={errors} skipped={skipped}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_06_1"
            out_dir.mkdir(parents=True, exist_ok=True)
            import shutil
            shutil.copy2(TEST_XML, out_dir / TEST_XML.name)

    def test_kotlin_test_file_exists_with_required_content(self):
        """Test file must reference RECOVERY_REQUIRED, EXECUTING, and no re-execution assertions."""
        self.assertTrue(KOTLIN_TEST.exists(), f"Missing: {KOTLIN_TEST}")
        content = KOTLIN_TEST.read_text()
        for term in ("RECOVERY_REQUIRED", "EXECUTING", "ACCEPTED", "JournalCrashRecoveryTest"):
            self.assertIn(term, content, f"Missing term {term} in test file")


if __name__ == "__main__":
    unittest.main()
