"""S3D-03.1 — payment slot mapping in profile_executor: PAYMENT_TYPE_UNSUPPORTED for
NHIF/DEFERRED/INTERNAL_CONSUMPTION; EUR currency gate.

Acceptance: profile_executor.cpp contains PAYMENT_TYPE_UNSUPPORTED guard for unsupported
payment types; payment_slots_bc50.json has CASH=0, CARD=1, CHEQUE=3, VOUCHER=4, COUPON=5
and unsupported=[NHIF,DEFERRED,INTERNAL_CONSUMPTION]; native saga test for NHIF passes.
"""
import json
import os
import re
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
PROFILE_EXECUTOR = (
    ROOT / "IoT" / "firmware" / "edge-agent-s3" / "idf" / "main" / "profile_executor.cpp"
)
FIXTURE = ROOT / "docs" / "MVP1" / "fixtures" / "payment_slots_bc50.json"
RUN_SAGA_SH = ROOT / "IoT" / "firmware" / "edge-agent-s3" / "idf" / "tests" / "run_native_saga.sh"

EXPECTED_SLOTS = {"CASH": 0, "CARD": 1, "CHEQUE": 3, "VOUCHER": 4, "COUPON": 5}
EXPECTED_UNSUPPORTED = {"NHIF", "DEFERRED", "INTERNAL_CONSUMPTION"}


class TestS3D03_1(unittest.TestCase):
    def test_fixture_exists(self):
        self.assertTrue(
            FIXTURE.exists(),
            f"payment_slots_bc50.json not found: {FIXTURE}"
        )

    def test_fixture_slot_mapping(self):
        """Fixture must define CASH=0, CARD=1, CHEQUE=3, VOUCHER=4, COUPON=5."""
        self.assertTrue(FIXTURE.exists())
        data = json.loads(FIXTURE.read_text())
        for name, expected_slot in EXPECTED_SLOTS.items():
            self.assertIn(name, data, f"slot '{name}' missing from fixture")
            self.assertEqual(
                data[name], expected_slot,
                f"slot '{name}': expected {expected_slot}, got {data[name]}"
            )

    def test_fixture_unsupported_types(self):
        """Fixture must list NHIF, DEFERRED, INTERNAL_CONSUMPTION as unsupported."""
        self.assertTrue(FIXTURE.exists())
        data = json.loads(FIXTURE.read_text())
        self.assertIn("unsupported", data, "fixture missing 'unsupported' list")
        unsupported_set = set(data["unsupported"])
        missing = EXPECTED_UNSUPPORTED - unsupported_set
        self.assertEqual(
            missing, set(),
            f"fixture 'unsupported' list missing: {sorted(missing)}"
        )

    def test_profile_executor_has_payment_type_unsupported(self):
        """profile_executor.cpp must contain PAYMENT_TYPE_UNSUPPORTED guard."""
        self.assertTrue(
            PROFILE_EXECUTOR.exists(),
            f"profile_executor.cpp not found: {PROFILE_EXECUTOR}"
        )
        src = PROFILE_EXECUTOR.read_text()
        self.assertIn(
            "PAYMENT_TYPE_UNSUPPORTED", src,
            "profile_executor.cpp must return 'PAYMENT_TYPE_UNSUPPORTED' for unsupported types"
        )

    def test_profile_executor_guards_all_three_unsupported_types(self):
        """profile_executor.cpp must check NHIF, DEFERRED, and INTERNAL_CONSUMPTION."""
        src = PROFILE_EXECUTOR.read_text()
        for unsupported in ("NHIF", "DEFERRED", "INTERNAL_CONSUMPTION"):
            self.assertIn(
                f'"{unsupported}"', src,
                f"profile_executor.cpp missing guard for '{unsupported}'"
            )

    def test_native_saga_unsupported_nhif_passes(self):
        """Native saga test 'unsupported-payment-nhif' must pass (TAP ok 3)."""
        self.assertTrue(RUN_SAGA_SH.exists(), f"run_native_saga.sh not found: {RUN_SAGA_SH}")
        result = subprocess.run(
            ["bash", str(RUN_SAGA_SH)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=300,
        )
        output = result.stdout + result.stderr
        # Check for the specific NHIF test passing
        self.assertIn(
            "ok", output,
            f"run_native_saga.sh produced no passing tests:\n{output[:500]}"
        )
        # Look for "ok N - unsupported-payment-nhif" in TAP output
        nhif_pass = re.search(r'ok \d+ - unsupported-payment-nhif', output)
        self.assertIsNotNone(
            nhif_pass,
            f"'unsupported-payment-nhif' not in TAP passing output:\n{output}"
        )

        # Count passing tests
        total_pass = len(re.findall(r'^ok \d+', output, re.MULTILINE))
        self.assertGreater(total_pass, 0, "no passing TAP tests found")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "s3d_03_1_saga.txt").write_text(output)


if __name__ == "__main__":
    unittest.main(verbosity=2)
