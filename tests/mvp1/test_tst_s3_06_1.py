"""TST-S3-06.1 — Power-loss at every write boundary (S3FIX-01 dependency).

Verifies: fault injection at each boundary (reserve→execute→outcome→outbox)
produces a valid state {not started, RECOVERY_REQUIRED, terminal};
executor never called twice; UNP not reused.
Wraps run_storage_host.sh which exercises the checkpoint/outbox/receipt/payment
fault modes and recovery scenarios.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestTSTS306_1(unittest.TestCase):
    def test_power_loss_boundaries(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_storage_host.sh')],
            capture_output=True, text=True, timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'run_storage_host.sh failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout

        # Fault at RESERVED→EXECUTING boundary: executor not called, state=RESERVED
        self.assertIn('checkpoint PASS', output)
        # Fault at EXECUTING→outbox boundary: state=EXECUTING, not re-executed on recovery
        self.assertIn('outbox PASS', output)
        # Fault during receipt update: state=EXECUTING
        self.assertIn('receipt PASS', output)
        # Fault during payment update: state=EXECUTING
        self.assertIn('payment PASS', output)
        # Recovery after EXECUTING: committed exactly once, not twice
        self.assertIn('recovery-normal PASS', output)
        # Storage error during recovery is contained
        self.assertIn('recovery-storage-error PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'tst-s3-06-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'tst_s3_06_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
