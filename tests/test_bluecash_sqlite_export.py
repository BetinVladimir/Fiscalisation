import json
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / 'scripts'))
import export_bluecash_sqlite as exporter
import export_s3_sqlite as common
SOURCE = ROOT / exporter.SCHEMA_RELATIVE


def create(path):
    db = sqlite3.connect(path)
    for sql in exporter.ddl(SOURCE):
        db.execute(sql)
    db.execute('CREATE TABLE android_metadata (locale TEXT)')
    db.execute("INSERT INTO android_metadata VALUES('bg_BG')")
    db.execute('PRAGMA user_version=3')
    db.execute('INSERT INTO sync_checkpoint VALUES(1,7,?,?,?)', ('a'*64, 'ack-retained', '2026-10-18T01:00:00Z'))
    db.execute('INSERT INTO journal_event VALUES(8,?,?,?,?,?,?,?,?,?,?,?)',
               ('op-8', 'op', 'PAYMENT_UNCERTAIN', '2026-10-18T02:00:00Z', '{"exact":"Български\\u0000"}',
                'a'*64, 'b'*64, 'kid', 'signature', None, None))
    db.execute('INSERT INTO payment_transaction VALUES(?,?,?,?,?,?)', ('op', 9007199254740993, 'rrn', 'auth', 'UNKNOWN', 'now'))
    db.execute('INSERT INTO ble_local_sale VALUES(?,?,?)', ('surrogate', '{"server_sale_id":"sale"}', 'now'))
    db.execute('INSERT INTO ble_unp_range VALUES(1,?,?,?)', ('DT000001', 9, 100))
    db.commit()
    return db


class BlueCashExportTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.db = self.root / 'copy.db'
        create(self.db).close()
        self.out = self.root / 'export'

    def tearDown(self):
        self.tmp.cleanup()

    def export(self):
        return exporter.extract(self.db, self.out, SOURCE, 'declared-bluecash', 'a'*40,
                                'Disposable SQLite fixture, not real device proof', True)

    def test_real_sqlite_all_tables_exact_replay_and_source_unchanged(self):
        before = self.db.read_bytes()
        manifest = self.export()
        self.assertEqual(before, self.db.read_bytes())
        self.assertEqual(manifest['format'], exporter.FORMAT)
        self.assertEqual(manifest['sqlite_user_version'], 3)
        self.assertEqual(len(manifest['tables']), 7)
        source = sqlite3.connect(self.db)
        replay = sqlite3.connect(':memory:')
        for sql in exporter.ddl(SOURCE): replay.execute(sql)
        replay.execute('CREATE TABLE android_metadata (locale TEXT)')
        for name, entry in manifest['tables'].items():
            rows = [[common.decode_typed(cell) for cell in json.loads(line)] for line in
                    (self.out / entry['file']).read_text().splitlines()]
            # sqlite_sequence is engine owned; compare after explicit high-water replay.
            replay.execute('DELETE FROM ' + common.ident(name))
            if rows:
                replay.executemany('INSERT INTO ' + common.ident(name) + ' VALUES(' + ','.join('?' for _ in rows[0]) + ')', rows)
            self.assertEqual(source.execute('SELECT * FROM ' + common.ident(name)).fetchall(),
                             replay.execute('SELECT * FROM ' + common.ident(name)).fetchall())
        source.close(); replay.close()
        self.assertEqual(common.verify(self.out, allowed_formats=(exporter.FORMAT,))['tables'], 7)
        with self.assertRaises(ValueError): common.verify(self.out)

    def test_wal_snapshot_backup_keeps_latest_committed_transaction(self):
        live = self.root / 'live.db'
        writer = create(live)
        writer.execute('PRAGMA journal_mode=WAL')
        writer.execute('PRAGMA wal_autocheckpoint=0')
        writer.execute("UPDATE payment_transaction SET rrn='latest-in-wal'")
        writer.commit()
        with self.assertRaises(ValueError):
            exporter.extract(live, self.out, SOURCE, 'test', 'a'*40, 'fixture', True)
        snapshot = self.root / 'snapshot.db'
        target = sqlite3.connect(snapshot)
        writer.backup(target)  # actual SQLite online-backup acquisition, not main-file copy.
        target.close(); writer.close()
        self.db = snapshot
        manifest = self.export()
        entry = manifest['tables']['payment_transaction']
        row = json.loads((self.out / entry['file']).read_text())
        self.assertEqual(common.decode_typed(row[2]), 'latest-in-wal')

    def test_version_and_schema_drift_fail_closed(self):
        db = sqlite3.connect(self.db)
        db.execute('PRAGMA user_version=2'); db.close()
        with self.assertRaises(ValueError): self.export()
        db = sqlite3.connect(self.db)
        db.execute('PRAGMA user_version=3'); db.execute('CREATE TABLE unexpected(secret TEXT)'); db.close()
        with self.assertRaises(ValueError): self.export()
        self.assertFalse(self.out.exists())

    def test_artifact_tamper_and_secret_siblings(self):
        (self.root / 'bootstrap.xml').write_text('DO_NOT_EXPORT_SECRET')
        manifest = self.export()
        self.assertFalse(any(b'DO_NOT_EXPORT_SECRET' in p.read_bytes() for p in self.out.iterdir()))
        path = self.out / manifest['tables']['journal_event']['file']
        path.write_text('[]\n')
        with self.assertRaises(ValueError): common.verify(self.out, allowed_formats=(exporter.FORMAT,))

    def test_cli_and_generated_dictionary(self):
        result = subprocess.run([sys.executable, str(ROOT/'scripts/export_bluecash_sqlite.py'), 'export',
            '--database', str(self.db), '--output', str(self.out), '--device-id', 'fixture',
            '--source-commit', 'a'*40, '--acquisition-note', 'unit fixture', '--quiescent-copy'], capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        expected = common.markdown(exporter.expected_schema(SOURCE))
        self.assertEqual(expected, (ROOT/'docs/remediation/2026-10-17/20_BLUECASH_SQLITE_DICTIONARY.generated.md').read_bytes())


if __name__ == '__main__': unittest.main()
