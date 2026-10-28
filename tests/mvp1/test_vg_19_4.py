"""VG-19.4 — make plan-check invokes check-vendor-docs.sh; Makefile wires it in."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MAKEFILE = ROOT / "Makefile"


class TestVG194PlanCheck(unittest.TestCase):
    def test_makefile_has_plan_check_target(self):
        self.assertTrue(MAKEFILE.exists(), f"Makefile not found at {MAKEFILE}")
        content = MAKEFILE.read_text()
        self.assertIn("plan-check", content,
                      "Makefile must define a plan-check target")

    def test_makefile_plan_check_calls_check_vendor_docs(self):
        content = MAKEFILE.read_text()
        self.assertIn("check-vendor-docs.sh", content,
                      "Makefile plan-check target must invoke scripts/check-vendor-docs.sh")

    def test_check_vendor_docs_passes_within_plan_check(self):
        """check-vendor-docs.sh must pass (exit 0) when invoked directly.
        NOTE: make plan-check may have pre-existing baseline failures from other scripts;
        this test verifies only that the vendor-docs check itself passes."""
        script = ROOT / "scripts" / "check-vendor-docs.sh"
        result = subprocess.run(
            ["bash", str(script)],
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            f"scripts/check-vendor-docs.sh (called from plan-check) must exit 0.\n"
            f"stdout: {result.stdout}\nstderr: {result.stderr}"
        )
        self.assertIn("VENDOR-DOCS OK", result.stdout)


if __name__ == "__main__":
    unittest.main()
