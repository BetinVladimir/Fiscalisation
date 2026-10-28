"""PIPE-02.1 — docs/MVP1/pipeline/ must contain all 4 prompt template files."""
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
PIPELINE = ROOT / "docs" / "MVP1" / "pipeline"
REQUIRED_FILES = [
    "qwen_code.md",
    "codex_review.md",
    "qwen_fix.md",
    "codex_e2e.md",
]
# Each file must be <= 800 tokens (approx 3200 chars at 4 chars/token)
MAX_CHARS = 3200


class TestPipe02PipelineTemplates(unittest.TestCase):
    def test_pipeline_dir_exists(self):
        self.assertTrue(PIPELINE.is_dir(), f"docs/MVP1/pipeline/ not found at {PIPELINE}")

    def test_all_four_files_present(self):
        for fname in REQUIRED_FILES:
            p = PIPELINE / fname
            self.assertTrue(p.exists(), f"Missing pipeline file: {p}")

    def test_files_not_empty(self):
        for fname in REQUIRED_FILES:
            p = PIPELINE / fname
            if p.exists():
                content = p.read_text()
                self.assertGreater(len(content.strip()), 0, f"{fname} is empty")

    def test_files_within_token_budget(self):
        """Each file must be <= 800 tokens (~3200 chars as a proxy)."""
        for fname in REQUIRED_FILES:
            p = PIPELINE / fname
            if p.exists():
                chars = len(p.read_text())
                self.assertLessEqual(
                    chars, MAX_CHARS,
                    f"{fname} is {chars} chars; exceeds ~800-token budget ({MAX_CHARS} chars)"
                )


if __name__ == "__main__":
    unittest.main()
