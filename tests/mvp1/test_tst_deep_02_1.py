"""TST-DEEP-02.1 — AllocateUNP 50 parallel → 50 unique sequential numbers without gaps.

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


class TestTstDeep02_1(unittest.TestCase):

    def test_allocate_unp_50_parallel_native(self):
        """50 parallel ReserveFiscalUNPRange calls → 50 unique sequential numbers 1..50."""
        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            out_dir = Path(artifact_dir) / "tst_deep_02_1"
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

    def test_native_test_file_exists(self):
        """The concurrent test file must be present in the domain package."""
        test_file = BACKEND / "internal" / "domain" / "unp_allocator_concurrent_test.go"
        self.assertTrue(
            test_file.exists(),
            f"Required test file missing: {test_file}",
        )
        content = test_file.read_text()
        self.assertIn("TestAllocateUNP50Parallel", content,
                      "Test function TestAllocateUNP50Parallel must be present")
        self.assertIn("50", content,
                      "Test must reference N=50 goroutines")


if __name__ == "__main__":
    unittest.main()
