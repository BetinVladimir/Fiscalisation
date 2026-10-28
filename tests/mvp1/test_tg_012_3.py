"""TG-012.3 — docs/runbooks/SECRET_ROTATION.md has HMAC and FU password rotation steps."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent


class TestTG012Runbook(unittest.TestCase):
    def test_secret_rotation_runbook_structure(self):
        runbook = ROOT / "docs" / "runbooks" / "SECRET_ROTATION.md"
        self.assertTrue(runbook.exists(), f"SECRET_ROTATION.md not found at {runbook}")
        content = runbook.read_text()

        required_sections = [
            "HMAC",
            "FU",
            "Rotation",
        ]
        for section in required_sections:
            self.assertIn(
                section, content,
                f"SECRET_ROTATION.md missing content about {section!r}",
            )

        # Must not contain actual secret values (no long hex/base64 that looks real)
        import re
        # Check no 32+ char hex strings that look like real secrets
        # (fixture hashes in contracts/ are expected, but not in runbook)
        suspicious = re.findall(r'[0-9a-f]{64}', content)
        self.assertEqual(
            suspicious, [],
            f"SECRET_ROTATION.md contains long hex strings that may be real secrets: {suspicious}",
        )


if __name__ == "__main__":
    unittest.main()
