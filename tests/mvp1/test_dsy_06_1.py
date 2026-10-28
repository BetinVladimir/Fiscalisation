"""DSY-06.1 — probe_results.json has PRB-07 VID/PID for Daisy Compact USB."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestDSY06_1(unittest.TestCase):
    def test_prb07_vid_pid(self):
        f = ROOT / 'docs/vendor/probe_results.json'
        self.assertTrue(f.exists(), f'File not found: {f}')
        data = json.loads(f.read_text())
        self.assertIn('PRB-07', data, f"Missing 'PRB-07' key; keys: {list(data.keys())}")
        prb = data['PRB-07']
        self.assertEqual(prb.get('vid'), '0483', f"vid mismatch: {prb}")
        self.assertEqual(prb.get('pid'), '5740', f"pid mismatch: {prb}")


if __name__ == '__main__':
    unittest.main()
