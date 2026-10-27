"""BUG-061.2 — No Number(price|amount|unitPrice|qty) in non-display production TS source."""
import re
import unittest
from pathlib import Path

POS_SRC = Path('/Users/freelancer/Documents/Beeloy/POS/pos-app/src')

# Patterns that are acceptable uses of Number() (display only)
ALLOWED_FILES = {
    'money.ts',  # Intl.NumberFormat display only
}
# Pattern: Number( followed by a money-related identifier
MONEY_PATTERN = re.compile(r'Number\s*\(\s*\w*(?:price|amount|qty|unit|total)\w*', re.IGNORECASE)


class TestBUG061_2(unittest.TestCase):
    def test_no_number_for_money_in_production_code(self):
        if not POS_SRC.exists():
            self.skipTest(f'POS src not found at {POS_SRC}')

        violations = []
        for ts_file in POS_SRC.rglob('*.ts'):
            # Skip test files, node_modules, store-build
            parts = ts_file.parts
            if any(p in ('.store-build', 'node_modules', '__mocks__') for p in parts):
                continue
            if ts_file.name.endswith('.test.ts') or ts_file.name.endswith('.spec.ts'):
                continue
            if ts_file.name in ALLOWED_FILES:
                continue
            try:
                content = ts_file.read_text(encoding='utf-8')
            except OSError:
                continue
            for lineno, line in enumerate(content.splitlines(), start=1):
                if MONEY_PATTERN.search(line):
                    violations.append(f'{ts_file.relative_to(POS_SRC)}:{lineno}: {line.strip()[:120]}')

        self.assertEqual(
            violations, [],
            'Number() used for money calculation in production code:\n' + '\n'.join(violations),
        )


if __name__ == '__main__':
    unittest.main()
