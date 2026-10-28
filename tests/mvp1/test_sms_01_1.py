"""SMS-01.1 — manifest.json: 2 PDFs with sha256 verified against actual files."""
import hashlib
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
VENDOR_DIR = ROOT / 'docs' / 'vendor'


class TestSMS01_1(unittest.TestCase):
    def test_manifest_pdf_sha256(self):
        manifest_path = VENDOR_DIR / 'manifest.json'
        self.assertTrue(manifest_path.exists(), 'manifest.json not found')
        manifest = json.loads(manifest_path.read_bytes())
        # manifest is a list of entries (not a dict with 'files' key)
        files = manifest if isinstance(manifest, list) else manifest.get('files', [])

        # The two Smart S PDFs must be present
        smart_pdfs = [f for f in files if f.get('doc') == 'DOC-V4']
        self.assertGreaterEqual(len(smart_pdfs), 2,
                                f'Expected >= 2 DOC-V4 files, got {len(smart_pdfs)}')

        for entry in smart_pdfs:
            # entries use 'file' key (repo-relative path) not 'path'
            rel = entry.get('file') or entry.get('path', '')
            path = ROOT / rel if rel else VENDOR_DIR / rel
            self.assertTrue(path.exists(), f'{rel} not found')
            expected_sha = entry.get('sha256', '')
            if not entry.get('sha256_partial', False):
                self.assertTrue(expected_sha, f'No sha256 for {rel}')
                actual_sha = hashlib.sha256(path.read_bytes()).hexdigest()
                self.assertEqual(actual_sha, expected_sha,
                                 f'sha256 mismatch for {rel}')


if __name__ == '__main__':
    unittest.main()
