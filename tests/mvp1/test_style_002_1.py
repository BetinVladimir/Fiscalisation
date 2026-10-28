"""STYLE-002.1 — docs/CONTRIBUTING.md must exist and contain 'Language policy' section."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
CONTRIBUTING = ROOT / "docs" / "CONTRIBUTING.md"


class TestStyle002LanguagePolicy(unittest.TestCase):
    def test_contributing_md_exists(self):
        self.assertTrue(CONTRIBUTING.exists(), f"docs/CONTRIBUTING.md not found at {CONTRIBUTING}")

    def test_language_policy_section_present(self):
        self.assertTrue(CONTRIBUTING.exists(), f"docs/CONTRIBUTING.md not found at {CONTRIBUTING}")
        content = CONTRIBUTING.read_text()
        count = content.count("Language policy")
        self.assertGreaterEqual(count, 1,
            f"'Language policy' must appear at least once in docs/CONTRIBUTING.md, found {count}")

    def test_language_policy_grep(self):
        """Validate via subprocess grep matching the DoD command."""
        result = subprocess.run(
            ["grep", "-c", "Language policy", str(CONTRIBUTING)],
            capture_output=True, text=True
        )
        count = int(result.stdout.strip()) if result.stdout.strip() else 0
        self.assertGreaterEqual(count, 1,
            f"grep -c 'Language policy' returned {count}; expected >= 1")


if __name__ == "__main__":
    unittest.main()
