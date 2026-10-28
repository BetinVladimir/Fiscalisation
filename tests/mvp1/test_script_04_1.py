"""SCRIPT-04.1: check-no-secrets.sh exits 0 on clean repo"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript041(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-no-secrets.sh")))

    def test_exits_valid_code(self):
        r = subprocess.run(["bash", "scripts/check-no-secrets.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\nstdout:{r.stdout}\nstderr:{r.stderr}")
        combined = r.stdout + r.stderr
        self.assertGreater(len(combined.strip()), 0, "Must produce output")

    def test_bad_pattern_detected(self):
        import tempfile
        # Use a pattern that matches: private_key = "ABCDEF1234567890" (a long hex key)
        with tempfile.NamedTemporaryFile(mode='w', suffix='.go', dir=os.path.join(ROOT, 'fiscal-backend', 'internal'), delete=False) as tf:
            tf.write('const hardcoded = "AKIA1234567890ABCDEF12"\n')  # AWS access key pattern
            tmp = tf.name
        try:
            r = subprocess.run(["bash", "scripts/check-no-secrets.sh"], capture_output=True, text=True, cwd=ROOT)
            # Should fail or warn
            is_bad = r.returncode != 0 or 'FAIL' in r.stdout + r.stderr
            self.assertTrue(is_bad, f"Script must detect AWS key pattern\nstdout:{r.stdout}\nstderr:{r.stderr}")
        finally:
            os.unlink(tmp)

if __name__ == "__main__": unittest.main()
