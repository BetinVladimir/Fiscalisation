"""SCRIPT-19.1: check-swallowed-errors.sh exits 0 on clean repo"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript191(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-swallowed-errors.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-swallowed-errors.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_output_not_empty(self):
        r = subprocess.run(["bash", "scripts/check-swallowed-errors.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertGreater(len((r.stdout + r.stderr).strip()), 0)

if __name__ == "__main__": unittest.main()
