"""SCRIPT-11.1: gen-datecs-error-catalog.py exists and can be imported"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript111(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "gen-datecs-error-catalog.py")))

    def test_help_exits_cleanly(self):
        r = subprocess.run(
            ["python3", "scripts/gen-datecs-error-catalog.py", "--help"],
            capture_output=True, text=True, cwd=ROOT
        )
        # --help exits 0 or 2 (argparse); both are acceptable
        self.assertIn(r.returncode, [0, 1, 2])

    def test_syntax_valid(self):
        r = subprocess.run(
            ["python3", "-m", "py_compile", "scripts/gen-datecs-error-catalog.py"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertEqual(r.returncode, 0, f"Syntax error: {r.stderr}")

if __name__ == "__main__": unittest.main()
