"""TST-DEEP-04.1 — DER vs P1363 signatures: 10 vectors, cross-format rejection.

Native deliverables:
  fiscal-backend/internal/domain/ecdsa_vectors_test.go
  docs/MVP1/fixtures/ecdsa_vectors.json
Run: go test ./internal/domain/ -run TestECDSA -v -json -count=1
"""
import json
import os
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "fiscal-backend"
FIXTURES = ROOT / "docs" / "MVP1" / "fixtures" / "ecdsa_vectors.json"
FIXES = ROOT / "docs" / "MVP1" / "fixes"
sys.path.insert(0, str(FIXES))
from native_harness import check_native_tests  # noqa: E402


class TestTstDeep04_1(unittest.TestCase):

    def test_ecdsa_vectors_fixture_exists_and_valid(self):
        """Fixture must have >= 10 entries with required fields."""
        self.assertTrue(FIXTURES.exists(), f"Fixture missing: {FIXTURES}")
        vectors = json.loads(FIXTURES.read_text())
        self.assertGreaterEqual(len(vectors), 10, "Need >= 10 vectors")
        for i, v in enumerate(vectors[:10]):
            for field in ("digest_hex", "pubkey_spki_b64", "sig_der_b64", "sig_p1363_b64"):
                self.assertIn(field, v, f"vector {i} missing field {field}")

    def test_ecdsa_vectors_native(self):
        """Go tests verify P1363 acceptance, DER acceptance, and cross-format rejection."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_04_1"
            out_dir.mkdir(parents=True, exist_ok=True)

        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestECDSA",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=[
                "TestECDSAP1363Verify",
                "TestECDSADERVerify",
                "TestECDSACrossFormatRejection",
            ],
            minimum=3,
            timeout=120,
        )


if __name__ == "__main__":
    unittest.main()
