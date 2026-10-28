"""TST-DEEP-05.2 — Datecs golden frames regression: test stays green (no defect found).

Native deliverable: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsGoldenFramesTest.kt
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


class TestTstDeep05_2(unittest.TestCase):

    def test_datecs_golden_frames_green(self):
        """All 18 Datecs golden frame tests pass."""
        result = subprocess.run(
            [str(GRADLEW), ":bluecash-app:testProdDebugUnitTest", "--no-daemon",
             "--tests", "com.beeloy.fiscal.bluecash.DatecsGoldenFramesTest"],
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
        self.assertGreaterEqual(total, 10, f"Need >= 10 tests; got {total}")
        self.assertEqual(failures + errors, 0, f"Expected 0 failures/errors")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_05_2"
            out_dir.mkdir(parents=True, exist_ok=True)
            import shutil
            shutil.copy2(TEST_XML, out_dir / TEST_XML.name)

    def test_full_bluecash_suite_passes(self):
        """Regression: the full bluecash-app test suite still passes."""
        result = subprocess.run(
            [str(GRADLEW), ":bluecash-app:testProdDebugUnitTest", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            f"Full suite failed (rc={result.returncode})\n{result.stdout[-3000:]}\n{result.stderr[-2000:]}",
        )


if __name__ == "__main__":
    unittest.main()
