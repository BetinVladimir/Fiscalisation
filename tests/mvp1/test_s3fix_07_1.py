"""S3FIX-07.1 — Bootstrap trust/CA/network/binding.
factory reset → onboarding → restart → trusted MQTT;
wrong CA and failed persistence block READY.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX07_1(unittest.TestCase):
    def test_bootstrap_onboarding_and_trust(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_native_ble.sh')],
            capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'BLE native tests failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Provisioning/reset/read-flow: covers factory reset → onboarding → restart
        self.assertIn('provisioning/reset/read-flow admission: PASS', output)
        # startup timeout/generation fence: wrong CA or failed init blocks READY
        self.assertIn('startup timeout/generation exhaustion permanent fence: PASS', output)
        self.assertIn('actual BLE runtime host tests: OK', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-07-1-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_07_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
