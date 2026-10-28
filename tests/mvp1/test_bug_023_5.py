"""BUG-023.5 — POS roundingGolden.test.ts passes with Jest."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_APP = ROOT.parent / 'POS/pos-app'
TEST_FILE = POS_APP / 'src/__tests__/roundingGolden.test.ts'


class TestBUG0235(unittest.TestCase):
    def test_pos_rounding_golden(self):
        self.assertTrue(POS_APP.exists(), f'POS app not found: {POS_APP}')
        self.assertTrue(TEST_FILE.exists(), f'roundingGolden.test.ts not found: {TEST_FILE}')
        result = subprocess.run(
            ['npx', 'jest', 'src/__tests__/roundingGolden.test.ts', '--no-coverage'],
            cwd=str(POS_APP),
            capture_output=True,
            text=True,
            timeout=120,
        )
        output = result.stdout + result.stderr
        self.assertEqual(
            result.returncode, 0,
            'Jest roundingGolden.test.ts failed:\n' + output[-4000:],
        )
        self.assertIn('pass', output.lower(), 'Jest output must contain pass')

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'native-output'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'bug023_5.stdout.log').write_text(output)


if __name__ == '__main__':
    unittest.main()
