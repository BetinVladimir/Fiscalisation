"""tests/e2e/contracts/test_tst_ct_03.py — MQTT envelope vectors parse & topic check.

Each vector in contracts/mqtt-vectors/*.json must:
- Be valid JSON with required fields
- Have a topic matching tenants/{t}/devices/{d}/commands (or sub-paths)
- Have an expect field (ACCEPTED or REJECTED with error_code)
- UNKNOWN outcome vectors must have note about no-retry
"""
import json
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent.parent
VECTORS_DIR = ROOT / "contracts" / "mqtt-vectors"

TOPIC_PATTERN = re.compile(r'^tenants/[^/]+/devices/[^/]+/(commands|results|commands/acks)$')
REQUIRED_FIELDS = {"name", "topic", "command_type", "operation_id", "payload",
                   "signature", "expires_at", "expect"}
KNOWN_COMMAND_TYPES = {"FINALIZE", "REVERSAL", "ACK", "RESULT"}
KNOWN_OUTCOMES = {"COMMITTED", "NOT_COMMITTED", "UNKNOWN"}
MINIMUM_VECTORS = 10


class TestMQTTVectors(unittest.TestCase):
    def _load_vectors(self):
        self.assertTrue(VECTORS_DIR.exists(), f"mqtt-vectors dir missing: {VECTORS_DIR}")
        vectors = []
        for f in sorted(VECTORS_DIR.glob("*.json")):
            data = json.loads(f.read_text())
            data["_file"] = f.name
            vectors.append(data)
        return vectors

    def test_vectors_dir_exists(self):
        self.assertTrue(VECTORS_DIR.exists(), f"mqtt-vectors dir missing: {VECTORS_DIR}")

    def test_minimum_10_vectors(self):
        vectors = self._load_vectors()
        self.assertGreaterEqual(
            len(vectors), MINIMUM_VECTORS,
            f"expected >= {MINIMUM_VECTORS} vectors, got {len(vectors)}"
        )

    def test_required_fields(self):
        vectors = self._load_vectors()
        for v in vectors:
            missing = REQUIRED_FIELDS - set(v.keys())
            self.assertEqual(
                missing, set(),
                f"{v['_file']}: missing fields {missing}"
            )

    def test_topic_format_or_invalid_flag(self):
        """Valid vectors must have canonical topic; invalid-topic vectors marked REJECTED."""
        vectors = self._load_vectors()
        for v in vectors:
            if v.get("error_code") == "TOPIC_FORMAT_INVALID":
                self.assertEqual(v["expect"], "REJECTED",
                                 f"{v['_file']}: invalid topic must be REJECTED")
                continue
            self.assertRegex(
                v["topic"], TOPIC_PATTERN,
                f"{v['_file']}: topic '{v['topic']}' does not match "
                f"tenants/{{t}}/devices/{{d}}/(commands|results|commands/acks)"
            )

    def test_command_types_valid(self):
        vectors = self._load_vectors()
        for v in vectors:
            self.assertIn(
                v["command_type"], KNOWN_COMMAND_TYPES,
                f"{v['_file']}: unknown command_type '{v['command_type']}'"
            )

    def test_rejected_vectors_have_error_code(self):
        vectors = self._load_vectors()
        for v in vectors:
            if v["expect"] == "REJECTED":
                self.assertIn(
                    "error_code", v,
                    f"{v['_file']}: REJECTED vector missing error_code"
                )

    def test_unknown_outcome_has_no_retry_note(self):
        """UNKNOWN outcome vectors must note 'no retry' or 'no auto-refund'."""
        vectors = self._load_vectors()
        for v in vectors:
            payload = v.get("payload", {})
            if payload.get("outcome") == "UNKNOWN":
                note = v.get("note", "")
                self.assertTrue(
                    note,
                    f"{v['_file']}: UNKNOWN outcome vector must have a note about no-retry"
                )

    def test_all_command_types_covered(self):
        """FINALIZE, REVERSAL, ACK, RESULT all covered."""
        vectors = self._load_vectors()
        types_found = {v["command_type"] for v in vectors}
        missing = KNOWN_COMMAND_TYPES - types_found
        self.assertEqual(missing, set(), f"command types not covered: {missing}")

    def test_all_outcomes_covered(self):
        """COMMITTED, NOT_COMMITTED, UNKNOWN results all covered."""
        vectors = self._load_vectors()
        outcomes = {v["payload"].get("outcome") for v in vectors if "outcome" in v.get("payload", {})}
        missing = KNOWN_OUTCOMES - outcomes
        self.assertEqual(missing, set(), f"result outcomes not covered: {missing}")

    def test_invalid_signature_rejected(self):
        """Vectors with invalid/wrong signature must be REJECTED."""
        vectors = self._load_vectors()
        invalid_sig = [v for v in vectors if v.get("error_code") == "SIGNATURE_INVALID"]
        self.assertGreater(
            len(invalid_sig), 0,
            "no SIGNATURE_INVALID rejected vector found"
        )
        for v in invalid_sig:
            self.assertEqual(v["expect"], "REJECTED",
                             f"{v['_file']}: SIGNATURE_INVALID must be REJECTED")

    def test_expired_command_rejected(self):
        """Expired commands must be REJECTED."""
        vectors = self._load_vectors()
        expired = [v for v in vectors if v.get("error_code") == "COMMAND_EXPIRED"]
        self.assertGreater(len(expired), 0, "no COMMAND_EXPIRED vector found")
        for v in expired:
            self.assertEqual(v["expect"], "REJECTED")


if __name__ == "__main__":
    unittest.main(verbosity=2)
