"""NAP-06.3 — Combined: check-prod-artifact.sh exit 0 + HIL_RUNBOOK.md «Место проведения» ≥1."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
SCRIPT = ROOT / 'scripts' / 'check-prod-artifact.sh'
HIL_RUNBOOK = ROOT / 'docs' / 'MVP1' / 'HIL_RUNBOOK.md'


class TestNAP06Combined(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def _save(self, name: str, content: str):
        with open(os.path.join(self._artifact, name), 'w') as f:
            f.write(content)

    def test_prod_artifact_and_runbook_combined(self):
        """check-prod-artifact.sh exit 0 AND HIL_RUNBOOK.md contains «Место проведения»."""
        # Part 1: script
        self.assertTrue(SCRIPT.exists(), f'check-prod-artifact.sh not found: {SCRIPT}')
        r = subprocess.run(
            ['bash', str(SCRIPT)],
            cwd=str(ROOT),
            capture_output=True,
            text=True,
            timeout=120,
        )
        self._save('nap_06_3_combined.txt', r.stdout + r.stderr)
        self.assertEqual(r.returncode, 0,
                         f'check-prod-artifact.sh failed:\n{r.stdout}\n{r.stderr}')
        self.assertIn('PROD-ARTIFACT OK', r.stdout, 'Missing PROD-ARTIFACT OK marker')

        # Part 2: runbook section
        self.assertTrue(HIL_RUNBOOK.exists(), f'HIL_RUNBOOK.md not found: {HIL_RUNBOOK}')
        text = HIL_RUNBOOK.read_text(encoding='utf-8')
        count = text.count('Место проведения')
        with open(os.path.join(self._artifact, 'nap_06_3_runbook.txt'), 'w') as f:
            f.write(f'Место проведения occurrences: {count}\n')
        self.assertGreaterEqual(count, 1,
                                'HIL_RUNBOOK.md missing «Место проведения» section')

    def test_hil_procedure_notification_documented(self):
        """HIL_RUNBOOK.md must document НАП notification procedure (Прил.32 п.20)."""
        self.assertTrue(HIL_RUNBOOK.exists(), f'HIL_RUNBOOK.md not found: {HIL_RUNBOOK}')
        text = HIL_RUNBOOK.read_text(encoding='utf-8')
        # The notification requirement (Прил.32 / уведомление НАП) must be referenced
        has_notification = 'уведомление' in text or 'Прил. 32' in text or 'Прил.32' in text
        self._save('nap_06_3_notification.txt',
                   f'notification reference found: {has_notification}\n')
        self.assertTrue(has_notification,
                        'HIL_RUNBOOK.md must reference уведомление НАП (Прил.32 п.20)')


if __name__ == '__main__':
    unittest.main()
