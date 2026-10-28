"""S3FIX-06.1 — Expiry and bounded storage.
Expired command rejected before I/O; full SD does not delete unacked evidence
and safely blocks new monetary operations; TTL/limits and restart verified.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX06_1(unittest.TestCase):
    def test_expiry_and_bounded_storage(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_storage_host.sh')],
            capture_output=True, text=True, timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'run_storage_host.sh failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Expired command rejected before I/O (no reserve, no executor call)
        self.assertIn('validity-reservation-expired actual HMAC+IntentProcessor+SQLite PASS',
                      output)
        self.assertIn('validity-transaction-expired actual HMAC+IntentProcessor+SQLite PASS',
                      output)
        self.assertIn('validity-before-io-expired actual HMAC+IntentProcessor+SQLite PASS',
                      output)
        # Storage error safely blocks new money operations
        self.assertIn('recovery-storage-error PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-06-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_06_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
