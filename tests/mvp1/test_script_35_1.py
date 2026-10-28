"""SCRIPT-35.1: make fuzz-short target exists in Makefile"""
import subprocess, os, unittest, re
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript351(unittest.TestCase):
    def test_fuzz_short_target_defined(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'^fuzz-short:', makefile, re.MULTILINE),
                             "Makefile must define 'fuzz-short:' target")

    def test_fuzz_short_calls_go_fuzz(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIn("-fuzz=", makefile, "fuzz-short must call go test -fuzz=...")
        self.assertIn("-fuzztime", makefile, "fuzz-short must use -fuzztime")

    def test_phony_includes_fuzz_short(self):
        makefile = open(os.path.join(ROOT, "Makefile")).read()
        self.assertIsNotNone(re.search(r'\.PHONY:.*\bfuzz-short\b', makefile),
                             "fuzz-short must be in .PHONY")

if __name__ == "__main__": unittest.main()
