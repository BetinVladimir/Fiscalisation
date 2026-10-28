"""TST-CT-04.1 — FiscalIntentPayload schema: required fields, additionalProperties: false.

Acceptance: ../POS/docs/contracts/fiscal-intent.v1.json has required fields
(clientOperationId, registerId, shiftRef, documentType, payloadSha256, checkId, payload);
additionalProperties: false; documentType enum covers SALE/REVERSAL/FISCAL_SALE/CANCEL_SALE;
valid intent passes schema; invalid intents (missing field, extra field, wrong enum) fail.
"""
import json
import os
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCHEMA_PATH = ROOT.parent / "POS" / "docs" / "contracts" / "fiscal-intent.v1.json"

REQUIRED_FIELDS = {
    "clientOperationId", "registerId", "shiftRef",
    "documentType", "payloadSha256", "checkId", "payload",
}
EXPECTED_DOCUMENT_TYPES = {"SALE", "REVERSAL", "FISCAL_SALE", "CANCEL_SALE", "COMPLIANCE_EXPORT"}

# --- Minimal valid intent from golden receipt (1367¢ = 1200+167, CASH 500/CARD 867) ---
VALID_INTENT = {
    "clientOperationId": "00000000-0000-4000-8000-000000000001",
    "registerId": "reg-01",
    "shiftRef": "shift-2026-01",
    "documentType": "SALE",
    "payloadSha256": "a" * 64,
    "checkId": "00000000-0000-4000-8000-000000000002",
    "payload": {"receiptSessionId": "00000000-0000-4000-8000-000000000003"},
}


def _try_validate(schema: dict, instance: dict) -> tuple[bool, str]:
    """Minimal JSON-schema Draft 2020-12 validator for this schema (no external deps)."""
    required = set(schema.get("required", []))
    missing = required - set(instance.keys())
    if missing:
        return False, f"missing required: {sorted(missing)}"
    if schema.get("additionalProperties") is False:
        allowed = set(schema.get("properties", {}).keys())
        extra = set(instance.keys()) - allowed
        if extra:
            return False, f"additionalProperties: {sorted(extra)}"
    props = schema.get("properties", {})
    for key, val in instance.items():
        if key not in props:
            continue
        prop_schema = props[key]
        if "enum" in prop_schema and val not in prop_schema["enum"]:
            return False, f"{key}: {val!r} not in enum {prop_schema['enum']}"
        if "pattern" in prop_schema and prop_schema["type"] == "string":
            import re
            if not re.fullmatch(prop_schema["pattern"], str(val)):
                return False, f"{key}: {val!r} does not match pattern {prop_schema['pattern']}"
        if "type" in prop_schema:
            expected_type = prop_schema["type"]
            type_map = {"string": str, "object": dict, "array": list,
                        "integer": int, "number": (int, float), "boolean": bool}
            if expected_type in type_map and not isinstance(val, type_map[expected_type]):
                return False, f"{key}: expected {expected_type}, got {type(val).__name__}"
    return True, ""


class TestTSTCT04_1(unittest.TestCase):
    def test_schema_file_exists(self):
        self.assertTrue(
            SCHEMA_PATH.exists(),
            f"fiscal-intent.v1.json not found at {SCHEMA_PATH}"
        )

    def test_schema_is_valid_json(self):
        self.assertTrue(SCHEMA_PATH.exists())
        data = json.loads(SCHEMA_PATH.read_text())
        self.assertIsInstance(data, dict)

    def test_schema_has_required_fields(self):
        """Schema must require all 7 canonical fields."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        required = set(schema.get("required", []))
        missing = REQUIRED_FIELDS - required
        self.assertEqual(
            missing, set(),
            f"schema 'required' missing fields: {sorted(missing)}"
        )

    def test_schema_additional_properties_false(self):
        """additionalProperties: false must be set to reject unknown fields."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        self.assertFalse(
            schema.get("additionalProperties", True),
            "schema must have additionalProperties: false"
        )

    def test_schema_document_type_enum(self):
        """documentType must include SALE, REVERSAL, FISCAL_SALE, CANCEL_SALE."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        doc_type_prop = schema.get("properties", {}).get("documentType", {})
        enum_vals = set(doc_type_prop.get("enum", []))
        for expected in ("SALE", "REVERSAL", "FISCAL_SALE", "CANCEL_SALE"):
            self.assertIn(
                expected, enum_vals,
                f"documentType enum missing {expected!r}: {sorted(enum_vals)}"
            )

    def test_valid_intent_accepted(self):
        """A well-formed intent with all required fields must pass validation."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        ok, reason = _try_validate(schema, VALID_INTENT)
        self.assertTrue(ok, f"valid intent rejected: {reason}")

    def test_missing_required_field_rejected(self):
        """Intent missing clientOperationId must be rejected."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        bad = dict(VALID_INTENT)
        del bad["clientOperationId"]
        ok, _ = _try_validate(schema, bad)
        self.assertFalse(ok, "missing clientOperationId must be rejected")

    def test_extra_field_rejected(self):
        """Intent with unknown extra field must be rejected (additionalProperties: false)."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        bad = dict(VALID_INTENT)
        bad["unknown_field_xyz"] = "should_be_rejected"
        ok, _ = _try_validate(schema, bad)
        self.assertFalse(ok, "extra field must be rejected by additionalProperties: false")

    def test_invalid_document_type_rejected(self):
        """Intent with documentType='INVALID' must be rejected."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        bad = dict(VALID_INTENT)
        bad["documentType"] = "INVALID_TYPE"
        ok, _ = _try_validate(schema, bad)
        self.assertFalse(ok, "invalid documentType must be rejected")

    def test_wrong_sha256_pattern_rejected(self):
        """payloadSha256 not matching [0-9a-f]{64} must be rejected."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        bad = dict(VALID_INTENT)
        bad["payloadSha256"] = "UPPERCASE-SHA256-SHOULD-FAIL"
        ok, _ = _try_validate(schema, bad)
        self.assertFalse(ok, "uppercase/invalid sha256 must be rejected")

    def test_missing_shift_ref_rejected(self):
        """Intent missing shiftRef must be rejected."""
        self.assertTrue(SCHEMA_PATH.exists())
        schema = json.loads(SCHEMA_PATH.read_text())
        bad = dict(VALID_INTENT)
        del bad["shiftRef"]
        ok, _ = _try_validate(schema, bad)
        self.assertFalse(ok, "missing shiftRef must be rejected")


if __name__ == "__main__":
    unittest.main(verbosity=2)
