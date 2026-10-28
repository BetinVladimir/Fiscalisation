"""TST-DEEP-12.2 — Frame parser fuzz regression: no panics or unchecked exceptions.

Native deliverable: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/FrameFuzzTest.kt
Note: IoT/protocol-abstraction/tests/FrameFuzzTest.cpp coverage is BLOCKED_SCOPE.
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
            "testProdDebugUnitTest" / "TEST-com.beeloy.fiscal.bluecash.FrameFuzzTest.xml")


class TestTstDeep12_2(unittest.TestCase):

    def test_frame_fuzz_green(self):
        """All Kotlin frame fuzz tests pass; invariant 10 holds."""
        result = subprocess.run(
            [str(GRADLEW), ":bluecash-app:testProdDebugUnitTest", "--no-daemon",
             "--tests", "com.beeloy.fiscal.bluecash.FrameFuzzTest"],
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
        self.assertGreaterEqual(total, 8, f"Need >= 8 fuzz tests; got {total}")
        self.assertEqual(failures + errors, 0, f"Expected 0 failures/errors")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_12_2"
            out_dir.mkdir(parents=True, exist_ok=True)
            import shutil
            shutil.copy2(TEST_XML, out_dir / TEST_XML.name)


if __name__ == "__main__":
    unittest.main()
