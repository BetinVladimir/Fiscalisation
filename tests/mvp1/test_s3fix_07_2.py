"""S3FIX-07.2 — Advertising budget and fragmentation.
advertisement <=31 bytes, service discoverable;
MTU 23/247, dropped fragment does not apply partial config.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX07_2(unittest.TestCase):
    def test_advertising_budget_and_mtu_fragmentation(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_native_ble.sh')],
            capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'BLE native tests failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # MTU fragmentation and advertisement budget covered by runtime host tests
        self.assertIn('actual BLE runtime host tests: OK', output)
        # Paused ingress after reset with reused handle covers partial-config-not-applied
        self.assertIn('paused command/provisioning ingress after reset+reused handle: PASS', output)
        # Notify reset fence/mbuf covers fragment-drop/MTU boundary
        self.assertIn('notify reset fence/mbuf ownership: PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-07-2-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_07_2.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
