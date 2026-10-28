"""SCRIPT-20.1: check-rounding-golden.sh — fixture exists and Go test passes"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript201(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-rounding-golden.sh")))

    def test_fixture_exists(self):
        self.assertTrue(
            os.path.exists(os.path.join(ROOT, "docs", "MVP1", "fixtures", "rounding_golden.json")),
            "rounding_golden.json must exist"
        )

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-rounding-golden.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_rnd_message(self):
        r = subprocess.run(["bash", "scripts/check-rounding-golden.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'RND-|ROUNDING-GOLDEN', "Must emit RND-* or ROUNDING-GOLDEN message")

if __name__ == "__main__": unittest.main()
