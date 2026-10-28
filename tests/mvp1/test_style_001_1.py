"""STYLE-001.1 — .golangci.yml exists; check-lint-rules.sh enforces money-float rule."""
import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
GOLANGCI_YML = ROOT / "fiscal-backend" / ".golangci.yml"
LINT_RULES_SCRIPT = ROOT / "scripts" / "check-lint-rules.sh"
TESTDATA_LINT = ROOT / "fiscal-backend" / "testdata" / "lint"
VIOLATION_FILE = TESTDATA_LINT / "money_float_violation.go"

REQUIRED_YML_KEYWORDS = ["golangci", "linters", "govet"]


class TestStyle001LintRules(unittest.TestCase):

    def test_golangci_yml_exists(self):
        self.assertTrue(GOLANGCI_YML.exists(),
                        f"fiscal-backend/.golangci.yml not found at {GOLANGCI_YML}")

    def test_golangci_yml_not_empty(self):
        self.assertTrue(GOLANGCI_YML.exists())
        content = GOLANGCI_YML.read_text()
        self.assertGreater(len(content.strip()), 0, ".golangci.yml must not be empty")

    def test_golangci_yml_has_linters_section(self):
        self.assertTrue(GOLANGCI_YML.exists())
        content = GOLANGCI_YML.read_text()
        self.assertIn("linters", content, ".golangci.yml must contain 'linters' section")

    def test_lint_rules_script_exists(self):
        self.assertTrue(LINT_RULES_SCRIPT.exists(),
                        f"check-lint-rules.sh not found at {LINT_RULES_SCRIPT}")

    def test_testdata_lint_dir_exists(self):
        self.assertTrue(TESTDATA_LINT.is_dir(),
                        f"fiscal-backend/testdata/lint/ not found at {TESTDATA_LINT}")

    def test_violation_file_exists(self):
        self.assertTrue(VIOLATION_FILE.exists(),
                        f"money_float_violation.go not found at {VIOLATION_FILE}")

    def test_violation_file_contains_float64(self):
        """The sentinel file must actually contain the forbidden float64 pattern."""
        self.assertTrue(VIOLATION_FILE.exists())
        content = VIOLATION_FILE.read_text()
        self.assertIn("float64", content,
                      "money_float_violation.go must contain 'float64' as the forbidden pattern")

    def test_lint_script_exits_0_on_clean_code(self):
        """check-lint-rules.sh must exit 0 on the current codebase (no violations in domain)."""
        result = subprocess.run(
            ["bash", str(LINT_RULES_SCRIPT)],
            capture_output=True, text=True
        )
        self.assertEqual(
            result.returncode, 0,
            f"check-lint-rules.sh failed on clean codebase:\n{result.stdout}\n{result.stderr}"
        )

    def test_violation_file_is_not_in_domain(self):
        """The testdata/lint/ directory must NOT be in fiscal-backend/internal/domain/
        (it lives in testdata/ so the main lint check won't pick it up)."""
        domain_dir = ROOT / "fiscal-backend" / "internal" / "domain"
        # violation file should be in testdata, not domain
        self.assertFalse(
            (domain_dir / "money_float_violation.go").exists(),
            "Violation sentinel file must not be in internal/domain/"
        )
        self.assertTrue(
            VIOLATION_FILE.is_relative_to(TESTDATA_LINT),
            "Violation file must be in testdata/lint/"
        )

    def test_grep_detects_violation_in_sentinel(self):
        """Direct grep on sentinel file must find float64."""
        result = subprocess.run(
            ["grep", "-n", "float64", str(VIOLATION_FILE)],
            capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0,
                         "grep for float64 in sentinel file returned no matches")
        self.assertIn("float64", result.stdout)


if __name__ == "__main__":
    unittest.main()
