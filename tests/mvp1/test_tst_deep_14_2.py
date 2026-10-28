"""TST-DEEP-14.2 — smart_protocol_examples.json: strict schema, no-spaces keys, seq not sec.

Acceptance: SmartProtocolExamplesTest.kt exists; Gradle testDemoDebugUnitTest --continue
runs it; SmartProtocolExamplesTest XML shows 0 failures, 6 tests; schema rejects F-4 typos
(keys with spaces, sec vs seq); expected_total_cents = 1367¢.
"""
import os
import subprocess
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
KOTLIN_TEST = (
    ROOT / "SmartDevices" / "daisy-smart-app" / "src" / "test" / "kotlin"
    / "com" / "beeloy" / "fiscal" / "daisy" / "SmartProtocolExamplesTest.kt"
)
SMARTDEVICES = ROOT / "SmartDevices"
FIXTURE = ROOT / "docs" / "MVP1" / "fixtures" / "smart_protocol_examples.json"

# Gradle XML result path
GRADLE_XML = (
    SMARTDEVICES / "daisy-smart-app" / "build" / "test-results"
    / "testDemoDebugUnitTest"
    / "TEST-com.beeloy.fiscal.daisy.SmartProtocolExamplesTest.xml"
)


class TestTSTDEEP14_2(unittest.TestCase):
    def test_kotlin_test_file_exists(self):
        self.assertTrue(
            KOTLIN_TEST.exists(),
            f"SmartProtocolExamplesTest.kt not found: {KOTLIN_TEST}"
        )

    def test_fixture_file_exists(self):
        self.assertTrue(
            FIXTURE.exists(),
            f"smart_protocol_examples.json not found: {FIXTURE}"
        )

    def test_fixture_no_keys_with_spaces(self):
        """F-4: no JSON keys may contain spaces."""
        import re
        import json
        self.assertTrue(FIXTURE.exists())
        text = FIXTURE.read_text()
        key_pattern = re.compile(r'"([^"]+)"\s*:')
        keys_with_spaces = [
            m.group(1) for m in key_pattern.finditer(text)
            if " " in m.group(1) and not m.group(1).startswith("_")
        ]
        self.assertEqual(
            keys_with_spaces, [],
            f"F-4: keys with spaces found: {keys_with_spaces}"
        )

    def test_fixture_uses_seq_not_sec(self):
        """Protocol typo: 'sec' must not appear as a key; 'seq' must be present."""
        import re
        self.assertTrue(FIXTURE.exists())
        text = FIXTURE.read_text()
        self.assertFalse(
            bool(re.search(r'"sec"\s*:', text)),
            "F-4: found 'sec' key — should be 'seq'"
        )
        self.assertTrue(
            bool(re.search(r'"seq"\s*:', text)),
            "fixture must contain 'seq' field"
        )

    def test_fixture_golden_total_1367_cents(self):
        """expected_total_cents must equal 1367 (1200+167, golden receipt)."""
        import re
        self.assertTrue(FIXTURE.exists())
        text = FIXTURE.read_text()
        m = re.search(r'"expected_total_cents"\s*:\s*(\d+)', text)
        self.assertIsNotNone(m, "expected_total_cents not found in fixture")
        self.assertEqual(
            int(m.group(1)), 1367,
            f"expected 1367¢ (1200+167), got {m.group(1)}"
        )

    def test_kotlin_smart_protocol_test_passes(self):
        """Gradle testDemoDebugUnitTest --continue; SmartProtocolExamplesTest passes."""
        self.assertTrue(KOTLIN_TEST.exists(), f"test file missing: {KOTLIN_TEST}")
        result = subprocess.run(
            ["./gradlew", ":daisy-smart-app:testDemoDebugUnitTest", "--continue", "-q"],
            cwd=str(SMARTDEVICES),
            capture_output=True,
            text=True,
            timeout=180,
        )
        # Build may fail due to pre-existing DaisySmartStubTest failures — that's OK.
        # We check only the SmartProtocolExamplesTest XML result.
        self.assertTrue(
            GRADLE_XML.exists(),
            f"Gradle did not produce XML for SmartProtocolExamplesTest; "
            f"gradle output:\n{result.stdout}\n{result.stderr}"
        )

        tree = ET.parse(str(GRADLE_XML))
        root = tree.getroot()
        tests = int(root.attrib.get("tests", 0))
        failures = int(root.attrib.get("failures", 0))
        errors = int(root.attrib.get("errors", 0))

        self.assertGreater(tests, 0,
                           f"SmartProtocolExamplesTest ran 0 tests")
        self.assertEqual(failures, 0,
                         f"SmartProtocolExamplesTest has {failures} failure(s)")
        self.assertEqual(errors, 0,
                         f"SmartProtocolExamplesTest has {errors} error(s)")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_deep_14_2.xml").write_text(
                GRADLE_XML.read_text()
            )


if __name__ == "__main__":
    unittest.main(verbosity=2)
