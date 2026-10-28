"""S3FIX-02.1 — Atomic terminal state + outbox.
SQLite transaction commits terminal+outbox together.
FAIL_AT on each write/commit: after restart both present or both absent.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestS3FIX02_1(unittest.TestCase):
    def test_atomic_terminal_outbox(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_durable_storage_native.sh')],
            capture_output=True, text=True, timeout=300,
        )
        self.assertEqual(
            result.returncode, 0,
            'durable_storage_native failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        output = result.stdout
        # Core S3FIX-02 assertion: outbox-fault rolls back both tables atomically
        self.assertIn('atomic-terminal-outbox PASS', output)
        # Baseline: storage error still stops fiscal I/O
        self.assertIn('storage-error-stops-io PASS', output)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 's3fix-02-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 's3fix_02_1.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
