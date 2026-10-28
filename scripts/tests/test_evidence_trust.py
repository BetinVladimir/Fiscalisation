"""EVIDENCE-01.1 — factory manifest image-type and hash validation; evidence key integrity."""
import hashlib
import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).parent.parent.parent
sys.path.insert(0, str(ROOT / "IoT" / "device-manufacturing" / "src"))

from beefiscal_factory.evidence import Evidence
from beefiscal_factory.flash_manifest import load_manifest


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


class TestEvidenceKeyIntegrity(unittest.TestCase):
    def _make_evidence(self, key_x="unit-key-fixture"):
        return Evidence(
            serial="SN-FIXTURE-001",
            batch="BATCH-2026-01",
            hardware_revision="R1",
            firmware_version="1.0.0",
            firmware_sha256="a" * 64,
            device_public_key_jwk={"kty": "OKP", "crv": "Ed25519", "x": key_x},
            manufacturing_station_id="STATION-1",
            captured_at="2026-01-01T00:00:00Z",
        )

    def test_valid_fixture_passes(self):
        ev = self._make_evidence()
        self.assertEqual(len(ev.digest), 64)
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "evidence.json"
            ev.save(p)
            saved = json.loads(p.read_text())
            self.assertEqual(saved["registration_evidence_sha256"], ev.digest)

    def test_replaced_key_fails(self):
        ev_original = self._make_evidence(key_x="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
        ev_replaced = self._make_evidence(key_x="BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=")
        # A replaced key produces a different digest — tampered evidence is detected.
        self.assertNotEqual(ev_original.digest, ev_replaced.digest)
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "evidence.json"
            ev_original.save(p)
            saved = json.loads(p.read_text())
            # The stored digest must not match what a replaced-key Evidence would compute.
            self.assertNotEqual(saved["registration_evidence_sha256"], ev_replaced.digest)


class TestManifestImageType(unittest.TestCase):
    def _write_manifest(self, path: Path, mode: str) -> str:
        doc = {
            "schema_version": 1,
            "chip": "esp32s3",
            "mode": mode,
            "flash_size_bytes": 1048576,
            "segments": [],
            "partition_layout": {},
        }
        raw = json.dumps(doc, sort_keys=True).encode()
        path.write_bytes(raw)
        return _sha256(raw)

    def test_broken_hash_fails(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "manifest.json"
            self._write_manifest(p, "segments")
            wrong_sha = "0" * 64
            with self.assertRaises(ValueError) as ctx:
                load_manifest(p, wrong_sha)
            self.assertIn("SHA-256", str(ctx.exception))

    def test_wrong_image_type_fails(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "manifest.json"
            sha = self._write_manifest(p, "unsupported_mode")
            with self.assertRaises(ValueError) as ctx:
                load_manifest(p, sha)
            self.assertIn("mode", str(ctx.exception))

    def test_valid_mode_passes_type_gate(self):
        """All three valid modes pass the image-type gate (further segment checks may follow)."""
        with tempfile.TemporaryDirectory() as td:
            for mode in ("segments", "app_only", "merged"):
                p = Path(td) / f"manifest_{mode}.json"
                sha = self._write_manifest(p, mode)
                try:
                    load_manifest(p, sha)
                except ValueError as e:
                    self.assertNotIn(
                        "explicit artifact mode required", str(e),
                        f"mode={mode!r} incorrectly rejected at type gate: {e}",
                    )


if __name__ == "__main__":
    unittest.main()
