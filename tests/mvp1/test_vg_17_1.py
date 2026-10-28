"""VG-17.1: Daisy CP1251 protocol — frame format, CP1251 tax groups, SYN/NAK.

Frame: 01 LEN SEQ CMD DATA 05 BCC 03
Tax groups: Cyrillic А–З (CP1251 0xC0..0xC7)
PC timeout ≥ 500 ms; device response ≤ 100 ms; SYN every 100 ms resets timeout.
"""
import os
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RUN_TESTS_SH = ROOT / 'IoT/protocol-abstraction/run-tests.sh'

ARTIFACT_DIR = os.environ.get('MVP1_ARTIFACT_DIR')


class TestVG17_1(unittest.TestCase):

    def test_run_tests_sh_exists(self):
        """Protocol test runner script must exist."""
        self.assertTrue(RUN_TESTS_SH.exists(), f'run-tests.sh not found: {RUN_TESTS_SH}')

    def test_protocol_tests_pass(self):
        """IoT/protocol-abstraction/run-tests.sh must exit 0 with non-trivial output."""
        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            capture_output=True, text=True, timeout=180,
            cwd=str(ROOT),
        )
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0,
                         f'run-tests.sh failed (rc={result.returncode}):\n{combined}')
        # Must produce non-trivial output confirming tests ran
        self.assertIn('OK', combined,
                      'run-tests.sh must print OK to confirm tests completed')

    def test_daisy_frame_format_confirmed(self):
        """run-tests.sh output must confirm the golden Daisy frame test ran."""
        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            capture_output=True, text=True, timeout=180,
            cwd=str(ROOT),
        )
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0,
                         f'run-tests.sh failed:\n{combined}')
        # The RuntimeCodecVendorTest includes literal vendor bytes from the spec
        self.assertIn('runtime vendor codec tests: OK', combined,
                      'runtime vendor codec tests must pass (confirms frame format)')

    def test_daisy_cp1251_tax_group_test_runs(self):
        """DaisyTaxGroupUsesCyrillicCp1251 named test must appear and pass."""
        result = subprocess.run(
            ['bash', str(RUN_TESTS_SH)],
            capture_output=True, text=True, timeout=180,
            cwd=str(ROOT),
        )
        combined = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0,
                         f'run-tests.sh failed:\n{combined}')
        self.assertIn('DaisyTaxGroupUsesCyrillicCp1251=pass', combined,
                      'DaisyTaxGroupUsesCyrillicCp1251 must be present and pass')

    def test_daisy_protocol_constants_defined(self):
        """DaisyProtocol.h must define DAISY_SYN=0x16 and DAISY_TAX_BASE=0xC0."""
        proto_h = ROOT / 'IoT/common-modules/daisy/DaisyProtocol.h'
        self.assertTrue(proto_h.exists(), f'DaisyProtocol.h not found: {proto_h}')
        content = proto_h.read_text()
        # DAISY_SYN
        self.assertRegex(content, r'#define\s+DAISY_SYN\s+0x16',
                         'DAISY_SYN must be defined as 0x16')
        # DAISY_TAX_BASE
        self.assertRegex(content, r'#define\s+DAISY_TAX_BASE\s+0xC0',
                         'DAISY_TAX_BASE must be defined as 0xC0 (CP1251 Cyrillic А)')
        # DAISY_TIMEOUT_MS ≥ 500
        match = __import__('re').search(r'#define\s+DAISY_TIMEOUT_MS\s+(\d+)', content)
        self.assertIsNotNone(match, 'DAISY_TIMEOUT_MS must be defined')
        timeout_ms = int(match.group(1))
        self.assertGreaterEqual(timeout_ms, 500,
                                f'DAISY_TIMEOUT_MS must be ≥ 500 ms; found {timeout_ms}')


if __name__ == '__main__':
    import sys
    result = unittest.main(exit=False, verbosity=2)
    if ARTIFACT_DIR:
        out = Path(ARTIFACT_DIR)
        out.mkdir(parents=True, exist_ok=True)
        (out / 'vg_17_1.log').write_text(
            f'ran={result.result.testsRun} failures={len(result.result.failures)} '
            f'errors={len(result.result.errors)}'
        )
    sys.exit(0 if result.result.wasSuccessful() else 1)
