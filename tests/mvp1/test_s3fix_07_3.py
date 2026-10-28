"""S3FIX-07.3 — Session/owner/origin isolation.
Two concurrent centrals, foreign proof/origin, stale nonce/replay:
unauthorized execute==0, valid flow works.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX07_3(unittest.TestCase):
    def test_session_owner_origin_isolation(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_native_ble.sh')],
            capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'BLE native tests failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Stale generation / foreign disconnect: unauthorized execute rejected
        self.assertIn('reset/handle-token/foreign-disconnect admission: PASS', output)
        # Overflow-connect: second concurrent central admission
        self.assertIn('actual BLE runtime host tests: OK', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-07-3-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_07_3.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
