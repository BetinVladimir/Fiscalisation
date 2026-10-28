"""TST-POS-02.1 — routing table: 8 methods → correct base URL; one base → fallback."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestTstPos021(unittest.TestCase):
    def test_routing_table(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'src/__tests__/tst-pos-02',
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
        self.assertRegex(combined, r'Tests:\s+\d+ passed',
                         'No test count found in jest output')
        # At least 8 routing tests (5 fiscal + 3 gateway)
        import re
        m = re.search(r'Tests:\s+(\d+) passed', combined)
        self.assertIsNotNone(m, 'Could not find Tests: X passed')
        self.assertGreaterEqual(int(m.group(1)), 8,
                                'Expected ≥8 routing tests, got ' + m.group(1))


if __name__ == '__main__':
    unittest.main()
