"""SCRIPT-32.1: mutate-vector.sh exists, exits 0/1/3"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript321(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "mutate-vector.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(
            ["bash", "scripts/mutate-vector.sh"],
            capture_output=True, text=True, cwd=ROOT,
            timeout=60
        )
        self.assertIn(r.returncode, [0, 1, 3], f"Must exit 0/1/3\n{r.stdout}\n{r.stderr}")

    def test_emits_mut_message(self):
        r = subprocess.run(
            ["bash", "scripts/mutate-vector.sh"],
            capture_output=True, text=True, cwd=ROOT,
            timeout=60
        )
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'MUT-|MUTATE-VECTOR|mutants', "Must emit MUT-* or mutants message")

if __name__ == "__main__": unittest.main()
