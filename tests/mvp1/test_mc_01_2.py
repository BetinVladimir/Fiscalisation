"""MC-01.2 — Go test verifies DER signatures against SPKI keys from fixture."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / 'fiscal-backend'


class TestMC01_2(unittest.TestCase):
    def test_backend_accepts_mobile_der_signature_vectors(self):
        r = subprocess.run(
            ['go', 'test', './internal/domain/...', '-run',
             'TestBackendAcceptsMobileDerSignatureVector|TestBackendRejectsP1363ForMobilePath',
             '-count=1', '-v'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120,
        )
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('TestBackendAcceptsMobileDerSignatureVector', r.stdout)
        self.assertIn('TestBackendRejectsP1363ForMobilePath', r.stdout)
        self.assertIn('PASS', r.stdout)


if __name__ == '__main__':
    unittest.main()
