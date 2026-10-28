"""VG-12.2 — ZTotalsStoredAndReconciled; exports.go ValidateZTotals."""
import subprocess, unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SMARTDEVICES = ROOT / 'SmartDevices'
BACKEND = ROOT / 'fiscal-backend'

class TestVG12_2(unittest.TestCase):
    def test_z_totals_stored(self):
        r = subprocess.run(
            ['./gradlew', ':bluecash-app:testProdDebugUnitTest', '--no-daemon',
             '--tests', 'com.beeloy.fiscal.bluecash.DailyReportTest.ZTotalsStoredAndReconciled'],
            cwd=str(SMARTDEVICES), capture_output=True, text=True, timeout=300)
        self.assertEqual(r.returncode, 0,
            f'rc={r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-1000:]}')
        self.assertIn('BUILD SUCCESSFUL', r.stdout + r.stderr)

    def test_exports_go_compiles(self):
        r = subprocess.run(
            ['go', 'build', './internal/domain/'],
            cwd=str(BACKEND), capture_output=True, text=True, timeout=120)
        self.assertEqual(r.returncode, 0,
            f'exports.go compile failed:\n{r.stderr}')

    def test_exports_go_has_required_symbols(self):
        exports_path = BACKEND / 'internal/domain/exports.go'
        self.assertTrue(exports_path.exists(), f'Missing: {exports_path}')
        src = exports_path.read_text(encoding='utf-8')
        self.assertIn('ErrZMismatch', src)
        self.assertIn('ZReportTotals', src)
        self.assertIn('ValidateZTotals', src)

if __name__ == '__main__':
    unittest.main()
