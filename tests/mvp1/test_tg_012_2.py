"""TG-012.2 — scripts/check-no-secrets.sh exits 0 on clean repo;
              scripts/check-mqtt-tls.sh runs without error."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG012Scripts(unittest.TestCase):
    def test_check_no_secrets_exits_zero(self):
        script = ROOT / "scripts" / "check-no-secrets.sh"
        self.assertTrue(script.exists(), f"check-no-secrets.sh not found at {script}")
        result = subprocess.run(
            ["bash", str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            "check-no-secrets.sh rc=" + str(result.returncode) +
            "\nstdout: " + result.stdout[-2000:] +
            "\nstderr: " + result.stderr[-1000:],
        )
        self.assertIn("OK", result.stdout + result.stderr,
                      "Expected 'OK' in check-no-secrets.sh output")

    def test_check_mqtt_tls_script_exists_and_runs(self):
        script = ROOT / "scripts" / "check-mqtt-tls.sh"
        self.assertTrue(script.exists(), f"check-mqtt-tls.sh not found at {script}")
        # Script exits 0 when no prod MQTT configs exist (nothing to check)
        result = subprocess.run(
            ["bash", str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=60,
        )
        # Exit code 0 = ok; exit code 3 = no infra (also acceptable for CI without real MQTT)
        self.assertIn(result.returncode, (0, 3),
                      f"check-mqtt-tls.sh unexpected rc={result.returncode}\n"
                      + result.stdout[-2000:] + "\n" + result.stderr[-1000:])


if __name__ == "__main__":
    unittest.main()
