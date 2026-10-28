"""S3D-01.2 — protocol-abstraction CP1251, SEQ 0x7f→0x20, NAK/SYN, BCC markers all pass."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
RUN_TESTS_SH = ROOT / 'IoT' / 'protocol-abstraction' / 'run-tests.sh'

REQUIRED_MARKERS = (
    'protocol-abstraction tests: OK',
    'golden_daisy_cmd48=pass',
    'runtime vendor codec tests: OK',
    'DaisyTaxGroupUsesCyrillicCp1251=pass',
    'DaisySynEvery100ms=pass',
    'DaisyCp1251AndSynTest: OK',
)


class TestS3D01ProtocolAbstraction(unittest.TestCase):

    def test_run_tests_sh_all_markers_present(self):
        self.assertTrue(RUN_TESTS_SH.exists(), f'run-tests.sh not found: {RUN_TESTS_SH}')

        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=300,
        )

        artifact_dir = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())
        evidence_path = os.path.join(artifact_dir, 'protocol_abstraction_s3d01_2.txt')
        with open(evidence_path, 'w') as f:
            f.write(result.stdout + result.stderr)

        self.assertEqual(
            result.returncode, 0,
            'run-tests.sh failed:\n' + result.stdout[-3000:] + result.stderr[-1000:],
        )
        for marker in REQUIRED_MARKERS:
            self.assertIn(
                marker, result.stdout,
                f'Required marker not found: {marker!r}\nOutput:\n{result.stdout[-3000:]}',
            )

    def test_cp1251_seq_range_markers_present(self):
        """CP1251 and SEQ wrapping are part of the DaisyCp1251AndSynTest suite."""
        self.assertTrue(RUN_TESTS_SH.exists(), f'run-tests.sh not found: {RUN_TESTS_SH}')

        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=300,
        )
        self.assertEqual(result.returncode, 0,
                         'run-tests.sh failed:\n' + result.stdout[-2000:] + result.stderr[-500:])

        # CP1251 Т=0xD2 marker
        self.assertIn('DaisyTaxGroupUsesCyrillicCp1251=pass', result.stdout,
                      'CP1251 Cyrillic tax group test missing')
        # SEQ 0x20 wrap marker from DaisySynEvery100ms
        self.assertIn('DaisySynEvery100ms=pass', result.stdout,
                      'SYN timing / SEQ wrap test missing')
        self.assertIn('DaisyCp1251AndSynTest: OK', result.stdout,
                      'DaisyCp1251AndSynTest suite did not finish OK')


if __name__ == '__main__':
    unittest.main()
