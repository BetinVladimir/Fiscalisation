import hashlib
import importlib.util
import json
from pathlib import Path
import re
import sqlite3
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location('s3_export', ROOT / 'scripts/export_s3_sqlite.py')
exporter = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(exporter)
SOURCE = ROOT / exporter.SCHEMA_RELATIVE


def make_database(path):
    # Use the actual production DDL, not a test-maintained imitation.
    sql = re.search(r'R"sql\((.*?)\)sql"', SOURCE.read_text(), re.S).group(1)
    db = sqlite3.connect(path)
    db.executescript(sql)
    db.execute("INSERT INTO operations VALUES(?,?,?,?,?,?,?,?)", ('op-1', 'a'*64, 'BLE', '{"tenant":"tenant-a"}', 'UNKNOWN', None, 1, 2))
    db.execute("INSERT INTO operations VALUES(?,?,?,?,?,?,?,?)", ('op-2', 'b'*64, 'MQTT', '{"tenant":"tenant-historical"}', 'COMPLETED', 'Български\n\u0000 exact text', 3, 4))
    db.execute("INSERT INTO receipts VALUES(?,?,?,?,?,?)", ('receipt-1', 'op-1', 'UNKNOWN', '{"amount":"2.00"}', 1, 2))
    db.execute("INSERT INTO payments VALUES(?,?,?,?,?,?,?,?,?,?)", ('pay-1', 'op-1', 'receipt-1', 'APPROVED', 9007199254740993, None, b'\x00\xff\x80', 'Код', 12.25, 2))
    db.execute("INSERT INTO terminal_payment_guard VALUES(?,?,?)", ('terminal-1', 'op-1', 2))
    db.execute("INSERT INTO payment_reconciliations VALUES(?,?,?,?,?,?,?,?,?,?,?)", ('reconcile-1','terminal-1','pay-1','op-1','admin','case-1','a'*64,'b'*64,'UNKNOWN','APPROVED',2))
    db.execute("INSERT INTO payment_saga VALUES(?,?,?)", ('op-1', 'PAYMENT_UNCERTAIN', 2))
    db.execute("INSERT INTO payment_saga_members VALUES(?,?,?,?,?)", ('op-1', 'pay-1', 'rev-1', None, 'r-auth'))
    db.execute("INSERT INTO event_outbox(sequence,event_id,operation_id,kind,payload,payload_digest,created_at) VALUES(?,?,?,?,?,?,?)", (9007199254740994, 'event-1', 'op-1', 'operation_received', '{"exact":"payload"}', 'c'*64, 1))
    db.execute("UPDATE sync_cursor SET acknowledged_sequence=7,ack_id='ack-1',committed_event_hash=?,updated_at=2", ('d'*64,))
    db.execute("INSERT INTO frozen_sync_batch VALUES(1,?,?,?,?,?,?)", ('batch-1', '{"exact":"batch-body"}', 8, 8, 'e'*64, 'd'*64))
    db.execute("INSERT INTO frozen_sync_members VALUES(?,?,?,?,?,?,?)", (8, 'event-frozen', 'op-1', 'kind', '{}', 'f'*64, 2))
    db.execute("INSERT INTO local_sales VALUES(?,?,?,?,?,?)", ('surrogate-1', 9223372036854775807, 'UNKNOWN', 'локална продажба', 1, 2))
    db.execute("INSERT INTO authority_state VALUES(1,?,?,?,?,?,?)", ('DT000001', 1, 1000, 2, 3, 7))
    db.execute("INSERT INTO authority_ranges VALUES(?,?,?,?)", (7, 'DT000001', 1, 1000))
    db.execute("INSERT INTO receipt_authority VALUES(?,?,?,?,?,?,?,?,?)", ('receipt-1', 'DT000001-0001-0000001', 'DT000001-0001-0000001', 1, 1, 'DT000001', 1, 1000, 7))
    db.execute("INSERT INTO sale_unp_claims VALUES(?,?,?,?)", ('DT000001-0001-0000001', 'op-1', 'DT000001', 1))
    db.execute("UPDATE operations SET result_payload=CAST(x'80ff' AS TEXT) WHERE operation_id='op-1'")
    db.commit()
    db.close()


class ExportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.db = self.root / 'captured.sqlite'
        self.output = self.root / 'extract'
        make_database(self.db)

    def tearDown(self):
        self.temp.cleanup()

    def run_export(self, **kwargs):
        return exporter.extract(self.db, self.output, SOURCE, 'adapter-declared', 'a'*40,
                                'Disposable unit fixture; not physical evidence', True, **kwargs)

    def test_actual_schema_all_rows_typed_roundtrip_and_unchanged_source(self):
        before = self.db.read_bytes()
        self.db.chmod(0o444)
        # Sibling stores/secrets must not be traversed/copied.
        (self.root / 'nvs.bin').write_text('PRIVATE-NVS-KEY-DO-NOT-EXTRACT')
        make_database(self.root / 'another-device.sqlite')
        result = self.run_export()
        self.assertEqual(self.db.read_bytes(), before)
        self.assertEqual(result['input']['sha256'], hashlib.sha256(before).hexdigest())
        self.assertEqual(set(result['tables']), set(exporter.PURPOSES))
        self.assertTrue(all(table['rows'] > 0 for table in result['tables'].values()))
        self.assertFalse(result['acquisition']['provenance_authenticated'])
        self.assertEqual(exporter.verify(self.output)['status'], 'ARCHIVE_INTEGRITY_VERIFIED')
        source_db = sqlite3.connect(self.db.as_uri() + '?mode=ro&immutable=1', uri=True)
        source_db.text_factory = exporter.sqlite_text
        restored = sqlite3.connect(':memory:')
        restored.text_factory = exporter.sqlite_text
        raw = re.search(r'R"sql\((.*?)\)sql"', SOURCE.read_text(), re.S).group(1)
        restored.executescript(raw)
        for name in result['tables']:
            restored.execute('DELETE FROM ' + exporter.ident(name))
        for name, table in result['tables'].items():
            rows = [[exporter.decode_typed(cell) for cell in json.loads(line)]
                    for line in (self.output / table['file']).read_text().splitlines()]
            columns = ','.join(exporter.ident(col) for col in table['columns'])
            expected = source_db.execute('SELECT ' + columns + ' FROM ' + exporter.ident(name) + ' ORDER BY ' + columns).fetchall()
            self.assertEqual([tuple(row) for row in rows], expected, name)
            if name == 'sqlite_sequence':
                restored.execute('DELETE FROM sqlite_sequence')
            for row in rows:
                placeholders = ','.join('CAST(? AS TEXT)' if isinstance(value, exporter.SQLiteTextBytes) else '?' for value in row)
                restored.execute('INSERT INTO ' + exporter.ident(name) + ' VALUES(' + placeholders + ')', row)
            actual = restored.execute('SELECT ' + columns + ' FROM ' + exporter.ident(name) + ' ORDER BY ' + columns).fetchall()
            self.assertEqual(actual, expected, name)
        self.assertEqual(restored.execute('PRAGMA foreign_key_check').fetchall(), [])
        source_db.close()
        restored.close()
        for path in self.output.iterdir():
            self.assertNotIn(b'PRIVATE-NVS-KEY', path.read_bytes())
        self.assertFalse(exporter.sidecars(self.db))

    def test_refuses_live_sidecars_without_deleting_them(self):
        for suffix in ['-wal', '-shm', '-journal']:
            with self.subTest(suffix=suffix):
                side = Path(str(self.db) + suffix)
                side.write_bytes(b'preserve-evidence')
                with self.assertRaisesRegex(ValueError, 'sidecars'):
                    self.run_export()
                self.assertEqual(side.read_bytes(), b'preserve-evidence')
                self.assertFalse(self.output.exists())
                side.unlink()

    def test_requires_quiescent_operator_declaration(self):
        with self.assertRaisesRegex(ValueError, 'quiescent'):
            exporter.extract(self.db, self.output, SOURCE, 'adapter', 'a'*40, 'note')
        self.assertFalse(self.output.exists())

    def test_schema_drift_unknown_table_not_silently_omitted(self):
        db = sqlite3.connect(self.db)
        db.execute('CREATE TABLE unseen_private_store(secret TEXT)')
        db.commit()
        db.close()
        with self.assertRaisesRegex(ValueError, 'schema differs'):
            self.run_export()
        self.assertFalse(self.output.exists())
        self.assertFalse(list(self.root.glob('.s3-extract-*')))

    def test_preserves_fk_issues_as_data_not_false_clean_export(self):
        db = sqlite3.connect(self.db)
        db.execute("UPDATE payments SET operation_id='missing-operation'")
        db.commit()
        db.close()
        result = self.run_export()
        self.assertEqual(result['status'], 'EXPORTED_WITH_FOREIGN_KEY_ISSUES')
        self.assertTrue(result['foreign_key_issues'])
        self.assertEqual(exporter.verify(self.output)['source_status'], result['status'])

    def test_no_clobber_existing_output_or_symlink_input(self):
        self.output.mkdir()
        sentinel = self.output / 'keep'
        sentinel.write_text('existing-evidence')
        with self.assertRaisesRegex(ValueError, 'exist'):
            self.run_export()
        self.assertEqual(sentinel.read_text(), 'existing-evidence')
        link = self.root / 'linked.sqlite'
        link.symlink_to(self.db)
        with self.assertRaisesRegex(ValueError, 'symlink'):
            exporter.extract(link, self.root/'new', SOURCE, 'adapter', 'a'*40, 'note', True)

    def test_tampered_omitted_or_foreign_artifact_rejected(self):
        self.run_export()
        table = next(self.output.glob('table-*.jsonl'))
        saved = table.read_bytes()
        table.write_bytes(saved + b'[]\n')
        with self.assertRaisesRegex(ValueError, 'hash/size'):
            exporter.verify(self.output)
        table.write_bytes(saved)
        table.unlink()
        with self.assertRaisesRegex(ValueError, 'omitted'):
            exporter.verify(self.output)
        table.write_bytes(saved)
        extra = self.output / 'foreign.txt'
        extra.write_text('foreign')
        with self.assertRaisesRegex(ValueError, 'Unexpected'):
            exporter.verify(self.output)
        extra.unlink()
        table.unlink()
        table.symlink_to(self.db)
        with self.assertRaisesRegex(ValueError, 'Unsafe'):
            exporter.verify(self.output)

    def test_actual_source_mutation_during_export_refuses_completion(self):
        real_hash = exporter.file_hash
        calls = 0
        def concurrent_writer(path):
            nonlocal calls
            digest = real_hash(path)
            calls += 1
            if calls == 1:
                db = sqlite3.connect(self.db)
                db.execute("UPDATE operations SET state='CHANGED' WHERE operation_id='op-1'")
                db.commit()
                db.close()
            return digest
        with mock.patch.object(exporter, 'file_hash', side_effect=concurrent_writer):
            with self.assertRaisesRegex(ValueError, 'Source changed'):
                self.run_export()
        self.assertFalse(self.output.exists())
        self.assertFalse(list(self.root.glob('.s3-extract-*')))

    def test_corrupt_database_fails_without_partial_archive(self):
        self.db.write_bytes(b'not a SQLite database')
        with self.assertRaises(sqlite3.DatabaseError):
            self.run_export()
        self.assertFalse(self.output.exists())
        self.assertFalse(list(self.root.glob('.s3-extract-*')))

    def test_committed_dictionary_exactly_matches_actual_firmware_source(self):
        committed = ROOT/'docs/remediation/2026-10-17/18_S3_SQLITE_DICTIONARY.generated.md'
        self.assertEqual(committed.read_bytes(), exporter.markdown(exporter.expected_schema(SOURCE)))

    def test_cli_complete_export_verify_and_generated_dictionary(self):
        command = [sys.executable, str(ROOT/'scripts/export_s3_sqlite.py')]
        p = subprocess.run(command + ['export', '--database', str(self.db), '--output', str(self.output), '--device-id', 'fixture', '--source-commit', 'b'*40, '--acquisition-note', 'Test fixture only', '--quiescent-copy'], capture_output=True, text=True)
        self.assertEqual(p.returncode, 0, p.stderr)
        verified = subprocess.run(command + ['verify', '--directory', str(self.output)], capture_output=True, text=True)
        self.assertEqual(verified.returncode, 0, verified.stderr)
        self.assertFalse(json.loads(verified.stdout)['provenance_authenticated'])
        doc = self.root/'dictionary.md'
        generated = subprocess.run(command + ['dictionary', '--output', str(doc)], capture_output=True, text=True)
        self.assertEqual(generated.returncode, 0, generated.stderr)
        for name in exporter.PURPOSES:
            self.assertIn('## '+name, doc.read_text())


if __name__ == '__main__':
    unittest.main()
