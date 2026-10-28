"""BUG-051.3 — DI container reads EXPO_PUBLIC_FISCAL_API_URL and EXPO_PUBLIC_GATEWAY_URL."""
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG051_3(unittest.TestCase):
    def test_container_reads_two_env_vars(self):
        container = POS / 'src/infrastructure/di/container.impl.ts'
        if not container.exists():
            self.skipTest(f'{container} not found')
        content = container.read_text(encoding='utf-8')
        self.assertIn('EXPO_PUBLIC_FISCAL_API_URL', content,
            'DI container should read EXPO_PUBLIC_FISCAL_API_URL')
        self.assertIn('EXPO_PUBLIC_GATEWAY_URL', content,
            'DI container should read EXPO_PUBLIC_GATEWAY_URL')
        # The client must be constructed with two args
        self.assertIn('new HttpFiscalApiClient(FISCAL, GATEWAY)', content,
            'HttpFiscalApiClient should receive both FISCAL and GATEWAY bases')

    def test_env_example_has_both_vars(self):
        env_example = Path('/Users/freelancer/Documents/Beeloy/POS/.env.example')
        if not env_example.exists():
            self.skipTest(f'{env_example} not found')
        content = env_example.read_text(encoding='utf-8')
        self.assertIn('EXPO_PUBLIC_GATEWAY_URL', content,
            '.env.example should document EXPO_PUBLIC_GATEWAY_URL')
        self.assertIn('EXPO_PUBLIC_FISCAL_API_URL', content,
            '.env.example should document EXPO_PUBLIC_FISCAL_API_URL')


if __name__ == '__main__':
    unittest.main()
