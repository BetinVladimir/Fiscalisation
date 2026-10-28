"""S3FIX-04.1 — Individual card payments.
Two CARDs with different payment_ids get separate RRN/AuthID/STAN;
partial failure and restart; successful purchase/refund not repeated.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX04_1(unittest.TestCase):
    def test_individual_card_payments_no_repeat(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_storage_host.sh')],
            capture_output=True, text=True, timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'run_storage_host.sh failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Two separate card payments with individual RRN/AuthID/STAN
        self.assertIn('cards PASS', output)
        # After restart, successful card payment is NOT re-executed
        self.assertIn('recovery-preserve-cards PASS', output)
        # Normal IntentProcessor+SQLite flow also passes
        self.assertIn('normal PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-04-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_04_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
