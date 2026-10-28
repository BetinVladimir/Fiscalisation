"""TST-CT-02.1 — OpenAPI ↔ handler contract: unknown field → non-200; valid → 200; error shape.

Acceptance: fiscal-backend/internal/api/contract_test.go exists; Go TestOpenAPIContract* pass;
unknown body field returns non-200; valid request returns 200 with JSON; error response
has error code field. Note: FISC-DOC-001 operation count gap is documented, not asserted.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"
CONTRACT_TEST = BACKEND / "internal" / "api" / "contract_test.go"
OPENAPI_SPEC = ROOT / "contracts" / "openapi-runtime-v1.yaml"


class TestTSTCT02_1(unittest.TestCase):
    def test_contract_test_file_exists(self):
        self.assertTrue(
            CONTRACT_TEST.exists(),
            f"contract_test.go not found: {CONTRACT_TEST}"
        )

    def test_openapi_spec_exists(self):
        self.assertTrue(
            OPENAPI_SPEC.exists(),
            f"openapi-runtime-v1.yaml not found: {OPENAPI_SPEC}"
        )

    def test_go_contract_tests_pass(self):
        """Go TestOpenAPIContract* suite: unknown field → non-200; valid → 200; error shape."""
        result = subprocess.run(
            ["go", "test", "./internal/api/...", "-run", "TestOpenAPIContract", "-v", "-count=1"],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f"TestOpenAPIContract* failed (rc={result.returncode}):\n{output}"
        )
        # Verify at least the key subtests ran
        self.assertIn("TestOpenAPIContractUnknownField", output,
                      f"TestOpenAPIContractUnknownField not found:\n{output}")
        self.assertIn("TestOpenAPIContractErrorResponseShape", output,
                      f"TestOpenAPIContractErrorResponseShape not found:\n{output}")
        self.assertIn("TestOpenAPIContractValidResponseShape", output,
                      f"TestOpenAPIContractValidResponseShape not found:\n{output}")

        # Count passed subtests
        passed = output.count("--- PASS: TestOpenAPIContract")
        self.assertGreater(passed, 0, f"no TestOpenAPIContract subtests passed:\n{output}")

        artifact_dir = os.environ.get("MVP1_ARTIFACT_DIR")
        if artifact_dir:
            Path(artifact_dir).mkdir(parents=True, exist_ok=True)
            (Path(artifact_dir) / "tst_ct_02_1.txt").write_text(output)

    def test_openapi_has_operations(self):
        """OpenAPI spec must define operations (paths with methods)."""
        import re
        content = OPENAPI_SPEC.read_text()
        ops = len(re.findall(r'^\s+(get|post|put|delete|patch):\s*$', content, re.MULTILINE))
        self.assertGreater(ops, 0, "OpenAPI spec has no operations")
        # Document actual count for FISC-DOC-001 tracking
        # (exact count not asserted due to known gap)

    def test_full_api_suite_compiles(self):
        """api package must compile clean (go vet)."""
        result = subprocess.run(
            ["go", "vet", "./internal/api/..."],
            cwd=str(BACKEND),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            f"go vet failed:\n{result.stdout}\n{result.stderr}"
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)
