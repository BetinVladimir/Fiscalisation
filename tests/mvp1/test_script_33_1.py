"""SCRIPT-33.1: make lint target exists in Makefile"""
import subprocess, os, unittest, re
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript331(unittest.TestCase):
    def test_makefile_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "Makefile")))

    def test_lint_target_defined(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'^lint:', makefile, re.MULTILINE),
                             "Makefile must define 'lint:' target")

    def test_lint_target_calls_scripts(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIn("check-lint-rules.sh", makefile, "lint target must call check-lint-rules.sh")
        self.assertIn("check-swallowed-errors.sh", makefile, "lint target must call check-swallowed-errors.sh")

    def test_phony_includes_lint(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'\.PHONY:.*\blint\b', makefile),
                             "lint must be in .PHONY")

if __name__ == "__main__": unittest.main()
