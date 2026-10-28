import hashlib
import json
import struct
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from beefiscal_factory.bundle import create_bundle
from beefiscal_factory.flash_manifest import load_manifest

class BundleTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.boot = self.root/'boot'; self.boot.write_bytes(b'\xe9' + b'B'*31)
        self.app = self.root/'app'; self.app.write_bytes(b'\xe9' + b'A'*31)
        self.table = self.root/'table'
        entry = struct.pack('<HBBII16sI',0x50aa,0,0,0x20000,0x10000,b'factory',0)
        self.table.write_bytes(entry+b'\xff'*32)
    def create(self, name='bundle'):
        return create_bundle(self.root/name,self.boot,self.table,self.app,1024*1024,0x8000,'factory')
    def test_roundtrip_nonstandard_app_offset_and_source_independence(self):
        with patch('subprocess.run') as run:
            manifest,digest=self.create();run.assert_not_called()
        self.app.write_bytes(b'changed')
        segments=load_manifest(manifest,digest)
        self.assertEqual([offset for _,offset,_ in segments],[0,0x8000,0x20000])
        self.assertEqual(segments[-1][2],b'\xe9'+b'A'*31)
        self.assertEqual((manifest.parent/'manifest.sha256').read_text(),digest+'  manifest.json\n')
    def test_reproducible_and_no_overwrite(self):
        a,ha=self.create('a');b,hb=self.create('b')
        self.assertEqual(ha,hb);self.assertEqual(a.read_bytes(),b.read_bytes())
        with self.assertRaises(ValueError):self.create('a')
        self.assertEqual(hashlib.sha256(a.read_bytes()).hexdigest(),ha)
    def test_invalid_sources_never_publish_bundle(self):
        self.app.write_bytes(b'not-an-ESP-image')
        with self.assertRaises(ValueError):self.create()
        self.assertFalse((self.root/'bundle').exists())
        self.assertEqual(list(self.root.glob('.beefiscal-bundle-*')),[])
    def test_corruption_detected_after_generation(self):
        path,digest=self.create();(path.parent/'application.bin').write_bytes(b'bad')
        with self.assertRaisesRegex(ValueError,'SHA-256'):load_manifest(path,digest)
