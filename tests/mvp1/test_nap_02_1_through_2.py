"""NAP-02.1-through-2 — INSUFFICIENT_CASH typed error on storno (чл. 31(3)).

Acceptance: `go test -run InsufficientCash` in fiscal-backend/internal/domain passes
with a nonzero test count. Numbers and constants from docs/MVP1/fixtures.
"""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DOMAIN_PKG = ROOT / "fiscal-backend" / "internal" / "domain"


class TestNAP021Through2(unittest.TestCase):

    def _run_go(self, run_pattern: str) -> tuple[int, str, int]:
        """Run `go test -v -json -run <run_pattern>` and return (rc, output, test_count)."""
        result = subprocess.run(
            ["go", "test", "-v", "-json", "./internal/domain/...", "-run", run_pattern, "-count=1"],
            cwd=str(ROOT / "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        count = 0
        for line in result.stdout.splitlines():
            try:
                obj = json.loads(line)
                if obj.get("Action") in ("pass", "fail") and obj.get("Test"):
                    count += 1
            except (json.JSONDecodeError, KeyError):
                continue
        return result.returncode, result.stdout + result.stderr, count

    def test_nap02_go_tests_pass(self):
        """All InsufficientCash Go tests pass."""
        rc, output, count = self._run_go("InsufficientCash")
        self.assertEqual(
            rc, 0,
            f"go test -run InsufficientCash failed (rc={rc}):\n{output[-3000:]}",
        )
        self.assertGreater(
            count, 0,
            f"test_count=0 — no InsufficientCash tests ran:\n{output[-2000:]}",
        )

    def test_nap02_typed_error_test_present(self):
        """TestReversalInsufficientCash_ReturnsTypedError must exist and pass."""
        rc, output, count = self._run_go("TestReversalInsufficientCash_ReturnsTypedError")
        self.assertEqual(rc, 0, f"ReturnsTypedError test failed:\n{output[-2000:]}")
        self.assertGreater(count, 0, "test not found")

    def test_nap02_sale_stays_completed_test_present(self):
        """TestReversalInsufficientCash_SaleStaysCompleted must exist and pass."""
        rc, output, count = self._run_go("TestReversalInsufficientCash_SaleStaysCompleted")
        self.assertEqual(rc, 0, f"SaleStaysCompleted test failed:\n{output[-2000:]}")
        self.assertGreater(count, 0, "test not found")

    def test_nap02_operator_error_cash_in_action_present(self):
        """TestReversalInsufficientCash_OperatorErrorHasCashInAction — AllowedActions contains CASH_IN."""
        rc, output, count = self._run_go("TestReversalInsufficientCash_OperatorErrorHasCashInAction")
        self.assertEqual(rc, 0, f"OperatorErrorHasCashInAction test failed:\n{output[-2000:]}")
        self.assertGreater(count, 0, "test not found")

    def test_nap02_reversal_service_insufficient_cash_handling(self):
        """reversal_service.go contains explicit INSUFFICIENT_CASH branch (code audit)."""
        reversal_src = ROOT / "fiscal-backend" / "internal" / "domain" / "reversal_service.go"
        self.assertTrue(reversal_src.exists(), f"reversal_service.go not found: {reversal_src}")
        text = reversal_src.read_text(encoding="utf-8")
        self.assertIn(
            '"INSUFFICIENT_CASH"',
            text,
            "reversal_service.go must contain explicit INSUFFICIENT_CASH branch (NAP-02 §чл.31(3))",
        )
        self.assertIn(
            '"CASH_IN"',
            text,
            "reversal_service.go must set AllowedActions CASH_IN for INSUFFICIENT_CASH (NAP-02)",
        )


if __name__ == "__main__":
    unittest.main()
