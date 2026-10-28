"""BUG-053.2 — normalizeStatus.test.ts passes: 3 fields independent."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG053_2(unittest.TestCase):
    def test_normalize_status_jest(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'normalizeStatus', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}')
        import re
        m = re.search(r'Tests:\s+(\d+)\s+passed', r.stdout + r.stderr)
        count = int(m.group(1)) if m else 0
        self.assertGreaterEqual(count, 5, f'Expected at least 5 normalization tests, got {count}')

    def test_no_unp_fallback_in_adapters(self):
        """Ensure the old unp ?? fiscalReference pattern is gone."""
        adapters = POS / 'src/infrastructure/runtime/Adapters.ts'
        if not adapters.exists():
            self.skipTest(f'{adapters} not found')
        content = adapters.read_text(encoding='utf-8')
        self.assertNotIn('unp ?? result.fiscalReference', content,
            'normalizeStatus must not use unp ?? fiscalReference fallback')
        self.assertNotIn('unp ?? result.fiscal_reference', content,
            'normalizeStatus must not use unp ?? fiscal_reference fallback')


if __name__ == '__main__':
    unittest.main()
