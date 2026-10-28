"""NAP-06.1 — HIL_RUNBOOK.md must have non-empty «Место проведения» section (п.20, Прил.32)."""
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
HIL_RUNBOOK = ROOT / 'docs' / 'MVP1' / 'HIL_RUNBOOK.md'


class TestNAP06HilRunbookLocation(unittest.TestCase):

    def setUp(self):
        self._artifact = os.environ.get('MVP1_ARTIFACT_DIR', tempfile.mkdtemp())

    def test_hil_runbook_exists(self):
        """HIL_RUNBOOK.md must exist."""
        self.assertTrue(HIL_RUNBOOK.exists(), f'HIL_RUNBOOK.md not found: {HIL_RUNBOOK}')

    def test_mesto_provede_section_present(self):
        """«Место проведения» section is present (at least one occurrence)."""
        self.assertTrue(HIL_RUNBOOK.exists(), f'HIL_RUNBOOK.md not found: {HIL_RUNBOOK}')
        text = HIL_RUNBOOK.read_text(encoding='utf-8')
        count = text.count('Место проведения')
        with open(os.path.join(self._artifact, 'nap_06_1_runbook_grep.txt'), 'w') as f:
            f.write(f'grep count of "Место проведения": {count}\n')
            f.write(text[:500])
        self.assertGreaterEqual(count, 1,
                                'HIL_RUNBOOK.md has no «Место проведения» section — required by NAP-06.1')

    def test_mesto_provede_section_nonempty(self):
        """Section body after «Место проведения» header must have non-trivial content."""
        self.assertTrue(HIL_RUNBOOK.exists(), f'HIL_RUNBOOK.md not found: {HIL_RUNBOOK}')
        text = HIL_RUNBOOK.read_text(encoding='utf-8')
        idx = text.find('Место проведения')
        self.assertGreaterEqual(idx, 0, 'No «Место проведения» in HIL_RUNBOOK.md')
        # Content after the header line (next ~500 chars) must have ≥ 20 non-whitespace chars
        body = text[idx + len('Место проведения'):idx + 600].strip()
        non_ws = len(body.replace(' ', '').replace('\n', '').replace('\t', ''))
        self.assertGreater(non_ws, 20,
                           f'«Место проведения» section body is nearly empty (non-ws chars: {non_ws})')


if __name__ == '__main__':
    unittest.main()
