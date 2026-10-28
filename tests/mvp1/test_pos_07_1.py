"""POS-07.1 — Installation mode: AUTHORIZED_TEST rejected in PROD."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
POS_GATEWAY = ROOT / '../POS/pos-gateway'


class TestPos071(unittest.TestCase):
    def test_startup_rejects_authorized_test_in_prod(self):
        result = subprocess.run(
            ['go', 'test', '-v', '-run', 'TestStartupRejectsAuthorizedTestInProd',
             '-count=1', './internal/handler/...'],
            cwd=str(POS_GATEWAY),
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(
            result.returncode, 0,
            'go test rc=' + str(result.returncode) + '\n' + result.stdout[-3000:] + '\n' + result.stderr[-1000:],
        )
        combined = result.stdout + result.stderr
        self.assertIn('PASS', combined,
                      'TestStartupRejectsAuthorizedTestInProd did not pass')
        self.assertIn('TestStartupRejectsAuthorizedTestInProd', combined,
                      'TestStartupRejectsAuthorizedTestInProd not found in test output')

    def test_authorized_test_branch_present_in_handler(self):
        handler_src = POS_GATEWAY / 'internal/handler/fiscal_handler.go'
        self.assertTrue(handler_src.exists(), f'{handler_src} not found')
        content = handler_src.read_text()
        self.assertIn('AUTHORIZED_TEST', content,
                      'AUTHORIZED_TEST check missing from fiscal_handler.go')
        self.assertIn('423', content,
                      '423 status code missing from fiscal_handler.go AUTHORIZED_TEST branch')


if __name__ == '__main__':
    unittest.main()
