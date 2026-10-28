"""TST-DEEP-11.1 — Nighttime DST transition (E2E-15): property test across all minutes.

Native deliverable: fiscal-backend/internal/domain/business_date_test.go
Run: go test ./internal/domain/ -run TestBusinessDateDST -v -json -count=1
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


class TestTstDeep11_1(unittest.TestCase):

    def test_business_date_dst_transitions_native(self):
        """Property test: BusinessDate correct for all 1440 minutes of spring-forward and fall-back days."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_11_1"
            out_dir.mkdir(parents=True, exist_ok=True)

        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestBusinessDateDST",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=[
                "TestBusinessDateDSTSpringForwardAllMinutes",
                "TestBusinessDateDSTFallBackAllMinutes",
                "TestBusinessDateDSTCrossMidnightSofia",
                "TestBusinessDateDSTLocationIsSofia",
            ],
            minimum=4,
            timeout=120,
        )

    def test_business_date_test_file_exists(self):
        """Required Go test file must exist."""
        test_file = BACKEND / "internal" / "domain" / "business_date_test.go"
        self.assertTrue(test_file.exists(), f"Missing: {test_file}")
        content = test_file.read_text()
        for term in ("TestBusinessDateDST", "1440", "Europe/Sofia", "spring", "fall"):
            self.assertIn(term, content, f"Missing term {term!r}")


if __name__ == "__main__":
    unittest.main()
