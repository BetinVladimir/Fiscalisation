"""SCRIPT-22.1: check-db-schema.sh exits 3 without DB env, not crash"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript221(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-db-schema.sh")))

    def test_exits_3_without_env(self):
        env = os.environ.copy()
        env.pop("DATABASE_URL", None)
        env.pop("PGHOST", None)
        r = subprocess.run(
            ["bash", "scripts/check-db-schema.sh"],
            capture_output=True, text=True, cwd=ROOT, env=env
        )
        self.assertEqual(r.returncode, 3, f"Must exit 3 without DB env\n{r.stdout}\n{r.stderr}")
        self.assertIn("DBS-ENV", r.stderr, "Must print DBS-ENV message")

if __name__ == "__main__": unittest.main()
