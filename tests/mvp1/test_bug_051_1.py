"""BUG-051.1 — HttpFiscalApiClient.routing.test.ts exists and passes."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG051_1(unittest.TestCase):
    def test_routing_test_passes(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'HttpFiscalApiClient.routing', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}',
        )
        # Verify at least 8 routing tests (5 fiscal + 3 gateway)
        import re
        m = re.search(r'Tests:\s+(\d+)\s+passed', r.stdout + r.stderr)
        count = int(m.group(1)) if m else 0
        self.assertGreaterEqual(count, 8,
            f'Expected at least 8 routing tests, got {count}')


if __name__ == '__main__':
    unittest.main()
