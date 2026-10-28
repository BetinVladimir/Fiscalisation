"""
ARCH-002.1 acceptance test:
- service_behavior_test.go with 10 TestServiceBehaviorSnapshot_* tests must exist
- testdata/service_api.sha256 must contain a valid sha256 hex digest
- All 10 characteristic tests must pass
"""
import os
import re
import subprocess
import unittest

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DOMAIN_DIR = os.path.join(REPO_ROOT, "fiscal-backend", "internal", "domain")
BEHAVIOR_TEST = os.path.join(DOMAIN_DIR, "service_behavior_test.go")
SHA256_FILE = os.path.join(DOMAIN_DIR, "testdata", "service_api.sha256")

REQUIRED_SCENARIOS = [
    "TestServiceBehaviorSnapshot_OpenShift",
    "TestServiceBehaviorSnapshot_CreateSaleWithLines",
    "TestServiceBehaviorSnapshot_PayCash",
    "TestServiceBehaviorSnapshot_PayCard",
    "TestServiceBehaviorSnapshot_Finalize",
    "TestServiceBehaviorSnapshot_Reverse",
    "TestServiceBehaviorSnapshot_Cancel",
    "TestServiceBehaviorSnapshot_Reconcile",
    "TestServiceBehaviorSnapshot_Readiness",
    "TestServiceBehaviorSnapshot_ClockSync",
]


class TestArch0021BehaviorTests(unittest.TestCase):

    def test_behavior_test_file_exists(self):
        self.assertTrue(
            os.path.isfile(BEHAVIOR_TEST),
            f"service_behavior_test.go missing: {BEHAVIOR_TEST}",
        )

    def test_behavior_test_contains_all_scenarios(self):
        with open(BEHAVIOR_TEST, encoding="utf-8") as f:
            src = f.read()
        missing = [s for s in REQUIRED_SCENARIOS if s not in src]
        self.assertEqual(
            missing, [],
            f"Missing scenario(s) in service_behavior_test.go: {missing}",
        )

    def test_sha256_file_exists_and_valid(self):
        self.assertTrue(
            os.path.isfile(SHA256_FILE),
            f"testdata/service_api.sha256 missing: {SHA256_FILE}",
        )
        with open(SHA256_FILE, encoding="utf-8") as f:
            content = f.read().strip()
        # Must contain a 64-char hex sha256 digest
        parts = content.split()
        self.assertGreater(len(parts), 0, "service_api.sha256 is empty")
        digest = parts[0]
        self.assertRegex(
            digest,
            r"^[0-9a-f]{64}$",
            f"service_api.sha256 does not contain a valid sha256: {content!r}",
        )

    def test_all_behavior_tests_pass(self):
        """Run TestServiceBehaviorSnapshot_* via go test and verify all pass."""
        result = subprocess.run(
            [
                "go", "test",
                "./internal/domain",
                "-run", "TestServiceBehaviorSnapshot",
                "-v",
                "-count=1",
            ],
            cwd=os.path.join(REPO_ROOT, "fiscal-backend"),
            capture_output=True,
            text=True,
            timeout=120,
        )
        combined = result.stdout + result.stderr

        # Count PASS lines
        pass_lines = [l for l in combined.splitlines() if "--- PASS:" in l and "TestServiceBehaviorSnapshot" in l]
        fail_lines = [l for l in combined.splitlines() if "--- FAIL:" in l and "TestServiceBehaviorSnapshot" in l]

        self.assertEqual(
            result.returncode, 0,
            f"go test returned non-zero.\nSTDOUT:\n{result.stdout}\nSTDERR:\n{result.stderr}",
        )
        self.assertEqual(
            fail_lines, [],
            f"Failing tests: {fail_lines}\nOutput:\n{combined[:3000]}",
        )
        self.assertGreaterEqual(
            len(pass_lines), 10,
            f"Expected >= 10 passing TestServiceBehaviorSnapshot_* tests, got {len(pass_lines)}.\n{combined[:3000]}",
        )


if __name__ == "__main__":
    unittest.main()
