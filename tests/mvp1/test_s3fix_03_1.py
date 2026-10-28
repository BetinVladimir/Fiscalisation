"""S3FIX-03.1 — Recovery UNP ownership: foreign receipt not cancelled, matching UNP calls io."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh'


class TestS3FIX03_1(unittest.TestCase):
    def test_recovery_ownership_and_foreign_receipt(self):
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True, timeout=180,
        )
        combined = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            f'run_native_saga.sh failed (rc={result.returncode}):\n{combined[:3000]}',
        )
        self.assertIn(
            'recovery_ownership_matrix: PASS', combined,
            'Native binary must print recovery_ownership_matrix: PASS; '
            f'stdout:\n{result.stdout[:2000]}',
        )

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out = Path(artifact_dir) / 's3fix-evidence'
            out.mkdir(parents=True, exist_ok=True)
            (out / 's3fix_03_1.log').write_text(combined[:8000])


if __name__ == '__main__':
    unittest.main()
