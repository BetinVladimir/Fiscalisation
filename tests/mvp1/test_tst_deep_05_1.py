"""TST-DEEP-05.1 — Datecs golden frames: 15 frames from PM v2.11.4 (cmd 48,49,53,56,60,74,76).

Native deliverable: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsGoldenFramesTest.kt
Run: gradlew :bluecash-app:testProdDebugUnitTest --tests DatecsGoldenFramesTest
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
            "testProdDebugUnitTest" / "TEST-com.beeloy.fiscal.bluecash.DatecsGoldenFramesTest.xml")
KOTLIN_TEST = (ROOT / "SmartDevices" / "bluecash-app" / "src" / "test" / "kotlin" /
               "com" / "beeloy" / "fiscal" / "bluecash" / "DatecsGoldenFramesTest.kt")


def _run_gradlew():
    return subprocess.run(
        [str(GRADLEW), ":bluecash-app:testProdDebugUnitTest", "--no-daemon",
         "--tests", "com.beeloy.fiscal.bluecash.DatecsGoldenFramesTest"],
        cwd=str(SMARTDEVICES),
        capture_output=True,
        text=True,
        timeout=300,
    )


class TestTstDeep05_1(unittest.TestCase):

    def test_datecs_golden_frames_native(self):
        """Kotlin test suite: 18 Datecs frame tests pass; verifies golden frames from PM v2.11.4."""
        result = _run_gradlew()
        self.assertEqual(
            result.returncode, 0,
            f"gradlew exited {result.returncode}\n{result.stdout[-3000:]}\n{result.stderr[-2000:]}",
        )
        # Verify that the test XML was generated and contains passing tests.
        self.assertTrue(TEST_XML.exists(), f"JUnit XML not found: {TEST_XML}")
        tree = ET.parse(TEST_XML)
        suite = tree.getroot()
        total = int(suite.get("tests", 0))
        failures = int(suite.get("failures", 0))
        errors = int(suite.get("errors", 0))
        skipped = int(suite.get("skipped", 0))
        self.assertGreaterEqual(total, 10,
            f"Need >= 10 test cases; got {total} (XML: {TEST_XML})")
        self.assertEqual(failures + errors + skipped, 0,
            f"Expected 0 failures/errors/skipped; got failures={failures} errors={errors} skipped={skipped}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_05_1"
            out_dir.mkdir(parents=True, exist_ok=True)
            import shutil
            shutil.copy2(TEST_XML, out_dir / TEST_XML.name)

    def test_datecs_golden_frames_test_file_exists(self):
        """Test file must be present at the required path with required content."""
        self.assertTrue(KOTLIN_TEST.exists(), f"Missing: {KOTLIN_TEST}")
        content = KOTLIN_TEST.read_text()
        for required in ("DatecsGoldenFramesTest", "encode", "decode", "CP1251", "0x7f"):
            self.assertIn(required, content, f"Test file must reference {required}")


if __name__ == "__main__":
    unittest.main()
