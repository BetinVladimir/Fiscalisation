"""TG-006.1-2 — DatecsResponsesTest: typed response parsers for Datecs commands.

Acceptance: ./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsResponses*' exits 0;
 - one test per command (48, 49, 53, 56, 43, 69/70, 38, 39, 42) + broken-response tests;
 - non-zero test count.
"""
import os
import subprocess
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / "SmartDevices"

REQUIRED_RESPONSE_TYPES = (
    "Cmd48Response",
    "Cmd49Response",
    "Cmd53Response",
    "Cmd56Response",
    "Cmd43Response",
    "Cmd69Response",
    "Cmd38Response",
    "Cmd39Response",
    "Cmd42Response",
)

REQUIRED_PARSERS = (
    "parseCmd48",
    "parseCmd49",
    "parseCmd53",
    "parseCmd56",
    "parseCmd43",
    "parseCmd69",
    "parseCmd38",
    "parseCmd39",
    "parseCmd42",
)


class TestTG006(unittest.TestCase):
    def test_datecs_protocol_has_response_types_and_parsers(self):
        """DatecsProtocol.kt must declare typed response classes and parser extensions."""
        proto = (
            SMARTDEVICES
            / "bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/DatecsProtocol.kt"
        )
        self.assertTrue(proto.exists(), f"DatecsProtocol.kt not found at {proto}")
        content = proto.read_text()
        for sym in REQUIRED_RESPONSE_TYPES:
            self.assertIn(sym, content, f"DatecsProtocol.kt missing type: {sym!r}")
        for sym in REQUIRED_PARSERS:
            self.assertIn(sym, content, f"DatecsProtocol.kt missing parser: {sym!r}")

    def test_datecs_responses_test_file_exists(self):
        """DatecsResponsesTest.kt must exist in the test source tree."""
        test_file = (
            SMARTDEVICES
            / "bluecash-app/src/test/kotlin/com/beeloy/fiscal/bluecash/DatecsResponsesTest.kt"
        )
        self.assertTrue(test_file.exists(), f"DatecsResponsesTest.kt not found at {test_file}")
        content = test_file.read_text()
        # Must cover each of the required commands with at least one @Test
        for cmd_suffix in ("48", "49", "53", "56", "43", "69", "70", "38", "39", "42"):
            self.assertIn(
                f"cmd{cmd_suffix}",
                content,
                f"DatecsResponsesTest.kt has no tests for cmd {cmd_suffix}",
            )
        # Must have broken-response (invariant 10) tests
        self.assertIn(
            "BrokenData",
            content,
            "DatecsResponsesTest.kt must contain broken-data tests (invariant 10)",
        )

    def test_datecs_responses_kotlin_tests_pass(self):
        """./gradlew :bluecash-app:testProdDebugUnitTest --tests '*DatecsResponses*' exits 0."""
        result = subprocess.run(
            [
                "./gradlew",
                ":bluecash-app:testProdDebugUnitTest",
                "--tests", "*DatecsResponses*",
                "--no-daemon",
            ],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tg-006"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Gradle returned rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )

        # Count tests from JUnit XML
        xml_dir = (
            SMARTDEVICES
            / "bluecash-app/build/test-results/testProdDebugUnitTest"
        )
        test_count = 0
        for xml_file in xml_dir.glob("*.xml"):
            try:
                tree = ET.parse(str(xml_file))
                suite = tree.getroot()
                if "DatecsResponses" in suite.get("name", ""):
                    test_count += int(suite.get("tests", "0"))
            except Exception:
                pass

        self.assertGreater(
            test_count, 0,
            f"DatecsResponsesTest must have at least 1 test; XML reports found in {xml_dir}",
        )

    def test_full_bluecash_suite_no_regressions(self):
        """Full bluecash-app test suite must still pass after adding TG-006 parsers."""
        result = subprocess.run(
            ["./gradlew", ":bluecash-app:testProdDebugUnitTest", "--no-daemon"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out = Path(artifact_dir) / "tg-006-regression"
            out.mkdir(parents=True, exist_ok=True)
            (out / "stdout.log").write_text(result.stdout)
            (out / "stderr.log").write_text(result.stderr)

        self.assertEqual(
            result.returncode, 0,
            f"Full bluecash-app suite regression: rc={result.returncode}\n"
            f"{result.stdout[-4000:]}\n{result.stderr[-2000:]}",
        )


if __name__ == "__main__":
    unittest.main()
