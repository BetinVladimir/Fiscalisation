"""MQ-03.1 — DeviceResult type: UNKNOWN must not serialise as FAILED."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"
SCHEMA_PATH = ROOT / "docs" / "MVP1" / "contracts" / "device_result.schema.json"

REQUIRED_TESTS = (
    "TestDeviceResultUnknownNotFailed",
    "TestDeviceResultStatusValues",
    "TestDeviceResultCommittedRoundTrip",
    "TestDeviceResultNotCommittedRoundTrip",
)

REQUIRED_SCHEMA_FIELDS = ("status", "COMMITTED", "NOT_COMMITTED", "UNKNOWN")


class TestMQ031DeviceResult(unittest.TestCase):

    def test_go_suite_passes(self):
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR", tempfile.mkdtemp())
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "DeviceResult",
             "-count=1", "-v"],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        evidence_path = os.path.join(artifact_dir, "mq_03_1_go_output.txt")
        try:
            with open(evidence_path, "w") as f:
                f.write(result.stdout)
                f.write(result.stderr)
        except OSError:
            pass

        self.assertEqual(
            result.returncode, 0,
            "go test rc=" + str(result.returncode) + "\n" +
            result.stdout[-4000:] + "\n" + result.stderr[-2000:],
        )
        pass_count = result.stdout.count("--- PASS")
        self.assertGreater(pass_count, 0, "No passing tests found in output")
        for name in REQUIRED_TESTS:
            self.assertIn(name, result.stdout, f"Required test not found: {name!r}")

    def test_unknown_not_failed_in_output(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run",
             "TestDeviceResultUnknownNotFailed", "-count=1", "-v"],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            "go test rc=" + str(result.returncode) + "\n" + result.stdout[-2000:],
        )
        self.assertIn("TestDeviceResultUnknownNotFailed", result.stdout)
        self.assertIn("--- PASS", result.stdout)

    def test_schema_file_structure(self):
        self.assertTrue(SCHEMA_PATH.exists(), f"Schema file not found: {SCHEMA_PATH}")
        with open(SCHEMA_PATH) as f:
            schema = json.load(f)
        self.assertIn("properties", schema, "Schema missing 'properties'")
        self.assertIn("status", schema["properties"], "Schema missing 'status' property")
        enum_values = schema["properties"]["status"].get("enum", [])
        for expected in ("COMMITTED", "NOT_COMMITTED", "UNKNOWN"):
            self.assertIn(expected, enum_values,
                          f"Schema enum missing {expected!r}")
        # FAILED must not appear in the enum
        self.assertNotIn("FAILED", enum_values,
                         "FAILED must not be a valid status in schema")

    def test_source_file_exists(self):
        src = BACKEND / "internal" / "domain" / "device_result.go"
        self.assertTrue(src.exists(), f"Source file not found: {src}")
        content = src.read_text()
        self.assertIn("DeviceResult", content)
        self.assertIn("UNKNOWN", content)
        # FAILED must not appear as a status constant value (may appear in comments)
        self.assertNotIn('"FAILED"', content,
                         'FAILED must not be a status constant in device_result.go')


if __name__ == "__main__":
    unittest.main()
