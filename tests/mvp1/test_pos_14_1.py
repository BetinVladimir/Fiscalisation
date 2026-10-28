"""POS-14.1 — Correlation/observability: e2e-trace-check.sh verifies operationId in logs."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
TRACE_SCRIPT = ROOT / 'scripts/e2e-trace-check.sh'


class TestPos141(unittest.TestCase):
    def test_trace_check_script_exists(self):
        self.assertTrue(TRACE_SCRIPT.exists(),
                        f'e2e-trace-check.sh not found at {TRACE_SCRIPT}')

    def test_trace_check_script_has_shebang(self):
        content = TRACE_SCRIPT.read_text()
        self.assertIn('#!/usr/bin/env bash', content[:50],
                      'e2e-trace-check.sh must have bash shebang')

    def test_trace_check_script_selftest_passes(self):
        result = subprocess.run(
            ['bash', str(TRACE_SCRIPT), '--selftest'],
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(
            result.returncode, 0,
            'e2e-trace-check.sh --selftest failed:\n' + result.stdout + '\n' + result.stderr,
        )
        combined = result.stdout + result.stderr
        self.assertIn('ok', combined.lower(),
                      'e2e-trace-check.sh --selftest did not print ok')

    def test_trace_check_script_references_correlation(self):
        content = TRACE_SCRIPT.read_text()
        self.assertTrue(
            'correlation_id' in content or 'operationId' in content or 'correlation' in content,
            'e2e-trace-check.sh must check for correlation_id or operationId',
        )


if __name__ == '__main__':
    unittest.main()
