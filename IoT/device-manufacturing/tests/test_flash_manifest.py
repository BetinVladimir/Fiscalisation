import argparse
import hashlib
import json
import struct
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from beefiscal_factory import cli

class FlashManifestTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        entry = struct.pack('<HBBII16sI', 0x50AA, 0, 0, 0x10000, 0x10000, b'factory', 0)
        table = entry + b'\xeb\xeb' + b'\xff' * 14 + hashlib.md5(entry).digest() + b'\xff' * 32
        self.record('table.bin', table)
        self.record('boot.bin', b'\xe9' + b'B' * 31)
        self.record('app.bin', b'\xe9' + b'A' * 31)
        self.manifest = {'schema_version': 1, 'chip': 'esp32s3', 'mode': 'segments', 'flash_size_bytes': 1024*1024,
            'partition_layout': dict(self.record('table.bin', table), table_offset=0x8000, app_label='factory'),
            'segments': [dict(self.record('boot.bin'), kind='bootloader', offset=0), dict(self.record('table.bin'), kind='partition_table', offset=0x8000), dict(self.record('app.bin'), kind='application', offset=0x10000)]}
    def record(self, name, data=None):
        p = self.root/name
        if data is not None: p.write_bytes(data)
        return {'path': name, 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
    def arguments(self):
        path = self.root/'manifest.json'
        path.write_text(json.dumps(self.manifest))
        return argparse.Namespace(manifest=str(path), expected_manifest_sha256=hashlib.sha256(path.read_bytes()).hexdigest(), port='MOCK-ONLY', firmware=str(self.root/'app.bin'), expected_sha256=hashlib.sha256((self.root/'app.bin').read_bytes()).hexdigest())
    def reject(self):
        with patch.object(cli.subprocess, 'run') as run:
            with self.assertRaises(SystemExit): cli.flash(self.arguments())
            run.assert_not_called()
    def test_actual_cli_passes_verified_segments_and_cleans_staging(self):
        paths = []
        def capture(command, check):
            self.assertTrue(check)
            self.assertEqual(command[:6], ['esptool.py','--chip','esp32s3','--port','MOCK-ONLY','write_flash'])
            self.assertEqual(command[6::2], ['0x0','0x8000','0x10000'])
            for p, record in zip(command[7::2], self.manifest['segments']):
                paths.append(p)
                self.assertEqual(Path(p).read_bytes(), (self.root/record['path']).read_bytes())
        with patch.object(cli.subprocess, 'run', side_effect=capture) as run:
            cli.flash(self.arguments()); run.assert_called_once()
        self.assertTrue(all(not Path(p).exists() for p in paths))
    def test_app_only_uses_verified_nonzero_offset(self):
        self.manifest['mode']='app_only';self.manifest['segments']=self.manifest['segments'][2:]
        with patch.object(cli.subprocess,'run') as run:
            cli.flash(self.arguments());self.assertEqual(run.call_args.args[0][6], '0x10000')
    def test_merged_requires_explicit_zero_and_embedded_layout(self):
        data=bytearray(b'\xff'*0x10020);data[:32]=(self.root/'boot.bin').read_bytes();table=(self.root/'table.bin').read_bytes();data[0x8000:0x8000+len(table)]=table;data[0x10000:]=(self.root/'app.bin').read_bytes()
        self.manifest['mode']='merged';self.manifest['segments']=[dict(self.record('merged.bin',data),kind='merged',offset=0)]
        with patch.object(cli.subprocess,'run') as run:cli.flash(self.arguments());self.assertEqual(run.call_args.args[0][6],'0x0')
        self.manifest['segments'][0]['offset']=0x1000;self.reject()
    def test_manifest_and_artifact_hash_mismatch(self):
        args=self.arguments();args.expected_manifest_sha256='0'*64
        with patch.object(cli.subprocess,'run') as run:
            with self.assertRaisesRegex(SystemExit,'manifest SHA'):cli.flash(args)
            run.assert_not_called()
        (self.root/'app.bin').write_bytes(b'changed');self.reject()
    def test_invalid_offsets(self):
        for offset in [True,-1,'0x10000',0,1024*1024]:
            with self.subTest(offset=offset):self.manifest['segments'][2]['offset']=offset;self.reject()
    def test_duplicates_wrong_kind_and_overlap(self):
        self.manifest['segments'].append(dict(self.manifest['segments'][2]));self.reject()
        self.manifest['segments'].pop();self.manifest['segments'][2]['kind']='unknown';self.reject()
        self.manifest['segments'][2]['kind']='application';self.manifest['segments'][2]['offset']=0x8000;self.reject()
    def test_bad_layout_and_magic(self):
        for change in ['missing','md5','magic','appoffset','capacity']:
            with self.subTest(change=change):
                old=json.loads(json.dumps(self.manifest))
                if change=='missing':self.manifest.pop('partition_layout')
                if change=='md5':
                    data=bytearray((self.root/'table.bin').read_bytes());data[48]^=1
                    self.manifest['partition_layout']=dict(self.record('bad-table.bin',data),table_offset=0x8000,app_label='factory')
                if change=='magic':self.manifest['segments'][2]=dict(self.record('bad-app.bin',b'BAD'),kind='application',offset=0x10000)
                if change=='appoffset':self.manifest['segments'][2]['offset']=0x20000
                if change=='capacity':self.manifest['segments'][2]=dict(self.record('large-app.bin',b'\xe9'+b'A'*0x10000),kind='application',offset=0x10000)
                self.reject();self.manifest=old
    def test_cli_parser_requires_manifest(self):
        with patch('sys.argv',['beefiscal-factory','flash','--port','MOCK','--firmware','app.bin','--expected-sha256','a'*64]),patch.object(cli.subprocess,'run') as run:
            with self.assertRaises(SystemExit):cli.main()
            run.assert_not_called()

    def test_failed_subprocess_cleans_verified_staging(self):
        paths=[]
        def failed(command,check):
            paths.extend(command[7::2])
            raise RuntimeError("mock esptool failure")
        with patch.object(cli.subprocess,'run',side_effect=failed):
            with self.assertRaises(RuntimeError):cli.flash(self.arguments())
        self.assertTrue(paths)
        self.assertTrue(all(not Path(p).exists() for p in paths))

    def test_all_partition_entries_respect_vendor_layout_invariants(self):
        def entry(kind=0, subtype=0, offset=0x10000, size=0x10000, label=b'factory', flags=0):
            return struct.pack('<HBBII16sI', 0x50AA, kind, subtype, offset, size, label, flags)
        variants = {
            'app-in-reserved-sector': entry(offset=0x8100,size=0x1000),
            'app-offset-unaligned': entry(offset=0x11000),
            'app-size-not-erase-aligned': entry(size=1),
            'data-in-reserved-sector': entry()+entry(kind=1,subtype=2,offset=0x8800,size=0x1000,label=b'nvs'),
            'data-offset-unaligned': entry()+entry(kind=1,subtype=2,offset=0x21001,size=0x1000,label=b'nvs'),
            'unknown-type': entry()+entry(kind=4,offset=0x30000,label=b'unknown'),
            'unknown-app-subtype': entry(subtype=0x30),
            'unknown-data-subtype': entry()+entry(kind=1,subtype=0x70,offset=0x30000,label=b'unknown'),
            'unknown-flags': entry(flags=4),
            'duplicate-app-label': entry()+entry(offset=0x30000),
            'duplicate-other-label': entry()+entry(kind=1,subtype=2,offset=0x30000,label=b'nvs')+entry(kind=1,subtype=2,offset=0x50000,label=b'nvs'),
            'empty-label': entry()+entry(kind=1,subtype=2,offset=0x30000,label=b''),
            'overlap-other-partition': entry()+entry(kind=1,subtype=2,offset=0x11000,size=0x1000,label=b'nvs'),
            'readonly-otadata': entry()+entry(kind=1,subtype=0,offset=0x30000,size=0x2000,label=b'otadata',flags=2),
            'wrong-size-otadata': entry()+entry(kind=1,subtype=0,offset=0x30000,size=0x1000,label=b'otadata'),
            'duplicate-otadata': entry()+entry(kind=1,subtype=0,offset=0x30000,size=0x2000,label=b'ota1')+entry(kind=1,subtype=0,offset=0x40000,size=0x2000,label=b'ota2'),
            'outside-flash': entry()+entry(kind=1,subtype=2,offset=0x100000,size=0x1000,label=b'nvs'),
        }
        for name, entries in variants.items():
            with self.subTest(name=name):
                table=entries+b'\xeb\xeb'+b'\xff'*14+hashlib.md5(entries).digest()+b'\xff'*32
                record=self.record('invalid-table.bin',table)
                self.manifest['partition_layout']=dict(record,table_offset=0x8000,app_label='factory')
                self.manifest['segments'][1]=dict(record,kind='partition_table',offset=0x8000)
                self.reject()
