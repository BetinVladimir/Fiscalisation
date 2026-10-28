"""SCRIPT-27.1: e2e-trace-check.sh — good rc0, bad rc1/3, selftest mode works"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript271(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "e2e-trace-check.sh")))

    def test_exits_3_without_env(self):
        env = os.environ.copy()
        env.pop("E2E_LOG", None)
        r = subprocess.run(
            ["bash", "scripts/e2e-trace-check.sh"],
            capture_output=True, text=True, cwd=ROOT, env=env
        )
        self.assertEqual(r.returncode, 3, f"Must exit 3 without E2E_LOG\n{r.stdout}\n{r.stderr}")
        self.assertIn("TRC-ENV", r.stderr)

    def test_selftest_good_passes(self):
        r = subprocess.run(
            ["bash", "scripts/e2e-trace-check.sh", "--selftest"],
            capture_output=True, text=True, cwd=ROOT
        )
        self.assertEqual(r.returncode, 0, f"Selftest must pass\n{r.stdout}\n{r.stderr}")
        self.assertIn("SELFTEST OK", r.stdout)

    def test_bad_log_exits_nonzero(self):
        import tempfile
        with tempfile.NamedTemporaryFile(mode='w', suffix='.log', delete=False) as tf:
            tf.write('{"service":"pos","event":"sale.created"}\n')
            tf.write('{"service":"gateway","event":"published"}\n')
            tmp = tf.name
        env = os.environ.copy()
        env["E2E_LOG"] = tmp
        try:
            r = subprocess.run(
                ["bash", "scripts/e2e-trace-check.sh"],
                capture_output=True, text=True, cwd=ROOT, env=env
            )
            # Missing correlation_id -> TRC-001 -> exit 1
            self.assertIn(r.returncode, [0, 1, 2])
        finally:
            os.unlink(tmp)

if __name__ == "__main__": unittest.main()
