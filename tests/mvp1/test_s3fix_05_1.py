"""S3FIX-05.1 — Immutable sync batch and canonical hash.
Same C++/Go bytes/hash; in-flight batch ID/hash/content unchanged on new events,
reboot and retransmit; wrong ACK deletes nothing.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX05_1(unittest.TestCase):
    def test_sync_batch_immutable_and_ack_fault(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_sync_host.sh')],
            capture_output=True, text=True, timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'sync_host tests failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Normal batch: same hash on restart
        self.assertIn('normal PASS', output)
        # Duplicate ACK is idempotent — in-flight content unchanged
        self.assertIn('duplicate PASS', output)
        # Wrong (stale/mismatched) ACK deletes nothing
        self.assertIn('ack-fault PASS', output)
        # Changed-row guard: content differs from in-flight hash → not deleted
        self.assertIn('changed-row PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-05-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_05_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
