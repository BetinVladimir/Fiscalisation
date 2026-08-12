import tempfile
import unittest
from pathlib import Path
from beefiscal_factory.evidence import Evidence, sha256_file

class EvidenceTest(unittest.TestCase):
    def test_evidence_is_deterministic(self):
        with tempfile.TemporaryDirectory() as directory:
            value=Evidence("S3-1","B1","R1","1.0","a"*64,{"kty":"EC"},"station","2026-01-01T00:00:00Z")
            self.assertEqual(len(value.digest),64)
            target=Path(directory)/"evidence.json";value.save(target)
            self.assertIn(value.digest,target.read_text())
    def test_file_hash(self):
        with tempfile.TemporaryDirectory() as directory:
            target=Path(directory)/"firmware.bin";target.write_bytes(b"firmware")
            self.assertEqual(sha256_file(target),"c3bf47ea1f4a4a605470313cacb3a44f4a461f68c6faeab07e737610cb5ac835")

if __name__ == "__main__": unittest.main()
