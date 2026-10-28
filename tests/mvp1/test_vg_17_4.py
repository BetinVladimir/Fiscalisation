"""VG-17.4: make iot-protocol-test must exit 0.

Acceptance: make iot-protocol-test runs all four protocol module test suites
(daisy, datecs, datecspay, protocol-abstraction) and all pass.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MAKEFILE = ROOT / 'Makefile'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


class TestVG17_4(unittest.TestCase):

    def test_makefile_has_iot_protocol_test_target(self):
        """Makefile must define iot-protocol-test target."""
        self.assertTrue(MAKEFILE.exists(), f'Makefile not found: {MAKEFILE}')
        content = MAKEFILE.read_text()
        self.assertIn('iot-protocol-test:', content,
                      'Makefile must define iot-protocol-test target')

    def test_iot_protocol_test_runs_protocol_abstraction(self):
        """iot-protocol-test target must invoke protocol-abstraction/run-tests.sh."""
        self.assertTrue(MAKEFILE.exists(), f'Makefile not found: {MAKEFILE}')
        content = MAKEFILE.read_text()
        self.assertIn('protocol-abstraction/run-tests.sh', content,
                      'iot-protocol-test must run IoT/protocol-abstraction/run-tests.sh')

    def test_make_iot_protocol_test_exits_zero(self):
        """make iot-protocol-test must exit 0 (all module tests pass)."""
        result = subprocess.run(
            ['make', 'iot-protocol-test'],
            capture_output=True, text=True, timeout=300,
            cwd=str(ROOT),
        )
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0,
                         f'make iot-protocol-test failed (rc={result.returncode}):\n{combined}')

    def test_make_iot_protocol_test_has_daisy_cp1251_named_tests(self):
        """make iot-protocol-test output must include DaisyTaxGroupUsesCyrillicCp1251 and DaisySynEvery100ms."""
        result = subprocess.run(
            ['make', 'iot-protocol-test'],
            capture_output=True, text=True, timeout=300,
            cwd=str(ROOT),
        )
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0,
                         f'make iot-protocol-test failed:\n{combined}')
        self.assertIn('DaisyTaxGroupUsesCyrillicCp1251=pass', combined,
                      'DaisyTaxGroupUsesCyrillicCp1251 must pass in make iot-protocol-test')
        self.assertIn('DaisySynEvery100ms=pass', combined,
                      'DaisySynEvery100ms must pass in make iot-protocol-test')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_17_4.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
