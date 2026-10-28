"""SMS-02.1 — Characteristic tests for BlueCashCommandProcessor before shared-runtime extraction.

Acceptance:
  - CommandProcessorCharacteristicTest.kt exists in bluecash-app/src/test/
  - Tests document: SALE 1200¢ cash → FISCALIZED; duplicate operationId → same Result, no second call
  - ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*CommandProcessorCharacteristic*' exits 0
  - Non-zero test count
"""
import os
import subprocess
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

TEST_FILE = (
    SMARTDEVICES
    / "bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash"
    / "CommandProcessorCharacteristicTest.kt"
)


class TestSMS02_1(unittest.TestCase):
    def test_characteristic_test_file_exists(self):
        """CommandProcessorCharacteristicTest.kt must exist in test source tree."""
        self.assertTrue(TEST_FILE.exists(), f"Test file not found: {TEST_FILE}")

    def test_characteristic_test_covers_1200_cash(self):
        """Test must document SALE 1200¢ cash → FISCALIZED behavior."""
        content = TEST_FILE.read_text()
        self.assertIn("1200", content, "Test must reference 1200¢ golden amount")
        self.assertIn("FISCALIZED", content, "Test must assert FISCALIZED state (invariant 4)")
        self.assertIn("12.00", content, "Test must use 1200¢ = 12.00 (invariant 1: cents only)")

    def test_characteristic_test_covers_idempotency(self):
        """Test must document duplicate operationId → same result without second I/O."""
        content = TEST_FILE.read_text()
        self.assertIn("duplicate", content.lower() + "dup",
                      "Test must cover duplicate operationId (invariant 4)")
        # Must assert that fiscal commands count does not increase on second call
        self.assertIn("commands.size", content,
                      "Test must verify no additional fiscal commands on replay")

    def test_characteristic_kotlin_tests_pass(self):
        """./gradlew :bluecash-app:testProdDebugUnitTest --tests '*CommandProcessorCharacteristic*' exits 0."""
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
            out = Path(artifact_dir) / "sms-02-1"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Gradle rc={result.returncode}\n{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

        # Verify non-zero test count from JUnit XML
        xml_dir = (
            SMARTDEVICES
            / "bluecash-app/build/test-results/testProdDebugUnitTest"
        )
        test_count = 0
        for xml_file in xml_dir.glob("*.xml"):
            try:
                tree = ET.parse(str(xml_file))
                suite = tree.getroot()
                if "CommandProcessorCharacteristic" in suite.get("name", ""):
                    test_count += int(suite.get("tests", "0"))
            except Exception:
                pass

        self.assertGreater(
            test_count, 0,
            "CommandProcessorCharacteristicTest must have at least 1 executed test",
        )

    def test_full_suite_no_regressions(self):
        """Full bluecash-app test suite must pass after adding SMS-02.1 tests."""
        result = subprocess.run(
            ["./gradlew", ":bluecash-app:testProdDebugUnitTest", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "sms-02-1-regression"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Full suite regression rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )


if __name__ == "__main__":
    unittest.main()
