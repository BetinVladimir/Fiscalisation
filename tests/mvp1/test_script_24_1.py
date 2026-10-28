"""SCRIPT-24.1: check-rabbit-topology.sh exits 0 or 1 with RBT-* message"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript241(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-rabbit-topology.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-rabbit-topology.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")

    def test_emits_rbt_message(self):
        r = subprocess.run(["bash", "scripts/check-rabbit-topology.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertRegex(combined, r'RBT-|RABBIT-TOPOLOGY', "Must emit RBT-* or RABBIT-TOPOLOGY message")

if __name__ == "__main__": unittest.main()
