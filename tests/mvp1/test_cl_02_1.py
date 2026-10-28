"""CL-02.1 — WebBleRoute: UNKNOWN result → next submit skips send (sendCalls==1)."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
MINIPOSWEB = ROOT / 'minipos/miniposweb'


class TestCL02_1(unittest.TestCase):
    def test_unknown_no_resend(self):
        config = MINIPOSWEB / 'src/fiscal/vitest-cl02.config.ts'
        if not config.exists():
            self.skipTest(f'Vitest config not found: {config}')
        r = subprocess.run(
            ['npx', 'vitest', 'run', '--config', 'src/fiscal/vitest-cl02.config.ts'],
            cwd=str(MINIPOSWEB),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            r.returncode, 0,
            f'vitest exited {r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )
        self.assertIn(
            'Tests  2 passed', r.stdout,
            f'Expected 2 passing tests:\n{r.stdout}',
        )


if __name__ == '__main__':
    unittest.main()
