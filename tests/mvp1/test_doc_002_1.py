"""DOC-002.1 — 6 ADR files exist with required structure; check-adr-links.sh passes."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
DECISIONS = ROOT / 'docs/decisions'
SCRIPT = ROOT / 'scripts/check-adr-links.sh'
REQUIRED_ADRS = [
    'ADR-MONEY-ROUNDING-001.md',
    'ADR-PERSIST-001.md',
    'ADR-SAGA-001.md',
    'ADR-UNKNOWN-OUTCOME-001.md',
    'ADR-TIME-BUSINESS-DATE-001.md',
    'ADR-PAYMENT-CAPABILITY-001.md',
]
REQUIRED_SECTION_PAIRS = [
    ('## Контекст', '## Context'),
    ('## Решение', '## Decision'),
    ('## Последствия', '## Consequences'),
]


class TestDoc002(unittest.TestCase):
    def test_decisions_dir_exists(self):
        self.assertTrue(DECISIONS.is_dir(), f'docs/decisions/ not found: {DECISIONS}')

    def test_all_required_adrs_exist(self):
        for adr in REQUIRED_ADRS:
            path = DECISIONS / adr
            self.assertTrue(path.exists(), f'Missing ADR: {adr}')

    def test_adr_files_have_required_sections(self):
        for adr in REQUIRED_ADRS:
            path = DECISIONS / adr
            if not path.exists():
                continue
            text = path.read_text(encoding='utf-8')
            for ru_section, en_section in REQUIRED_SECTION_PAIRS:
                self.assertTrue(
                    ru_section in text or en_section in text,
                    f'{adr} missing section: {ru_section!r} or {en_section!r}'
                )

    def test_check_adr_links_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f'Script not found: {SCRIPT}')

    def test_check_adr_links_passes(self):
        result = subprocess.run(
            ['bash', str(SCRIPT)],
            capture_output=True, text=True,
            cwd=str(ROOT),
        )
        self.assertEqual(result.returncode, 0,
                         f'check-adr-links.sh failed:\n{result.stderr}')
        self.assertIn('OK:', result.stdout)

    def test_adr_count_gte_6(self):
        count = len(list(DECISIONS.glob('ADR-*-001.md')))
        self.assertGreaterEqual(count, 6, f'Need >= 6 ADR-*-001.md files, found {count}')


if __name__ == '__main__':
    unittest.main()
