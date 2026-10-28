"""TST-CT-01.1 — Golden rounding: contracts/money-rounding-golden-v1.json verified by Go (34/34).

Acceptance: contracts file exists with 34 vectors matching the fixture; Go TestMoneyRoundingGolden
passes; check-rounding-golden.sh passes with sha256 parity check.
"""
import hashlib
import json
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRACT = ROOT / "contracts" / "money-rounding-golden-v1.json"
FIXTURE = ROOT / "docs" / "MVP1" / "fixtures" / "rounding_golden.json"
SCRIPT = ROOT / "scripts" / "check-rounding-golden.sh"

REQUIRED_FIELDS = {"name", "unit_cents", "qty_milli", "discount_cents", "expect_line_cents"}
EXPECTED_COUNT = 34


class TestTSTCT01_1(unittest.TestCase):
    def test_contract_file_exists(self):
        self.assertTrue(CONTRACT.exists(), f"contracts/money-rounding-golden-v1.json not found: {CONTRACT}")

    def test_contract_has_34_vectors(self):
        self.assertTrue(CONTRACT.exists(), f"contract file missing: {CONTRACT}")
        data = json.loads(CONTRACT.read_text())
        self.assertIsInstance(data, list, "contract file must be a JSON array")
        self.assertEqual(
            len(data), EXPECTED_COUNT,
            f"expected {EXPECTED_COUNT} vectors, got {len(data)}"
        )

    def test_contract_vectors_have_required_fields(self):
        self.assertTrue(CONTRACT.exists(), f"contract file missing: {CONTRACT}")
        data = json.loads(CONTRACT.read_text())
        for i, v in enumerate(data):
            missing = REQUIRED_FIELDS - set(v.keys())
            self.assertEqual(
                missing, set(),
                f"vector[{i}] missing fields: {missing}"
            )

    def test_contract_matches_fixture_sha256(self):
        """Divergent copies → test fails (RND-003)."""
        self.assertTrue(CONTRACT.exists(), f"contract file missing: {CONTRACT}")
        self.assertTrue(FIXTURE.exists(), f"fixture missing: {FIXTURE}")
        sha_contract = hashlib.sha256(CONTRACT.read_bytes()).hexdigest()
        sha_fixture = hashlib.sha256(FIXTURE.read_bytes()).hexdigest()
        self.assertEqual(
            sha_contract, sha_fixture,
            f"sha256 mismatch: contract={sha_contract[:16]}... fixture={sha_fixture[:16]}..."
        )

    def test_no_float_amounts(self):
        """Invariant 1: money = integer cents only."""
        self.assertTrue(CONTRACT.exists(), f"contract file missing: {CONTRACT}")
        data = json.loads(CONTRACT.read_text())
        for i, v in enumerate(data):
            for field in ("unit_cents", "discount_cents", "expect_line_cents"):
                val = v.get(field, 0)
                self.assertIsInstance(val, int, f"vector[{i}].{field}={val!r} must be int")
            qty = v.get("qty_milli", 0)
            self.assertIsInstance(qty, int, f"vector[{i}].qty_milli={qty!r} must be int")

    def test_go_rounding_golden_34_of_34(self):
        """Go TestMoneyRoundingGolden must pass all 34 vectors."""
        self.assertTrue((ROOT / "fiscal-backend").is_dir(), "fiscal-backend dir missing")
        result = subprocess.run(
            ["go", "test", "./internal/domain/...", "-run", "TestMoneyRoundingGolden", "-v", "-count=1"],
            cwd=str(ROOT / "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"Go TestMoneyRoundingGolden failed (rc={result.returncode}):\n{output}"
        )
        # Count subtests passed
        passed = output.count("--- PASS: TestMoneyRoundingGolden/golden_")
        self.assertGreaterEqual(
            passed, EXPECTED_COUNT,
            f"expected >= {EXPECTED_COUNT} subtest passes, got {passed}:\n{output}"
        )

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_ct_01_1_go.txt").write_text(output)

    def test_check_rounding_script_passes(self):
        """scripts/check-rounding-golden.sh must exit 0 (includes sha256 parity)."""
        self.assertTrue(SCRIPT.exists(), f"script missing: {SCRIPT}")
        result = subprocess.run(
            ["bash", str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"check-rounding-golden.sh failed (rc={result.returncode}):\n{output}"
        )
        self.assertIn("ROUNDING-GOLDEN OK", result.stdout,
                      f"success marker missing:\n{output}")


if __name__ == "__main__":
    unittest.main(verbosity=2)
