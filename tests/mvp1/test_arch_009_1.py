"""ARCH-009.1 — unified contract schema: make contract-test passes and JSON schemas are valid."""
import json
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRACTS = ROOT / 'contracts'
MAKEFILE = ROOT / 'Makefile'

SCHEMA_FILES = [
    'openapi-runtime-v1.yaml',
    'openapi-corrections-v1.yaml',
]

JSON_FILES = [
    'bg-unp-v1-golden.json',
    'ble-crypto-golden-v1.json',
    'money-rounding-golden-v1.json',
    'mvp-acceptance-v1.json',
]


class TestARCH009_1(unittest.TestCase):
    def test_makefile_has_contract_test_target(self):
        content = MAKEFILE.read_text()
        self.assertIn('contract-test:', content, 'Makefile must have contract-test: target')

    def test_json_contract_files_are_valid_json(self):
        for name in JSON_FILES:
            path = CONTRACTS / name
            if not path.exists():
                continue
            with self.subTest(file=name):
                try:
                    json.loads(path.read_text())
                except json.JSONDecodeError as e:
                    self.fail(f'{name} is not valid JSON: {e}')

    def test_contract_test_make_target_passes(self):
        result = subprocess.run(
            ['make', 'contract-test'],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=180,
        )
        self.assertEqual(
            result.returncode, 0,
            'make contract-test failed:\n' + result.stdout[-3000:] + '\n' + result.stderr[-2000:],
        )
        self.assertIn('OK', result.stdout + result.stderr,
                      'contract-test produced no OK marker in output')

    def test_generated_contracts_dir_exists(self):
        generated = CONTRACTS / 'generated'
        self.assertTrue(generated.is_dir(), 'contracts/generated/ directory must exist')
        ts_files = list(generated.glob('*.ts'))
        self.assertGreater(len(ts_files), 0, 'contracts/generated/ must contain at least one .ts file')


if __name__ == '__main__':
    unittest.main()
