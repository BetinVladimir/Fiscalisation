"""NAP-13.1-through-2 — E-чек excluded from MVP: negative gate + contract validation."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"
FEATURE_SCOPE = ROOT / "contracts" / "supto-feature-scope.json"


class TestEReceiptExclusionNegativeGate(unittest.TestCase):
    def test_ereceipt_always_unsupported_go(self):
        result = subprocess.run(
            ["go", "test", "./internal/domain", "-run", "EReceipt", "-count=1", "-v", "-json"],
            capture_output=True, text=True, timeout=120, cwd=str(BACKEND),
        )
        self.assertEqual(
            result.returncode, 0,
            f"E-receipt negative gate tests failed:\nstdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-500:]}",
        )
        events = []
        for line in result.stdout.splitlines():
            try:
                events.append(json.loads(line.strip()))
            except json.JSONDecodeError:
                pass
        passed = [e.get("Test", "") for e in events if e.get("Action") == "pass" and e.get("Test")]
        gate_tests = [t for t in passed if "EReceipt" in t]
        self.assertGreaterEqual(
            len(gate_tests), 2,
            f"Expected ≥2 EReceipt negative gate tests to pass, got: {gate_tests}",
        )

    def test_feature_scope_contract_exists(self):
        self.assertTrue(
            FEATURE_SCOPE.exists(),
            f"contracts/supto-feature-scope.json not found: {FEATURE_SCOPE}",
        )

    def test_ereceipt_excluded_from_mvp_in_contract(self):
        self.assertTrue(FEATURE_SCOPE.exists(), "contracts/supto-feature-scope.json not found")
        data = json.loads(FEATURE_SCOPE.read_text(encoding="utf-8"))
        self.assertIn("features", data, "contract must have 'features' key")
        self.assertIn(
            "electronic_fiscal_receipts", data["features"],
            "contract must have 'electronic_fiscal_receipts' feature entry",
        )
        ereceipt = data["features"]["electronic_fiscal_receipts"]
        self.assertFalse(
            ereceipt.get("in_mvp_scope", True),
            "electronic_fiscal_receipts must have in_mvp_scope=false (excluded from MVP)",
        )
        self.assertNotEqual(
            ereceipt.get("status"), "IMPLEMENTED",
            "electronic_fiscal_receipts must not be marked IMPLEMENTED — absence must not be hidden",
        )
        self.assertTrue(
            ereceipt.get("production_blocked", False),
            "electronic_fiscal_receipts must have production_blocked=true",
        )

    def test_ereceipt_status_is_excluded(self):
        data = json.loads(FEATURE_SCOPE.read_text(encoding="utf-8"))
        ereceipt = data["features"]["electronic_fiscal_receipts"]
        status = ereceipt.get("status", "")
        self.assertIn(
            "EXCLUDED" in status or "NOT_IN" in status or status == "IMPLEMENTATION_REQUIRED",
            [True],
            f"e-receipt status {status!r} should indicate exclusion (EXCLUDED_FROM_MVP or similar)",
        )
        # Explicitly must NOT be IMPLEMENTED or SOFTWARE_IMPLEMENTED
        self.assertNotIn(
            "IMPLEMENTED", status,
            f"e-receipt status must not claim IMPLEMENTED, got {status!r}",
        )

    def test_no_ereceipt_delivery_in_domain(self):
        domain_dir = BACKEND / "internal" / "domain"
        for go_file in domain_dir.glob("*.go"):
            if go_file.name.endswith("_test.go"):
                continue
            text = go_file.read_text(encoding="utf-8", errors="replace")
            self.assertNotIn(
                "smtp", text.lower(),
                f"{go_file.name} must not import smtp (no email delivery in domain)",
            )
            self.assertNotIn(
                "sendgrid", text.lower(),
                f"{go_file.name} must not reference sendgrid (no email delivery in domain)",
            )


if __name__ == "__main__":
    unittest.main()
