"""SCRIPT-34.1: make plan-check target exists and calls all sub-scripts"""
import subprocess, os, unittest, re
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

REQUIRED_SCRIPTS = [
    "check_plan_coverage.rb",
    "check_plan_acceptance.rb",
    "check-test-matrix.sh",
    "check-vendor-docs.sh",
    "check-golden-vectors.sh",
    "check-decisions.sh",
    "check-norm-decisions.sh",
    "check-doc-invariants.sh",
    "check-adr-links.sh",
    "check-doc-drift.sh",
]

class TestScript341(unittest.TestCase):
    def test_makefile_plan_check_defined(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'^plan-check:', makefile, re.MULTILINE),
                             "Makefile must define 'plan-check:' target")

    def test_plan_check_calls_all_scripts(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        for script in REQUIRED_SCRIPTS:
            self.assertIn(script, makefile, f"plan-check must reference {script}")

    def test_plan_check_prints_ok(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIn("PLAN-CHECK OK", makefile, "plan-check must print PLAN-CHECK OK")

    def test_phony_includes_plan_check(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'\.PHONY:.*\bplan-check\b', makefile),
                             "plan-check must be in .PHONY")

if __name__ == "__main__": unittest.main()
