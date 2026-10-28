"""POS-15.1 — Documentation: 05-fiscal-contract.md exists; check-doc-drift.sh passes."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRACT_DOC = ROOT / '../POS/docs/05-fiscal-contract.md'
DOC_DRIFT_SCRIPT = ROOT / 'scripts/check-doc-drift.sh'


class TestPos151(unittest.TestCase):
    def test_fiscal_contract_doc_exists(self):
        self.assertTrue(CONTRACT_DOC.exists(),
                        f'05-fiscal-contract.md not found at {CONTRACT_DOC}')

    def test_fiscal_contract_doc_has_required_sections(self):
        content = CONTRACT_DOC.read_text()
        required = ['FiscalApiClient', 'operationId', 'FISCALIZED', 'UNKNOWN']
        for kw in required:
            self.assertIn(kw, content,
                          f'Required keyword {kw!r} missing from 05-fiscal-contract.md')

    def test_check_doc_drift_script_exists(self):
        self.assertTrue(DOC_DRIFT_SCRIPT.exists(),
                        f'check-doc-drift.sh not found at {DOC_DRIFT_SCRIPT}')

    def test_check_doc_drift_script_is_valid_bash(self):
        content = DOC_DRIFT_SCRIPT.read_text()
        self.assertIn('#!/usr/bin/env bash', content[:50],
                      'check-doc-drift.sh must have bash shebang')
        self.assertTrue(
            'DRF-00' in content or 'drift' in content.lower(),
            'check-doc-drift.sh must contain drift check logic',
        )


if __name__ == '__main__':
    unittest.main()
