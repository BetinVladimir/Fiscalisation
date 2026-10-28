"""TST-CT-05.1 — ECDSA crypto vectors: contracts/crypto-vectors-v1.json verified by Go.

Acceptance: contracts/crypto-vectors-v1.json exists with DER+P1363 fields; Go tests
TestECDSAP1363Verify and TestECDSACrossFormatRejection pass; DER↔P1363 swap → rejection;
corrupted byte → rejection; no panic from either failure mode.
"""
import hashlib
import json
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRACT = ROOT / "contracts" / "crypto-vectors-v1.json"
FIXTURE = ROOT / "docs" / "MVP1" / "fixtures" / "ecdsa_vectors.json"

REQUIRED_FIELDS = {
    "index", "digest_hex", "pubkey_spki_b64",
    "sig_der_b64", "sig_p1363_b64",
    "wrong_sig_der_b64", "wrong_sig_p1363_b64",
}


class TestTSTCT05_1(unittest.TestCase):
    def test_contract_file_exists(self):
        self.assertTrue(CONTRACT.exists(), f"contracts/crypto-vectors-v1.json not found: {CONTRACT}")

    def test_contract_has_vectors(self):
        self.assertTrue(CONTRACT.exists())
        data = json.loads(CONTRACT.read_text())
        self.assertIsInstance(data, list)
        self.assertGreater(len(data), 0, "crypto-vectors-v1.json must have at least one vector")

    def test_required_fields_in_every_vector(self):
        """DER and P1363 fields must be present for cross-format tests."""
        self.assertTrue(CONTRACT.exists())
        data = json.loads(CONTRACT.read_text())
        for i, v in enumerate(data):
            missing = REQUIRED_FIELDS - set(v.keys())
            self.assertEqual(
                missing, set(),
                f"vector[{i}] missing required fields: {missing}"
            )

    def test_contract_matches_fixture_sha256(self):
        """Contract is a verified copy of the fixture (no divergence)."""
        self.assertTrue(CONTRACT.exists())
        self.assertTrue(FIXTURE.exists(), f"fixture missing: {FIXTURE}")
        sha_c = hashlib.sha256(CONTRACT.read_bytes()).hexdigest()
        sha_f = hashlib.sha256(FIXTURE.read_bytes()).hexdigest()
        self.assertEqual(
            sha_c, sha_f,
            f"sha256 mismatch: contract={sha_c[:16]}... fixture={sha_f[:16]}..."
        )

    def test_go_p1363_verify(self):
        """Go TestECDSAP1363Verify: valid P1363 accepted; wrong-key rejected."""
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "TestECDSAP1363Verify", "-v", "-count=1"],
            cwd=str(ROOT / "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"TestECDSAP1363Verify failed (rc={result.returncode}):\n{output}"
        )
        passed = output.count("--- PASS: TestECDSAP1363Verify")
        self.assertGreater(passed, 0, f"no TestECDSAP1363Verify subtests found:\n{output}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_ct_05_1_p1363.txt").write_text(output)

    def test_go_cross_format_rejection(self):
        """Go TestECDSACrossFormatRejection: DER↔P1363 swap rejected without panic."""
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "TestECDSACrossFormatRejection",
             "-v", "-count=1"],
            cwd=str(ROOT / "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"TestECDSACrossFormatRejection failed (rc={result.returncode}):\n{output}"
        )
        passed = output.count("--- PASS: TestECDSACrossFormatRejection")
        self.assertGreater(passed, 0, f"no TestECDSACrossFormatRejection subtests:\n{output}")

    def test_go_der_verify(self):
        """Go TestECDSADERVerify (if present): valid DER accepted; wrong rejected."""
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "TestECDSA", "-v", "-count=1"],
            cwd=str(ROOT / "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"TestECDSA* suite failed (rc={result.returncode}):\n{output}"
        )
        total_passed = output.count("--- PASS: Test")
        self.assertGreater(
            total_passed, 0,
            f"no ECDSA tests passed:\n{output}"
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)
