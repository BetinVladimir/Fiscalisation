#!/usr/bin/env python3
"""Offline BlueCash journal extraction; acquired checkpointed copy only, no keys."""
import argparse
import json
from pathlib import Path
import re
import sqlite3
import os
import export_s3_sqlite as common

ROOT = Path(__file__).resolve().parent.parent
SCHEMA_RELATIVE = 'SmartDevices/bluecash-app/src/main/kotlin/com/beeloy/fiscal/bluecash/AndroidJournal.kt'
FORMAT = 'beefiscal-bluecash-sqlite-extract-v1'
PURPOSES = {
 'journal_event': 'Retained signed journal events; operation correlation, exact payload, hash chain and ACK metadata. Signatures are exported, not verified.',
 'sync_checkpoint': 'Durable acknowledged journal prefix. Earlier acknowledged history may have been pruned.',
 'payment_transaction': 'Local card transaction amount in minor units, state, RRN and authorization reference.',
 'ble_local_sale': 'Local BLE sale surrogate and exact serialized payload.',
 'ble_unp_range': 'Local UNP lease prefix and next/end sequence.',
 'sqlite_sequence': 'SQLite AUTOINCREMENT high-water mark.',
 'android_metadata': 'Android SQLite platform locale metadata; not application business data.',
}
PROFILE = {'table_purposes': PURPOSES, 'format': FORMAT, 'scope': 'BlueCash Android local durable journal; acquired offline copy only',
 'user_version': 3, 'entrypoint': 'scripts/export_bluecash_sqlite.py',
 'not_included': ['AndroidKeyStore keys', 'SharedPreferences/bootstrap credentials', 'S3 NVS and SQLite',
                  'Fiscal and MiniPOS PostgreSQL stores', 'Web IndexedDB/native secure stores',
                  'Cryptographic signature validation or authenticated acquisition provenance']}


def ddl(source):
    text = Path(source).read_text()
    if not re.search(r'SQLiteOpenHelper\(context, "beefiscal-bluecash-journal.db", null, 3\)', text):
        raise ValueError('BlueCash database identity/version changed; explicit profile update required')
    statements = []
    for match in re.finditer(r'db\.execSQL\(\s*("(?:[^"\\]|\\.)*")\s*\)', text):
        sql = json.loads(match.group(1))
        if sql.startswith(('CREATE TABLE ', 'CREATE INDEX ')):
            statements.append(sql)
    names = set(re.findall(r'CREATE TABLE (?:IF NOT EXISTS )?(\w+)', '\n'.join(statements)))
    if names != set(PURPOSES) - {'sqlite_sequence', 'android_metadata'}:
        raise ValueError('Unreviewed BlueCash table inventory')
    return statements


def expected_schema(source):
    statements = ddl(source)
    db = sqlite3.connect(':memory:')
    try:
        for sql in statements:
            db.execute(sql)
        # SQLiteConnection.setLocaleFromConfiguration creates this on Android.
        db.execute('CREATE TABLE android_metadata (locale TEXT)')
        result = common.schema(db)
    finally:
        db.close()
    # Preserve common.schema structural comparison; purposes are documentation metadata.
    for table in result['tables']:
        table['purpose'] = PURPOSES[table['name']]
    result['source'] = {'path': SCHEMA_RELATIVE, 'sha256': common.sha(Path(source).read_bytes()),
        'schema_sql_sha256': common.sha('\n'.join(statements).encode()), 'migration_sql': [],
        'store_label': 'BlueCash Android SQLite',
        'android_metadata_reference': 'https://android.googlesource.com/platform/frameworks/base/+/master/core/java/android/database/sqlite/SQLiteConnection.java'}
    return result


def extract(database, output, source, device_id, source_commit, acquisition_note, quiescent_copy=False):
    return common.extract(database, output, source, device_id, source_commit, acquisition_note,
                          quiescent_copy, schema_provider=expected_schema, storage_profile={**PROFILE,
                              'tool_metadata': {'entrypoint_sha256': common.sha(Path(__file__).read_bytes())}})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--schema-source', type=Path, default=ROOT / SCHEMA_RELATIVE)
    sub = parser.add_subparsers(dest='command', required=True)
    dictionary = sub.add_parser('dictionary')
    dictionary.add_argument('--output', type=Path, required=True)
    verify = sub.add_parser('verify')
    verify.add_argument('--directory', type=Path, required=True)
    export = sub.add_parser('export')
    for name in ('database', 'output'):
        export.add_argument('--' + name, type=Path, required=True)
    for name in ('device-id', 'source-commit', 'acquisition-note'):
        export.add_argument('--' + name, required=True)
    export.add_argument('--quiescent-copy', action='store_true')
    args = parser.parse_args()
    try:
        if args.command == 'verify':
            result = common.verify(args.directory, allowed_formats=(FORMAT,))
        elif args.command == 'dictionary':
            value = expected_schema(args.schema_source)
            with args.output.open('xb') as handle:
                os.chmod(args.output, 0o600)
                handle.write(common.markdown(value))
            result = {'status': 'DICTIONARY_GENERATED'}
        else:
            result = extract(args.database, args.output, args.schema_source, args.device_id,
                             args.source_commit, args.acquisition_note, args.quiescent_copy)
        print(json.dumps(result))
        return 2 if result.get('foreign_key_issues') else 0
    except (ValueError, OSError, sqlite3.Error, KeyError, TypeError) as error:
        parser.exit(1, 'Extraction refused: ' + str(error) + '\n')


if __name__ == '__main__':
    raise SystemExit(main())
