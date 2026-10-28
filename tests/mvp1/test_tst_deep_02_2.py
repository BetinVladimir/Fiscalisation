"""TST-DEEP-02.2 — AllocateUNP concurrent regression: test stays green (no defect found).

Native deliverable: fiscal-backend/internal/domain/unp_allocator_concurrent_test.go
Run: go test ./internal/domain/ -run TestAllocateUNP50Parallel -v -json -count=1
"""
import os
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "fiscal-backend"
FIXES = ROOT / "docs" / "MVP1" / "fixes"
sys.path.insert(0, str(FIXES))
from native_harness import check_native_tests  # noqa: E402


class TestTstDeep02_2(unittest.TestCase):

    def test_allocate_unp_50_parallel_green(self):
        """All 50 concurrent allocations succeed; result is the sequential range 1..50."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_02_2"
            out_dir.mkdir(parents=True, exist_ok=True)

        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestAllocateUNP50Parallel",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=["TestAllocateUNP50Parallel"],
            minimum=1,
            timeout=120,
        )

    def test_existing_concurrent_disjoint_range_test_still_passes(self):
        """Regression: existing 20-goroutine disjoint test must stay green."""
        check_native_tests(
            self,
            argv=[
                "go", "test", "./internal/domain/",
                "-run", "TestFiscalUNPAllocatorConcurrentDisjointRanges",
                "-v", "-json", "-count=1",
            ],
            root=str(BACKEND),
            required_tests=["TestFiscalUNPAllocatorConcurrentDisjointRanges"],
            minimum=1,
            timeout=120,
        )


if __name__ == "__main__":
    unittest.main()
