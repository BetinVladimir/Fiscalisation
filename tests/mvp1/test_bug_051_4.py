"""BUG-051.4 — .env.example has both EXPO_PUBLIC_FISCAL_API_URL and EXPO_PUBLIC_GATEWAY_URL."""
import unittest
from pathlib import Path


class TestBUG051_4(unittest.TestCase):
    def test_pos_env_example_has_fiscal_api_url(self):
        env_example = Path('/Users/freelancer/Documents/Beeloy/POS/.env.example')
        if not env_example.exists():
            self.skipTest(f'{env_example} not found')
        content = env_example.read_text(encoding='utf-8')
        self.assertIn('EXPO_PUBLIC_FISCAL_API_URL', content)
        self.assertIn('EXPO_PUBLIC_GATEWAY_URL', content)

    def test_adapters_no_longer_uses_single_base(self):
        adapters = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app/src/infrastructure/runtime/Adapters.ts')
        if not adapters.exists():
            self.skipTest(f'{adapters} not found')
        content = adapters.read_text(encoding='utf-8')
        # The old single-base pattern should be gone
        self.assertNotIn('constructor(private base: string)', content,
            'Single-base constructor should be replaced')
        self.assertIn('fiscalBase', content)
        self.assertIn('gatewayBase', content)


if __name__ == '__main__':
    unittest.main()
