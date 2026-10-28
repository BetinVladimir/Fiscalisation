"""NAP-10.1-through-2 — AUDITOR role: read-only across all mutation routes."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
BACKEND = ROOT / "fiscal-backend"


class TestAuditorReadOnly(unittest.TestCase):
    def _go_test(self, run_pattern: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["go", "test", "./internal/api", f"-run={run_pattern}", "-count=1", "-v", "-json"],
            capture_output=True, text=True, timeout=120, cwd=str(BACKEND),
        )

    def test_auditor_readonly_across_all_mutations(self):
        result = self._go_test("TestAuditorReadOnlyAcrossAllMutations")
        self.assertEqual(
            result.returncode, 0,
            f"go test failed (rc={result.returncode}):\n"
            f"stdout: {result.stdout[-3000:]}\nstderr: {result.stderr[-1000:]}",
        )
        # Verify test actually ran (non-zero count)
        events = []
        for line in result.stdout.splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                events.append(json.loads(line))
            except json.JSONDecodeError:
                pass
        test_names = [e.get("Test", "") for e in events if e.get("Action") in ("pass", "fail")]
        self.assertIn(
            "TestAuditorReadOnlyAcrossAllMutations", test_names,
            f"TestAuditorReadOnlyAcrossAllMutations not found in test output. events: {test_names}",
        )
        passed = [e for e in events if e.get("Action") == "pass" and "TestAuditorReadOnly" in e.get("Test", "")]
        self.assertTrue(passed, "TestAuditorReadOnlyAcrossAllMutations did not pass")

    def test_routes_golden_has_public_routes(self):
        golden = BACKEND / "internal" / "api" / "testdata" / "routes.golden"
        self.assertTrue(golden.exists(), f"routes.golden not found: {golden}")
        lines = golden.read_text().splitlines()
        public_routes = [l for l in lines if l.strip().startswith("/public/v1/")]
        self.assertGreater(len(public_routes), 5, "routes.golden should have >5 /public/v1/ routes")

    def test_auditor_blocked_in_auth_unit_tests(self):
        result = subprocess.run(
            ["go", "test", "./internal/auth", "-run=TestRBACLeastPrivilege", "-count=1", "-v", "-json"],
            capture_output=True, text=True, timeout=60, cwd=str(BACKEND),
        )
        self.assertEqual(
            result.returncode, 0,
            f"auth RBAC tests failed:\nstdout: {result.stdout[-2000:]}\nstderr: {result.stderr[-500:]}",
        )
        events = []
        for line in result.stdout.splitlines():
            try:
                events.append(json.loads(line.strip()))
            except json.JSONDecodeError:
                pass
        passed = [e for e in events if e.get("Action") == "pass" and e.get("Test")]
        self.assertGreater(len(passed), 0, "TestRBACLeastPrivilege not found/passed")
        # Verify the test specifically covers AUDITOR cannot mutate
        combined = result.stdout + result.stderr
        self.assertIn(
            "auditor cannot mutate", combined.lower().replace("_", " "),
            "RBAC test output should mention auditor cannot mutate (from TestRBACLeastPrivilege subtests)",
        )


if __name__ == "__main__":
    unittest.main()
