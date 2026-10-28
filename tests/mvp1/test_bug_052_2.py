"""BUG-052.2 — reconcileOperation key is ${id}-reconcile-${attemptNo}."""
import subprocess
import unittest
from pathlib import Path

POS = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app')


class TestBUG052_2(unittest.TestCase):
    def test_reconcile_uses_attempt_no_key(self):
        if not POS.exists():
            self.skipTest(f'POS not found at {POS}')
        r = subprocess.run(
            ['npx', 'jest', 'ReconcileAttempt', '--no-coverage'],
            cwd=str(POS), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'Jest rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-2000:]}')

    def test_reconcile_method_signature_accepts_attempt_no(self):
        fiscal_client = POS / 'src/infrastructure/fiscal/FiscalApiClient.ts'
        if not fiscal_client.exists():
            self.skipTest(f'{fiscal_client} not found')
        content = fiscal_client.read_text(encoding='utf-8')
        self.assertIn('reconcileOperation(operationId: string, attemptNo?: number)', content,
            'FiscalApiClient.reconcileOperation must accept optional attemptNo')

    def test_adapters_uses_attempt_numbered_key(self):
        adapters = POS / 'src/infrastructure/runtime/Adapters.ts'
        if not adapters.exists():
            self.skipTest(f'{adapters} not found')
        content = adapters.read_text(encoding='utf-8')
        self.assertIn('reconcile-${attemptNo}', content,
            'reconcileOperation must use attemptNo in Idempotency-Key')


if __name__ == '__main__':
    unittest.main()
