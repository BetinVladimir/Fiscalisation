"""TST-DEEP-11.2 — DST transition regression: BusinessDate stays correct (no defect found).

Native deliverable: fiscal-backend/internal/domain/business_date_test.go
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


class TestTstDeep11_2(unittest.TestCase):

    def test_business_date_dst_green(self):
        """All 4 DST property tests pass; BusinessDate is correct across 2880 minutes."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_11_2"
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

    def test_existing_business_date_tests_still_pass(self):
        """Regression: pre-existing BusinessDate tests in clock_drift_test.go stay green."""
        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestBusinessDate",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=[
                "TestBusinessDateUsesEuropeSofia",
                "TestBusinessDateMidnightSofia",
            ],
            minimum=2,
            timeout=120,
        )


if __name__ == "__main__":
    unittest.main()
