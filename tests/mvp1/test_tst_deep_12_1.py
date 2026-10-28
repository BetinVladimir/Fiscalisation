"""TST-DEEP-12.1 — Frame parser fuzz: any byte input → frame or typed error, no panic.

Native deliverable: SmartDevices/bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/FrameFuzzTest.kt
Note: IoT/protocol-abstraction/tests/FrameFuzzTest.cpp is BLOCKED_SCOPE (IoT/ is off-limits
for this agent); Kotlin fuzz coverage provided here instead.
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
KOTLIN_TEST = (ROOT / "SmartDevices" / "bluecash-app" / "src" / "test" / "kotlin" /
               "com" / "beeloy" / "fiscal" / "bluecash" / "FrameFuzzTest.kt")


class TestTstDeep12_1(unittest.TestCase):

    def test_frame_fuzz_kotlin_native(self):
        """10 Kotlin fuzz tests: 10k random inputs decoded without unchecked exceptions."""
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
        skipped = int(suite.get("skipped", 0))
        self.assertGreaterEqual(total, 8, f"Need >= 8 fuzz tests; got {total}")
        self.assertEqual(failures + errors + skipped, 0,
            f"Expected 0 failures/errors/skipped; got failures={failures} errors={errors} skipped={skipped}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_12_1"
            out_dir.mkdir(parents=True, exist_ok=True)
            import shutil
            shutil.copy2(TEST_XML, out_dir / TEST_XML.name)

    def test_frame_fuzz_test_file_exists(self):
        """Kotlin fuzz test file must exist with required content."""
        self.assertTrue(KOTLIN_TEST.exists(), f"Missing: {KOTLIN_TEST}")
        content = KOTLIN_TEST.read_text()
        for term in ("FrameFuzzTest", "IllegalArgumentException", "decode", "unchecked"):
            self.assertIn(term, content, f"Missing term {term!r}")


if __name__ == "__main__":
    unittest.main()
