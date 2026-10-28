"""EMU-06.1 — FakePeripheral BLE: MTU 23/247, fragmented send; transport S8 pass."""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
TESTS = ROOT / 'IoT/firmware/edge-agent-s3/idf/tests'


class TestEMU06_1(unittest.TestCase):
    def test_ble_transport_fragmentation_s8(self):
        result = subprocess.run(
            ['bash', str(TESTS / 'run_native_ble.sh')],
            capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(
            result.returncode, 0,
            'BLE native tests failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        # S8: fragmented notification reassembly and MTU boundary correctness
        self.assertIn('actual BLE runtime host tests: OK', result.stdout)
        # S8 sub-scenarios: notify reset fence covers mbuf ownership on fragment boundary
        self.assertIn('notify reset fence/mbuf ownership: PASS', result.stdout)
        # Provisioning fragmentation at MTU 23 (smallest legal MTU)
        self.assertIn('provisioning/reset/read-flow admission: PASS', result.stdout)

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR')
        if artifact_dir:
            out_dir = Path(artifact_dir) / 'emu-06-evidence'
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / 'emu_06_1.log').write_text(result.stdout)


if __name__ == '__main__':
    unittest.main()
