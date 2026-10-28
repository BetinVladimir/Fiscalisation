"""TG-014.2 — scripts/hil/run.sh --dry-run creates artifact dir;
              result.json validates hil-result.schema.json."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


def _validate_against_schema(instance: dict, schema: dict) -> list:
    """Minimal JSON Schema validation for required fields and enum values."""
    errors = []
    required = schema.get("required", [])
    props = schema.get("properties", {})
    for field in required:
        if field not in instance:
            errors.append(f"missing required field {field!r}")
    for field, prop_schema in props.items():
        if field not in instance:
            continue
        val = instance[field]
        if "enum" in prop_schema and val not in prop_schema["enum"]:
            errors.append(f"field {field!r}={val!r} not in enum {prop_schema['enum']}")
        if "type" in prop_schema:
            expected_type = prop_schema["type"]
            type_map = {"string": str, "boolean": bool, "array": list, "object": dict}
            if expected_type in type_map and not isinstance(val, type_map[expected_type]):
                errors.append(f"field {field!r} expected {expected_type}, got {type(val).__name__}")
    return errors


class TestTG014HILDryRun(unittest.TestCase):
    def test_dry_run_creates_artifact_dir(self):
        script = ROOT / "scripts" / "hil" / "run.sh"
        self.assertTrue(script.exists(), f"scripts/hil/run.sh not found at {script}")

        result = subprocess.run(
            ["bash", str(script), "--dry-run", "HIL-BC-01"],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(
            result.returncode, 0,
            f"scripts/hil/run.sh --dry-run exited {result.returncode}\n"
            + result.stdout + "\n" + result.stderr,
        )

        # Find the artifact directory created
        artifact_base = ROOT / "artifacts" / "hil"
        self.assertTrue(artifact_base.exists(), f"artifacts/hil/ not created at {artifact_base}")

        # Find the result.json file
        result_files = list(artifact_base.rglob("HIL-BC-01.result.json"))
        self.assertGreater(len(result_files), 0,
                           "No HIL-BC-01.result.json found in artifacts/hil/")

        result_json = json.loads(result_files[0].read_text())
        self.assertEqual(result_json.get("scenario"), "HIL-BC-01")
        self.assertEqual(result_json.get("dry_run"), True)

    def test_result_json_validates_schema(self):
        schema_path = ROOT / "contracts" / "hil-result.schema.json"
        self.assertTrue(schema_path.exists(), "contracts/hil-result.schema.json not found")
        schema = json.loads(schema_path.read_text())

        script = ROOT / "scripts" / "hil" / "run.sh"
        subprocess.run(
            ["bash", str(script), "--dry-run", "HIL-SCHEMA-TEST"],
            cwd=str(ROOT), capture_output=True, timeout=30,
        )

        result_files = list((ROOT / "artifacts" / "hil").rglob("HIL-SCHEMA-TEST.result.json"))
        if not result_files:
            # Try HIL-BC-01 from previous test
            result_files = list((ROOT / "artifacts" / "hil").rglob("HIL-BC-01.result.json"))
        self.assertGreater(len(result_files), 0, "No result.json to validate")

        instance = json.loads(result_files[0].read_text())
        errors = _validate_against_schema(instance, schema)
        self.assertEqual(errors, [], f"Schema validation errors: {errors}")


if __name__ == "__main__":
    unittest.main()
