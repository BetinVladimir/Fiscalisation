"""
BF-01.1 acceptance test: Validate docs/MVP1/specs/ble_provisioning.md structure and content.

RED phase: document missing or required row absent → assertion failure.
GREEN phase: document present with all required content.
"""
import os
import re
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DOC_PATH = os.path.join(REPO_ROOT, "docs", "MVP1", "specs", "ble_provisioning.md")

# Required UUIDs from smartDeviceBle.ts (independent spec literals)
REQUIRED_UUIDS = [
    "7b6f2000-7c6d-4c7a-9e4f-424545464953",  # EDGE_V2_SERVICE
    "7b6f2001-7c6d-4c7a-9e4f-424545464953",  # EDGE_V2_DEVICE_INFO
    "7b6f2002-7c6d-4c7a-9e4f-424545464953",  # EDGE_V2_SESSION_CONTROL
    "7b6f2003-7c6d-4c7a-9e4f-424545464953",  # EDGE_V2_SESSION_RX
    "7b6f2004-7c6d-4c7a-9e4f-424545464953",  # EDGE_V2_SESSION_TX
]

# Required command rows in the command table (from spec, independent literals)
REQUIRED_COMMANDS = [
    "AUTH_V2",
    "AUTH_RESPONSE",
]

# Required error codes (from card spec, independent literals)
REQUIRED_ERROR_CODES = [
    "EDGE_AUTH_TIMEOUT",
    "EDGE_AUTH_REJECTED",
    "EDGE_CONNECT_TIMEOUT",
    "EDGE_SCAN_TIMEOUT",
    "CAPABILITY_DEVICE_MISMATCH",
    "BLE_SESSION_UNAUTHORIZED",
]

# Required timeout values (from spec, independent literals)
REQUIRED_TIMEOUT_ROWS = [
    "15",  # 15s for connect/scan
    "10",  # 10s for auth
]

# Required sections
REQUIRED_SECTIONS = [
    "GATT Service",
    "Authentication Command Table",
    "Timeouts",
    "Fragmentation",
    "BLOCKED_SPEC",
]

# Required BFPE magic from BINDING_PROVISIONING.md
REQUIRED_BFPE_CONSTANTS = [
    "BFPE",   # magic
]

# Required BFPF fields from PROVISIONING_V2_CRYPTO.md
REQUIRED_BFPF_FIELDS = [
    "transfer_id",
    "sha256",
    "final_flag",
]


class TestBleProvisioningDocument(unittest.TestCase):

    def setUp(self):
        self.assertTrue(
            os.path.isfile(DOC_PATH),
            f"Protocol document missing: {DOC_PATH}",
        )
        with open(DOC_PATH, encoding="utf-8") as f:
            self.content = f.read()

    def test_document_not_empty(self):
        """Document must have substantial content (> 500 chars)."""
        self.assertGreater(
            len(self.content),
            500,
            "Document appears empty or too short — required content missing",
        )

    def test_required_uuids_present(self):
        """All five GATT characteristic UUIDs must appear in the document."""
        for uuid in REQUIRED_UUIDS:
            self.assertIn(
                uuid,
                self.content,
                f"Required GATT UUID missing from document: {uuid}",
            )

    def test_required_command_rows(self):
        """Command table must contain AUTH_V2 and AUTH_RESPONSE rows."""
        for cmd in REQUIRED_COMMANDS:
            self.assertIn(
                cmd,
                self.content,
                f"Required command missing from command table: {cmd}",
            )

    def test_required_error_codes(self):
        """All required error codes must be documented."""
        for code in REQUIRED_ERROR_CODES:
            self.assertIn(
                code,
                self.content,
                f"Required error code missing from document: {code}",
            )

    def test_required_timeout_values(self):
        """Document must state 15s and 10s timeout values."""
        for val in REQUIRED_TIMEOUT_ROWS:
            self.assertIn(
                val,
                self.content,
                f"Required timeout value missing from document: {val}s",
            )

    def test_required_sections(self):
        """Document must contain required section headings."""
        for section in REQUIRED_SECTIONS:
            self.assertIn(
                section,
                self.content,
                f"Required section missing from document: {section}",
            )

    def test_bfpe_constants(self):
        """BFPE magic and envelope format from BINDING_PROVISIONING.md must appear."""
        for const in REQUIRED_BFPE_CONSTANTS:
            self.assertIn(
                const,
                self.content,
                f"Required BFPE constant missing from document: {const}",
            )

    def test_bfpf_framing_fields(self):
        """BFPF framing fields from PROVISIONING_V2_CRYPTO.md must appear."""
        for field in REQUIRED_BFPF_FIELDS:
            self.assertIn(
                field,
                self.content,
                f"Required BFPF framing field missing from document: {field}",
            )

    def test_command_table_has_payload_and_response_columns(self):
        """Command table row must document payload and response for AUTH_V2."""
        # Check that AUTH_V2 appears in a table row where it is the FIRST column value
        lines = self.content.splitlines()
        auth_v2_table_lines = [
            l for l in lines
            if "|" in l and l.strip().lstrip("|").strip().startswith("`AUTH_V2`")
        ]
        self.assertGreater(
            len(auth_v2_table_lines),
            0,
            "AUTH_V2 must appear as first column in a markdown table row",
        )
        # The table row must have at least 4 columns (command, direction, characteristic, payload/response)
        for line in auth_v2_table_lines:
            cols = [c.strip() for c in line.split("|") if c.strip()]
            self.assertGreaterEqual(
                len(cols),
                4,
                f"AUTH_V2 table row must have ≥ 4 columns (cmd, direction, char, payload): {line}",
            )

    def test_protocol_version_constant(self):
        """Protocol version 2 must be mentioned in the document."""
        self.assertIn(
            "protocol_version",
            self.content,
            "protocol_version field must be documented",
        )
        # Value 2 must appear near protocol_version
        idx = self.content.find("protocol_version")
        surrounding = self.content[max(0, idx - 50):idx + 100]
        self.assertIn(
            "2",
            surrounding,
            "Protocol version value '2' must appear near protocol_version in document",
        )

    def test_blocked_spec_items_are_labeled(self):
        """Unresolved protocol items must be explicitly labeled BLOCKED_SPEC."""
        count = self.content.count("BLOCKED_SPEC")
        self.assertGreater(
            count,
            0,
            "Document must explicitly label at least one unresolved item as BLOCKED_SPEC",
        )

    def test_no_guessed_values(self):
        """Document must not claim HIL PASS or assert hardware completion."""
        for forbidden in ["HIL PASS", "hardware PASS", "production ready"]:
            self.assertNotIn(
                forbidden,
                self.content,
                f"Document must not claim '{forbidden}' — this is a spec doc, not evidence",
            )


if __name__ == "__main__":
    unittest.main()
