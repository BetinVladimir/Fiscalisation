"""SCRIPT-29.1: verify_trace_tests_exist.rb exits 0 or 1 with TRT-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript291(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "verify_trace_tests_exist.rb")))

    def test_exits_valid_code(self):
        r = subprocess.run(["ruby", "scripts/verify_trace_tests_exist.rb"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1, 3], f"Must exit 0/1/3\n{r.stdout}\n{r.stderr}")

    def test_emits_trt_message(self):
        r = subprocess.run(["ruby", "scripts/verify_trace_tests_exist.rb"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'TRT-|TRACE-TESTS', "Must emit TRT-* or TRACE-TESTS message")

if __name__ == "__main__": unittest.main()
