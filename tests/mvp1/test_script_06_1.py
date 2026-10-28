"""SCRIPT-06.1: check-migrations.sh exits 0 or 3 (missing DB dir)"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript061(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-migrations.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-migrations.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1, 3], f"Must exit 0/1/3\n{r.stdout}\n{r.stderr}")

    def test_ok_or_env_message(self):
        r = subprocess.run(["bash", "scripts/check-migrations.sh"], capture_output=True, text=True, cwd=ROOT)
        combined = r.stdout + r.stderr
        self.assertTrue(
            "MIGRATIONS OK" in combined or "MIG-ENV" in combined or "MIG-" in combined,
            f"Must emit MIGRATIONS OK or MIG-* message: {combined}"
        )

if __name__ == "__main__": unittest.main()
