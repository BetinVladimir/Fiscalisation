"""MC-01.1 — execution_signature_vectors.json has >=3 entries with message/spki/signature_der."""
import json, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


class TestMC01_1(unittest.TestCase):
    def test_fixture_has_at_least_3_vectors(self):
        f = ROOT / 'minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json'
        self.assertTrue(f.exists(), f'File not found: {f}')
        data = json.loads(f.read_text())
        self.assertGreaterEqual(len(data), 3, f'Expected >=3 vectors, got {len(data)}')
        for i, v in enumerate(data):
            self.assertIn('message', v, f'Vector {i} missing "message"')
            self.assertIn('spki', v, f'Vector {i} missing "spki"')
            self.assertIn('signature_der', v, f'Vector {i} missing "signature_der"')
            self.assertTrue(v['message'], f'Vector {i} has empty message')
            self.assertTrue(v['spki'], f'Vector {i} has empty spki')
            self.assertTrue(v['signature_der'], f'Vector {i} has empty signature_der')


if __name__ == '__main__':
    unittest.main()
