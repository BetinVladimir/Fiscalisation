"""DSY-08.2 — CurrencyParamGate: non-EUR device returns DeviceUnavailable before I/O."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY08_2(unittest.TestCase):
    def test_currency_param_gate(self):
        script = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests/run_native_saga.sh'
        if not script.exists():
            self.skipTest(f'Script not found: {script}')
        r = subprocess.run(
            ['bash', str(script)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(
            r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-3000:]}\n{r.stderr[-1000:]}',
        )
        self.assertIn(
            'CurrencyParamGate: PASS', r.stdout,
            f'CurrencyParamGate test not found in output:\n{r.stdout}',
        )


if __name__ == '__main__':
    unittest.main()
