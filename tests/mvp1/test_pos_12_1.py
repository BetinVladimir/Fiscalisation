"""POS-12.1 — Offline queue: no print before FISCALIZED+UNP confirmation."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestPos121(unittest.TestCase):
    def test_offline_queue_no_print_before_confirmation(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'offlineQueue.order',
             '--verbose'],
            cwd=str(POS_APP),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'jest rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined, 'No PASS in jest output')
        self.assertIn('print', combined.lower(),
                      '"print" not mentioned in test output')
        import re
        m = re.search(r'Tests:\s+(\d+) passed', combined)
        self.assertIsNotNone(m, 'No test count found')
        self.assertGreaterEqual(int(m.group(1)), 3,
                                'Expected ≥3 offline queue tests')


if __name__ == '__main__':
    unittest.main()
