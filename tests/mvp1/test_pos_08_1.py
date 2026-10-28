"""POS-08.1 — RabbitMQ topology: fiscal-results queue has x-dead-letter-exchange."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS = ROOT / '../POS'
CONSUMER_SRC = POS / 'pos-gateway/internal/fiscal/result_consumer.go'
TOPOLOGY_SCRIPT = POS / 'scripts/check-rabbit-topology.sh'


class TestPos081(unittest.TestCase):
    def test_check_rabbit_topology_script_exists(self):
        self.assertTrue(TOPOLOGY_SCRIPT.exists(),
                        f'check-rabbit-topology.sh not found at {TOPOLOGY_SCRIPT}')

    def test_result_consumer_declares_dlx(self):
        self.assertTrue(CONSUMER_SRC.exists(),
                        f'result_consumer.go not found at {CONSUMER_SRC}')
        content = CONSUMER_SRC.read_text()
        self.assertIn('x-dead-letter-exchange', content,
                      'result_consumer.go must declare x-dead-letter-exchange on the fiscal-results queue')

    def test_result_consumer_declares_dlq(self):
        content = CONSUMER_SRC.read_text()
        self.assertIn('dlq', content.lower(),
                      'result_consumer.go must declare a DLQ')

    def test_check_rabbit_topology_passes_source_check(self):
        result = subprocess.run(
            ['bash', str(TOPOLOGY_SCRIPT)],
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(
            result.returncode, 0,
            'check-rabbit-topology.sh failed:\n' + result.stdout + '\n' + result.stderr,
        )
        combined = result.stdout + result.stderr
        self.assertIn('OK', combined,
                      'check-rabbit-topology.sh did not print OK')

    def test_result_consumer_test_nack_for_invalid_envelope(self):
        """Source-level: result_consumer_test.go verifies Nack(false, false) for invalid envelopes."""
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestInvalidEnvelopeGoesToDLQNotRequeue',
             '-count=1', './internal/fiscal/...'],
            cwd=str(POS / 'pos-gateway'),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-2000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined,
                      'TestInvalidEnvelopeGoesToDLQNotRequeue did not pass')


if __name__ == '__main__':
    unittest.main()
