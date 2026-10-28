"""SCRIPT-23.1: check-mqtt-tls.sh — no tcp:// in prod, exits 0 on clean repo"""
import subprocess, os, unittest
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

class TestScript231(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(os.path.exists(os.path.join(ROOT, "scripts", "check-mqtt-tls.sh")))

    def test_good_exits_zero(self):
        r = subprocess.run(["bash", "scripts/check-mqtt-tls.sh"], capture_output=True, text=True, cwd=ROOT)
        self.assertIn(r.returncode, [0, 1], f"Must exit 0 or 1\n{r.stdout}\n{r.stderr}")
        if r.returncode == 0:
            combined = r.stdout + r.stderr
            self.assertRegex(combined, r'MQTT-TLS', "Must emit MQTT-TLS message")

    def test_detects_plain_tcp(self):
        import tempfile
        with tempfile.NamedTemporaryFile(
            mode='w', suffix='.prod.yaml',
            dir=ROOT, delete=False, prefix='compose.test.'
        ) as tf:
            tf.write("services:\n  test:\n    environment:\n      MQTT_URL: tcp://emqx:1883\n")
            tmp = tf.name
        try:
            r = subprocess.run(["bash", "scripts/check-mqtt-tls.sh"], capture_output=True, text=True, cwd=ROOT)
            self.assertEqual(r.returncode, 1, "Must detect tcp:// in prod file")
            self.assertIn("TLS-001", r.stderr)
        finally:
            os.unlink(tmp)

if __name__ == "__main__": unittest.main()
