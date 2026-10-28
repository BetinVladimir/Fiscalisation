"""TST-POS-04.1 — empty UNP: FISCALIZED with unp="" → 0 print calls; field independence."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT / '../POS/pos-app'


class TestTstPos041(unittest.TestCase):
    def test_empty_unp_no_print(self):
        result = subprocess.run(
            ['npx', 'jest', '--no-coverage',
             '--testPathPattern', 'src/__tests__/tst-pos-04',
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
        self.assertIn('unp', combined.lower(),
                      'UNP not mentioned in test output')
        self.assertRegex(combined, r'Tests:\s+\d+ passed',
                         'No test count found')


if __name__ == '__main__':
    unittest.main()
