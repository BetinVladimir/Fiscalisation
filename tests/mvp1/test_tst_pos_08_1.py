"""TST-POS-08.1 — result consumer: invalid envelope → DLQ; valid → applied; DB error → requeue."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_GATEWAY = ROOT / '../POS/pos-gateway'


class TestTstPos081(unittest.TestCase):
    def test_invalid_envelope_goes_to_dlq(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestInvalidEnvelope',
             '-count=1', './internal/fiscal/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'TestInvalidEnvelope* tests did not pass')

    def test_envelope_error_classification(self):
        result = subprocess.run(
            ['go', 'test', '-v',
             '-run', 'TestEnvelopeErrorClassification|TestApplyBadJSON|TestApplyValidEnvelope',
             '-count=1', './internal/fiscal/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-2000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'Envelope classification tests did not pass')

    def test_no_command_bindings_in_consumer(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestNoCommandBindings',
             '-count=1', './internal/fiscal/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-2000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'TestNoCommandBindings test did not pass')


if __name__ == '__main__':
    unittest.main()
