"""DSY-07.2 — protocol-abstraction tests pass and payment_slots_daisy.json is consistent."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY07_2(unittest.TestCase):
    def test_protocol_abstraction_suite(self):
        script = ROOT / 'IoT/protocol-abstraction/run-tests.sh'
        if not script.exists():
            self.skipTest(f'run-tests.sh not found: {script}')
        r = subprocess.run(
            ['bash', str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'Protocol abstraction tests failed rc={r.returncode}\n'
            f'{r.stdout[-2000:]}\n{r.stderr[-1000:]}',
        )

    def test_slots_consistent_with_fixture(self):
        """Fixture codes match expected Daisy protocol payment type codes."""
        f = ROOT / 'docs/MVP1/fixtures/payment_slots_daisy.json'
        self.assertTrue(f.exists(), f'Fixture missing: {f}')
        data = json.loads(f.read_text())
        slots = {s['type']: s['code'] for s in data.get('slots', [])}
        # Daisy CASH = 'P', CARD = 'N' (per protocol spec §7 payment types)
        self.assertEqual(slots.get('CASH'), 'P', f'CASH code must be P, got {slots}')
        self.assertEqual(slots.get('CARD'), 'N', f'CARD code must be N, got {slots}')


if __name__ == '__main__':
    unittest.main()
