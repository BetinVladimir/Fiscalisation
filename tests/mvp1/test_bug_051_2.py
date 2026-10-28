"""BUG-051.2 — HttpFiscalApiClient uses fiscalBase/gatewayBase constructor."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG051_2(unittest.TestCase):
    def test_routing_jest_passes(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'HttpFiscalApiClient.routing', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}')

    def test_constructor_signature_uses_two_bases(self):
        """Verify Adapters.ts constructor has fiscalBase and gatewayBase params."""
        adapters = POS / 'src/infrastructure/runtime/Adapters.ts'
        if not adapters.exists():
            self.skipTest(f'{adapters} not found')
        content = adapters.read_text(encoding='utf-8')
        self.assertIn('fiscalBase', content,
            'HttpFiscalApiClient should have fiscalBase param')
        self.assertIn('gatewayBase', content,
            'HttpFiscalApiClient should have gatewayBase param')
        self.assertNotIn("constructor(private base: string)", content,
            'Old single-base constructor should be replaced')


if __name__ == '__main__':
    unittest.main()
