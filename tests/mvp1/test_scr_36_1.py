"""SCR-36.1 — check_plan_coverage.rb self-test: good fixture exit 0, bad fixture exit 1."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SELFTEST = ROOT / 'scripts' / 'tests' / 'test_check_plan_coverage.sh'
SCRIPT = ROOT / 'scripts' / 'check_plan_coverage.rb'
GOOD = ROOT / 'scripts' / 'tests' / 'fixtures' / 'check_plan_coverage' / 'good'
BAD = ROOT / 'scripts' / 'tests' / 'fixtures' / 'check_plan_coverage' / 'bad'


class TestSCR36_1(unittest.TestCase):
    def _rb(self, plan, coverage):
        r = subprocess.run(
            ['ruby', str(SCRIPT), '--plan', str(plan), '--coverage', str(coverage)],
            capture_output=True, text=True,
        )
        return r.returncode, r.stdout, r.stderr

    def test_good_fixture_exits_zero(self):
        rc, out, _ = self._rb(GOOD / 'plan.json', GOOD / 'plan_coverage.json')
        self.assertEqual(rc, 0, f'Expected exit 0 for good fixture, got {rc}\n{out}')
        self.assertIn('OK', out)

    def test_bad_fixture_exits_nonzero(self):
        rc, out, _ = self._rb(BAD / 'plan.json', BAD / 'plan_coverage.json')
        self.assertNotEqual(rc, 0, f'Expected nonzero exit for bad fixture, got {rc}\n{out}')
        self.assertIn('FAIL', out)

    def test_selftest_script_passes(self):
        self.assertTrue(SELFTEST.exists(), f'Self-test script not found: {SELFTEST}')
        r = subprocess.run(
            ['bash', str(SELFTEST)],
            capture_output=True, text=True,
        )
        self.assertEqual(
            r.returncode, 0,
            f'Self-test script failed (rc={r.returncode}):\n{r.stdout}\n{r.stderr}',
        )
        self.assertIn('2 passed', r.stdout, 'Expected 2 passed in self-test output')


if __name__ == '__main__':
    unittest.main()
