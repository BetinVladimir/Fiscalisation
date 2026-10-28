"""TST-DEEP-04.2 — DER vs P1363 regression: test stays green (no defect found in ecdsa_p1363.go).

Native deliverables:
  fiscal-backend/internal/domain/ecdsa_vectors_test.go
  docs/MVP1/fixtures/ecdsa_vectors.json
"""
import os
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "fiscal-backend"
FIXES = ROOT / "docs" / "MVP1" / "fixes"
sys.path.insert(0, str(FIXES))
from native_harness import check_native_tests  # noqa: E402


class TestTstDeep04_2(unittest.TestCase):

    def test_ecdsa_all_vectors_green(self):
        """All 10 vectors: P1363 accepted, DER accepted, cross-format rejected without panic."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_04_2"
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

    def test_ecdsa_p1363_signverify_round_trip(self):
        """verifyP1363 round-trip: sign then verify with Go's own signP1363 (via existing tests)."""
        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestECDSA",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=["TestECDSAP1363Verify"],
            minimum=1,
            timeout=120,
        )


if __name__ == "__main__":
    unittest.main()
