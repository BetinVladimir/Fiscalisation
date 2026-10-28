"""SMS-02.3 — bluecash-app depends on :shared-runtime; characteristic tests SMS-02.1 still green.

Acceptance:
  - bluecash-app/build.gradle.kts contains implementation(project(":shared-runtime"))
  - Full bluecash-app test suite exits 0 (SMS-02.1 characteristic tests included)
"""
import os
import subprocess
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"


class TestSMS02_3(unittest.TestCase):
    def test_bluecash_build_gradle_references_shared_runtime(self):
        """bluecash-app/build.gradle.kts must declare implementation(project(':shared-runtime'))."""
        build_file = SMARTDEVICES / "bluecash-app/build.gradle.kts"
        self.assertTrue(build_file.exists(), f"bluecash-app/build.gradle.kts not found at {build_file}")
        content = build_file.read_text()
        self.assertIn(
            ":shared-runtime", content,
            "bluecash-app/build.gradle.kts must reference :shared-runtime",
        )

    def test_characteristic_tests_still_pass(self):
        """SMS-02.1 characteristic tests must still pass after wiring shared-runtime."""
        result = subprocess.run(
            [
                "./gradlew",
                ":bluecash-app:testProdDebugUnitTest",
                "--tests", "*CommandProcessorCharacteristic*",
                "--no-daemon",
            ],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-3-char"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Characteristic tests failed: rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

        xml_dir = SMARTDEVICES / "bluecash-app/build/test-results/testProdDebugUnitTest"
        test_count = 0
        for xml_file in xml_dir.glob("*.xml"):
            try:
                tree = ET.parse(str(xml_file))
                suite = tree.getroot()
                if "CommandProcessorCharacteristic" in suite.get("name", ""):
                    test_count += int(suite.get("tests", "0"))
            except Exception:
                pass
        self.assertGreater(test_count, 0,
                           "CommandProcessorCharacteristicTest must have at least 1 test")

    def test_full_bluecash_suite_green(self):
        """Full bluecash-app test suite must pass (no regressions from shared-runtime wiring)."""
        result = subprocess.run(
            ["./gradlew", ":bluecash-app:testProdDebugUnitTest", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-3-full"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Full suite rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )


if __name__ == "__main__":
    unittest.main()
