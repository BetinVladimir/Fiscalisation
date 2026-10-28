"""POS-06.1 — fiscalStore state transitions: FISCALIZED / UNKNOWN / REJECTED."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestPos061(unittest.TestCase):
    def test_fiscal_store_state_transitions(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'stores/fiscalStore',
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
        # Verify UNKNOWN blocking test was exercised
        self.assertIn('UNKNOWN', combined,
                      'UNKNOWN state not mentioned in test output')
        # Verify state transition tests ran
        import re
        m = re.search(r'Tests:\s+(\d+) passed', combined)
        self.assertIsNotNone(m, 'No test count found')
        self.assertGreaterEqual(int(m.group(1)), 6,
                                'Expected ≥6 state transition tests')


if __name__ == '__main__':
    unittest.main()
