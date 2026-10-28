"""VG-17.3: DaisyTaxGroupUsesCyrillicCp1251 and DaisySynEvery100ms tests pass.

Both named tests must appear in run-tests.sh output with =pass.
Uses IoT/protocol-abstraction/run-tests.sh as the native test runner.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RUN_TESTS_SH = ROOT / 'IoT/protocol-abstraction/run-tests.sh'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


class TestVG17_3(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        """Run the test suite once and cache output."""
        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            capture_output=True, text=True, timeout=180,
            cwd=str(ROOT),
        )
        cls._output = result.stdout + result.stderr
        cls._returncode = result.returncode

    def test_run_tests_exits_zero(self):
        """run-tests.sh must exit 0."""
        self.assertEqual(self._returncode, 0,
                         f'run-tests.sh failed (rc={self._returncode}):\n{self._output}')

    def test_daisy_tax_group_uses_cyrillic_cp1251(self):
        """DaisyTaxGroupUsesCyrillicCp1251=pass must appear in output."""
        self.assertIn('DaisyTaxGroupUsesCyrillicCp1251=pass', self._output,
                      f'DaisyTaxGroupUsesCyrillicCp1251 must pass.\n'
                      f'Output:\n{self._output}')

    def test_daisy_syn_every_100ms(self):
        """DaisySynEvery100ms=pass must appear in output."""
        self.assertIn('DaisySynEvery100ms=pass', self._output,
                      f'DaisySynEvery100ms must pass.\n'
                      f'Output:\n{self._output}')

    def test_both_tests_pass(self):
        """Both named tests must pass together (not one without the other)."""
        self.assertIn('DaisyTaxGroupUsesCyrillicCp1251=pass', self._output)
        self.assertIn('DaisySynEvery100ms=pass', self._output)

    def test_golden_daisy_cmd48_also_passes(self):
        """Golden Daisy frame test (golden_daisy_cmd48) must also pass (fixture TST-VENDOR-04)."""
        self.assertIn('golden_daisy_cmd48=pass', self._output,
                      'Golden Daisy cmd48 test must pass; fixture daisy_golden_frames.json required')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_17_3.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
